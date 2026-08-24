package authcontext

import "context"

type Identity struct {
	UserID  int64
	Email   string
	IsAdmin bool
}

type contextKey struct{}

func With(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

func From(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	return identity, ok
}
