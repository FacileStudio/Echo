package rooms

import (
	"net/http"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the rooms endpoints. Mutating and ownership routes
// sit behind requireAuth; the token endpoint is public because guests join
// without an account, so it resolves the caller itself via the resolver.
func RegisterRoutes(router chi.Router, service *Service, resolver middleware.IdentityResolver, requireAuth func(http.Handler) http.Handler) {
	handler := newHandler(service, resolver)
	router.Post("/rooms", handler.create)
	router.With(requireAuth).Get("/rooms", handler.list)
	router.With(requireAuth).Patch("/rooms/{slug}", handler.rename)
	router.With(requireAuth).Delete("/rooms/{slug}", handler.delete)
	router.Get("/rooms/{slug}", handler.get)
	router.Post("/rooms/{slug}/token", handler.token)
}
