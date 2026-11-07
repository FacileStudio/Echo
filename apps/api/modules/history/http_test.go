package history

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/authcontext"
	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/directory"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

const sso = "https://sso.example"

func actingAs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		if err != nil {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(authcontext.With(r.Context(), authcontext.Identity{UserID: id})))
	})
}

func newPlannedService(t *testing.T, root string) (*Service, *gorm.DB, int64) {
	t.Helper()
	t.Setenv("LIVEKIT_API_KEY", "devkey")
	t.Setenv("LIVEKIT_API_SECRET", "secret")
	t.Setenv("LIVEKIT_URL", "ws://localhost:7880")
	mediaService, err := media.NewServiceFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db := testdb.Migrated(t)
	planner := plans.New(db, sso, nil)
	pro := seedUser(t, db, "pro@example.test", "Pro", "")
	err = db.Exec(`INSERT INTO porte_identities (user_id, provider, subject) VALUES (?, ?, 'sub-pro')`, pro, sso).Error
	if err != nil {
		t.Fatal(err)
	}
	snapshot := directory.Snapshot{Version: 1, EntitlementsRevision: 1, People: []directory.Person{{
		Subject:      "sub-pro",
		Entitlements: map[string]json.RawMessage{plans.KeyRecording: json.RawMessage(`true`), plans.KeyAIAccess: json.RawMessage(`true`)},
	}}}
	if err := planner.Apply(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	return NewService(db, rooms.NewService(db, mediaService, planner, nil), nil, root), db, pro
}

type request struct {
	method, target, user string
	body                 []byte
	header               http.Header
}

func serve(t *testing.T, s *Service, c request) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	RegisterRoutes(router, s, actingAs)
	req := httptest.NewRequest(c.method, c.target, bytes.NewReader(c.body))
	for key, values := range c.header {
		req.Header[key] = values
	}
	if c.user != "" {
		req.Header.Set("X-Test-User", c.user)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func get(t *testing.T, s *Service, target, user string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, s, request{method: http.MethodGet, target: target, user: user})
}

func patch(t *testing.T, s *Service, target, user, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, s, request{method: http.MethodPatch, target: target, user: user, body: []byte(body)})
}

func TestRecordingStreamsWithRangesAndOnlyToThoseAllowed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.mp4"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, db, pro := newPlannedService(t, root)
	bob := seedUser(t, db, "bob@example.test", "Bob", "")
	stranger := seedUser(t, db, "eve@example.test", "Eve", "")
	call := seedRoomCall(t, db, "standup", &pro, time.Now().UTC())
	seat(t, db, call, bob, "Bob")
	rec := seedRecording(t, db, call.ID, "a.mp4", false)
	file := "/recordings/" + rec.ID.String() + "/file"

	ranged := serve(t, s, request{method: http.MethodGet, target: file, user: strconv.FormatInt(pro, 10), header: http.Header{"Range": {"bytes=2-5"}}})
	if ranged.Code != http.StatusPartialContent || ranged.Body.String() != "2345" {
		t.Fatalf("range = %d %q, want 206 and the four bytes", ranged.Code, ranged.Body.String())
	}
	if got := ranged.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := ranged.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "inline;") {
		t.Fatalf("Content-Disposition = %q, want inline for playback", got)
	}
	download := get(t, s, file+"?download=1", strconv.FormatInt(pro, 10))
	if got := download.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment;") {
		t.Fatalf("Content-Disposition = %q, want attachment", got)
	}

	if anon := get(t, s, file, ""); anon.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d, want 401", anon.Code)
	}
	if eve := get(t, s, file, strconv.FormatInt(stranger, 10)); eve.Code != http.StatusForbidden {
		t.Fatalf("a stranger = %d, want 403", eve.Code)
	}
	if hidden := get(t, s, file, strconv.FormatInt(bob, 10)); hidden.Code != http.StatusForbidden {
		t.Fatalf("an unshared recording for a participant = %d, want 403", hidden.Code)
	}
}

