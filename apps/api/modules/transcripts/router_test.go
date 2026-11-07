package transcripts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/internal/seat"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/go-chi/chi/v5"
)

func post(t *testing.T, keys *seat.Keys, s *Service, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	RegisterRoutes(router, s, keys, middleware.NewRateLimiter())
	req := httptest.NewRequest(http.MethodPost, "/rooms/standup/transcript", strings.NewReader(body))
	if key != "" {
		req.Header.Set(seat.Header, key)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestTheSpeakerIsTheKeyHoldersAndNeverTheBody(t *testing.T) {
	db := testdb.Migrated(t)
	s := NewService(db, nil)
	keys := seat.NewKeys("secret")
	call := seedCall(t, db, "standup", "guest-ab12cd34", ready())

	if rec := post(t, keys, s, "", `{"text":"bonjour","start_ms":0,"end_ms":900}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no key = %d, want 401", rec.Code)
	}
	if rec := post(t, keys, s, "1.guest-ab12cd34.bad", `{"text":"bonjour","start_ms":0,"end_ms":900}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged key = %d, want 401", rec.Code)
	}
	key := keys.Sign("standup", "guest-ab12cd34")
	if rec := post(t, keys, s, key, `{"speaker":"user-1","text":"bonjour","start_ms":0,"end_ms":900}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("a body naming a speaker = %d, want 400", rec.Code)
	}
	if rec := post(t, keys, s, key, `{"text":"bonjour","start_ms":0,"end_ms":900}`); rec.Code != http.StatusNoContent {
		t.Fatalf("valid key = %d %s", rec.Code, rec.Body.String())
	}
	var line schemas.TranscriptLine
	if err := db.Where("call_id = ?", call.ID).First(&line).Error; err != nil || line.Speaker != "guest-ab12cd34" {
		t.Fatalf("speaker = %q, %v, want the key holder", line.Speaker, err)
	}
	if rec := post(t, keys, s, key, strings.Repeat("x", maxBodyBytes+1)); rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized body = %d, want 400", rec.Code)
	}
}

func TestLinesAreRateLimitedPerSeat(t *testing.T) {
	db := testdb.Migrated(t)
	keys := seat.NewKeys("secret")
	seedCall(t, db, "standup", "guest-ab12cd34", ready())
	router := chi.NewRouter()
	RegisterRoutes(router, NewService(db, nil), keys, middleware.NewRateLimiter())
	key := keys.Sign("standup", "guest-ab12cd34")

	refused := 0
	for range linesPerMinute + 10 {
		req := httptest.NewRequest(http.MethodPost, "/rooms/standup/transcript", strings.NewReader(`{"text":"a","start_ms":0,"end_ms":1}`))
		req.Header.Set(seat.Header, key)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			refused++
		}
	}
	if refused == 0 {
		t.Fatal("a seat sent more than a minute's budget of lines without being refused")
	}
}
