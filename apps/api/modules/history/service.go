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

// Service serves call history, recordings, transcripts and summaries. The
// room owner sees everything of their calls; someone who only sat in a call
// sees its participants, transcript and summary, and a recording only when
// the owner shared it.
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

// Detail returns one call the caller owns or sat in, with its participants and
// summary. The transcript has its own endpoint, and the recordings theirs.
func (s *Service) Detail(ctx context.Context, callID string, callerID int64) (*callDetail, error) {
	got, err := s.accessible(ctx, callID, callerID)
	if err != nil {
		return nil, err
	}
	detail := &callDetail{
		callResponse: toResponse(*got.call),
		RoomSlug:     got.room.Slug,
		RoomName:     got.room.Name,
		Role:         got.role,
	}
	if err := s.describe(ctx, got, detail); err != nil {
		return nil, err
	}
	summary, err := s.summaryOf(ctx, got.call.ID)
	if err != nil {
		return nil, err
	}
	if summary != nil {
		payload := toSummary(*summary)
		detail.Summary = &payload
	}
	return detail, nil
}

// describe fills what a call shows beside its summary: the people, whether the
// caller can watch a recording, and whether a transcript exists.
func (s *Service) describe(ctx context.Context, got *access, detail *callDetail) error {
	people, err := s.participantsOf(ctx, []uuid.UUID{got.call.ID})
	if err != nil {
		return err
	}
	detail.Participants = people[got.call.ID]
	if detail.Participants == nil {
		detail.Participants = []participantOut{}
	}
	if detail.HasRecording, err = s.hasVisibleRecording(ctx, got); err != nil {
		return err
	}
	transcript, err := s.transcriptOf(ctx, got.call.ID)
	detail.HasTranscript = transcript != nil && transcript.Content != ""
	return err
}

// Summarize generates (or regenerates) the AI summary of an owned call,
// through Bulle acting for the room owner, who is the caller.
func (s *Service) Summarize(ctx context.Context, callID string, callerID int64) (*summaryPayload, error) {
	got, err := s.accessible(ctx, callID, callerID)
	if err != nil {
		return nil, err
	}
	if !got.owner() {
		return nil, troncerrors.Forbidden("only the room owner can do this")
	}
	call := got.call
	if err := s.rooms.RequireAI(ctx, got.room); err != nil {
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
	content, model, err := s.summarizer.Summarize(ctx, callerID, transcript.Content)
	if err != nil {
		return nil, err
	}
	return s.storeSummary(ctx, call.ID, content, model)
}

// storeSummary writes the call's summary as a single upsert on call_id. A
// read-then-write would either duplicate the row or, once call_id is unique,
// lose the race to a constraint violation.
func (s *Service) storeSummary(ctx context.Context, callID uuid.UUID, content, model string) (*summaryPayload, error) {
	row := schemas.Summary{
		ID:      uuid.New(),
		CallID:  callID,
		Content: content,
		Model:   model,
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
