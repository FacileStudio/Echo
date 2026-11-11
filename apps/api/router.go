package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/internal/env"
	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/internal/seat"
	"github.com/FacileStudio/Echo/apps/api/internal/summarize"
	"github.com/FacileStudio/Echo/apps/api/modules/capture"
	"github.com/FacileStudio/Echo/apps/api/modules/deletions"
	"github.com/FacileStudio/Echo/apps/api/modules/history"
	"github.com/FacileStudio/Echo/apps/api/modules/integrations"
	"github.com/FacileStudio/Echo/apps/api/modules/recording"
	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	"github.com/FacileStudio/Echo/apps/api/modules/transcripts"
	"github.com/FacileStudio/Echo/apps/api/modules/webhooks"
	troncenv "github.com/FacileStudio/tronc/env"
	"github.com/FacileStudio/tronc/health"
	"github.com/FacileStudio/tronc/httpx"
	troncmiddleware "github.com/FacileStudio/tronc/middleware"
	"github.com/FacileStudio/tronc/spa"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// server is the router Echo serves and the loops that run beside it.
type server struct {
	router     chi.Router
	background []func(context.Context)
}

// start runs every background loop until ctx ends.
func (s *server) start(ctx context.Context) {
	for _, loop := range s.background {
		go loop(ctx)
	}
}

// apiDeps is what the /api routes are built from.
type apiDeps struct {
	db             *gorm.DB
	cfg            env.Config
	auth           *authKit
	media          *media.Service
	rooms          *rooms.Service
	seats          *seat.Keys
	captureLimiter *middleware.RateLimiter
	capturing      *capture.Service
	keys           integrations.Keys
}

// buildServer wires every service and route. It starts nothing: the caller
// starts the background loops once the router is built.
func buildServer(shutdown context.Context, cfg env.Config, db *gorm.DB, log *slog.Logger) (*server, error) {
	authKit, err := setupAuth(shutdown, cfg, db, log)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	mediaService, err := media.NewServiceFromEnv()
	if err != nil {
		return nil, fmt.Errorf("media config: %w", err)
	}
	planning, err := setupPlans(shutdown, cfg, db, mediaService, log)
	if err != nil {
		return nil, fmt.Errorf("plans: %w", err)
	}
	clientDir := spa.DirFromEnv()
	router := newRouter(cfg, clientDir, log)
	deps := apiDeps{db: db, cfg: cfg, auth: authKit, media: mediaService, keys: integrations.NewKeys(cfg.IntegrationKeys)}
	forwarder := integrations.NewForwarder(db, deps.keys, log)
	planning.limiter.NotifyWith(forwarder)
	deps.rooms = rooms.NewService(db, mediaService, planning.plans, planning.limiter)
	deps.seats = seat.NewKeys(mediaService.Secret())
	deps.captureLimiter = middleware.NewRateLimiter()
	deps.capturing = capture.NewService(db, deps.rooms, deps.seats, deps.captureLimiter, cfg.RecordingsDir)
	webhooks.New(db, troncenv.String("LIVEKIT_API_KEY", ""), mediaService.Secret()).
		NotifyWith(forwarder).
		NotifyWith(planning.limiter).
		NotifyWith(deps.rooms).
		NotifyWith(deps.capturing).
		RegisterRoutes(router)
	planning.Mount(router)
	s := &server{router: router, background: []func(context.Context){
		func(ctx context.Context) { sweepSessions(ctx, authKit.sessions, log) },
		func(ctx context.Context) { planning.Run(ctx, log) },
		deps.capturing.Run,
	}}
	if erasure := setupDeletions(cfg, db, log); erasure != nil {
		deletions.RegisterRoutes(router, erasure)
		s.background = append(s.background, erasure.Run)
	}
	router.Route("/api", func(r chi.Router) { mountAPI(r, deps) })
	mountClient(router, clientDir, cfg.EmbedOrigins, log)
	return s, nil
}

// newRouter is tronc's chassis with Echo's proxies, origins and headers, and
// the health and version routes at the root.
func newRouter(cfg env.Config, clientDir string, log *slog.Logger) chi.Router {
	router := httpx.NewRouter(httpx.Config{
		Logger:         log,
		TrustedProxies: cfg.TrustedProxies,
		CORS: troncmiddleware.CORSConfig{
			AllowedOrigins:   cfg.CORSAllowedOrigins,
			AllowCredentials: true,
		},
		Security: securityConfig(clientDir, log),
	})
	health.Mount(router)
	router.Get("/version", versionHandler)
	return router
}

// mountAPI registers every route under /api.
func mountAPI(r chi.Router, deps apiDeps) {
	r.Use(extendWriteDeadline("/summary", summaryWriteBudget))
	r.Use(extendWriteDeadline("/file", recordingWriteBudget))
	deps.auth.Mount(r)
	rooms.RegisterRoutes(r, deps.rooms, deps.auth.service, deps.auth.requireAuth, middleware.NewRateLimiter())
	recording.RegisterRoutes(r, recording.NewRecording(deps.rooms, deps.media), deps.auth.requireAuth)
	historyService := history.NewService(deps.db, deps.rooms, summarize.NewFromEnv(), deps.cfg.RecordingsDir)
	history.RegisterRoutes(r, historyService, deps.auth.requireAuth)
	transcripts.RegisterRoutes(r, transcripts.NewService(deps.db, deps.rooms.RequireAI), deps.seats, deps.captureLimiter)
	capture.RegisterRoutes(r, deps.capturing, deps.auth.requireAuth)
	integrations.RegisterRoutes(r, integrations.NewService(deps.db, deps.media, deps.keys))
}
