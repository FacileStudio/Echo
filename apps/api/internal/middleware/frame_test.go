package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func policyFor(origins []string, path string) string {
	recorder := httptest.NewRecorder()
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	FrameAncestors(origins)(ok).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder.Header().Get("Content-Security-Policy")
}

func TestOnlyTheEmbedRouteCanBeFramedAndOnlyByListedOrigins(t *testing.T) {
	origins := []string{"https://prisme.facile.studio"}
	cases := []struct {
		origins []string
		path    string
		want    string
	}{
		{origins, "/embed/prisme-1", "frame-ancestors https://prisme.facile.studio"},
		{origins, "/room/standup", "frame-ancestors 'none'"},
		{nil, "/embed/prisme-1", "frame-ancestors 'none'"},
	}
	for _, c := range cases {
		if got := policyFor(c.origins, c.path); got != c.want {
			t.Fatalf("%s with %v: %q, want %q", c.path, c.origins, got, c.want)
		}
	}
}
