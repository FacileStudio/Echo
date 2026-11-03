package integrations

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
)

func TestCreatingTheSameExternalIDTwiceReturnsOneRoom(t *testing.T) {
	h := newHarness(t)
	first := h.call(http.MethodPost, "/integrations/rooms", prismeSecret,
		CreateRoomRequest{ExternalID: "!abc:prisme", Title: "#general"})
	second := h.call(http.MethodPost, "/integrations/rooms", prismeSecret,
		CreateRoomRequest{ExternalID: "!abc:prisme", Title: "#general"})
	if first.Code != http.StatusCreated || second.Code != http.StatusOK {
		t.Fatalf("statuses %d then %d, want 201 then 200", first.Code, second.Code)
	}
	a, b := decode[RoomCreated](t, first), decode[RoomCreated](t, second)
	if a.RoomID != b.RoomID || !strings.HasPrefix(a.RoomID, "prisme-") {
		t.Fatalf("room ids %q and %q, want one prisme- room", a.RoomID, b.RoomID)
	}
	if a.JoinURL != "http://example.com/embed/"+a.RoomID {
		t.Fatalf("join_url = %q", a.JoinURL)
	}
}

func TestTwoAppsWithTheSameExternalIDGetTwoRooms(t *testing.T) {
	h := newHarness(t)
	if h.createRoom(prismeSecret, "shared").RoomID == h.createRoom(otherSecret, "shared").RoomID {
		t.Fatal("two apps share one room for the same external id")
	}
}

func TestARepeatedCreateUpdatesTheTitle(t *testing.T) {
	h := newHarness(t)
	room := h.createRoom(prismeSecret, "!abc:prisme")
	h.call(http.MethodPost, "/integrations/rooms", prismeSecret, CreateRoomRequest{ExternalID: "!abc:prisme", Title: "#renamed"})
	got := decode[RoomState](t, h.call(http.MethodGet, "/integrations/rooms/"+room.RoomID, prismeSecret, nil))
	if got.Title != "#renamed" {
		t.Fatalf("title = %q, want #renamed", got.Title)
	}
}

func TestCreateRejectsABadCallbackURL(t *testing.T) {
	h := newHarness(t)
	got := h.call(http.MethodPost, "/integrations/rooms", prismeSecret,
		CreateRoomRequest{ExternalID: "x", CallbackURL: "ftp://nope"})
	if got.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", got.Code)
	}
}

func TestAnotherAppCannotSeeTheRoom(t *testing.T) {
	h := newHarness(t)
	room := h.createRoom(prismeSecret, "private")
	if got := h.call(http.MethodGet, "/integrations/rooms/"+room.RoomID, otherSecret, nil); got.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404 for another app's room", got.Code)
	}
}

func TestThePublicGuestRouteCannotJoinAnIntegrationRoom(t *testing.T) {
	h := newHarness(t)
	room := h.createRoom(prismeSecret, "private")
	public := rooms.NewService(h.db, h.service.media)
	if _, err := public.Token(context.Background(), room.RoomID, nil, "", "Mallory"); err == nil {
		t.Fatal("a guest minted a token for an integration room")
	}
}
