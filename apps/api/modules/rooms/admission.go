package rooms

import (
	"context"
	"log/slog"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
)

// cooldown is how long a room cut at its plan deadline stays closed before a
// new call can start in it. Without it a free call would only be paused.
const cooldown = 2 * time.Minute

// Admission is a join token plus the moment the call it joins must end, when
// the owner's plan limits call length.
type Admission struct {
	Token    string
	Deadline *time.Time
}

// admit enforces the owner's plan before a token is minted and returns the
// session deadline the client counts down to.
//
// It refuses a room still cooling down after a cut, creates the LiveKit room
// with the plan's ceiling (LiveKit never creates one by itself in Echo's
// deployment), and counts who is in the call plus every token admitted in
// the last token lifetime that has not connected yet, so a burst of tokens
// cannot outrun the count.
func (s *Service) admit(ctx context.Context, room *schemas.Room, identity string) (*time.Time, error) {
	policy, err := s.plans.ForRoom(ctx, *room)
	if err != nil {
		return nil, err
	}
	call, err := s.latestCall(ctx, room)
	if err != nil {
		return nil, err
	}
	if err := s.coolingDown(policy, room.Slug, call); err != nil {
		return nil, err
	}
	live, err := s.provisionRoom(ctx, room.Slug, s.plans.Limit(policy, plans.KeyMaxParticipants))
	if err != nil {
		return nil, err
	}
	present := s.present(ctx, room.Slug, call)
	err = s.ledger.admit(room.Slug, identity, present, func(count int64) error {
		return s.plans.Check(policy, plans.KeyMaxParticipants, count, 1)
	})
	if err != nil {
		return nil, err
	}
	return s.sessionDeadline(policy, room.Slug, call, live), nil
}

// provisionRoom creates the LiveKit room with its ceiling and timeouts. When
// the plan caps participants, a failure refuses the join: a room LiveKit did
// not create with the ceiling must not be joined at all. An unlimited room
// logs the failure and lets LiveKit answer the join itself.
func (s *Service) provisionRoom(ctx context.Context, slug string, ceiling int64) (media.LiveRoom, error) {
	live, err := s.live.EnsureRoom(ctx, slug, media.RoomSettings{MaxParticipants: ceiling})
	if err == nil {
		return live, nil
	}
	slog.Warn("livekit room provisioning failed", slog.String("room", slug), slog.Any("error", err))
	if ceiling != -1 {
		return media.LiveRoom{}, troncerrors.Unavailable("the call service is unavailable, try again shortly")
	}
	return media.LiveRoom{}, nil
}

// coolingDown refuses a room whose last session was cut at its plan deadline
// less than the cooldown ago, on a plan that limits call length.
func (s *Service) coolingDown(policy plans.Policy, slug string, call *schemas.Call) error {
	now := s.now()
	if _, limited := s.plans.Deadline(policy, now); !limited {
		return nil
	}
	cutAt, cut := s.deadlines.CutAt(slug)
	if !cut && call != nil && call.EndedAt != nil {
		deadline, _ := s.plans.Deadline(policy, call.StartedAt)
		cutAt, cut = *call.EndedAt, !call.EndedAt.Before(deadline)
	}
	if !cut || !now.Before(cutAt.Add(cooldown)) {
		return nil
	}
	return s.plans.Refusal(policy, plans.KeyMaxCallMinutes, s.plans.Limit(policy, plans.KeyMaxCallMinutes))
}

// sessionDeadline is when the running session must end. The session start is
// LiveKit's own room creation time, else the open call's started_at, else
// now. A session no call row records yet gets a room timer, so the deadline
// holds even when no webhook ever arrives.
func (s *Service) sessionDeadline(policy plans.Policy, slug string, call *schemas.Call, live media.LiveRoom) *time.Time {
	open := call != nil && call.EndedAt == nil
	start := live.CreatedAt
	if start.IsZero() && open {
		start = call.StartedAt
	}
	if start.IsZero() {
		start = s.now()
	}
	deadline, limited := s.plans.Deadline(policy, start)
	if !limited {
		return nil
	}
	if !open || call.LivekitRoomSID != live.SID {
		s.deadlines.ArmRoom(slug, deadline)
	}
	return &deadline
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
