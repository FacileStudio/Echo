package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/internal/env"
	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/internal/seat"
	"github.com/FacileStudio/Echo/apps/api/modules/capture"
	"github.com/FacileStudio/Echo/apps/api/modules/deletions"
	"github.com/FacileStudio/Echo/apps/api/modules/history"
	"github.com/FacileStudio/Echo/apps/api/modules/integrations"
	"github.com/FacileStudio/Echo/apps/api/modules/mcp"
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
	history        *history.Service
	mcp            *mcp.Service
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
	summarizer, err := newSummarizer(cfg, authKit, log)
	if err != nil {
		return nil, fmt.Errorf("summaries: %w", err)
	}
	deps.history = history.NewService(db, deps.rooms, summarizer, cfg.RecordingsDir)
	if err := mountMCP(shutdown, router, &deps, log); err != nil {
		return nil, fmt.Errorf("mcp: %w", err)
	}
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
		deps.mcp.Run,
	}}
	if erasure := authKit.erasure; erasure != nil {
		deletions.RegisterRoutes(router, erasure)
		s.background = append(s.background, erasure.Run)
	}
	var mountErr error
	router.Route("/api", func(r chi.Router) { mountErr = mountAPI(r, deps) })
	if mountErr != nil {
		return nil, fmt.Errorf("routes: %w", mountErr)
	}
	mountClient(router, clientDir, cfg.EmbedOrigins, log)
	return s, nil
}

// newRouter is tronc's chassis with Echo's proxies, origins and headers, and
// the health and version routes at the root.
func newRouter(cfg env.Config, clientDir string, log *slog.Logger) chi.Router {
	router := httpx.NewRouter(httpx.Config{
		Logger:         log,
		TrustedProxies: cfg.TrustedProxies,
		CDNProxies:     troncmiddleware.CloudflareProxies,
		CDNHeader:      middleware.CloudflareHeader,
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
func mountAPI(r chi.Router, deps apiDeps) error {
	r.Use(extendWriteDeadline("/summary", summaryWriteBudget))
	r.Use(extendWriteDeadline("/file", recordingWriteBudget))
	if err := deps.auth.Mount(r); err != nil {
		return err
	}
	rooms.RegisterRoutes(r, deps.rooms, deps.auth.users, deps.auth.requireAuth, middleware.NewRateLimiter())
	recording.RegisterRoutes(r, recording.NewRecording(deps.rooms, deps.media), deps.auth.requireAuth)
	history.RegisterRoutes(r, deps.history, deps.auth.requireAuth)
	transcripts.RegisterRoutes(r, transcripts.NewService(deps.db, deps.rooms.RequireAI), deps.seats, deps.captureLimiter)
	capture.RegisterRoutes(r, deps.capturing, deps.auth.requireAuth)
	integrations.RegisterRoutes(r, integrations.NewService(deps.db, deps.media, deps.keys))
	if deps.mcp != nil {
		mcp.RegisterPendingRoutes(r, deps.mcp, deps.auth.requireAuth)
	}
	return nil
}

// mountMCP builds the MCP server over the rooms and history services and
// mounts /mcp on the root router, when MCP_RESOURCE and OIDC are set. The
// confirm page's API is mounted with the other /api routes.
func mountMCP(ctx context.Context, router chi.Router, deps *apiDeps, log *slog.Logger) error {
	var err error
	deps.mcp, err = newMCP(ctx, deps, mcp.Deps{
		Rooms: deps.rooms, History: deps.history, Accounts: deps.auth.users, Limiter: middleware.NewRateLimiter(),
	}, log)
	if err != nil || deps.mcp == nil {
		return err
	}
	mcp.RegisterRoutes(router, deps.mcp)
	return nil
}
