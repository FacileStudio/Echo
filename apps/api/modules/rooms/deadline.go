package rooms

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

// deadlineKeyTTL is how long a deadline key lets its holder poll. It outlasts
// any free-plan call with room to spare; a key cannot be renewed, so one
// leaked after the call is worthless a little later.
const deadlineKeyTTL = 2 * time.Hour

// DeadlineKey proves its holder was admitted to the room until expires:
// "<unix expiry>.<HMAC of the slug and the expiry under the LiveKit API
// secret>", which never leaves the server. Every token response carries
// one, and the deadline route asks for it.
func (s *Service) DeadlineKey(slug string, expires time.Time) string {
	exp := strconv.FormatInt(expires.Unix(), 10)
	return exp + "." + s.deadlineMAC(slug, exp)
}

// freshDeadlineKey is a deadline key for slug valid from now for one window.
func (s *Service) freshDeadlineKey(slug string) string {
	return s.DeadlineKey(slug, s.now().Add(deadlineKeyTTL))
}

// validDeadlineKey reports whether key was issued for slug and is still
// within its window. An expiry further ahead than one window is forged.
func (s *Service) validDeadlineKey(slug, key string, now time.Time) bool {
	exp, mac, ok := strings.Cut(key, ".")
	if !ok {
		return false
	}
	expires, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || !now.Before(time.Unix(expires, 0)) || time.Unix(expires, 0).After(now.Add(deadlineKeyTTL)) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(mac), []byte(s.deadlineMAC(slug, exp))) == 1
}

func (s *Service) deadlineMAC(slug, exp string) string {
	mac := hmac.New(sha256.New, []byte(s.media.Secret()))
	mac.Write([]byte("echo-deadline-v2\x00" + slug + "\x00" + exp))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// deadline serves GET /rooms/{slug}/deadline, which a client in a limited
// call polls so an upgrade or a restart reaches its countdown. A caller
// without the room's deadline key gets the same 404 as for a missing room.
func (h *Handler) deadline(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !h.limits.Allow(r, deadlineBudget()) {
		middleware.Refuse(w)
		return
	}
	if !h.service.validDeadlineKey(slug, r.Header.Get("X-Echo-Deadline-Key"), h.service.now()) {
		httpjson.WriteError(w, errors.NotFound("room not found"))
		return
	}
	room, err := h.service.BySlug(r.Context(), slug)
	if err == nil && (room == nil || room.IntegrationApp != "") {
		err = errors.NotFound("room not found")
	}
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	deadline, err := h.service.CurrentDeadline(r.Context(), room)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, toDeadlineResponse(deadline))
}

// CurrentDeadline is when the room's running session must end under the
// owner's current plan, or nil when nothing runs or the plan does not limit
// call length. The start is LiveKit's room creation time, else the open
// call's started_at.
func (s *Service) CurrentDeadline(ctx context.Context, room *schemas.Room) (*time.Time, error) {
	policy, err := s.plans.ForRoom(ctx, *room)
	if err != nil {
		return nil, err
	}
	var start time.Time
	if live, err := s.live.Rooms(ctx, room.Slug); err == nil && len(live) > 0 {
		start = live[0].CreatedAt
	}
	if start.IsZero() {
		call, err := s.latestCall(ctx, room)
		if err != nil || call == nil || call.EndedAt != nil {
			return nil, err
		}
		start = call.StartedAt
	}
	deadline, limited := s.plans.Deadline(policy, start)
	if !limited {
		return nil, nil
	}
	return &deadline, nil
}
