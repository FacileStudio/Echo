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
	"gorm.io/gorm/clause"
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

// storeSummary writes the call's summary as a single upsert on call_id. A
// read-then-write would either duplicate the row or, once call_id is unique,
// lose the race to a constraint violation.
func (s *Service) storeSummary(ctx context.Context, callID uuid.UUID, content string) (*summaryPayload, error) {
	row := schemas.Summary{
		ID:      uuid.New(),
		CallID:  callID,
		Content: content,
		Model:   s.summarizer.Model(),
	}
	upsert := clause.OnConflict{
		Columns:   []clause.Column{{Name: "call_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"content", "model", "updated_at"}),
	}
	if err := s.orm.WithContext(ctx).Clauses(upsert).Create(&row).Error; err != nil {
		return nil, err
	}
	stored, err := s.summaryOf(ctx, callID)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, troncerrors.Internal("the summary vanished immediately after being written", nil)
	}
	payload := toSummary(*stored)
	return &payload, nil
}

// transcriptOf returns the call's newest transcript, or nil when it has none.
// The ordering is explicit because First() otherwise falls back to the
// primary key, and that is a v4 UUID — lexical noise, unrelated to recency.
func (s *Service) transcriptOf(ctx context.Context, callID uuid.UUID) (*schemas.Transcript, error) {
	var transcript schemas.Transcript
	err := s.orm.WithContext(ctx).
		Where("call_id = ?", callID).
		Order("created_at DESC").
		First(&transcript).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &transcript, nil
}

// summaryOf returns the call's newest summary, or nil when it has none. Same
// reason as transcriptOf for the explicit ordering.
func (s *Service) summaryOf(ctx context.Context, callID uuid.UUID) (*schemas.Summary, error) {
	var summary schemas.Summary
	err := s.orm.WithContext(ctx).
		Where("call_id = ?", callID).
		Order("created_at DESC").
		First(&summary).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &summary, nil
}
