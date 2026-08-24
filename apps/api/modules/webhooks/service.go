package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Webhooks turns LiveKit webhook events into call and participant rows.
// LiveKit room names are Echo slugs by construction, so the mapping is direct.
type Webhooks struct {
	orm       *gorm.DB
	apiSecret string
	now       func() time.Time
}

// New builds a Webhooks receiver verifying against the LiveKit API secret.
func New(orm *gorm.DB, apiSecret string) *Webhooks {
	return &Webhooks{orm: orm, apiSecret: apiSecret, now: time.Now}
}

// verify checks the LiveKit webhook Authorization header: a JWT signed with
// the API secret whose sha256 claim is the base64 hash of the body. Both
// halves matter — a valid signature over a different body is a replay.
func (w *Webhooks) verify(authorization string, body []byte) error {
	if authorization == "" {
		return errors.New("missing authorization header")
	}
	var claims struct {
		SHA256 string `json:"sha256"`
		jwt.RegisteredClaims
	}
	token, err := jwt.ParseWithClaims(authorization, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return []byte(w.apiSecret), nil
	})
	if err != nil || !token.Valid {
		return errors.New("invalid webhook token")
	}
	sum := sha256.Sum256(body)
	expected := base64.StdEncoding.EncodeToString(sum[:])
	if !hmac.Equal([]byte(expected), []byte(claims.SHA256)) {
		return errors.New("body hash mismatch")
	}
	return nil
}

// dispatch routes one verified event. Every handler takes the same
// context-bound handle so the whole chain shares one connection scope.
func (w *Webhooks) dispatch(ctx context.Context, p *payload) error {
	db := w.orm.WithContext(ctx)
	switch p.Event {
	case eventRoomStarted:
		return w.roomStarted(db, p.Room)
	case eventRoomFinished:
		return w.roomFinished(db, p.Room)
	case eventParticipantJoined:
		return w.participantJoined(db, p.Room, p.Participant)
	case eventParticipantLeft:
		return w.participantLeft(db, p.Room, p.Participant)
	case eventEgressEnded:
		return w.egressEnded(db, p.Room, p.Egress)
	default:
		return nil
	}
}

// roomStarted opens a call for the room, and is idempotent when one is
// already open — LiveKit retries webhooks.
//
// A slug with no matching rooms row is deliberately a silent no-op: LiveKit
// accepts any room name, so ad-hoc and unowned rooms exist that Echo never
// created. There is no owner to show history to and no room row to hang a
// call off, so there is nothing to record and nothing to report.
func (w *Webhooks) roomStarted(db *gorm.DB, room *roomInfo) error {
	slug, ok := w.slugOf(room)
	if !ok {
		return nil
	}
	var existing schemas.Call
	err := db.Where("livekit_room_name = ? AND ended_at IS NULL", slug).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var rm schemas.Room
	if err := db.Where("slug = ?", slug).First(&rm).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return db.Create(&schemas.Call{
		ID:              uuid.New(),
		RoomID:          rm.ID,
		StartedAt:       w.now(),
		LivekitRoomName: slug,
	}).Error
}

// roomFinished closes the room's open call.
func (w *Webhooks) roomFinished(db *gorm.DB, room *roomInfo) error {
	call, ok, err := w.openCall(db, room)
	if err != nil || !ok {
		return err
	}
	return db.Model(call).Updates(map[string]any{"ended_at": w.now()}).Error
}

// participantJoined stamps the participant row, reopening it when the same
// identity rejoins the same call after a drop.
func (w *Webhooks) participantJoined(db *gorm.DB, room *roomInfo, participant *who) error {
	call, ok, err := w.openCall(db, room)
	if err != nil || !ok || !named(participant) {
		return err
	}
	var existing schemas.CallParticipant
	err = db.Where("call_id = ? AND identity = ?", call.ID, participant.Identity).First(&existing).Error
	if err == nil {
		return db.Model(&existing).Updates(map[string]any{"left_at": nil}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&schemas.CallParticipant{
		CallID:   call.ID,
		Identity: participant.Identity,
		Name:     participant.Name,
		JoinedAt: w.now(),
	}).Error
}

// participantLeft stamps left_at on the participant row.
func (w *Webhooks) participantLeft(db *gorm.DB, room *roomInfo, participant *who) error {
	call, ok, err := w.openCall(db, room)
	if err != nil || !ok || !named(participant) {
		return err
	}
	return db.Model(&schemas.CallParticipant{}).
		Where("call_id = ? AND identity = ?", call.ID, participant.Identity).
		Updates(map[string]any{"left_at": w.now()}).Error
}

// egressEnded records where the recording landed.
//
// The target call is selected first and then updated by primary key: Postgres
// does not apply ORDER BY to a bulk UPDATE, so an ordered Update would stamp
// whichever unrecorded rows the planner reached — every call of a busy room,
// not the latest one.
func (w *Webhooks) egressEnded(db *gorm.DB, room *roomInfo, egress *egressInfo) error {
	slug, ok := w.slugOf(room)
	if !ok || egress == nil || len(egress.FileResults) == 0 {
		return nil
	}
	path := egress.FileResults[0].Filename
	if path == "" {
		return nil
	}
	var call schemas.Call
	err := db.Where("livekit_room_name = ? AND recording_path = ''", slug).
		Order("started_at DESC").
		First(&call).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return db.Model(&schemas.Call{}).Where("id = ?", call.ID).Update("recording_path", path).Error
}

// openCall finds the room's most recent call that has not ended.
func (w *Webhooks) openCall(db *gorm.DB, room *roomInfo) (*schemas.Call, bool, error) {
	slug, ok := w.slugOf(room)
	if !ok {
		return nil, false, nil
	}
	var call schemas.Call
	err := db.Where("livekit_room_name = ? AND ended_at IS NULL", slug).
		Order("started_at DESC").
		First(&call).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &call, true, nil
}

func (w *Webhooks) slugOf(room *roomInfo) (string, bool) {
	if room == nil || room.Name == "" {
		return "", false
	}
	return room.Name, true
}

func named(participant *who) bool {
	return participant != nil && participant.Identity != ""
}
