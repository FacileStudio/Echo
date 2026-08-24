package rooms

import (
	"context"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/authcontext"
	"github.com/FacileStudio/porte"
)

type stubResolver struct {
	identity authcontext.Identity
	err      error
	calls    int
}

func (s *stubResolver) IdentityForUser(context.Context, int64) (authcontext.Identity, error) {
	s.calls++
	return s.identity, s.err
}

// The rooms create and get routes are public: they are mounted without
// RequireAuth so guests can use them. authcontext is hydrated only inside
// RequireAuth, so a public route that reads it sees every logged-in caller
// as anonymous, and every room it creates gets a nil owner. That shipped,
// and it locked history, recording and summaries out for everyone.
func TestCallerIDResolvesALoggedInCallerOnAPublicRoute(t *testing.T) {
	resolver := &stubResolver{identity: authcontext.Identity{UserID: 42}}
	handler := newHandler(nil, resolver)

	ctx := porte.WithIdentity(context.Background(), porte.Identity{UserID: 42})
	got, err := handler.callerID(ctx)
	if err != nil {
		t.Fatalf("callerID: %v", err)
	}
	if got == nil {
		t.Fatal("logged-in caller resolved to nil: the room would get no owner")
	}
	if *got != 42 {
		t.Fatalf("caller = %d, want 42", *got)
	}
}

func TestCallerIDIsNilForAGuest(t *testing.T) {
	resolver := &stubResolver{identity: authcontext.Identity{UserID: 42}}
	handler := newHandler(nil, resolver)

	got, err := handler.callerID(context.Background())
	if err != nil {
		t.Fatalf("callerID: %v", err)
	}
	if got != nil {
		t.Fatalf("guest resolved to %d, want nil", *got)
	}
	if resolver.calls != 0 {
		t.Fatalf("resolver consulted %d times for a guest, want 0", resolver.calls)
	}
}

// A route behind RequireAuth already carries the hydrated identity, so the
// resolver must not be consulted a second time on every request.
func TestCallerIDPrefersTheHydratedIdentity(t *testing.T) {
	resolver := &stubResolver{identity: authcontext.Identity{UserID: 7}}
	handler := newHandler(nil, resolver)

	ctx := authcontext.With(context.Background(), authcontext.Identity{UserID: 99})
	got, err := handler.callerID(ctx)
	if err != nil {
		t.Fatalf("callerID: %v", err)
	}
	if got == nil || *got != 99 {
		t.Fatalf("caller = %v, want 99", got)
	}
	if resolver.calls != 0 {
		t.Fatalf("resolver consulted %d times, want 0", resolver.calls)
	}
}
