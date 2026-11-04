package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/porte"
)

func hit(handler http.Handler, addr string, ctx context.Context) int {
	request := httptest.NewRequest(http.MethodPost, "/api/rooms/x/token", nil).WithContext(ctx)
	request.RemoteAddr = addr
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Code
}

func TestGuestsAreLimitedPerAddressAndMembersPerAccount(t *testing.T) {
	limiter := NewRateLimiter(3, 5)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for range 3 {
		if code := hit(handler, "203.0.113.7:1000", context.Background()); code != http.StatusNoContent {
			t.Fatalf("guest within budget answered %d", code)
		}
	}
	if code := hit(handler, "203.0.113.7:2000", context.Background()); code != http.StatusTooManyRequests {
		t.Fatalf("fourth guest request from one address answered %d, want 429", code)
	}
	if code := hit(handler, "198.51.100.1:1000", context.Background()); code != http.StatusNoContent {
		t.Fatalf("another address shared the bucket: %d", code)
	}
	member := porte.WithIdentity(context.Background(), porte.Identity{UserID: 9})
	for i := range 5 {
		if code := hit(handler, "203.0.113.7:3000", member); code != http.StatusNoContent {
			t.Fatalf("signed-in request %d answered %d", i, code)
		}
	}
	if code := hit(handler, "203.0.113.7:3000", member); code != http.StatusTooManyRequests {
		t.Fatalf("signed-in caller over budget answered %d, want 429", code)
	}
}

func TestIdleBucketsAreSwept(t *testing.T) {
	limiter := NewRateLimiter(1, 1)
	clock := time.Now()
	limiter.now = func() time.Time { return clock }
	limiter.allow("ip:a", 1)
	clock = clock.Add(idleBucket + 2*time.Minute)
	limiter.allow("ip:b", 1)
	if _, kept := limiter.buckets["ip:a"]; kept || len(limiter.buckets) != 1 {
		t.Fatalf("buckets after a sweep = %d, want only the fresh one", len(limiter.buckets))
	}
}
