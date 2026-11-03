package integrations

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

const (
	prismeSecret  = "prisme-integration-secret-long-enough-0001"
	otherSecret   = "other-app-integration-secret-long-enough-02"
	livekitSecret = "livekit-api-secret-that-is-long-enough"
)

var testKeys = NewKeys(map[string]string{"prisme": prismeSecret, "other": otherSecret})

type harness struct {
	t       *testing.T
	db      *gorm.DB
	router  chi.Router
	service *Service
}

func newMedia(t *testing.T) *media.Service {
	t.Helper()
	t.Setenv("LIVEKIT_API_KEY", "devkey")
	t.Setenv("LIVEKIT_API_SECRET", livekitSecret)
	t.Setenv("LIVEKIT_URL", "wss://live.example.test")
	svc, err := media.NewServiceFromEnv()
	if err != nil {
		t.Fatalf("media: %v", err)
	}
	return svc
}

func newHarnessOn(t *testing.T, db *gorm.DB) *harness {
	t.Helper()
	service := NewService(db, newMedia(t), testKeys)
	router := chi.NewRouter()
	RegisterRoutes(router, service)
	return &harness{t: t, db: db, router: router, service: service}
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessOn(t, testdb.Migrated(t))
}

func (h *harness) call(method, path, secret string, body any) *httptest.ResponseRecorder {
	h.t.Helper()
	var reader io.Reader = http.NoBody
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("encode: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	if secret != "" {
		request.Header.Set("Authorization", "Bearer "+secret)
	}
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)
	return recorder
}

func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(recorder.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", recorder.Body.String(), err)
	}
	return out
}

func (h *harness) createRoom(secret, externalID string) RoomCreated {
	h.t.Helper()
	got := h.call(http.MethodPost, "/integrations/rooms", secret, CreateRoomRequest{ExternalID: externalID, Title: "#general"})
	if got.Code != http.StatusCreated && got.Code != http.StatusOK {
		h.t.Fatalf("create: %d %s", got.Code, got.Body.String())
	}
	return decode[RoomCreated](h.t, got)
}

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
