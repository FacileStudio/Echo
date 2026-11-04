package calllimit

import (
	"context"
	"errors"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// liveCall loads a call and its room, or nil when the call ended or a newer
// call of the same room superseded it. Ending a superseded call's room would
// cut the newer meeting short; it carries its own timer.
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

// schedule (re)arms the call's timer for deadline.
func (l *Limiter) schedule(callID uuid.UUID, deadline time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry, ok := l.armed[callID]; ok {
		entry.timer.Stop()
	}
	timer := time.AfterFunc(deadline.Sub(l.now()), func() {
		if l.base.Err() == nil {
			l.evaluate(l.base, callID)
		}
	})
	l.armed[callID] = armed{timer: timer, deadline: deadline}
}

// disarm drops the call's timer, if any.
func (l *Limiter) disarm(callID uuid.UUID) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry, ok := l.armed[callID]; ok {
		entry.timer.Stop()
		delete(l.armed, callID)
	}
}
