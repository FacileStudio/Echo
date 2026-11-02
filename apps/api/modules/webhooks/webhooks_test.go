package webhooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
)

const (
	testKey    = "devkey"
	testSecret = "livekit-api-secret-that-is-long-enough"
)

var base = time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)

// harness drives the real HTTP handler. Nothing here builds an internal
// struct and calls dispatch: the bug this module was rewritten for lived
// entirely in the gap between the wire format and the Go types, so a test
// that skips the wire proves nothing.
type harness struct {
	t      *testing.T
	router chi.Router
	db     *gorm.DB
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db := testdb.Migrated(t)
	router := chi.NewRouter()
	New(db, testKey, testSecret).RegisterRoutes(router)
	return &harness{t: t, router: router, db: db}
}

// encode renders the event the way livekit-server does, with protojson.
func encode(t *testing.T, event *livekit.WebhookEvent) []byte {
	t.Helper()
	body, err := protojson.Marshal(event)
	if err != nil {
		t.Fatalf("protojson marshal: %v", err)
	}
	return body
}

// sign mints the Authorization token livekit-server sends: an API access
// token whose sha256 grant is the base64 digest of the exact body.
func sign(t *testing.T, key, secret string, body []byte) string {
	t.Helper()
	sum := sha256.Sum256(body)
	token, err := auth.NewAccessToken(key, secret).
		SetValidFor(5 * time.Minute).
		SetSha256(base64.StdEncoding.EncodeToString(sum[:])).
		ToJWT()
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return token
}

func (h *harness) deliver(body []byte, token string) *httptest.ResponseRecorder {
	h.t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/livekit/webhook", bytes.NewReader(body))
	request.Header.Set("Authorization", token)
	request.Header.Set("Content-Type", "application/webhook+json")
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)
	return recorder
}

// send delivers a correctly signed event and fails unless it was accepted.
func (h *harness) send(event *livekit.WebhookEvent) {
	h.t.Helper()
	body := encode(h.t, event)
	got := h.deliver(body, sign(h.t, testKey, testSecret, body))
	if got.Code != http.StatusNoContent {
		h.t.Fatalf("%s: status %d, body %s", event.GetEvent(), got.Code, got.Body.String())
	}
}

func (h *harness) seedRoom(slug string) schemas.Room {
	h.t.Helper()
	room := schemas.Room{ID: uuid.New(), Slug: slug, Name: slug}
	if err := h.db.Create(&room).Error; err != nil {
		h.t.Fatalf("seed room: %v", err)
	}
	return room
}

func (h *harness) calls() []schemas.Call {
	h.t.Helper()
	var calls []schemas.Call
	if err := h.db.Order("started_at ASC").Find(&calls).Error; err != nil {
		h.t.Fatalf("load calls: %v", err)
	}
	return calls
}

func (h *harness) participants() []schemas.CallParticipant {
	h.t.Helper()
	var rows []schemas.CallParticipant
	if err := h.db.Order("id ASC").Find(&rows).Error; err != nil {
		h.t.Fatalf("load participants: %v", err)
	}
	return rows
}

func roomEvent(name, slug, sid string, at time.Time) *livekit.WebhookEvent {
	return &livekit.WebhookEvent{
		Event:     name,
		Room:      &livekit.Room{Sid: sid, Name: slug},
		Id:        uuid.NewString(),
		CreatedAt: at.Unix(),
	}
}

func peopleEvent(name, slug, sid string, who *livekit.ParticipantInfo, at time.Time) *livekit.WebhookEvent {
	event := roomEvent(name, slug, sid, at)
	event.Participant = who
	return event
}

func egressEvent(sid, slug, egressID, filename string, at time.Time) *livekit.WebhookEvent {
	return &livekit.WebhookEvent{
		Event: eventEgressEnded,
		EgressInfo: &livekit.EgressInfo{
			EgressId:    egressID,
			RoomId:      sid,
			RoomName:    slug,
			FileResults: []*livekit.FileInfo{{Filename: filename}},
		},
		Id:        uuid.NewString(),
		CreatedAt: at.Unix(),
	}
}
