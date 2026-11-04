package integrations

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
)

type fakeRooms struct {
	created []string
	fail    bool
}

func (f *fakeRooms) EnsureRoom(_ context.Context, name string, settings media.RoomSettings) (media.LiveRoom, error) {
	if f.fail {
		return media.LiveRoom{}, errors.New("livekit down")
	}
	if settings.MaxParticipants != 0 {
		return media.LiveRoom{}, errors.New("an integration room got a participant ceiling")
	}
	f.created = append(f.created, name)
	return media.LiveRoom{Name: name}, nil
}

func TestATokenCreatesTheLiveKitRoomFirst(t *testing.T) {
	h := newHarness(t)
	room := h.createRoom(prismeSecret, "!provision:prisme")
	path := "/integrations/rooms/" + room.RoomID + "/tokens"
	if got := h.call(http.MethodPost, path, prismeSecret, TokenRequest{Identity: "@a:prisme", Name: "A"}); got.Code != http.StatusOK {
		t.Fatalf("status %d: %s", got.Code, got.Body.String())
	}
	if len(h.rooms.created) != 1 || h.rooms.created[0] != room.RoomID {
		t.Fatalf("created rooms %v, want [%s]: with auto_create off the token would join nothing", h.rooms.created, room.RoomID)
	}
	h.rooms.fail = true
	if got := h.call(http.MethodPost, path, prismeSecret, TokenRequest{Identity: "@a:prisme", Name: "A"}); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d with LiveKit down, want 503", got.Code)
	}
}
