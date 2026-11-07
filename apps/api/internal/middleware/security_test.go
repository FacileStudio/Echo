package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
)

func headersFor(t *testing.T, origins []string, path string) http.Header {
	t.Helper()
	cfg := SecurityConfig("wss://echo-live.facile.studio", []string{"'sha256-abc'"})
	mux := http.NewServeMux()
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	mux.Handle("/embed/", EmbedFrameable(origins)(ok))
	mux.Handle("/", ok)
	handler := troncmiddleware.SecurityHeaders(cfg)(mux)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder.Header()
}

func TestOnlyTheEmbedRouteCanBeFramedAndOnlyByListedOrigins(t *testing.T) {
	origins := []string{"https://prisme.facile.studio"}
	cases := []struct {
		origins []string
		path    string
		want    string
		deny    bool
	}{
		{origins, "/embed/prisme-1", "frame-ancestors https://prisme.facile.studio", false},
		{origins, "/room/standup", "frame-ancestors 'none'", true},
		{nil, "/embed/prisme-1", "frame-ancestors 'none'", true},
	}
	for _, c := range cases {
		header := headersFor(t, c.origins, c.path)
		if got := header.Get("Content-Security-Policy"); !strings.Contains(got, c.want) {
			t.Fatalf("%s with %v: %q lacks %q", c.path, c.origins, got, c.want)
		}
		if denied := header.Get("X-Frame-Options") == "DENY"; denied != c.deny {
			t.Fatalf("%s with %v: X-Frame-Options DENY = %v, want %v", c.path, c.origins, denied, c.deny)
		}
	}
}

func TestCallsKeepCameraMicrophoneAndTheLiveKitSocket(t *testing.T) {
	header := headersFor(t, nil, "/room/standup")
	policy := header.Get("Permissions-Policy")
	for _, want := range []string{"camera=(self)", "microphone=(self)", "display-capture=(self)"} {
		if !strings.Contains(policy, want) {
			t.Fatalf("Permissions-Policy %q lacks %q", policy, want)
		}
	}
	csp := header.Get("Content-Security-Policy-Report-Only")
	for _, want := range []string{"wss://echo-live.facile.studio", "https://echo-live.facile.studio", "'sha256-abc'"} {
		if !strings.Contains(csp, want) {
			t.Fatalf("CSP %q lacks %q", csp, want)
		}
	}
}
