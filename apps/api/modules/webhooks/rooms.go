package webhooks

import (
	"errors"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// roomStarted opens the call for this LiveKit room session.
//
// The insert is keyed on the room sid and conflicts do nothing, so a retried
// delivery is a no-op at the database rather than at a preceding SELECT.
//
// A slug with no matching rooms row is deliberately a silent no-op: LiveKit
// accepts any room name, so ad-hoc rooms exist that Echo never created. There
// is no owner to show history to and no room row to hang a call off.
func roomStarted(db *gorm.DB, room *livekit.Room, at time.Time) error {
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
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&schemas.Call{
		ID:              uuid.New(),
		RoomID:          rm.ID,
		StartedAt:       at,
		LivekitRoomName: slug,
		LivekitRoomSID:  sid,
	}).Error
}

// roomFinished closes the call and everyone still shown as connected to it.
//
// Both updates guard on the column still being NULL, so a redelivered event
// cannot move a timestamp that is already correct.
func roomFinished(db *gorm.DB, room *livekit.Room, at time.Time) error {
	call, err := callBySID(db, room.GetSid())
	if err != nil || call == nil {
		return err
	}
	if err := db.Model(&schemas.Call{}).
		Where("id = ? AND ended_at IS NULL", call.ID).
		Update("ended_at", at).Error; err != nil {
		return err
	}
	return db.Model(&schemas.CallParticipant{}).
		Where("call_id = ? AND left_at IS NULL", call.ID).
		Update("left_at", at).Error
}
