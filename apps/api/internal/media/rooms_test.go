package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/livekit/protocol/auth"
)

func TestEnsureRoomSendsTheCeilingAndReadsTheSession(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		if r.URL.Path != "/twirp/livekit.RoomService/CreateRoom" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if _, err := io.WriteString(w, `{"sid":"RM_x","name":"standup","creationTime":"1793790000","futureField":1}`); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	defer server.Close()
	s := &Service{apiKey: "key", apiSecret: "secret", url: server.URL}
	room, err := s.EnsureRoom(context.Background(), "standup", RoomSettings{MaxParticipants: 4})
	if err != nil {
		t.Fatalf("EnsureRoom: %v", err)
	}
	if room.SID != "RM_x" || !room.CreatedAt.Equal(time.Unix(1793790000, 0)) {
		t.Fatalf("room = %+v", room)
	}
	if !strings.Contains(body, `"max_participants":4`) || !strings.Contains(body, `"empty_timeout":120`) {
		t.Fatalf("CreateRoom body = %s", body)
	}
	if err := s.EndRoom(context.Background(), "standup"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a 404 answered %v, want ErrNotFound", err)
	}
}

func TestAJoinTokenLivesTenMinutes(t *testing.T) {
	s := &Service{apiKey: "key", apiSecret: "secret", url: "ws://localhost"}
	token, err := s.Issue("room", "guest-1", "Guest", Grant{CanSubscribe: true})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	parsed, err := auth.ParseAPIToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	claims, _, err := parsed.Verify("secret")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if time.Until(claims.ExpiresAt.Time) > TokenTTL {
		t.Fatalf("token lives until %s, longer than %s", claims.ExpiresAt.Time, TokenTTL)
	}
}

func TestAnUnlimitedRoomSendsAnExplicitCeiling(t *testing.T) {
	for limit, want := range map[int64]uint32{-1: unlimitedParticipants, 0: unlimitedParticipants, 4: 4, 1 << 40: unlimitedParticipants} {
		if got := (RoomSettings{MaxParticipants: limit}).ceiling(); got != want {
			t.Errorf("ceiling for %d = %d, want %d: 0 would keep a lower ceiling after an upgrade", limit, got, want)
		}
	}
}
