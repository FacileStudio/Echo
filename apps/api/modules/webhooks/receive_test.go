package webhooks

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The bug that made this rewrite necessary: LiveKit serialises with
// protojson, so the egress payload is carried under "egressInfo". The
// hand-rolled struct tag said "egress", so egress_ended silently did nothing
// in production and no recording path was ever written.
func TestTheWireFormatCarriesEgressInfoNotEgress(t *testing.T) {
	body := string(encode(t, egressEvent("RM_1", "standup", "EG_1", "standup.mp4", base)))

	if !strings.Contains(body, `"egressInfo"`) {
		t.Fatalf("protojson body has no egressInfo key: %s", body)
	}
	if strings.Contains(body, `"egress":`) {
		t.Fatalf("body still uses the hand-rolled key: %s", body)
	}
	if !strings.Contains(body, `"createdAt":"`) {
		t.Fatalf("protojson renders int64 as a JSON string; body was: %s", body)
	}
}

// The same body, end to end, has to land in the database. This is the test
// that would have caught the shipped bug.
func TestARealProtojsonEgressBodyStampsTheRecording(t *testing.T) {
	h := newHarness(t)
	h.seedRoom("standup")
	h.send(roomEvent(eventRoomStarted, "standup", "RM_1", base))

	h.send(egressEvent("RM_1", "standup", "EG_1", "recordings/standup.mp4", base.Add(time.Hour)))

	calls := h.calls()
	if len(calls) != 1 || calls[0].RecordingPath != "recordings/standup.mp4" {
		t.Fatalf("recording_path never landed: %+v", calls)
	}
}

func TestAnUnsignedRequestIsRefused(t *testing.T) {
	h := newHarness(t)
	body := encode(t, roomEvent(eventRoomStarted, "standup", "RM_1", base))

	if got := h.deliver(body, ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401 for an unsigned webhook", got.Code)
	}
}

func TestAWrongSecretIsRefused(t *testing.T) {
	h := newHarness(t)
	body := encode(t, roomEvent(eventRoomStarted, "standup", "RM_1", base))

	got := h.deliver(body, sign(t, testKey, "not-the-secret-at-all-no", body))
	if got.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401 for a token signed with the wrong secret", got.Code)
	}
}

// An unknown issuer must not fall back to the one secret we hold. The key
// provider resolves the secret by the token's iss claim, so a foreign key
// simply has no secret.
func TestAnUnknownAPIKeyIsRefused(t *testing.T) {
	h := newHarness(t)
	body := encode(t, roomEvent(eventRoomStarted, "standup", "RM_1", base))

	got := h.deliver(body, sign(t, "someone-elses-key", testSecret, body))
	if got.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401 for a token from an unknown API key", got.Code)
	}
}

// A valid signature over a different body is the interesting attack: only the
// sha256 body claim catches the swap.
func TestATamperedBodyIsRefused(t *testing.T) {
	h := newHarness(t)
	signed := sign(t, testKey, testSecret, encode(t, roomEvent(eventRoomStarted, "standup", "RM_1", base)))
	tampered := encode(t, roomEvent(eventRoomStarted, "payroll", "RM_2", base))

	if got := h.deliver(tampered, signed); got.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401 for a valid signature over another body", got.Code)
	}
}

// The SDK reads the whole body itself, so the cap has to be installed on
// r.Body before the call rather than around it.
func TestAnOversizedBodyIsRefused(t *testing.T) {
	h := newHarness(t)
	body := bytes.Repeat([]byte("a"), maxBodyBytes+1024)

	if got := h.deliver(body, sign(t, testKey, testSecret, body)); got.Code == http.StatusNoContent {
		t.Fatal("a body over the cap was accepted")
	}
}
