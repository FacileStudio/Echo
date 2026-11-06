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

// RateLimiter caps requests per minute within named scopes: per signed-in
// account, and per client address (ClientAddr) for everyone else. An IPv6
// caller spends from its /64 and from its /48 at networkFactor times the
// budget. A scope keeps its own buckets, so a strict budget for creating
// rooms never throttles joining one.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*list.Element
	order   *list.List
	now     func() time.Time
}

type bucket struct {
	key     string
	limiter *rate.Limiter
}

// NewRateLimiter builds an empty RateLimiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{buckets: map[string]*list.Element{}, order: list.New(), now: time.Now}
}

// Budget is a scope's per-minute allowance for guests and for signed-in
// accounts.
type Budget struct {
	Scope  string
	Guest  int
	Member int
}

// Limit is middleware refusing a request over budget with 429.
func (l *RateLimiter) Limit(budget Budget) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(r, budget) {
				Refuse(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Refuse answers 429 with the suite error envelope.
func Refuse(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	httpjson.WriteError(w, errors.RateLimited("too many requests, try again in a minute"))
}

// Allow spends one request from every bucket the caller has in the scope,
// widest first, and stops at the first that is empty.
func (l *RateLimiter) Allow(r *http.Request, budget Budget) bool {
	if identity, ok := porte.From(r.Context()); ok {
		return l.allow(budget.Scope+"|user:"+strconv.FormatInt(identity.UserID, 10), budget.Member)
	}
	keys := addressKeys(ClientAddr(r))
	for i := len(keys) - 1; i >= 0; i-- {
		perMinute := budget.Guest
		if strings.HasPrefix(keys[i], "net:") {
			perMinute *= networkFactor
		}
		if !l.allow(budget.Scope+"|"+keys[i], perMinute) {
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
