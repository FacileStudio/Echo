package webhooks

import (
	"errors"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/bus"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// roomStarted opens the call for this LiveKit room session and queues
// call.started when it did.
//
// The insert is keyed on the room sid and conflicts do nothing, so a retried
// delivery is a no-op at the database rather than at a preceding SELECT, and
// publishes nothing.
//
// A slug with no matching rooms row is deliberately a silent no-op: LiveKit
// accepts any room name, so ad-hoc rooms exist that Echo never created. There
// is no owner to show history to and no room row to hang a call off.
func roomStarted(db *gorm.DB, out *bus.Outbox, room *livekit.Room, at time.Time) error {
	sid, slug := room.GetSid(), room.GetName()
	if sid == "" || slug == "" {
		return nil
	}
	var rm schemas.Room
	err := db.Where("slug = ?", slug).First(&rm).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	call := schemas.Call{
		ID:              uuid.New(),
		RoomID:          rm.ID,
		StartedAt:       at,
		LivekitRoomName: slug,
		LivekitRoomSID:  sid,
	}
	created := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&call)
	if created.Error != nil || created.RowsAffected == 0 {
		return created.Error
	}
	return out.Started(db, call.ID)
}

// roomFinished closes the call and everyone still shown as connected to it,
// and queues call.ended when this delivery is the one that closed it.
//
// Both updates guard on the column still being NULL, so a redelivered event
// cannot move a timestamp that is already correct.
func roomFinished(db *gorm.DB, out *bus.Outbox, room *livekit.Room, at time.Time) error {
	call, err := callBySID(db, room.GetSid())
	if err != nil || call == nil {
		return err
	}
	closed := db.Model(&schemas.Call{}).
		Where("id = ? AND ended_at IS NULL", call.ID).
		Update("ended_at", at)
	if closed.Error != nil {
		return closed.Error
	}
	if err := db.Model(&schemas.CallParticipant{}).
		Where("call_id = ? AND left_at IS NULL", call.ID).
		Update("left_at", at).Error; err != nil {
		return err
	}
	if closed.RowsAffected == 0 {
		return nil
	}
	return out.Ended(db, call.ID)
}
