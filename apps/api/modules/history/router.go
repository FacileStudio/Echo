package history

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the owner-gated history endpoints. A call is the room
// owner's business record — transcript, participants and recording all sit
// behind requireAuth plus the service-level owner check.
func RegisterRoutes(router chi.Router, service *Service, requireAuth func(http.Handler) http.Handler) {
	h := handler{service: service}
	router.With(requireAuth).Get("/rooms/{slug}/calls", h.list)
	router.With(requireAuth).Get("/calls/{id}", h.detail)
	router.With(requireAuth).Post("/calls/{id}/summary", h.summarize)
	router.With(requireAuth).Get("/calls/{id}/recording", h.recording)
}
