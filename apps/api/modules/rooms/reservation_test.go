package rooms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/livekit/protocol/auth"
)

func TestGuestsLeaveOneSeatForTheAbsentOwner(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-reserve")
	pushPlan(t, s, "sub-reserve", capAt(`2`))
	openCallWith(t, s, "reserve", owner)
	s.live = &fakeLive{participants: []string{"guest-1"}}
	if err := guestJoin(s, "reserve"); !refusedOn(err, "echo.max_participants") {
		t.Fatalf("a guest took the absent owner's seat: %v", err)
	}
	s.live = &fakeLive{participants: []string{"guest-1", "user-" + strconv.FormatInt(owner, 10)}}
	if err := ownerJoin(s, "reserve", owner); err != nil {
		t.Fatalf("owner refused: %v", err)
	}
	s.live = &fakeLive{participants: []string{"user-" + strconv.FormatInt(owner, 10)}}
	if err := guestJoin(s, "reserve"); err != nil {
		t.Fatalf("with the owner present the last seat stayed reserved: %v", err)
	}
}

func TestTheOwnerJoinsARoomGuestsFilled(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-full")
	pushPlan(t, s, "sub-full", capAt(`3`))
	openCallWith(t, s, "full", owner)
	s.live = &fakeLive{participants: []string{"guest-1", "guest-2", "guest-3"}}
	if err := guestJoin(s, "full"); !refusedOn(err, "echo.max_participants") {
		t.Fatalf("a fourth guest = %v, want a refusal", err)
	}
	if err := ownerJoin(s, "full", owner); err != nil {
		t.Fatalf("the owner was locked out of a room guests filled: %v", err)
	}
}

func TestOneAddressHoldsOnePendingSeatPerRoom(t *testing.T) {
	s := newTestService(t)
	pushPlan(t, s, "someone-else", nil)
	for i := range 6 {
		if err := guestFrom(s, "lobby", "203.0.113.7"); err != nil {
			t.Fatalf("token %d from one address refused: %v", i, err)
		}
	}
	for i := range 3 {
		if err := guestFrom(s, "lobby", "198.51.100."+strconv.Itoa(i)); err != nil {
			t.Fatalf("guest from address %d refused: %v", i, err)
		}
	}
	if err := guestFrom(s, "lobby", "192.0.2.1"); !refusedOn(err, "echo.max_participants") {
		t.Fatalf("a fifth address = %v, want a refusal: one address held more than one seat", err)
	}
}

func TestAJoinRacingTheCutIsRefused(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-race")
	pushPlan(t, s, "someone-else", nil)
	if _, err := s.Create(context.Background(), "race", "race", &owner); err != nil {
		t.Fatalf("create: %v", err)
	}
	deadlines := s.deadlines.(*fakeDeadlines)
	s.live = &fakeLive{onEnsure: func() { deadlines.cuts = map[string]time.Time{"race": time.Now()} }}
	if err := guestJoin(s, "race"); !refusedOn(err, "echo.max_call_minutes") {
		t.Fatalf("a join the limiter cut mid-admission = %v, want a refusal", err)
	}
}

func deadlineRouter(t *testing.T, s *Service) (func(key string) *httptest.ResponseRecorder, *fakeLimits) {
	t.Helper()
	router := chi.NewRouter()
	limits := &fakeLimits{}
	RegisterRoutes(router, s, &stubResolver{}, func(next http.Handler) http.Handler { return next }, limits)
	return func(key string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/rooms/poll/deadline", nil)
		request.Header.Set("X-Echo-Deadline-Key", key)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder
	}, limits
}

func pollRoom(t *testing.T, s *Service) {
	t.Helper()
	owner := addOwner(t, s, "sub-poll")
	pushPlan(t, s, "someone-else", nil)
	if _, err := s.Create(context.Background(), "poll", "poll", &owner); err != nil {
		t.Fatalf("create: %v", err)
	}
}

func TestOnlyACallerAdmittedToTheRoomReadsItsDeadline(t *testing.T) {
	s := newTestService(t)
	pollRoom(t, s)
	poll, limits := deadlineRouter(t, s)
	keys := []string{
		"",
		s.DeadlineKey("other-room", time.Now().Add(time.Hour)),
		s.DeadlineKey("poll", time.Now().Add(-time.Second)),
		s.DeadlineKey("poll", time.Now().Add(deadlineKeyTTL+time.Hour)),
		"not-a-key",
	}
	for _, key := range keys {
		if got := poll(key); got.Code != http.StatusNotFound {
			t.Fatalf("a caller with key %q read this room's deadline: %d", key, got.Code)
		}
	}
	if limits.spent[0] != "deadline" {
		t.Fatalf("deadline polls spent %v, want the per-address deadline budget", limits.spent)
	}
}

func TestTheDeadlineRouteFollowsTheRunningSession(t *testing.T) {
	s := newTestService(t)
	pollRoom(t, s)
	created := time.Now().Add(-time.Minute).Truncate(time.Second).UTC()
	s.live = &fakeLive{room: dummyLive("RM_poll", created)}
	poll, _ := deadlineRouter(t, s)
	recorder := poll(s.DeadlineKey("poll", time.Now().Add(time.Hour)))
	want := created.Add(40 * time.Minute).Format(time.RFC3339)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), want) {
		t.Fatalf("deadline route = %d %s, want %s", recorder.Code, recorder.Body.String(), want)
	}
	pushPlan(t, s, "sub-poll", map[string]json.RawMessage{"echo.max_call_minutes": json.RawMessage(`-1`)})
	if body := poll(s.DeadlineKey("poll", time.Now().Add(time.Hour))).Body.String(); strings.Contains(body, "call_deadline") {
		t.Fatalf("an upgraded owner's call still reports a deadline: %s", body)
	}
}

func TestTheOwnerGetsIntoARoomLiveKitReportsFull(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-evict")
	pushPlan(t, s, "sub-evict", capAt(`3`))
	openCallWith(t, s, "evict", owner)
	start := time.Now().Add(-time.Hour)
	live := &fakeLive{
		participants: []string{"user-424242", "guest-old", "guest-new"},
		joined: map[string]time.Time{
			"user-424242": start.Add(30 * time.Minute), "guest-old": start, "guest-new": start.Add(10 * time.Minute),
		},
	}
	s.live = live
	if err := ownerJoin(s, "evict", owner); err != nil {
		t.Fatalf("owner refused: %v", err)
	}
	if len(live.removed) != 1 || live.removed[0] != "guest-new" {
		t.Fatalf("removed %v, want only the newest guest so LiveKit admits the owner", live.removed)
	}
	if err := ownerJoin(s, "evict", owner); err != nil || len(live.removed) != 1 {
		t.Fatalf("an owner rejoining a room with space disconnected someone: %v %v", live.removed, err)
	}
}

func TestAGuestTokenCannotBeHoarded(t *testing.T) {
	s := newTestService(t)
	admission, err := s.Token(context.Background(), JoinRequest{Slug: "hoard", DisplayName: "Guest"})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	parsed, err := auth.ParseAPIToken(admission.Token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	claims, _, err := parsed.Verify("secret")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if time.Until(claims.ExpiresAt.Time) > guestTokenTTL {
		t.Fatalf("a guest token lives until %s, longer than %s", claims.ExpiresAt.Time, guestTokenTTL)
	}
}
