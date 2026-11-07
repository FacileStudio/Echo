package history

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the history endpoints. Every one sits behind
// requireAuth and a service-level check against the caller's own account: the
// room owner sees their calls, a participant sees the calls they sat in, and a
// recording reaches a participant only once the owner shared it.
func RegisterRoutes(router chi.Router, service *Service, requireAuth func(http.Handler) http.Handler) {
	h := handler{service: service}
	router.With(requireAuth).Get("/rooms/{slug}/calls", h.list)
	router.With(requireAuth).Get("/calls/{id}", h.detail)
	router.With(requireAuth).Post("/calls/{id}/summary", h.summarize)
	router.With(requireAuth).Get("/meetings", h.meetings)
	router.With(requireAuth).Get("/calls/{id}/recordings", h.recordings)
	router.With(requireAuth).Get("/calls/{id}/transcript", h.transcript)
	router.With(requireAuth).Get("/recordings/{id}/file", h.recordingFile)
	router.With(requireAuth).Patch("/recordings/{id}", h.share)
}
