package middleware

import (
	"container/list"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FacileStudio/porte"
	"github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"golang.org/x/time/rate"
)

// maxBuckets caps how many buckets the limiter keeps. Past it the least
// recently used bucket is dropped, so a flood of fresh addresses costs a
// bounded amount of memory and never pools anyone with anyone else.
const maxBuckets = 50000

// networkFactor is how many /64 budgets one IPv6 /48 may spend in total.
const networkFactor = 4

// RateLimiter caps requests per minute: per signed-in account at Member, and
// per client address (ClientAddr) at Guest for everyone else. An IPv6 caller
// spends from its /64 at Guest and from its /48 at networkFactor times that.
type RateLimiter struct {
	Guest  int
	Member int

	mu      sync.Mutex
	buckets map[string]*list.Element
	order   *list.List
	now     func() time.Time
}

type bucket struct {
	key     string
	limiter *rate.Limiter
}

// NewRateLimiter builds a RateLimiter with the given per-minute budgets.
func NewRateLimiter(guest, member int) *RateLimiter {
	return &RateLimiter{Guest: guest, Member: member, buckets: map[string]*list.Element{}, order: list.New(), now: time.Now}
}

// Middleware refuses a request over its caller's budget with 429.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allowRequest(r) {
			w.Header().Set("Retry-After", "60")
			httpjson.WriteError(w, errors.RateLimited("too many requests, try again in a minute"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowRequest spends one request from every bucket the caller belongs to,
// widest first, and stops at the first that is empty.
func (l *RateLimiter) allowRequest(r *http.Request) bool {
	if identity, ok := porte.From(r.Context()); ok {
		return l.allow("user:"+strconv.FormatInt(identity.UserID, 10), l.Member)
	}
	keys := addressKeys(ClientAddr(r))
	for i := len(keys) - 1; i >= 0; i-- {
		perMinute := l.Guest
		if strings.HasPrefix(keys[i], "net:") {
			perMinute *= networkFactor
		}
		if !l.allow(keys[i], perMinute) {
			return false
		}
	}
	return true
}

// allow spends one request from key's bucket, creating it on first use and
// dropping the least recently used bucket when the limiter is full.
func (l *RateLimiter) allow(key string, perMinute int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	element, ok := l.buckets[key]
	if ok {
		l.order.MoveToFront(element)
	} else {
		if l.order.Len() >= maxBuckets {
			oldest := l.order.Back()
			delete(l.buckets, oldest.Value.(*bucket).key)
			l.order.Remove(oldest)
		}
		limiter := rate.NewLimiter(rate.Limit(float64(perMinute)/60), perMinute)
		element = l.order.PushFront(&bucket{key: key, limiter: limiter})
		l.buckets[key] = element
	}
	return element.Value.(*bucket).limiter.AllowN(l.now(), 1)
}
