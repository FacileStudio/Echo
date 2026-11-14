package mcp

import (
	"net/http"

	mcpkit "github.com/FacileStudio/tronc/mcp"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the MCP endpoint at the resource's path and its RFC
// 9728 metadata on router, the root router outside /api.
func RegisterRoutes(router chi.Router, service *Service) {
	service.server.Mount(router)
}

// RegisterPendingRoutes mounts the confirm page's API under /mcp/pending on
// the /api router, for the signed-in person. A nil service, as in the docs
// test, still mounts the routes.
func RegisterPendingRoutes(router chi.Router, service *Service, requireAuth func(http.Handler) http.Handler) {
	service.pending().Mount(router, requireAuth)
}

// pending is the confirm page, nil on a nil service.
func (s *Service) pending() *mcpkit.PendingPage[PendingItem] {
	if s == nil {
		return nil
	}
	return s.page
}
