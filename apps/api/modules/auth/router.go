package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the routes porte does not own. /auth/config,
// /auth/logout and the whole OIDC flow come from the kit; sign-in goes through
// Registre only, so there is no register or password login route.
func RegisterRoutes(router chi.Router, service *Service, requireAuth func(http.Handler) http.Handler) {
	handler := newHandler(service)
	router.With(requireAuth).Get("/auth/me", handler.me)
}
