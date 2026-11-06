package rooms

import (
	"context"
	"net/http"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

// Limits spends one request of a budget for the caller.
type Limits interface {
	Allow(r *http.Request, budget middleware.Budget) bool
}

// createBudget is the per-minute allowance for creating a room, shared by
// POST /rooms and a token for a slug that does not exist yet.
func createBudget() middleware.Budget {
	return middleware.Budget{Scope: "room-create", Guest: 10, Member: 30}
}

// joinBudget is the per-minute allowance for joining one existing room, per
// address and room, sized for a whole office behind one NAT.
func joinBudget(slug string) middleware.Budget {
	return middleware.Budget{Scope: "join:" + slug, Guest: 60, Member: 120}
}

// deadlineBudget is the per-minute allowance for reading one room's call
// deadline, which every client in a limited call polls.
func deadlineBudget(slug string) middleware.Budget {
	return middleware.Budget{Scope: "deadline:" + slug, Guest: 120, Member: 240}
}

func budgeted(limits Limits, budget middleware.Budget) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limits.Allow(r, budget) {
				middleware.Refuse(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// joinWithinBudget spends the join budget of an existing room, or the
// creation budget for a slug the token route would create, and answers 429
// when it is spent.
func (h *Handler) joinWithinBudget(w http.ResponseWriter, r *http.Request, slug string) bool {
	room, err := h.service.BySlug(r.Context(), slug)
	if err != nil {
		httpjson.WriteError(w, err)
		return false
	}
	budget := createBudget()
	if room != nil {
		budget = joinBudget(slug)
	}
	if !h.limits.Allow(r, budget) {
		middleware.Refuse(w)
		return false
	}
	return true
}

// deadline serves GET /rooms/{slug}/deadline, which a client in a limited
// call polls so an upgrade or a restart reaches its countdown.
func (h *Handler) deadline(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !h.limits.Allow(r, deadlineBudget(slug)) {
		middleware.Refuse(w)
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