func TestOwnerSharesARecordingWithTheParticipants(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.mp4"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, db, pro := newPlannedService(t, root)
	bob := seedUser(t, db, "bob@example.test", "Bob", "")
	call := seedRoomCall(t, db, "standup", &pro, time.Now().UTC())
	seat(t, db, call, bob, "Bob")
	rec := seedRecording(t, db, call.ID, "a.mp4", false)
	target := "/recordings/" + rec.ID.String()
	owner, guest := strconv.FormatInt(pro, 10), strconv.FormatInt(bob, 10)

	if refused := patch(t, s, target, guest, `{"shared":true}`); refused.Code != http.StatusForbidden {
		t.Fatalf("a participant sharing = %d, want 403", refused.Code)
	}
	if shared := patch(t, s, target, owner, `{"shared":true}`); shared.Code != http.StatusOK {
		t.Fatalf("owner sharing = %d %s", shared.Code, shared.Body)
	}
	seen := get(t, s, target+"/file", guest)
	body, _ := io.ReadAll(seen.Body)
	if seen.Code != http.StatusOK || string(body) != "0123456789" {
		t.Fatalf("a shared recording = %d %q", seen.Code, body)
	}
	if off := patch(t, s, target, owner, `{"shared":false}`); off.Code != http.StatusOK {
		t.Fatalf("unsharing = %d", off.Code)
	}
	if again := get(t, s, target+"/file", guest); again.Code != http.StatusForbidden {
		t.Fatalf("after unsharing = %d, want 403", again.Code)
	}
}

func TestSharingNeedsTheRecordingEntitlementButUnsharingDoesNot(t *testing.T) {
	s, db, _ := newPlannedService(t, t.TempDir())
	free := seedUser(t, db, "free@example.test", "Free", "")
	call := seedRoomCall(t, db, "standup", &free, time.Now().UTC())
	rec := seedRecording(t, db, call.ID, "a.mp4", true)
	target := "/recordings/" + rec.ID.String()
	owner := strconv.FormatInt(free, 10)

	if on := patch(t, s, target, owner, `{"shared":true}`); on.Code != http.StatusPaymentRequired {
		t.Fatalf("sharing on a plan without echo.recording = %d, want 402", on.Code)
	}
	if off := patch(t, s, target, owner, `{"shared":false}`); off.Code != http.StatusOK {
		t.Fatalf("unsharing = %d, want 200 whatever the plan", off.Code)
	}
}

func TestTranscriptsAndSummariesNeedAIAccess(t *testing.T) {
	s, db, pro := newPlannedService(t, t.TempDir())
	free := seedUser(t, db, "free@example.test", "Free", "")
	paid := seedRoomCall(t, db, "paid", &pro, time.Now().UTC())
	unpaid := seedRoomCall(t, db, "unpaid", &free, time.Now().UTC())
	seedLines(t, s, paid.ID, schemas.TranscriptLine{Speaker: "user-1", Text: "salut", OffsetMS: 1000})
	seedLines(t, s, unpaid.ID, schemas.TranscriptLine{Speaker: "user-1", Text: "salut", OffsetMS: 1000})
	proUser, freeUser := strconv.FormatInt(pro, 10), strconv.FormatInt(free, 10)

	if ok := get(t, s, "/calls/"+paid.ID.String()+"/transcript", proUser); ok.Code != http.StatusOK {
		t.Fatalf("a plan with ai.access = %d, want 200", ok.Code)
	}
	for _, target := range []string{"/transcript", "/transcript?format=txt"} {
		if refused := get(t, s, "/calls/"+unpaid.ID.String()+target, freeUser); refused.Code != http.StatusPaymentRequired ||
			!strings.Contains(refused.Body.String(), "ai.access") {
			t.Fatalf("%s without ai.access = %d %s, want 402 naming ai.access", target, refused.Code, refused.Body)
		}
	}
	summary := serve(t, s, request{method: http.MethodPost, target: "/calls/" + unpaid.ID.String() + "/summary", user: freeUser})
	if summary.Code != http.StatusPaymentRequired {
		t.Fatalf("summary without ai.access = %d, want 402", summary.Code)
	}
}
