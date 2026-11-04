package calllimit

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// session is the LiveKit session a timer ends: the room, the session id it
// must still carry (empty when any session of the room counts), and its
// deadline when the plan limits it.
type session struct {
	slug     string
	sid      string
	deadline time.Time
	limited  bool
}

func callKey(id uuid.UUID) string { return "call:" + id.String() }

func roomKey(slug string) string { return "room:" + slug }

// liveCall loads a call and its room, or nil when the call ended or a newer
// call of the same room superseded it.
func (l *Limiter) liveCall(ctx context.Context, callID uuid.UUID) (*schemas.Call, *schemas.Room, error) {
	var call schemas.Call
	err := l.orm.WithContext(ctx).Preload("Room").Where("id = ?", callID).First(&call).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}
	if err != nil || call.EndedAt != nil {
		return nil, nil, err
	}
	var newer int64
	err = l.orm.WithContext(ctx).Model(&schemas.Call{}).
		Where("room_id = ? AND started_at > ?", call.RoomID, call.StartedAt).
		Count(&newer).Error
	if err != nil || newer > 0 {
		return nil, nil, err
	}
	return &call, &call.Room, nil
}

// settle drops the timer of an unlimited session, arms one for a deadline
// still ahead, and ends a session whose deadline passed.
func (l *Limiter) settle(ctx context.Context, key string, s session, retry func(context.Context)) {
	switch {
	case !s.limited:
		l.disarm(key)
	case s.deadline.After(l.now()):
		l.schedule(key, s.deadline, retry)
	default:
		l.endSession(ctx, key, s, retry)
	}
}

// endSession closes the room when LiveKit still runs the session the timer
// was armed for. A session LiveKit no longer runs is over already, and a
// different session of the same room is a newer meeting with its own
// deadline: ending it would cut that meeting short.
func (l *Limiter) endSession(ctx context.Context, key string, s session, retry func(context.Context)) {
	rooms, err := l.live.Rooms(ctx, s.slug)
	if err == nil && (len(rooms) == 0 || (s.sid != "" && rooms[0].SID != s.sid)) {
		l.disarm(key)
		return
	}
	if err == nil {
		err = l.live.EndRoom(ctx, s.slug)
	}
	if err != nil && !errors.Is(err, media.ErrNotFound) {
		l.logger.Warn("call limit: ending the room failed, retrying", slog.String("room", s.slug), slog.Any("error", err))
		l.schedule(key, l.now().Add(retryDelay), retry)
		return
	}
	l.logger.Info("call limit: call ended at its plan deadline", slog.String("room", s.slug))
	l.mu.Lock()
	l.cuts[s.slug] = l.now()
	l.mu.Unlock()
	l.disarm(key)
}

// schedule (re)arms the timer under key to run at deadline.
func (l *Limiter) schedule(key string, deadline time.Time, run func(context.Context)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry, ok := l.armed[key]; ok {
		entry.timer.Stop()
	}
	timer := time.AfterFunc(deadline.Sub(l.now()), func() {
		if l.base.Err() == nil {
			run(l.base)
		}
	})
	l.armed[key] = armed{timer: timer, deadline: deadline}
}

// disarm drops the timer under key, if any.
func (l *Limiter) disarm(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry, ok := l.armed[key]; ok {
		entry.timer.Stop()
		delete(l.armed, key)
	}
}

func (l *Limiter) deadlineOfKey(key string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.armed[key]
	return entry.deadline, ok
}
