package integrations

import (
	"context"
	"net/http"
	"strings"

	"github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
)

type appKey struct{}

// requireApp admits a request whose bearer token is an integration secret
// and puts the app it belongs to in the context.
func requireApp(keys Keys) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			presented, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || presented == "" {
				httpjson.WriteError(w, errors.Unauthorized("missing integration key"))
				return
			}
			app, ok := keys.AppFor(presented)
			if !ok {
				httpjson.WriteError(w, errors.Unauthorized("invalid integration key"))
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), appKey{}, app)))
		})
	}
}

func appFrom(ctx context.Context) string {
	app, _ := ctx.Value(appKey{}).(string)
	return app
}
