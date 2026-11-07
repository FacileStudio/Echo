package seat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/porte"
)

func TestKeyNamesOnlyTheIdentityItWasSignedFor(t *testing.T) {
	keys := NewKeys("secret")
	key := keys.Sign("standup", "guest-ab12cd34")

	if who, ok := keys.Verify("standup", key); !ok || who != "guest-ab12cd34" {
		t.Fatalf("Verify = %q, %v", who, ok)
	}
	if _, ok := keys.Verify("other-room", key); ok {
		t.Fatal("a key worked in a room it was not issued for")
	}
	forged := strings.Replace(key, "guest-ab12cd34", "user-1", 1)
	if _, ok := keys.Verify("standup", forged); ok {
		t.Fatal("a key with a swapped identity was accepted")
	}
	if _, ok := NewKeys("another").Verify("standup", key); ok {
		t.Fatal("a key signed under another secret was accepted")
	}
	for _, junk := range []string{"", "x", "1.2", "9999999999.guest-1.", "a.b.c"} {
		if _, ok := keys.Verify("standup", junk); ok {
			t.Fatalf("junk key %q accepted", junk)
		}
	}
}

func TestKeyExpires(t *testing.T) {
	keys := NewKeys("secret")
	key := keys.Sign("standup", "guest-ab12cd34")
	keys.now = func() time.Time { return time.Now().Add(KeyTTL + time.Minute) }
	if _, ok := keys.Verify("standup", key); ok {
		t.Fatal("an expired key was accepted")
	}
}

func TestIdentityPrefersTheSessionAndIgnoresAnyClaim(t *testing.T) {
	keys := NewKeys("secret")
	guest := keys.Sign("standup", "guest-ab12cd34")

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(Header, guest)
	request.Header.Set("X-Speaker", "user-1")
	if who, err := keys.Identity(request, "standup"); err != nil || who != "guest-ab12cd34" {
		t.Fatalf("guest = %q, %v", who, err)
	}

	signedIn := request.WithContext(porte.WithIdentity(request.Context(), porte.Identity{UserID: 7}))
	if who, err := keys.Identity(signedIn, "standup"); err != nil || who != "user-7" {
		t.Fatalf("signed in = %q, %v", who, err)
	}

	if _, err := keys.Identity(httptest.NewRequest(http.MethodGet, "/", nil), "standup"); err == nil {
		t.Fatal("a caller with neither a session nor a key was identified")
	}
}
