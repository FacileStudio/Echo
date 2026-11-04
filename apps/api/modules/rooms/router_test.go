package rooms

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestBothRoutesThatCreateARoomAreRateLimited(t *testing.T) {
	router := chi.NewRouter()
	refuse := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) })
	}
	RegisterRoutes(router, nil, &stubResolver{}, refuse, refuse)
	for _, path := range []string{"/rooms", "/rooms/junk-slug/token"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if recorder.Code != http.StatusTooManyRequests {
			t.Errorf("POST %s answered %d, want the limiter's 429", path, recorder.Code)
		}
	}
}
