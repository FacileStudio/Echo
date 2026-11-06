package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"testing"

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
	limiter := NewRateLimiter()
	handler := limiter.Limit(Budget{Scope: "t", Guest: 3, Member: 5})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func keysOf(raw string) []string { return addressKeys(netip.MustParseAddr(raw)) }

func TestIPv6IsLimitedPer64AndPer48(t *testing.T) {
	if keysOf("2001:db8:1:2::1")[0] != keysOf("2001:db8:1:2:ffff::9")[0] {
		t.Fatal("two addresses of one /64 got separate buckets")
	}
	if keysOf("2001:db8:1:2::1")[0] == keysOf("2001:db8:1:3::1")[0] {
		t.Fatal("two /64s shared a bucket")
	}
	if keysOf("2001:db8:1:2::1")[1] != keysOf("2001:db8:1:3::1")[1] {
		t.Fatal("two /64s of one /48 do not share the network bucket")
	}
	if keysOf("::ffff:203.0.113.7")[0] != keysOf("203.0.113.7")[0] {
		t.Fatal("an IPv4-mapped address escaped its IPv4 bucket")
	}
	limiter := NewRateLimiter()
	handler := limiter.Limit(Budget{Scope: "t", Guest: 1, Member: 1})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	refused := 0
	for i := range 2 * networkFactor {
		if hit(handler, "[2001:db8:7:"+strconv.Itoa(i)+"::1]:443", context.Background()) == http.StatusTooManyRequests {
			refused++
		}
	}
	if refused != networkFactor {
		t.Fatalf("%d of %d fresh /64s in one /48 refused, want %d", refused, 2*networkFactor, networkFactor)
	}
}

func TestAFullLimiterEvictsTheLeastRecentlyUsedBucket(t *testing.T) {
	limiter := NewRateLimiter()
	if !limiter.allow("user:1", 1) {
		t.Fatal("first member request refused")
	}
	for i := range maxBuckets - 1 {
		limiter.allow("ip:filler-"+strconv.Itoa(i), 1)
	}
	if limiter.allow("user:1", 1) {
		t.Fatal("a member over budget was let through")
	}
	if !limiter.allow("ip:newcomer", 1) {
		t.Fatal("a newcomer to a full limiter was pooled or refused instead of getting its own bucket")
	}
	if _, kept := limiter.buckets["user:1"]; !kept || limiter.order.Len() != maxBuckets {
		t.Fatal("eviction dropped the recently used member instead of the oldest bucket")
	}
	if _, kept := limiter.buckets["ip:filler-0"]; kept {
		t.Fatal("the least recently used bucket survived a full limiter")
	}
}

func TestScopesKeepSeparateBuckets(t *testing.T) {
	limiter := NewRateLimiter()
	create := Budget{Scope: "create", Guest: 1, Member: 1}
	join := Budget{Scope: "join:standup", Guest: 1, Member: 1}
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.RemoteAddr = "203.0.113.7:1000"
	if !limiter.Allow(request, create) || limiter.Allow(request, create) {
		t.Fatal("the create budget is not one request")
	}
	if !limiter.Allow(request, join) {
		t.Fatal("an exhausted create budget throttled joining a room")
	}
}
