package rooms

import (
	"context"
	"log/slog"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
)

// Admission is a join token plus the moment the call it joins must end, when
// the owner's plan limits call length.
type Admission struct {
	Token    string
	Deadline *time.Time
}

// admit enforces the owner's participant ceiling before a token is minted
// and returns the call deadline the client counts down to.
//
// The count is the call's connected people, read from the webhook-fed
// participant rows, minus the caller's own identity so a signed-in user
// reloading the page is not refused by their previous tab. A token minted but
// not used yet is not counted; the LiveKit room's own max_participants, set
// here before the first join, is the hard backstop for that race.
func (s *Service) admit(ctx context.Context, room *schemas.Room, identity string) (*time.Time, error) {
	policy, err := s.plans.ForRoom(ctx, *room)
	if err != nil {
		return nil, err
	}
	call, err := s.openCall(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	current, err := s.connected(ctx, call, identity)
	if err != nil {
		return nil, err
	}
	if err := s.plans.Check(policy, plans.KeyMaxParticipants, current, 1); err != nil {
		return nil, err
	}
	s.provisionRoom(ctx, room.Slug, s.plans.Limit(policy, plans.KeyMaxParticipants))
	startedAt := time.Now()
	if call != nil {
		startedAt = call.StartedAt
	}
	deadline, limited := s.plans.Deadline(policy, startedAt)
	if !limited {
		return nil, nil
	}
	return &deadline, nil
}

// provisionRoom creates the LiveKit room with its ceiling and timeouts. A
// failure is logged and the join goes on: Echo's own count already admitted
// the caller, and LiveKit creates the room on join anyway.
func (s *Service) provisionRoom(ctx context.Context, slug string, maxParticipants int64) {
	if err := s.provision(ctx, slug, media.RoomSettings{MaxParticipants: maxParticipants}); err != nil {
		slog.Warn("livekit room provisioning failed", slog.String("room", slug), slog.Any("error", err))
	}
}

// openCall is the room's latest call while it is still open, or nil.
func (s *Service) openCall(ctx context.Context, roomID uuid.UUID) (*schemas.Call, error) {
	var calls []schemas.Call
	err := s.orm.WithContext(ctx).
		Where("room_id = ?", roomID).
		Order("started_at DESC").Limit(1).
		Find(&calls).Error
	if err != nil || len(calls) == 0 || calls[0].EndedAt != nil {
		return nil, err
	}
	return &calls[0], nil
}

// connected counts the people Echo minted tokens for who are in the call:
// signed-in users and guests, not recorders or agents, and not except.
func (s *Service) connected(ctx context.Context, call *schemas.Call, except string) (int64, error) {
	if call == nil {
		return 0, nil
	}
	var count int64
	err := s.orm.WithContext(ctx).Model(&schemas.CallParticipant{}).
		Where("call_id = ? AND left_at IS NULL AND identity <> ?", call.ID, except).
		Where("(identity LIKE 'user-%' OR identity LIKE 'guest-%')").
		Count(&count).Error
	return count, err
}

// RequireRecording fails unless callerID owns the room and the owner's plan
// includes recording. A plan without it answers a *forfait.LimitError.
func (s *Service) RequireRecording(ctx context.Context, slug string, callerID int64) error {
	room, err := s.bySlugOwned(ctx, slug, callerID)
	if err != nil {
		return err
	}
	policy, err := s.plans.ForRoom(ctx, *room)
	if err != nil {
		return err
	}
	return s.plans.Require(policy, plans.KeyRecording)
}
