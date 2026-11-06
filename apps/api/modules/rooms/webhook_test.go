package rooms

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
)

func joined(s *Service, live *fakeLive, slug, identity string, at time.Time) {
	live.participants = append(live.participants, identity)
	live.joined[identity] = at
	s.Notify(context.Background(), &livekit.WebhookEvent{
		Event:       "participant_joined",
		Room:        &livekit.Room{Name: slug},
		Participant: &livekit.ParticipantInfo{Identity: identity},
	})
}

func connectedNow(live *fakeLive) []string {
	return slices.DeleteFunc(slices.Clone(live.participants), func(identity string) bool {
		return slices.Contains(live.removed, identity)
	})
}

func TestAReconnectingBotCannotRefillTheOwnersSeat(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-webhook")
	pushPlan(t, s, "sub-webhook", capAt(`3`))
	openCallWith(t, s, "seat", owner)
	live := &fakeLive{joined: map[string]time.Time{}}
	s.live = live
	start := time.Now().Add(-time.Hour)

	joined(s, live, "seat", "guest-1", start)
	joined(s, live, "seat", "guest-2", start.Add(time.Minute))
	if len(live.removed) != 0 {
		t.Fatalf("guests within limit-1 were disconnected: %v", live.removed)
	}
	joined(s, live, "seat", "guest-bot", start.Add(2*time.Minute))
	if !slices.Equal(live.removed, []string{"guest-bot"}) {
		t.Fatalf("removed %v, want the guest that took the absent owner's seat", live.removed)
	}
	joined(s, live, "seat", "user-50", start.Add(3*time.Minute))
	if !slices.Equal(live.removed, []string{"guest-bot", "guest-2"}) {
		t.Fatalf("removed %v, want the newest guest before a signed-in member", live.removed)
	}
	joined(s, live, "seat", "user-"+strconv.FormatInt(owner, 10), start.Add(4*time.Minute))
	joined(s, live, "seat", "guest-3", start.Add(5*time.Minute))
	if len(live.removed) != 2 || len(connectedNow(live)) != 4 {
		t.Fatalf("with the owner present Echo disconnected %v; LiveKit's ceiling is the only limit then", live.removed)
	}
}

func TestTheOwnerSeatLeavesOtherRoomsAlone(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-other")
	pushPlan(t, s, "sub-other", capAt(`2`))
	if _, err := s.Create(context.Background(), "unowned", "unowned", nil); err != nil {
		t.Fatalf("create: %v", err)
	}
	app := schemas.Room{ID: uuid.New(), Slug: "prisme-x", Name: "x", OwnerID: &owner, IntegrationApp: "prisme"}
	if err := s.orm.Create(&app).Error; err != nil {
		t.Fatalf("integration room: %v", err)
	}
	live := &fakeLive{joined: map[string]time.Time{}}
	s.live = live
	for _, slug := range []string{"unowned", "prisme-x"} {
		live.participants, live.removed = nil, nil
		for i := range 4 {
			joined(s, live, slug, "guest-"+strconv.Itoa(i), time.Now())
		}
		if len(live.removed) != 0 {
			t.Fatalf("%s: disconnected %v from a room no owner seat applies to", slug, live.removed)
		}
	}
}

func TestAnUnlimitedRoomKeepsNoOwnerSeat(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-paid")
	pushPlan(t, s, "sub-paid", map[string]json.RawMessage{"echo.max_participants": json.RawMessage(`-1`)})
	if _, err := s.Create(context.Background(), "paid", "paid", &owner); err != nil {
		t.Fatalf("create: %v", err)
	}
	live := &fakeLive{joined: map[string]time.Time{}}
	s.live = live
	for i := range 4 {
		joined(s, live, "paid", "guest-"+strconv.Itoa(i), time.Now())
	}
	if len(live.removed) != 0 {
		t.Fatalf("an unlimited room disconnected %v", live.removed)
	}
}
