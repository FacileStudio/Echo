package main

import (
	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	troncenv "github.com/FacileStudio/tronc/env"
	troncmiddleware "github.com/FacileStudio/tronc/middleware"
	"github.com/FacileStudio/tronc/spa"
	"github.com/go-chi/chi/v5"
)

// securityConfig is tronc's security headers plus what Echo's built client
// needs: camera, microphone and screen capture, the LiveKit socket, and the
// inline scripts adapter-static writes into its pages, which script-src 'self'
// would otherwise block. A client directory that cannot be hashed is logged
// and leaves the default script policy, which only reports.
func securityConfig(clientDir string, log *slog.Logger) troncmiddleware.SecurityConfig {
	var hashes []string
	if spa.Available(clientDir) {
		found, err := spa.InlineScriptHashes(clientDir)
		if err != nil {
			log.Warn("failed to hash the client's inline scripts", slog.Any("error", err))
		}
		hashes = found
	}
	return middleware.SecurityConfig(troncenv.String("LIVEKIT_URL", ""), hashes)
}

// mountClient serves the built client. The embed route is registered first
// behind the override that lets the configured origins frame it; every other
// path keeps the chassis frame-ancestors 'none'. The catch-all must stay LAST:
// anything registered after it is unreachable.
func mountClient(router chi.Router, clientDir string, embedOrigins []string, log *slog.Logger) {
	if !spa.Available(clientDir) {
		return
	}
	client := spa.Handler(spa.Config{Dir: clientDir})
	router.With(middleware.EmbedFrameable(embedOrigins)).Handle("/embed/*", client)
	router.Handle("/*", client)
	log.Info("serving client", slog.String("dir", clientDir))
}
