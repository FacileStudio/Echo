package integrations

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/livekit/protocol/livekit"
)

type received struct {
	body      []byte
	signature string
}

func callbackServer(t *testing.T, failures int) (*httptest.Server, chan received) {
	t.Helper()
	got := make(chan received, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if failures > 0 {
			failures--
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		got <- received{body: body, signature: r.Header.Get(SignatureHeader)}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	return server, got
}

func joinedEvent(room string) *livekit.WebhookEvent {
	return &livekit.WebhookEvent{
		Event:       "participant_joined",
		Room:        &livekit.Room{Name: room, Sid: "RM_1"},
		Participant: &livekit.ParticipantInfo{Identity: "@alice:prisme#a1b2c3d4", Name: "Alice"},
		CreatedAt:   1_790_000_000,
	}
}

func TestAForwardedEventIsSignedWithTheAppSecret(t *testing.T) {
	h := newHarness(t)
	server, got := callbackServer(t, 1)
	room := h.call(http.MethodPost, "/integrations/rooms", prismeSecret,
		CreateRoomRequest{ExternalID: "!abc:prisme", CallbackURL: server.URL})
	roomID := decode[RoomCreated](t, room).RoomID
	forwarder := NewForwarder(h.db, testKeys, quietLog())
	forwarder.retry = 0

	out, err := forwarder.prepare(context.Background(), joinedEvent(roomID))
	if err != nil || out == nil {
		t.Fatalf("prepare: %v, %v", out, err)
	}
	if !forwarder.deliver(context.Background(), out) {
		t.Fatal("delivery failed after a retry")
	}
	delivered := <-got
	mac := hmac.New(sha256.New, []byte(prismeSecret))
	mac.Write(delivered.body)
	if want := hex.EncodeToString(mac.Sum(nil)); delivered.signature != want {
		t.Fatalf("signature %q, want %q", delivered.signature, want)
	}
	checkPayload(t, delivered.body, roomID)
}

func checkPayload(t *testing.T, body []byte, roomID string) {
	t.Helper()
	var event CallbackEvent
	if err := json.Unmarshal(body, &event); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if event.Event != "participant_joined" || event.RoomID != roomID || event.ExternalID != "!abc:prisme" {
		t.Fatalf("payload %+v", event)
	}
	if event.Participant == nil || event.Participant.Identity != "@alice:prisme" || event.At != "2026-09-21T14:13:20Z" {
		t.Fatalf("payload %+v", event)
	}
}

func TestOnlyIntegrationRoomsWithACallbackAreForwarded(t *testing.T) {
	h := newHarness(t)
	silent := h.createRoom(prismeSecret, "no-callback")
	forwarder := NewForwarder(h.db, testKeys, quietLog())
	for _, event := range []*livekit.WebhookEvent{joinedEvent(silent.RoomID), joinedEvent("someone-elses-room")} {
		if out, err := forwarder.prepare(context.Background(), event); err != nil || out != nil {
			t.Fatalf("room %s: prepared %+v, %v; want nothing", event.Room.Name, out, err)
		}
	}
	h.call(http.MethodPost, "/integrations/rooms", prismeSecret,
		CreateRoomRequest{ExternalID: "no-callback", CallbackURL: "https://prisme.test/events"})
	agent := joinedEvent(silent.RoomID)
	agent.Participant.Identity = "agent-AJ_x"
	if out, _ := forwarder.prepare(context.Background(), agent); out != nil {
		t.Fatal("the transcriber agent was forwarded as a participant")
	}
	started := joinedEvent(silent.RoomID)
	started.Event = "room_started"
	if out, _ := forwarder.prepare(context.Background(), started); out != nil {
		t.Fatal("room_started was forwarded")
	}
}
