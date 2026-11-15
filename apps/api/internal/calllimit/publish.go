package calllimit

import (
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/bus"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PublishWith queues call.ended for every call row the limiter closes
// itself, in the transaction that closes it. A nil outbox publishes nothing.
// Call it before the limiter runs.
func (l *Limiter) PublishWith(o *bus.Outbox) {
	l.outbox = o
}

// closeCalls marks the calls ended at now along with everyone still shown in
// them, and queues call.ended for each. A call some other path closed first
// keeps its time, and its ended event's key is already taken.
func (l *Limiter) closeCalls(tx *gorm.DB, ids []uuid.UUID, now time.Time) error {
	err := tx.Model(&schemas.CallParticipant{}).
		Where("left_at IS NULL AND call_id IN ?", ids).Update("left_at", now).Error
	if err != nil {
		return err
	}
	if err := tx.Model(&schemas.Call{}).Where("id IN ? AND ended_at IS NULL", ids).Update("ended_at", now).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := l.outbox.Ended(tx, id); err != nil {
			return err
		}
	}
	return nil
}
