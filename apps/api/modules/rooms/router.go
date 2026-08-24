package rooms

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(router chi.Router, service *Service, requireAuth func(http.Handler) http.Handler) {
	handler := newHandler(service)
	router.Post("/rooms", handler.create)
	router.With(requireAuth).Get("/rooms", handler.list)
	router.With(requireAuth).Patch("/rooms/{slug}", handler.rename)
	router.With(requireAuth).Delete("/rooms/{slug}", handler.delete)
	router.Get("/rooms/{slug}", handler.get)
	router.Post("/rooms/{slug}/token", handler.token)
}
