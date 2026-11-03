package integrations

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
)

func TestATokenJoinsOnlyItsRoomWithAShortLife(t *testing.T) {
	h := newHarness(t)
	room := h.createRoom(prismeSecret, "!abc:prisme")
	got := h.call(http.MethodPost, "/integrations/rooms/"+room.RoomID+"/tokens", prismeSecret,
		TokenRequest{Identity: "@alice:prisme", Name: "Alice", AvatarURL: "https://prisme.test/a.png"})
	if got.Code != http.StatusOK {
		t.Fatalf("status %d: %s", got.Code, got.Body.String())
	}
	out := decode[TokenResponse](t, got)
	claims := verify(t, out.Token)
	if claims.Video.Room != room.RoomID || !claims.Video.RoomJoin || claims.Video.RoomAdmin {
		t.Fatalf("grant %+v, want a non-admin join on %s", claims.Video, room.RoomID)
	}
	if !strings.HasPrefix(claims.Identity, "@alice:prisme#") || AppIdentity(claims.Identity) != "@alice:prisme" {
		t.Fatalf("identity %q", claims.Identity)
	}
	if claims.Name != "Alice" || !strings.Contains(claims.Metadata, "https://prisme.test/a.png") {
		t.Fatalf("name %q, metadata %q", claims.Name, claims.Metadata)
	}
	if out.LivekitURL != "wss://live.example.test" {
		t.Fatalf("livekit_url %q", out.LivekitURL)
	}
	checkEmbedURL(t, out, room.RoomID)
}

func verify(t *testing.T, token string) *auth.ClaimGrants {
	t.Helper()
	parsed, err := auth.ParseAPIToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	registered, claims, err := parsed.Verify(livekitSecret)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if expiry := registered.ExpiresAt.Time; time.Until(expiry) > tokenTTL {
		t.Fatalf("token lives until %s, longer than %s", expiry, tokenTTL)
	}
	return claims
}

func checkEmbedURL(t *testing.T, out TokenResponse, roomID string) {
	t.Helper()
	address, fragment, _ := strings.Cut(out.EmbedURL, "#")
	values, err := url.ParseQuery(fragment)
	if err != nil || address != "http://example.com/embed/"+roomID {
		t.Fatalf("embed_url %q", out.EmbedURL)
	}
	if values.Get("token") != out.Token || values.Get("url") != out.LivekitURL {
		t.Fatalf("embed_url fragment %q does not carry the token and url", fragment)
	}
}

func TestTwoDevicesOfOnePersonGetDistinctLiveKitIdentities(t *testing.T) {
	h := newHarness(t)
	room := h.createRoom(prismeSecret, "!abc:prisme")
	path := "/integrations/rooms/" + room.RoomID + "/tokens"
	req := TokenRequest{Identity: "@alice:prisme", Name: "Alice"}
	a := verify(t, decode[TokenResponse](t, h.call(http.MethodPost, path, prismeSecret, req)).Token)
	b := verify(t, decode[TokenResponse](t, h.call(http.MethodPost, path, prismeSecret, req)).Token)
	if a.Identity == b.Identity {
		t.Fatalf("both devices got %q: LiveKit would disconnect the first", a.Identity)
	}
}

func TestATokenNeedsAnIdentityAndAName(t *testing.T) {
	h := newHarness(t)
	room := h.createRoom(prismeSecret, "!abc:prisme")
	path := "/integrations/rooms/" + room.RoomID + "/tokens"
	for _, req := range []TokenRequest{{Name: "Alice"}, {Identity: "@alice:prisme"}} {
		if got := h.call(http.MethodPost, path, prismeSecret, req); got.Code != http.StatusBadRequest {
			t.Fatalf("%+v: status %d, want 400", req, got.Code)
		}
	}
}

func TestAnotherAppCannotMintATokenForTheRoom(t *testing.T) {
	h := newHarness(t)
	room := h.createRoom(prismeSecret, "!abc:prisme")
	got := h.call(http.MethodPost, "/integrations/rooms/"+room.RoomID+"/tokens", otherSecret,
		TokenRequest{Identity: "x", Name: "X"})
	if got.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", got.Code)
	}
}

func TestParticipantsListTheOpenCallOncePerPerson(t *testing.T) {
	h := newHarness(t)
	room := h.createRoom(prismeSecret, "!abc:prisme")
	var stored schemas.Room
	h.db.Where("slug = ?", room.RoomID).First(&stored)
	now := time.Now().UTC()
	call := schemas.Call{ID: uuid.New(), RoomID: stored.ID, StartedAt: now, LivekitRoomName: room.RoomID, LivekitRoomSID: "RM_1"}
	h.db.Create(&call)
	h.db.Create(&[]schemas.CallParticipant{
		{CallID: call.ID, Identity: "@alice:prisme#aa", Name: "Alice", JoinedAt: now},
		{CallID: call.ID, Identity: "@alice:prisme#bb", Name: "Alice", JoinedAt: now.Add(time.Second)},
		{CallID: call.ID, Identity: "@bob:prisme#cc", Name: "Bob", JoinedAt: now, LeftAt: &now},
	})
	got := decode[RoomState](t, h.call(http.MethodGet, "/integrations/rooms/"+room.RoomID, prismeSecret, nil))
	if len(got.Participants) != 1 || got.Participants[0] != (Participant{Identity: "@alice:prisme", Name: "Alice"}) {
		t.Fatalf("participants %+v, want only Alice once", got.Participants)
	}
}
