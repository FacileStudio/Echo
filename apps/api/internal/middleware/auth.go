package middleware

import (
	"context"
	"net/http"

	"github.com/FacileStudio/Echo/apps/api/internal/authcontext"
	"github.com/FacileStudio/porte"
	"github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
)

// IdentityResolver turns the user id porte authenticated into Echo's own
// identity.
type IdentityResolver interface {
	IdentityForUser(ctx context.Context, userID int64) (authcontext.Identity, error)
}

// RequireAuth runs behind porte's own middleware and hydrates what porte
// deliberately does not carry: the profile lives in this app's table, and
// lands in the context every handler reads.
func RequireAuth(sessions authenticator, resolver IdentityResolver) func(http.Handler) http.Handler {
	hydrate := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			authenticated, ok := porte.From(request.Context())
			if !ok {
				httpjson.WriteError(w, errors.Unauthorized("missing auth token"))
				return
			}
			identity, err := resolver.IdentityForUser(request.Context(), authenticated.UserID)
			if err != nil {
				httpjson.WriteError(w, err)
				return
			}
			next.ServeHTTP(w, request.WithContext(authcontext.With(request.Context(), identity)))
		})
	}
	return func(next http.Handler) http.Handler {
		return sessions.RequireAuth(hydrate(next))
	}
}

type authenticator interface {
	RequireAuth(http.Handler) http.Handler
}
