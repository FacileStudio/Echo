package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/FacileStudio/porte"
	"github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"golang.org/x/time/rate"
)

// idleBucket is how long an unused bucket is kept before a sweep drops it.
const idleBucket = 10 * time.Minute

// RateLimiter caps requests per minute: per signed-in user at Member, and per
// client address at Guest for everyone else. The address is RemoteAddr,
// which tronc's router has already rewritten from X-Forwarded-For only when
// the peer is a trusted proxy, so a client cannot hand itself a fresh bucket.
type RateLimiter struct {
	Guest  int
	Member int

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
	now       func() time.Time
}

type bucket struct {
	limiter *rate.Limiter
	seen    time.Time
}

// NewRateLimiter builds a RateLimiter with the given per-minute budgets.
func NewRateLimiter(guest, member int) *RateLimiter {
	return &RateLimiter{Guest: guest, Member: member, buckets: map[string]*bucket{}, now: time.Now}
}

// Middleware refuses a request over its caller's budget with 429.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, perMinute := l.keyOf(r)
		if !l.allow(key, perMinute) {
			w.Header().Set("Retry-After", "60")
			httpjson.WriteError(w, errors.RateLimited("too many requests, try again in a minute"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) keyOf(r *http.Request) (string, int) {
	if identity, ok := porte.From(r.Context()); ok {
		return "user:" + strconv.FormatInt(identity.UserID, 10), l.Member
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host, l.Guest
}

// allow spends one request from key's bucket, and drops idle buckets at most
// once a minute so the map cannot grow without bound.
func (l *RateLimiter) allow(key string, perMinute int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastSweep) > time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.seen) > idleBucket {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(rate.Limit(float64(perMinute)/60), perMinute)}
		l.buckets[key] = b
	}
	b.seen = now
	return b.limiter.AllowN(now, 1)
}
