package webhooks

import (
	"context"
	"errors"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
)

// Webhooks turns LiveKit webhook events into call and participant rows.
//
// Every handler keys on LiveKit's own session identity — the room sid, the
// participant sid, the egress id — never on "the newest open call". A lost or
// reordered delivery then costs one missing fact instead of merging two
// meetings, and a retried delivery changes nothing.
type Webhooks struct {
	orm  *gorm.DB
	keys auth.KeyProvider
	now  func() time.Time
}

// New builds a Webhooks receiver that verifies LiveKit's signed requests
// against the same API key pair the media service signs join tokens with.
func New(orm *gorm.DB, apiKey, apiSecret string) *Webhooks {
	return &Webhooks{
		orm:  orm,
		keys: auth.NewSimpleKeyProvider(apiKey, apiSecret),
		now:  time.Now,
	}
}

// dispatch routes one verified event. The whole handler runs in a single
// transaction: closing a call and closing its participants is one fact, and
// a crash between the two would leave attendees connected to a finished call
// forever.
func (w *Webhooks) dispatch(ctx context.Context, event *livekit.WebhookEvent) error {
	at := w.eventTime(event.GetCreatedAt())
	return w.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch event.GetEvent() {
		case eventRoomStarted:
			return roomStarted(tx, event.GetRoom(), at)
		case eventRoomFinished:
			return roomFinished(tx, event.GetRoom(), at)
		case eventParticipantJoined:
			return participantJoined(tx, event.GetRoom(), event.GetParticipant(), at)
		case eventParticipantLeft:
			return participantLeft(tx, event.GetRoom(), event.GetParticipant(), at)
		case eventEgressEnded:
			return egressEnded(tx, event.GetEgressInfo(), event.GetRoom())
		default:
			return nil
		}
	})
}

// eventTime prefers LiveKit's own timestamp so a delivery retried an hour
// later still stamps the moment the thing happened.
func (w *Webhooks) eventTime(unixSeconds int64) time.Time {
	if unixSeconds <= 0 {
		return w.now()
	}
	return time.Unix(unixSeconds, 0).UTC()
}

// callBySID loads the call recorded for one LiveKit room session, or nil when
// Echo never opened one for it.
func callBySID(db *gorm.DB, sid string) (*schemas.Call, error) {
	if sid == "" {
		return nil, nil
	}
	var call schemas.Call
	err := db.Where("livekit_room_sid = ?", sid).First(&call).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &call, nil
}
