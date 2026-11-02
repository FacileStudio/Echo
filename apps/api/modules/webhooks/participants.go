package webhooks

import (
	"errors"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
)

// participantJoined records the attendee against the call the room sid names.
//
// The call is resolved by sid and not by "still open" on purpose: LiveKit
// routinely delivers a trailing participant event after room_finished, and
// dropping it loses an attendee from the record entirely.
func participantJoined(db *gorm.DB, room *livekit.Room, who *livekit.ParticipantInfo, at time.Time) error {
	call, err := callBySID(db, room.GetSid())
	if err != nil || call == nil || who.GetIdentity() == "" {
		return err
	}
	joined := at
	if who.GetJoinedAt() > 0 {
		joined = time.Unix(who.GetJoinedAt(), 0).UTC()
	}
	var existing schemas.CallParticipant
	err = db.Where("call_id = ? AND identity = ?", call.ID, who.GetIdentity()).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&schemas.CallParticipant{
			CallID:   call.ID,
			Identity: who.GetIdentity(),
			SID:      who.GetSid(),
			Name:     who.GetName(),
			JoinedAt: joined,
		}).Error
	}
	if err != nil {
		return err
	}
	return rejoin(db, &existing, who, joined)
}

// rejoin clears left_at only for a genuinely later connection.
//
// A redelivered join carries the participant sid that is already stored, and
// an out-of-order one carries a join time no later than the recorded one.
// Neither is a reason to resurrect someone the record says has left.
func rejoin(db *gorm.DB, existing *schemas.CallParticipant, who *livekit.ParticipantInfo, joined time.Time) error {
	if who.GetSid() != "" && who.GetSid() == existing.SID {
		return nil
	}
	if !joined.After(existing.JoinedAt) {
		return nil
	}
	return db.Model(&schemas.CallParticipant{}).
		Where("id = ?", existing.ID).
		Updates(map[string]any{"left_at": nil, "sid": who.GetSid(), "joined_at": joined}).Error
}

// participantLeft stamps left_at, leaving an already-stamped row alone so a
// retry cannot overwrite the first, correct departure time.
func participantLeft(db *gorm.DB, room *livekit.Room, who *livekit.ParticipantInfo, at time.Time) error {
	call, err := callBySID(db, room.GetSid())
	if err != nil || call == nil || who.GetIdentity() == "" {
		return err
	}
	return db.Model(&schemas.CallParticipant{}).
		Where("call_id = ? AND identity = ? AND left_at IS NULL", call.ID, who.GetIdentity()).
		Update("left_at", at).Error
}
