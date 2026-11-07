package capture

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/authcontext"
	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/internal/seat"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte"
	"github.com/FacileStudio/porte/directory"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const issuer = "https://sso.example.test"

type world struct {
	t      *testing.T
	db     *gorm.DB
	svc    *Service
	keys   *seat.Keys
	dir    string
	pro    int64
	free   int64
	slug   string
	call   schemas.Call
	guest  string
	people []directory.Person
}

func newWorld(t *testing.T) *world {
	t.Helper()
	t.Setenv("LIVEKIT_API_KEY", "devkey")
	t.Setenv("LIVEKIT_API_SECRET", "secret")
	t.Setenv("LIVEKIT_URL", "ws://localhost:7880")
	mediaService, err := media.NewServiceFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db := testdb.Migrated(t)
	planner := plans.New(db, issuer, nil)
	w := &world{t: t, db: db, dir: t.TempDir(), slug: "standup", guest: "guest-ab12cd34"}
	w.pro = w.user("pro", map[string]json.RawMessage{plans.KeyRecording: json.RawMessage(`true`), plans.KeyAIAccess: json.RawMessage(`true`)})
	w.free = w.user("free", nil)
	snapshot := directory.Snapshot{Version: 1, EntitlementsRevision: 1, People: w.people}
	if err := planner.Apply(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	w.keys = seat.NewKeys("secret")
	w.svc = NewService(db, rooms.NewService(db, mediaService, planner, nil), w.keys, middleware.NewRateLimiter(), w.dir)
	return w
}

func (w *world) user(subject string, entitlements map[string]json.RawMessage) int64 {
	w.t.Helper()
	user := schemas.User{Email: subject + "@example.test", Name: subject}
	if err := w.db.Create(&user).Error; err != nil {
		w.t.Fatal(err)
	}
	err := w.db.Exec(`INSERT INTO porte_identities (user_id, provider, subject) VALUES (?, ?, ?)`, user.ID, issuer, subject).Error
	if err != nil {
		w.t.Fatal(err)
	}
	w.people = append(w.people, directory.Person{Subject: subject, Entitlements: entitlements})
	return user.ID
}

func (w *world) open(owner int64, seated ...string) {
	w.t.Helper()
	room := schemas.Room{ID: uuid.New(), Slug: w.slug, Name: w.slug, OwnerID: &owner}
	if err := w.db.Create(&room).Error; err != nil {
		w.t.Fatal(err)
	}
	w.call = schemas.Call{ID: uuid.New(), RoomID: room.ID, StartedAt: time.Now().UTC(), LivekitRoomName: w.slug, LivekitRoomSID: "RM_1"}
	if err := w.db.Create(&w.call).Error; err != nil {
		w.t.Fatal(err)
	}
	for _, identity := range seated {
		row := schemas.CallParticipant{CallID: w.call.ID, Identity: identity, Name: identity, JoinedAt: time.Now().UTC()}
		if err := w.db.Create(&row).Error; err != nil {
			w.t.Fatal(err)
		}
	}
}

type call struct {
	method, path string
	session      int64
	guest        string
	body         string
	raw          []byte
}

func (w *world) do(c call) *httptest.ResponseRecorder {
	w.t.Helper()
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if id, err := strconv.ParseInt(r.Header.Get("X-Test-Session"), 10, 64); err == nil {
				ctx := porte.WithIdentity(r.Context(), porte.Identity{UserID: id})
				r = r.WithContext(authcontext.With(ctx, authcontext.Identity{UserID: id}))
			}
			next.ServeHTTP(rw, r)
		})
	})
	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if _, ok := authcontext.From(r.Context()); !ok {
				http.Error(rw, "unauthenticated", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(rw, r)
		})
	}
	RegisterRoutes(router, w.svc, requireAuth)
	body := c.raw
	if c.body != "" {
		body = []byte(c.body)
	}
	req := httptest.NewRequest(c.method, "/rooms/"+w.slug+"/capture"+c.path, bytes.NewReader(body))
	if c.session != 0 {
		req.Header.Set("X-Test-Session", strconv.FormatInt(c.session, 10))
	}
	if c.guest != "" {
		req.Header.Set(seat.Header, w.keys.Sign(w.slug, c.guest))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func (w *world) status(c call, want int) *httptest.ResponseRecorder {
	w.t.Helper()
	rec := w.do(c)
	if rec.Code != want {
		w.t.Fatalf("%s %s = %d %s, want %d", c.method, c.path, rec.Code, rec.Body.String(), want)
	}
	return rec
}

func (w *world) state(c call) State {
	w.t.Helper()
	c.method = http.MethodGet
	var out State
	if err := json.Unmarshal(w.status(c, http.StatusOK).Body.Bytes(), &out); err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *world) owner() call { return call{session: w.pro} }

func put(c call, body string) call  { c.method, c.body = http.MethodPut, body; return c }
func with(c call, path string) call { c.path = path; return c }
