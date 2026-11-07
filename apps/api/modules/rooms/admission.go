package rooms

import (
	"context"
	"fmt"
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

// joiner is who asks for a seat: their LiveKit identity, the client address
// the request came from, and whether they own the room.
type joiner struct {
	identity string
	address  string
	owner    bool
}

// admit enforces the owner's plan before a token is minted and returns the
// session deadline the client counts down to.
//
// It refuses a room still cooling down after a cut, then counts who is in
// the call plus the tokens admitted in the last minute that have not
// connected yet, so a burst of tokens cannot outrun the count. The signed-in
// owner skips that count, so anonymous requests can never lock them out of
// their own room; LiveKit's max_participants still holds for everyone. Only
// an admitted join creates the LiveKit room (LiveKit never creates one by
// itself in Echo's deployment), with the plan's ceiling.
func (s *Service) admit(ctx context.Context, room *schemas.Room, who joiner) (*time.Time, error) {
	started := s.now()
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
	if err := s.seat(ctx, policy, room, who, call); err != nil {
		return nil, err
	}
	ceiling := s.plans.Limit(policy, plans.KeyMaxParticipants)
	live, err := s.provisionRoom(ctx, room.Slug, ceiling)
	if err == nil && who.owner {
		s.makeRoomForOwner(ctx, room.Slug, ceiling, who.identity, call)
	}
	if err == nil {
		err = s.cutSince(policy, room.Slug, started)
	}
	if err != nil {
		s.ledger.release(room.Slug, who.identity)
		return nil, err
	}
	return s.sessionDeadline(policy, room.Slug, call, live), nil
}

// seat counts the people present with the pending admissions, checks the
// participant ceiling, and takes a pending seat for the joiner. While the
// owner is absent one seat stays theirs, so guests alone can fill at most
// limit-1 seats and the owner always finds room. The owner skips this and is
// checked against an empty room, so only a plan allowing nobody refuses them.
func (s *Service) seat(ctx context.Context, policy plans.Policy, room *schemas.Room, who joiner, call *schemas.Call) error {
	if who.owner {
		return s.plans.Check(policy, plans.KeyMaxParticipants, 0, 1)
	}
	reserved := ""
	if room.OwnerID != nil {
		reserved = fmt.Sprintf("user-%d", *room.OwnerID)
	}
	return s.ledger.admit(room.Slug, who, identitiesOf(s.present(ctx, room.Slug, call)), reserved, func(count int64) error {
		return s.plans.Check(policy, plans.KeyMaxParticipants, count, 1)
	})
}

// cutSince refuses a join when the limiter cut the room at its deadline
// while this join was being admitted. Without it a join that passed the
// cooldown check a moment before the cut would start a fresh session.
func (s *Service) cutSince(policy plans.Policy, slug string, since time.Time) error {
	if cutAt, cut := s.deadlines.CutAt(slug); cut && !cutAt.Before(since) {
		return s.plans.Refusal(policy, plans.KeyMaxCallMinutes, s.plans.Limit(policy, plans.KeyMaxCallMinutes))
	}
	return nil
}

// provisionRoom creates the LiveKit room with its ceiling and timeouts. A
// failure refuses the join: with auto_create off, a token for a room LiveKit
// did not create would only meet a 404, and a capped room must never be
// joined without its ceiling.
func (s *Service) provisionRoom(ctx context.Context, slug string, ceiling int64) (media.LiveRoom, error) {
	live, err := s.live.EnsureRoom(ctx, slug, media.RoomSettings{MaxParticipants: ceiling})
	if err != nil {
		slog.Warn("livekit room provisioning failed", slog.String("room", slug), slog.Any("error", err))
		return media.LiveRoom{}, troncerrors.Unavailable("the call service is unavailable, try again shortly")
	}
	return live, nil
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

// RequireAI fails unless the plan of the room's owner includes ai.access.
// Transcripts and summaries are AI features, so they follow the owner's plan
// for everyone who reads them. A plan without it answers a *forfait.LimitError.
func (s *Service) RequireAI(ctx context.Context, room schemas.Room) error {
	policy, err := s.plans.ForRoom(ctx, room)
	if err != nil {
		return err
	}
	return s.plans.Require(policy, plans.KeyAIAccess)
}
