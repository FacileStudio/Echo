package capture

import (
	"context"
	"errors"

	"github.com/FacileStudio/Echo/apps/api/internal/seat"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Answer is one participant's reply to the organizer's request. Nil means the
// question was not answered, which counts as a refusal.
type Answer struct {
	Transcribe *bool `json:"transcribe"`
	Record     *bool `json:"record"`
}

// Person is one seated participant and what they accepted. Only the organizer
// receives the list, to see who is left out of the recording.
type Person struct {
	Identity string `json:"identity"`
	Name     string `json:"name"`
	Answer
}

// State is what every participant polls: what the organizer turned on and the
// caller's own answer. The organizer also gets the people.
type State struct {
	Transcription bool     `json:"transcription"`
	Recording     bool     `json:"recording"`
	Owner         bool     `json:"owner"`
	You           Answer   `json:"you"`
	People        []Person `json:"people,omitempty"`
}

// Flags are the organizer's switches; a nil field leaves its switch alone.
type Flags struct {
	Transcription *bool `json:"transcription"`
	Recording     *bool `json:"recording"`
}

var (
	errNotOwner      = troncerrors.Forbidden("only the room owner can do this")
	errIntegration   = troncerrors.Forbidden("transcription and recording are not available in this room")
	errNotRequested  = troncerrors.Conflict("the organizer has not asked for this")
	errNoStorage     = troncerrors.Conflict("recording storage is not configured on this node")
	errRecordingOff  = troncerrors.Conflict("recording is not on for this call")
	errUploadMissing = troncerrors.NotFound("recording not found")
)

func ownerIdentity(room schemas.Room) string {
	if room.OwnerID == nil {
		return ""
	}
	return identityOf(*room.OwnerID)
}

func (s *Service) room(ctx context.Context, slug string) (schemas.Room, error) {
	room, err := s.rooms.BySlug(ctx, slug)
	if err != nil {
		return schemas.Room{}, err
	}
	if room == nil {
		return schemas.Room{}, troncerrors.NotFound("room not found")
	}
	if room.IntegrationApp != "" {
		return schemas.Room{}, errIntegration
	}
	return *room, nil
}

// State reads the capture state of the room's open call for the seated caller.
func (s *Service) State(ctx context.Context, slug, identity string) (State, error) {
	room, err := s.room(ctx, slug)
	if err != nil {
		return State{}, err
	}
	var out State
	err = s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		call, err := seat.OpenCall(tx, slug, false)
		if err != nil {
			return err
		}
		if err := seat.Seated(tx, call.ID, identity); err != nil {
			return err
		}
		out = State{Transcription: call.TranscribeOn, Recording: call.RecordOn, Owner: identity == ownerIdentity(room)}
		var mine schemas.CaptureConsent
		if err := tx.Where("call_id = ? AND identity = ?", call.ID, identity).First(&mine).Error; err == nil {
			out.You = Answer{Transcribe: mine.Transcribe, Record: mine.Record}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if !out.Owner {
			return nil
		}
		out.People, err = people(tx, call.ID)
		return err
	})
	return out, err
}

func people(tx *gorm.DB, callID uuid.UUID) ([]Person, error) {
	var seated []schemas.CallParticipant
	if err := tx.Where("call_id = ? AND left_at IS NULL", callID).Order("joined_at, id").Find(&seated).Error; err != nil {
		return nil, err
	}
	var answers []schemas.CaptureConsent
	if err := tx.Where("call_id = ?", callID).Find(&answers).Error; err != nil {
		return nil, err
	}
	byIdentity := make(map[string]Answer, len(answers))
	for _, row := range answers {
		byIdentity[row.Identity] = Answer{Transcribe: row.Transcribe, Record: row.Record}
	}
	out := make([]Person, 0, len(seated))
	for _, who := range seated {
		out = append(out, Person{Identity: who.Identity, Name: who.Name, Answer: byIdentity[who.Identity]})
	}
	return out, nil
}

