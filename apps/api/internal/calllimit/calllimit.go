// Package calllimit ends a call when the room owner's plan says its time is
// up.
//
// A session's deadline is its start plus echo.max_call_minutes, read from
// server-held state only: the call row's started_at, or LiveKit's own room
// creation time when no webhook recorded the call. A client can neither hide
// nor extend it. Timers live in memory; they are re-armed from the open calls
// at boot, and a periodic reconcile against LiveKit's room list arms the
// sessions no webhook told Echo about and closes call rows LiveKit has
// forgotten.
package calllimit

import (
	"context"
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

// cutMemory is how long the limiter remembers a cut, well past the cooldown
// that reads it, after which the entry is dropped.
const cutMemory = 10 * time.Minute

// Notifier hears the room_finished the limiter reports for a call LiveKit
// forgot, the same way the webhook receiver reports LiveKit's own.
type Notifier interface {
	Notify(ctx context.Context, event *livekit.WebhookEvent)
}

// LiveKit is the part of the LiveKit room service the limiter drives.
type LiveKit interface {
	EndRoom(ctx context.Context, roomName string) error
	Rooms(ctx context.Context, names ...string) ([]media.LiveRoom, error)
}

// Limiter holds one timer per limited session: per call row, or per room
// when the session has no call row yet.
type Limiter struct {
	orm    *gorm.DB
	plans  *plans.Plans
	live   LiveKit
	base   context.Context
	logger *slog.Logger
	now    func() time.Time

	mu        sync.Mutex
	armed     map[string]armed
	cuts      map[string]time.Time
	notifiers []Notifier
}

type armed struct {
	timer    *time.Timer
	deadline time.Time
}

// New builds a Limiter. base is the context timers run under, normally the
// process's shutdown context.
func New(base context.Context, orm *gorm.DB, plans *plans.Plans, live LiveKit, logger *slog.Logger) *Limiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Limiter{
		orm: orm, plans: plans, live: live, base: base, logger: logger,
		now: time.Now, armed: map[string]armed{}, cuts: map[string]time.Time{},
	}
}

// Rearm arms a timer for every call still open, from its stored started_at.
// Run it at boot and after every applied snapshot.
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
	return l.deadlineOfKey(callKey(callID))
}

// NotifyWith adds a notifier told about every call the reconcile closes.
func (l *Limiter) NotifyWith(n Notifier) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.notifiers = append(l.notifiers, n)
}

// recordCut remembers that the room is being cut now, and forgets cuts older
// than cutMemory. It runs before the room is closed, so a join racing the cut
// already sees it.
func (l *Limiter) recordCut(slug string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for room, at := range l.cuts {
		if now.Sub(at) > cutMemory {
			delete(l.cuts, room)
		}
	}
	l.cuts[slug] = now
}

// CutAt is when the limiter last ended the room at its plan deadline.
func (l *Limiter) CutAt(slug string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	at, ok := l.cuts[slug]
	return at, ok
}

// evaluate brings one call's timer in line with the database and the plan:
// no timer for a call that ended, was superseded or is unlimited, a timer
// for a deadline still ahead, and the end of the session for one that
// passed. The plan is read again every time, so an owner who upgrades
// mid-call is not cut off at the old deadline.
func (l *Limiter) evaluate(ctx context.Context, callID uuid.UUID) {
	key, retry := callKey(callID), func(ctx context.Context) { l.evaluate(ctx, callID) }
	call, room, err := l.liveCall(ctx, callID)
	if err != nil {
		l.logger.Warn("call limit: call lookup failed", slog.Any("error", err))
		l.schedule(key, l.now().Add(retryDelay), retry)
		return
	}
	if call == nil {
		l.disarm(key)
		return
	}
	policy, err := l.plans.ForRoom(ctx, *room)
	if err != nil {
		l.schedule(key, l.now().Add(retryDelay), retry)
		return
	}
	deadline, limited := l.plans.Deadline(policy, call.StartedAt)
	l.settle(ctx, key, session{slug: room.Slug, sid: call.LivekitRoomSID, deadline: deadline, limited: limited}, retry)
}
