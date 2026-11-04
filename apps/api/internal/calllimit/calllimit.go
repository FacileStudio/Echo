// Package calllimit ends a call when the room owner's plan says its time is
// up.
//
// The deadline is the call's started_at plus echo.max_call_minutes, both read
// from server-held state, so a client can neither hide nor extend it. Timers
// live in memory and are re-armed from the open calls at boot, so a restart
// never hands a running call unlimited time.
package calllimit

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
)

// retryDelay is how long a failed attempt to end a call waits before the next.
const retryDelay = time.Minute

// RoomEnder closes a LiveKit room and disconnects everyone in it.
type RoomEnder interface {
	EndRoom(ctx context.Context, roomName string) error
}

// Limiter holds one timer per open call whose plan limits its length.
type Limiter struct {
	orm    *gorm.DB
	plans  *plans.Plans
	ender  RoomEnder
	base   context.Context
	logger *slog.Logger
	now    func() time.Time

	mu    sync.Mutex
	armed map[uuid.UUID]armed
}

type armed struct {
	timer    *time.Timer
	deadline time.Time
}

// New builds a Limiter. base is the context timers run under; cancelling it
// stops nothing already armed, so pass the process's shutdown context and let
// the process exit.
func New(base context.Context, orm *gorm.DB, plans *plans.Plans, ender RoomEnder, logger *slog.Logger) *Limiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Limiter{
		orm: orm, plans: plans, ender: ender, base: base, logger: logger,
		now: time.Now, armed: map[uuid.UUID]armed{},
	}
}

// Rearm arms a timer for every call still open, from its stored started_at.
// Run it once at boot.
func (l *Limiter) Rearm(ctx context.Context) error {
	var ids []uuid.UUID
	if err := l.orm.WithContext(ctx).Model(&schemas.Call{}).Where("ended_at IS NULL").Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		l.evaluate(ctx, id)
	}
	return nil
}

// Notify arms the call a room_started opened and drops the timer of the call
// a room_finished closed. The webhook receiver calls it after its
// transaction commits.
func (l *Limiter) Notify(ctx context.Context, event *livekit.WebhookEvent) {
	switch event.GetEvent() {
	case "room_started", "room_finished":
	default:
		return
	}
	var ids []uuid.UUID
	err := l.orm.WithContext(ctx).Model(&schemas.Call{}).
		Where("livekit_room_sid = ? AND livekit_room_sid <> ''", event.GetRoom().GetSid()).
		Pluck("id", &ids).Error
	if err != nil {
		l.logger.Warn("call limit: call lookup failed", slog.Any("error", err))
		return
	}
	for _, id := range ids {
		l.evaluate(ctx, id)
	}
}

// Deadline is the armed deadline of a call, if it has one.
func (l *Limiter) Deadline(callID uuid.UUID) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.armed[callID]
	return entry.deadline, ok
}

// evaluate brings one call's timer in line with the database and the plan:
// no timer for a call that ended, was superseded or is unlimited, a timer
// for a deadline still ahead, and the end of the room for one that passed.
// The plan is read again every time, so an owner who upgrades mid-call is
// not cut off at the old deadline.
func (l *Limiter) evaluate(ctx context.Context, callID uuid.UUID) {
	call, room, err := l.liveCall(ctx, callID)
	if err != nil {
		l.logger.Warn("call limit: call lookup failed", slog.Any("error", err))
		l.schedule(callID, l.now().Add(retryDelay))
		return
	}
	deadline, limited := time.Time{}, false
	if call != nil {
		deadline, limited = l.deadlineOf(ctx, *call, *room)
	}
	switch {
	case !limited:
		l.disarm(callID)
	case deadline.After(l.now()):
		l.schedule(callID, deadline)
	default:
		l.end(ctx, callID, room.Slug)
	}
}

// deadlineOf is the call's deadline under its room's current policy.
func (l *Limiter) deadlineOf(ctx context.Context, call schemas.Call, room schemas.Room) (time.Time, bool) {
	policy, err := l.plans.ForRoom(ctx, room)
	if err != nil {
		l.logger.Warn("call limit: plan lookup failed, retrying", slog.Any("error", err))
		return l.now().Add(retryDelay), true
	}
	return l.plans.Deadline(policy, call.StartedAt)
}

// end closes the room, and tries again later when LiveKit could not be
// reached. A room LiveKit no longer knows is already over.
func (l *Limiter) end(ctx context.Context, callID uuid.UUID, slug string) {
	err := l.ender.EndRoom(ctx, slug)
	if err == nil || errors.Is(err, media.ErrNotFound) {
		l.logger.Info("call limit: call ended at its plan deadline", slog.String("room", slug))
		l.disarm(callID)
		return
	}
	l.logger.Warn("call limit: ending the room failed, retrying", slog.String("room", slug), slog.Any("error", err))
	l.schedule(callID, l.now().Add(retryDelay))
}
