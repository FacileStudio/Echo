package history

import (
	"context"
	"errors"

	"github.com/FacileStudio/Echo/apps/api/internal/summarize"
	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const listLimit = 200

// Service serves per-room call history, transcripts and summaries. Owner
// gated like recording: a call is the room owner's business record.
type Service struct {
	orm           *gorm.DB
	rooms         *rooms.Service
	summarizer    *summarize.Summarizer
	recordingsDir string
}

// NewService builds a Service. summarizer may be nil, which disables AI
// summaries on this deployment; recordingsDir may be empty, which disables
// recording downloads on this node.
func NewService(
	orm *gorm.DB,
	roomsService *rooms.Service,
	summarizer *summarize.Summarizer,
	recordingsDir string,
) *Service {
	return &Service{orm: orm, rooms: roomsService, summarizer: summarizer, recordingsDir: recordingsDir}
}

// List returns the room's calls, newest first, for its owner.
func (s *Service) List(ctx context.Context, slug string, callerID int64) ([]callResponse, error) {
	if err := s.rooms.RequireOwner(ctx, slug, callerID); err != nil {
		return nil, err
	}
	var room schemas.Room
	if err := s.orm.WithContext(ctx).Where("slug = ?", slug).First(&room).Error; err != nil {
		return nil, err
	}
	var calls []schemas.Call
	err := s.orm.WithContext(ctx).
		Where("room_id = ?", room.ID).
		Order("started_at DESC").
		Limit(listLimit).
		Find(&calls).Error
	if err != nil {
		return nil, err
	}
	out := make([]callResponse, 0, len(calls))
	for _, call := range calls {
		out = append(out, toResponse(call))
	}
	return out, nil
}

// ownedCall loads the call after checking its room belongs to callerID.
func (s *Service) ownedCall(ctx context.Context, callID string, callerID int64) (*schemas.Call, error) {
	id, err := uuid.Parse(callID)
	if err != nil {
		return nil, troncerrors.NotFound("call not found")
	}
	var call schemas.Call
	if err := s.orm.WithContext(ctx).First(&call, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, troncerrors.NotFound("call not found")
		}
		return nil, err
	}
	var room schemas.Room
	if err := s.orm.WithContext(ctx).First(&room, "id = ?", call.RoomID).Error; err != nil {
		return nil, err
	}
	if room.OwnerID == nil || *room.OwnerID != callerID {
		return nil, troncerrors.Forbidden("only the room owner can do this")
	}
	return &call, nil
}

// Detail returns one owned call with its participants, transcript and summary.
func (s *Service) Detail(ctx context.Context, callID string, callerID int64) (*callDetail, error) {
	call, err := s.ownedCall(ctx, callID, callerID)
	if err != nil {
		return nil, err
	}
	detail := &callDetail{callResponse: toResponse(*call)}

	var participants []schemas.CallParticipant
	err = s.orm.WithContext(ctx).Where("call_id = ?", call.ID).Order("joined_at").Find(&participants).Error
	if err != nil {
		return nil, err
	}
	detail.Participants = toParticipants(participants)

	transcript, err := s.transcriptOf(ctx, call.ID)
	if err != nil {
		return nil, err
	}
	if transcript != nil {
		detail.Transcript = transcript.Content
	}

	summary, err := s.summaryOf(ctx, call.ID)
	if err != nil {
		return nil, err
	}
	if summary != nil {
		payload := toSummary(*summary)
		detail.Summary = &payload
	}
	return detail, nil
}

// Summarize generates (or regenerates) the AI summary of an owned call.
func (s *Service) Summarize(ctx context.Context, callID string, callerID int64) (*summaryPayload, error) {
	call, err := s.ownedCall(ctx, callID, callerID)
	if err != nil {
		return nil, err
	}
	if s.summarizer == nil {
		return nil, troncerrors.Unavailable("AI summaries are not configured on this deployment")
	}
	transcript, err := s.transcriptOf(ctx, call.ID)
	if err != nil {
		return nil, err
	}
	if transcript == nil {
		return nil, troncerrors.Conflict("no transcript recorded for this call")
	}
	content, err := s.summarizer.Summarize(ctx, transcript.Content)
	if err != nil {
		return nil, err
	}
	return s.storeSummary(ctx, call.ID, content)
}

func (s *Service) storeSummary(ctx context.Context, callID uuid.UUID, content string) (*summaryPayload, error) {
	existing, err := s.summaryOf(ctx, callID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		existing.Content = content
		existing.Model = s.summarizer.Model()
		if err := s.orm.WithContext(ctx).Save(existing).Error; err != nil {
			return nil, err
		}
		payload := toSummary(*existing)
		return &payload, nil
	}
	row := schemas.Summary{
		ID:      uuid.New(),
		CallID:  callID,
		Content: content,
		Model:   s.summarizer.Model(),
	}
	if err := s.orm.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	payload := toSummary(row)
	return &payload, nil
}

func (s *Service) transcriptOf(ctx context.Context, callID uuid.UUID) (*schemas.Transcript, error) {
	var transcript schemas.Transcript
	err := s.orm.WithContext(ctx).Where("call_id = ?", callID).First(&transcript).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &transcript, nil
}

func (s *Service) summaryOf(ctx context.Context, callID uuid.UUID) (*schemas.Summary, error) {
	var summary schemas.Summary
	err := s.orm.WithContext(ctx).Where("call_id = ?", callID).First(&summary).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &summary, nil
}
