package rooms

import (
	"net/http"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the rooms endpoints. Mutating and ownership routes
// sit behind requireAuth; the token endpoint is public because guests join
// without an account, so it resolves the caller itself via the resolver.
// Creating a room, through POST /rooms or a token for an unknown slug, spends
// the strict creation budget, so nobody can mint unbounded room rows, LiveKit
// rooms and timers. Joining an existing room spends a far larger budget per
// address and room, so a whole office behind one NAT can join one call.
func RegisterRoutes(router chi.Router, service *Service, resolver middleware.IdentityResolver,
	requireAuth func(http.Handler) http.Handler, limits Limits) {
	handler := newHandler(service, resolver)
	handler.limits = limits
	router.With(budgeted(limits, createBudget())).Post("/rooms", handler.create)
	router.With(requireAuth).Get("/rooms", handler.list)
	router.With(requireAuth).Patch("/rooms/{slug}", handler.rename)
	router.With(requireAuth).Delete("/rooms/{slug}", handler.delete)
	router.Get("/rooms/{slug}", handler.get)
	router.Post("/rooms/{slug}/token", handler.token)
	router.Get("/rooms/{slug}/deadline", handler.deadline)
}