// SetFlags lets the owner switch transcription and recording for the open
// call. Turning one on needs the owner's plan (ai.access, echo.recording),
// checked here and not in the client, and forgets every earlier answer to that
// request: consent belongs to one run of the feature, so a person is asked
// again each time it is switched on. The owner's own answer is yes, since they
// asked. Turning recording off finalizes what was uploaded.
func (s *Service) SetFlags(ctx context.Context, slug string, ownerID int64, flags Flags) error {
	room, err := s.ownedRoom(ctx, slug, ownerID)
	if err != nil {
		return err
	}
	if err := s.requirePlan(ctx, room, flags); err != nil {
		return err
	}
	return s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		call, err := seat.OpenCall(tx, slug, true)
		if err != nil {
			return err
		}
		if err := s.apply(tx, &call, ownerIdentity(room), flags); err != nil {
			return err
		}
		return tx.Model(&schemas.Call{}).Where("id = ?", call.ID).
			Updates(map[string]any{"transcribe_on": call.TranscribeOn, "record_on": call.RecordOn}).Error
	})
}

func (s *Service) ownedRoom(ctx context.Context, slug string, ownerID int64) (schemas.Room, error) {
	room, err := s.room(ctx, slug)
	if err != nil {
		return room, err
	}
	if room.OwnerID == nil || *room.OwnerID != ownerID {
		return room, errNotOwner
	}
	return room, nil
}

func (s *Service) requirePlan(ctx context.Context, room schemas.Room, flags Flags) error {
	if flags.Transcription != nil && *flags.Transcription {
		if err := s.rooms.RequireAI(ctx, room); err != nil {
			return err
		}
	}
	if flags.Recording == nil || !*flags.Recording {
		return nil
	}
	if s.dir == "" {
		return errNoStorage
	}
	return s.rooms.RequireRecording(ctx, room.Slug, *room.OwnerID)
}

func (s *Service) apply(tx *gorm.DB, call *schemas.Call, owner string, flags Flags) error {
	if flags.Transcription != nil && *flags.Transcription != call.TranscribeOn {
		call.TranscribeOn = *flags.Transcription
		if err := resetAnswers(tx, call.ID, "transcribe", owner, call.TranscribeOn); err != nil {
			return err
		}
	}
	if flags.Recording != nil && *flags.Recording != call.RecordOn {
		call.RecordOn = *flags.Recording
		if err := resetAnswers(tx, call.ID, "record", owner, call.RecordOn); err != nil {
			return err
		}
		if !call.RecordOn {
			return finalizeCall(tx, s.dir, call.ID)
		}
	}
	return nil
}

func resetAnswers(tx *gorm.DB, callID uuid.UUID, column, owner string, on bool) error {
	if err := tx.Model(&schemas.CaptureConsent{}).Where("call_id = ?", callID).Update(column, nil).Error; err != nil {
		return err
	}
	if !on || owner == "" {
		return nil
	}
	return setAnswer(tx, callID, owner, column, true)
}

func setAnswer(tx *gorm.DB, callID uuid.UUID, identity, column string, value bool) error {
	row := map[string]any{"call_id": callID, "identity": identity, column: value}
	return tx.Model(&schemas.CaptureConsent{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "call_id"}, {Name: "identity"}},
		DoUpdates: clause.Assignments(map[string]any{column: value, "updated_at": gorm.Expr("now()")}),
	}).Create(row).Error
}

// SetAnswer records the seated caller's reply. Declining is always allowed.
// Accepting something the organizer has not switched on is a conflict, so a
// stale tab cannot pre-consent to a request that does not exist.
func (s *Service) SetAnswer(ctx context.Context, slug, identity string, answer Answer) error {
	if _, err := s.room(ctx, slug); err != nil {
		return err
	}
	return s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		call, err := seat.OpenCall(tx, slug, true)
		if err != nil {
			return err
		}
		if err := seat.Seated(tx, call.ID, identity); err != nil {
			return err
		}
		if err := reply(tx, call.ID, identity, "transcribe", answer.Transcribe, call.TranscribeOn); err != nil {
			return err
		}
		return reply(tx, call.ID, identity, "record", answer.Record, call.RecordOn)
	})
}

func reply(tx *gorm.DB, callID uuid.UUID, identity, column string, value *bool, requested bool) error {
	if value == nil {
		return nil
	}
	if *value && !requested {
		return errNotRequested
	}
	return setAnswer(tx, callID, identity, column, *value)
}
