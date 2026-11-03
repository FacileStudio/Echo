package middleware

import (
	"net/http"
	"strings"
)

// FrameAncestors sets CSP frame-ancestors on every response. Only the embed
// route may be framed, and only by the configured origins; every other page
// refuses all framing, which is what keeps the embed allowlist meaningful.
//
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/frame-ancestors
func FrameAncestors(embedOrigins []string) func(http.Handler) http.Handler {
	embedPolicy := "frame-ancestors 'none'"
	if len(embedOrigins) > 0 {
		embedPolicy = "frame-ancestors " + strings.Join(embedOrigins, " ")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			policy := "frame-ancestors 'none'"
			if strings.HasPrefix(r.URL.Path, "/embed/") {
				policy = embedPolicy
			}
			w.Header().Set("Content-Security-Policy", policy)
			next.ServeHTTP(w, r)
		})
	}
}
