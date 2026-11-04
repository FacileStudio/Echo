package calllimit

import (
	"context"
	"errors"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"gorm.io/gorm"
)

// staleAfter is how old an open call row must be before the reconcile may
// close it for LiveKit not running its session, so a row whose room_started
// raced the room list is not closed.
const staleAfter = 2 * time.Minute

// ArmRoom arms a deadline for a room's session before any webhook recorded
// it. It fires through the same checks as a call timer, against LiveKit's own
// room creation time, so it still holds when webhooks never arrive.
func (l *Limiter) ArmRoom(slug string, deadline time.Time) {
	if _, armed := l.deadlineOfKey(roomKey(slug)); armed {
		return
	}
	l.schedule(roomKey(slug), deadline, func(ctx context.Context) { l.evaluateRoom(ctx, slug) })
}

// RoomDeadline is the armed deadline of a room's session, if it has one.
func (l *Limiter) RoomDeadline(slug string) (time.Time, bool) {
	return l.deadlineOfKey(roomKey(slug))
}

// Reconcile compares Echo's open calls with the rooms LiveKit runs. It closes
// the call rows of sessions LiveKit forgot, a room_finished that never came,
// and arms or ends every running session's deadline.
func (l *Limiter) Reconcile(ctx context.Context) error {
	rooms, err := l.live.Rooms(ctx)
	if err != nil {
		return err
	}
	if err := l.closeStale(ctx, rooms); err != nil {
		return err
	}
	for _, room := range rooms {
		l.reconcileRoom(ctx, room)
	}
	return nil
}

// evaluateRoom re-reads a room's session from LiveKit and settles its timer.
func (l *Limiter) evaluateRoom(ctx context.Context, slug string) {
	rooms, err := l.live.Rooms(ctx, slug)
	if err != nil {
		l.schedule(roomKey(slug), l.now().Add(retryDelay), func(ctx context.Context) { l.evaluateRoom(ctx, slug) })
		return
	}
	if len(rooms) == 0 {
		l.disarm(roomKey(slug))
		return
	}
	l.reconcileRoom(ctx, rooms[0])
}

// reconcileRoom settles the deadline of one running session from LiveKit's
// creation time. A session a call row records belongs to that call's timer.
func (l *Limiter) reconcileRoom(ctx context.Context, live media.LiveRoom) {
	key := roomKey(live.Name)
	var room schemas.Room
	err := l.orm.WithContext(ctx).Where("slug = ?", live.Name).First(&room).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || live.CreatedAt.IsZero() {
		l.disarm(key)
		return
	}
	retry := func(ctx context.Context) { l.evaluateRoom(ctx, live.Name) }
	if err != nil {
		l.schedule(key, l.now().Add(retryDelay), retry)
		return
	}
	var recorded int64
	err = l.orm.WithContext(ctx).Model(&schemas.Call{}).
		Where("livekit_room_sid = ? AND ended_at IS NULL", live.SID).Count(&recorded).Error
	policy, policyErr := l.plans.ForRoom(ctx, room)
	if err != nil || policyErr != nil {
		l.schedule(key, l.now().Add(retryDelay), retry)
		return
	}
	deadline, limited := l.plans.Deadline(policy, live.CreatedAt)
	l.settle(ctx, key, session{slug: live.Name, sid: live.SID, deadline: deadline, limited: limited && recorded == 0}, retry)
}

// closeStale ends the open call rows whose session LiveKit no longer runs,
// and marks everyone still shown in them as gone. "-" is never a session id;
// it keeps the NOT IN list non-empty when LiveKit runs no room at all.
func (l *Limiter) closeStale(ctx context.Context, rooms []media.LiveRoom) error {
	live := make([]string, 0, len(rooms)+1)
	live = append(live, "-")
	for _, room := range rooms {
		live = append(live, room.SID)
	}
	now := l.now().UTC()
	var stale []string
	err := l.orm.WithContext(ctx).Model(&schemas.Call{}).
		Where("ended_at IS NULL AND started_at < ? AND livekit_room_sid NOT IN ?", now.Add(-staleAfter), live).
		Pluck("id", &stale).Error
	if err != nil || len(stale) == 0 {
		return err
	}
	return l.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&schemas.CallParticipant{}).
			Where("left_at IS NULL AND call_id IN ?", stale).Update("left_at", now).Error
		if err != nil {
			return err
		}
		return tx.Model(&schemas.Call{}).Where("id IN ? AND ended_at IS NULL", stale).Update("ended_at", now).Error
	})
}
