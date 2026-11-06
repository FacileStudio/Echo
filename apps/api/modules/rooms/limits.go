package rooms

import (
	"net/http"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/tronc/httpjson"
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

// joinAnyBudget is the per-minute allowance for joining any rooms at all, per
// address, on top of each room's own budget, so one address cannot spread
// joins over many rooms.
func joinAnyBudget() middleware.Budget {
	return middleware.Budget{Scope: "join", Guest: 120, Member: 240}
}

// deadlineBudget is the per-minute allowance for reading call deadlines, per
// address across every room, which each client in a limited call polls once
// a minute.
func deadlineBudget() middleware.Budget {
	return middleware.Budget{Scope: "deadline", Guest: 120, Member: 240}
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
	allowed := room == nil && h.limits.Allow(r, createBudget()) ||
		room != nil && h.limits.Allow(r, joinAnyBudget()) && h.limits.Allow(r, joinBudget(slug))
	if !allowed {
		middleware.Refuse(w)
		return false
	}
	return true
}
