package authcontext

import "context"

// Identity is Echo's own view of the caller, hydrated from its user table.
type Identity struct {
	UserID  int64
	Email   string
	IsAdmin bool
}

type contextKey struct{}

// With stores identity in ctx.
func With(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

// From returns the identity stored by With, if any.
func From(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	return identity, ok
}
