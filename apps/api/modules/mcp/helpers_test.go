package mcp

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/modules/history"
	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte"
	"github.com/FacileStudio/porte/kit"
	"github.com/FacileStudio/porte/mcpauth"
	troncerrors "github.com/FacileStudio/tronc/errors"
	mcpkit "github.com/FacileStudio/tronc/mcp"
	"github.com/FacileStudio/tronc/mcp/mcptest"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

// identities resolves each test subject to its user.
type identities struct {
	porte.IdentityStore
	users map[string]int64
}

// Find resolves a known subject.
func (s identities) Find(_ context.Context, provider, sub string) (porte.StoredIdentity, error) {
	id, ok := s.users[sub]
	if !ok {
		return porte.StoredIdentity{}, porte.ErrNotFound
	}
	return porte.StoredIdentity{UserID: id, Provider: provider, Subject: sub}, nil
}

// accounts resolves a user row the way kit.Store does: a missing row is
// refused.
type accounts struct {
	db *gorm.DB
}

// Identity is the kit's lookup.
func (a accounts) Identity(ctx context.Context, userID int64) (kit.Identity, error) {
	var user schemas.User
	if err := a.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return kit.Identity{}, troncerrors.Unauthorized("account not found")
	}
	return kit.Identity{UserID: user.ID, Email: user.Email, Name: user.Name}, nil
}

// fixture is the MCP endpoint over Echo's real rooms and history services:
// an owner, a teammate who sat in the owner's call, and a stranger.
type fixture struct {
	db       *gorm.DB
	issuer   *mcptest.Issuer
	resource string
	users    map[string]int64
	service  *Service
	router   chi.Router
}

// newFixture starts the endpoint and the confirm page routes.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	t.Setenv("LIVEKIT_API_KEY", "devkey")
	t.Setenv("LIVEKIT_API_SECRET", "secret")
	t.Setenv("LIVEKIT_URL", "ws://localhost:7880")
	db := testdb.Migrated(t)
	f := &fixture{db: db, issuer: mcptest.NewIssuer(t), users: map[string]int64{}}
	for i, name := range []string{"owner", "teammate", "stranger"} {
		user := schemas.User{ID: int64(i + 1), Email: name + "@example.test", Name: name}
		must(t, db.Create(&user).Error)
		f.users[name] = user.ID
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.router.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	f.resource = srv.URL + "/mcp"
	f.service = f.build(t)
	f.router = chi.NewRouter()
	RegisterRoutes(f.router, f.service)
	f.router.Route("/api", func(r chi.Router) { RegisterPendingRoutes(r, f.service, signedIn) })
	return f
}

