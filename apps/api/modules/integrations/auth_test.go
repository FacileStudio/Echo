package integrations

import (
	"net/http"
	"testing"
)

func TestTheAPIRefusesACallerWithoutAKey(t *testing.T) {
	h := newHarnessOn(t, nil)
	for _, secret := range []string{"", "not-a-key", prismeSecret + "x"} {
		got := h.call(http.MethodPost, "/integrations/rooms", secret, CreateRoomRequest{ExternalID: "x"})
		if got.Code != http.StatusUnauthorized {
			t.Fatalf("secret %q: status %d, want 401", secret, got.Code)
		}
	}
}

func TestEveryRouteSitsBehindTheKey(t *testing.T) {
	h := newHarnessOn(t, nil)
	routes := [][2]string{
		{http.MethodGet, "/integrations/rooms/prisme-0011223344556677"},
		{http.MethodPost, "/integrations/rooms/prisme-0011223344556677/tokens"},
	}
	for _, route := range routes {
		if got := h.call(route[0], route[1], "", nil); got.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status %d, want 401", route[0], route[1], got.Code)
		}
	}
}

func TestAppForNamesTheAppOfEachSecret(t *testing.T) {
	if app, ok := testKeys.AppFor(prismeSecret); !ok || app != "prisme" {
		t.Fatalf("AppFor(prisme secret) = %q, %v", app, ok)
	}
	if app, ok := testKeys.AppFor(otherSecret); !ok || app != "other" {
		t.Fatalf("AppFor(other secret) = %q, %v", app, ok)
	}
	if _, ok := NewKeys(nil).AppFor(""); ok {
		t.Fatal("an empty key set admitted an empty secret")
	}
}
