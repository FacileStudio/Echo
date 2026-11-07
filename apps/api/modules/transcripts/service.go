package transcripts

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/seat"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Gate decides whether a room's plan allows transcripts. A nil Gate allows
// them, which is the deployment without Registre.
type Gate func(ctx context.Context, room schemas.Room) error

// Service persists final transcript lines for the open call of a room.
type Service struct {
	orm  *gorm.DB
	gate Gate
}

// NewService builds a Service over the given database. gate may be nil.
func NewService(orm *gorm.DB, gate Gate) *Service { return &Service{orm: orm, gate: gate} }

// admit refuses a line for a room whose owner's plan has no AI access.
func (s *Service) admit(ctx context.Context, tx *gorm.DB, call schemas.Call) error {
	if s.gate == nil {
		return nil
	}
	var room schemas.Room
	if err := tx.First(&room, "id = ?", call.RoomID).Error; err != nil {
		return err
	}
	return s.gate(ctx, room)
}

// Utterance is one final line a browser transcribed from its own microphone.
// StartMS and EndMS are on the sender's clock and only have to be sane: the
// stored offset comes from the server's clock, which a client cannot move.
type Utterance struct {
	Slug     string
	Identity string
	Text     string
	StartMS  int64
	EndMS    int64
}

var (
	errTranscriptionOff = troncerrors.Conflict("transcription is not on for this call")
	errNoConsent        = troncerrors.Forbidden("you have not accepted to be transcribed")
)

func (u Utterance) check() error {
	if strings.TrimSpace(u.Text) == "" {
		return troncerrors.Invalid("text is required")
	}
	if u.StartMS < 0 || u.EndMS < u.StartMS || u.EndMS-u.StartMS > maxUtteranceMS {
		return troncerrors.Invalid("start_ms and end_ms must be a forward span of at most two minutes")
	}
	return nil
}

// Append adds one final utterance to the room's open call transcript. The
// call must still be open (after room_finished the record is history), the
// owner must have switched transcription on and their plan must include AI,
// the speaker must hold an open seat, and the speaker must have accepted. The
// speaker is the identity the caller proved, never a name from the request.
func (s *Service) Append(ctx context.Context, in Utterance) error {
	if err := in.check(); err != nil {
		return err
	}
	return s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		call, err := seat.OpenCall(tx, in.Slug, true)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, tx, call, in.Identity); err != nil {
			return err
		}
		line, err := transcriptLine(labelOf(tx, call.ID, in.Identity), in.Text)
		if err != nil {
			return err
		}
		if err := appendLine(tx, call.ID, line); err != nil {
			return err
		}
		return tx.Create(&schemas.TranscriptLine{
			CallID: call.ID, Speaker: in.Identity, Text: in.Text, OffsetMS: offsetSince(call.StartedAt),
		}).Error
	})
}

func (s *Service) authorize(ctx context.Context, tx *gorm.DB, call schemas.Call, identity string) error {
	if err := s.admit(ctx, tx, call); err != nil {
		return err
	}
	if !call.TranscribeOn {
		return errTranscriptionOff
	}
	if err := seat.Seated(tx, call.ID, identity); err != nil {
		return err
	}
	return consented(tx, call.ID, identity)
}

func consented(tx *gorm.DB, callID uuid.UUID, identity string) error {
	var count int64
	err := tx.Model(&schemas.CaptureConsent{}).
		Where("call_id = ? AND identity = ? AND transcribe IS TRUE", callID, identity).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return errNoConsent
	}
	return nil
}

// labelOf is how the summariser's text blob names a speaker: the display name
// when it is a clean one, else the identity.
func labelOf(tx *gorm.DB, callID uuid.UUID, identity string) string {
	var who schemas.CallParticipant
	if err := tx.Where("call_id = ? AND identity = ?", callID, identity).First(&who).Error; err != nil {
		return identity
	}
	if checkField("speaker", who.Name, maxSpeakerBytes) != nil || strings.TrimSpace(who.Name) == "" {
		return identity
	}
	return who.Name
}

// offsetSince is how far into the call a line arrived, in milliseconds. A
// final utterance lands a moment after it was spoken, so the offset marks the
// end of the cue.
func offsetSince(startedAt time.Time) int64 {
	return max(0, time.Since(startedAt).Milliseconds())
}

// appendLine concatenates one line onto the call's transcript, or starts the
// transcript when this is the first line. It refuses outright past the
// ceiling: a silent truncation would leave a record that reads complete.
func appendLine(tx *gorm.DB, callID uuid.UUID, line string) error {
	var transcript schemas.Transcript
	err := tx.Where("call_id = ?", callID).Order("created_at, id").First(&transcript).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&schemas.Transcript{ID: uuid.New(), CallID: callID, Content: line}).Error
	}
	if err != nil {
		return err
	}
	if len(transcript.Content)+len(line) > maxTranscriptBytes {
		return troncerrors.Conflict("this call's transcript has reached its maximum size")
	}
	return tx.Model(&transcript).Update("content", gorm.Expr("content || ?", line)).Error
}