// build makes the MCP server over the fixture's database and issuer.
func (f *fixture) build(t *testing.T) *Service {
	t.Helper()
	mediaService, err := media.NewServiceFromEnv()
	must(t, err)
	roomService := rooms.NewService(f.db, mediaService, plans.New(f.db, "", nil), nil)
	verifier, err := mcpauth.Verifier(context.Background(), mcpauth.Config{
		Issuer: f.issuer.URL, Resource: f.resource, ScopePrefix: "echo", Identities: identities{users: f.users},
	})
	must(t, err)
	sqlDB, err := f.db.DB()
	must(t, err)
	service, err := NewService(Deps{
		Rooms: roomService, History: history.NewService(f.db, roomService, nil, t.TempDir()),
		Accounts: accounts{db: f.db}, Limiter: middleware.NewRateLimiter(),
	}, Config{
		Resource: f.resource, Issuer: f.issuer.URL, Version: "test", StateKey: []byte("state-key"),
		Verifier: verifier, Pending: mcpkit.PostgresPending{DB: sqlDB}, PerMinute: 100_000,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	must(t, err)
	return service
}

// token is a token for the owner with scopes, or both when none is given.
func (f *fixture) token(t testing.TB, scopes ...string) string {
	if len(scopes) == 0 {
		scopes = []string{"echo.read", "echo.write"}
	}
	return f.issuer.Token(t, f.resource, "owner", scopes...)
}

// connect opens a session for subject with both scopes.
func (f *fixture) connect(t *testing.T, subject string) *sdk.ClientSession {
	t.Helper()
	return mcptest.Connect(t, f.resource, f.issuer.Token(t, f.resource, subject, "echo.read", "echo.write"), mcptest.ClientOptions{})
}

// room creates a room owned by owner.
func (f *fixture) room(t testing.TB, slug, owner string) schemas.Room {
	t.Helper()
	ownerID := f.users[owner]
	room := schemas.Room{ID: uuid.New(), Slug: slug, Name: strings.ToUpper(slug), OwnerID: &ownerID}
	must(t, f.db.Create(&room).Error)
	return room
}

// meeting records a finished call in room that teammate and a guest sat in,
// with three transcript lines.
func (f *fixture) meeting(t testing.TB, room schemas.Room) schemas.Call {
	t.Helper()
	start := time.Now().UTC().Add(-time.Hour)
	end := start.Add(30 * time.Minute)
	call := schemas.Call{ID: uuid.New(), RoomID: room.ID, StartedAt: start, EndedAt: &end, LivekitRoomName: room.Slug}
	must(t, f.db.Create(&call).Error)
	teammate := f.users["teammate"]
	for _, seat := range []schemas.CallParticipant{
		{CallID: call.ID, Identity: "user-" + strconv.FormatInt(f.users["owner"], 10), UserID: ptr(f.users["owner"]), Name: "owner", JoinedAt: start},
		{CallID: call.ID, Identity: "user-" + strconv.FormatInt(teammate, 10), UserID: &teammate, Name: "teammate", JoinedAt: start},
		{CallID: call.ID, Identity: "guest-1a2b3c4d", Name: "Visitor", JoinedAt: start},
	} {
		must(t, f.db.Create(&seat).Error)
	}
	for i, line := range []string{"Welcome everyone", "The budget is approved", "See you next week"} {
		must(t, f.db.Create(&schemas.TranscriptLine{CallID: call.ID, Speaker: "guest-1a2b3c4d", Text: line, OffsetMS: int64(i) * 61_000}).Error)
	}
	return call
}

// exists reports 1 while the room is stored, 0 once it is gone.
func (f *fixture) exists(slug string) int {
	var count int64
	f.db.Model(&schemas.Room{}).Where("slug = ?", slug).Count(&count)
	return int(count)
}

// call runs a tool and decodes its result, failing on a tool error.
func call(t *testing.T, session *sdk.ClientSession, tool string, args map[string]any, out any) {
	t.Helper()
	result := mcptest.Call(t, session, tool, args)
	if result.IsError {
		t.Fatalf("%s: %s", tool, mcptest.Text(result))
	}
	mcptest.Decode(t, result, out)
}

// refused runs a tool and returns its error text, failing when it worked.
func refused(t *testing.T, session *sdk.ClientSession, tool string, args map[string]any) string {
	t.Helper()
	result := mcptest.Call(t, session, tool, args)
	if !result.IsError {
		t.Fatalf("%s must be refused, got %s", tool, mcptest.Text(result))
	}
	return mcptest.Text(result)
}

// signedIn stands in for porte on the confirm page: X-Test-User is the
// signed-in user, and X-Test-Actor makes the request a delegated token.
func signedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		if err != nil {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		identity := porte.Identity{UserID: id, SessionID: 1, Actor: r.Header.Get("X-Test-Actor")}
		if identity.Actor != "" {
			identity.SessionID = 0
		}
		next.ServeHTTP(w, r.WithContext(porte.WithIdentity(r.Context(), identity)))
	})
}

// onPage calls a confirm page route as user, through a delegated token when
// actor is set.
func (f *fixture) onPage(method, path string, user int64, actor string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/mcp/pending"+path, nil)
	req.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
	if actor != "" {
		req.Header.Set("X-Test-Actor", actor)
	}
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, req)
	return recorder
}

// ptr is a pointer to v.
func ptr[T any](v T) *T {
	return &v
}

// must fails the test on err.
func must(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
