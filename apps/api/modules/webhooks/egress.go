package webhooks

import (
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
)

// egressEnded records where the recorder put the file.
//
// The target is the call whose room sid the egress names, never "the newest
// call with no recording yet" — that guess used to hand a retried delivery a
// second, unrelated call and serve two owners the same MP4. The egress id is
// stored with the path, so the retry finds its own id and stops.
func egressEnded(db *gorm.DB, egress *livekit.EgressInfo, room *livekit.Room) error {
	path := recordingPath(egress)
	if path == "" {
		return nil
	}
	stamped, err := alreadyStamped(db, egress.GetEgressId())
	if err != nil || stamped {
		return err
	}
	sid := egress.GetRoomId()
	if sid == "" {
		sid = room.GetSid()
	}
	call, err := callBySID(db, sid)
	if err != nil || call == nil {
		return err
	}
	if name := egress.GetRoomName(); name != "" && name != call.LivekitRoomName {
		return nil
	}
	return db.Model(&schemas.Call{}).Where("id = ?", call.ID).
		Updates(map[string]any{"recording_path": path, "egress_id": egress.GetEgressId()}).Error
}

// alreadyStamped reports whether some call already carries this egress id,
// which is the whole of the idempotency check.
func alreadyStamped(db *gorm.DB, egressID string) (bool, error) {
	if egressID == "" {
		return false, nil
	}
	var count int64
	err := db.Model(&schemas.Call{}).Where("egress_id = ?", egressID).Count(&count).Error
	return count > 0, err
}

// recordingPath returns the first file the egress actually produced.
func recordingPath(egress *livekit.EgressInfo) string {
	for _, file := range egress.GetFileResults() {
		if file.GetFilename() != "" {
			return file.GetFilename()
		}
	}
	return ""
}
