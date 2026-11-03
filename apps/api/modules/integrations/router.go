package integrations

import (
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the server-to-server integration API. Every route
// authenticates with an app's integration secret as a bearer token; porte
// sessions play no part, so the routes carry no cookie and need no CSRF
// header.
func RegisterRoutes(router chi.Router, service *Service) {
	h := handler{service: service}
	router.Route("/integrations/rooms", func(r chi.Router) {
		r.Use(requireApp(service.keys))
		r.Post("/", h.create)
		r.Get("/{id}", h.get)
		r.Post("/{id}/tokens", h.token)
	})
}
