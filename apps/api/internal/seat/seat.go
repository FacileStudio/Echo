// Package seat answers who is calling a capture route: the participant seated
// in a call, never a name the request claims. A signed-in caller is identified
// by their session. A guest has no session, so the token response hands them a
// seat key, signed under the LiveKit API secret, that names their identity.
package seat

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/FacileStudio/porte"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/FacileStudio/Echo/apps/api/schemas"
)

// Header carries a guest's seat key.
const Header = "X-Echo-Seat-Key"

// KeyTTL bounds a seat key. A key is also useless once its holder has left the
// call, because every capture route checks the open participant row.
const KeyTTL = 12 * time.Hour

// Keys signs and verifies seat keys.
type Keys struct {
	secret []byte
	now    func() time.Time
}

// NewKeys builds Keys over the LiveKit API secret.
func NewKeys(secret string) *Keys { return &Keys{secret: []byte(secret), now: time.Now} }

// Sign is the seat key for identity in slug: "<unix expiry>.<identity>.<mac>".
func (k *Keys) Sign(slug, identity string) string {
	exp := strconv.FormatInt(k.now().Add(KeyTTL).Unix(), 10)
	return exp + "." + identity + "." + k.mac(slug, identity, exp)
}

// Verify returns the identity a seat key was issued for in slug.
func (k *Keys) Verify(slug, key string) (string, bool) {
	exp, rest, ok := strings.Cut(key, ".")
	if !ok {
		return "", false
	}
	cut := strings.LastIndex(rest, ".")
	if cut < 1 {
		return "", false
	}
	identity, mac := rest[:cut], rest[cut+1:]
	expires, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || !k.now().Before(time.Unix(expires, 0)) || time.Unix(expires, 0).After(k.now().Add(KeyTTL)) {
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(mac), []byte(k.mac(slug, identity, exp))) != 1 {
		return "", false
	}
	return identity, true
}

func (k *Keys) mac(slug, identity, exp string) string {
	mac := hmac.New(sha256.New, k.secret)
	mac.Write([]byte("echo-seat-v1\x00" + slug + "\x00" + identity + "\x00" + exp))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

var errNoSeat = troncerrors.Unauthorized("you are not seated in this call")

// Identity resolves the caller of r: the account behind the session, else the
// identity a valid seat key names. It reads nothing else from the request.
func (k *Keys) Identity(r *http.Request, slug string) (string, error) {
	if who, ok := porte.From(r.Context()); ok && who.UserID > 0 {
		return fmt.Sprintf("user-%d", who.UserID), nil
	}
	if identity, ok := k.Verify(slug, r.Header.Get(Header)); ok {
		return identity, nil
	}
	return "", errNoSeat
}

// OpenCall locks the room's open call. It is NotFound when no call is open.
func OpenCall(tx *gorm.DB, slug string, lock bool) (schemas.Call, error) {
	var call schemas.Call
	query := tx
	if lock {
		query = tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.Where("livekit_room_name = ? AND ended_at IS NULL", slug).
		Order("started_at DESC, id").First(&call).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return call, troncerrors.NotFound("no open call for this room")
	}
	return call, err
}

// Seated fails unless identity holds an open seat in the call: the webhook-fed
// participant row has no left_at. A person who left, or never joined, is not
// seated, whatever key they hold.
func Seated(tx *gorm.DB, callID uuid.UUID, identity string) error {
	var count int64
	err := tx.Model(&schemas.CallParticipant{}).
		Where("call_id = ? AND identity = ? AND left_at IS NULL", callID, identity).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return errNoSeat
	}
	return nil
}
