package webhooks

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "livekit-api-secret"

// signBody mints the token LiveKit sends: HS256 over a sha256 claim holding
// the base64 digest of the exact request body.
func signBody(t *testing.T, secret string, body []byte) string {
	t.Helper()
	sum := sha256.Sum256(body)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sha256": base64.StdEncoding.EncodeToString(sum[:]),
		"iss":    "devkey",
		"exp":    time.Now().Add(time.Minute).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

func TestVerifyAcceptsAGenuineSignature(t *testing.T) {
	w := New(nil, testSecret)
	body := []byte(`{"event":"room_started","room":{"name":"standup"}}`)

	if err := w.verify(signBody(t, testSecret, body), body); err != nil {
		t.Fatalf("a genuine LiveKit signature was rejected: %v", err)
	}
}

func TestVerifyRejectsAWrongSecret(t *testing.T) {
	w := New(nil, testSecret)
	body := []byte(`{"event":"room_started","room":{"name":"standup"}}`)

	if err := w.verify(signBody(t, "not-the-secret", body), body); err == nil {
		t.Fatal("a token signed with the wrong secret was accepted")
	}
}

// A correct signature over a different body is the interesting attack: the
// JWT verifies, so only the sha256 claim check catches the swap.
func TestVerifyRejectsATamperedBody(t *testing.T) {
	w := New(nil, testSecret)
	signed := signBody(t, testSecret, []byte(`{"event":"room_started","room":{"name":"standup"}}`))
	tampered := []byte(`{"event":"room_started","room":{"name":"payroll"}}`)

	if err := w.verify(signed, tampered); err == nil {
		t.Fatal("a valid signature over a different body was accepted")
	}
}

func TestVerifyRejectsAMissingHeader(t *testing.T) {
	if err := New(nil, testSecret).verify("", []byte(`{}`)); err == nil {
		t.Fatal("an unsigned webhook was accepted")
	}
}
