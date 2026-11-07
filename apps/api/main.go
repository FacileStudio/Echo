package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/database"
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
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/session"

	troncenv "github.com/FacileStudio/tronc/env"
	"github.com/FacileStudio/tronc/health"
	"github.com/FacileStudio/tronc/healthcheck"
	"github.com/FacileStudio/tronc/httpx"
	"github.com/FacileStudio/tronc/logger"
	troncmiddleware "github.com/FacileStudio/tronc/middleware"
	"github.com/FacileStudio/tronc/spa"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func main() {
	if healthcheck.Handle(os.Args) {
		return
	}

	os.Exit(run())
}

func run() int {
	cfg, err := env.Load()
	if err != nil {
		slog.Error("config", slog.Any("error", err))
		return 1
	}

	log := logger.New(logger.Config{Level: cfg.LogLevel})

	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, closeDB, err := openDatabase(cfg.DatabaseURL, log)
	if err != nil {
		log.Error("database", slog.Any("error", err))
		return 1
	}
	defer closeDB()

	authKit, err := setupAuth(shutdown, cfg, db, log)
	if err != nil {
		log.Error("auth", slog.Any("error", err))
		return 1
	}

	mediaService, err := media.NewServiceFromEnv()
	if err != nil {
		log.Error("media config", slog.Any("error", err))
		return 1
	}

	planning, err := setupPlans(shutdown, cfg, db, mediaService, log)
	if err != nil {
		log.Error("plans", slog.Any("error", err))
		return 1
	}

	clientDir := spa.DirFromEnv()
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
	integrationKeys := integrations.NewKeys(cfg.IntegrationKeys)
	forwarder := integrations.NewForwarder(db, integrationKeys, log)
	planning.limiter.NotifyWith(forwarder)
	roomsService := rooms.NewService(db, mediaService, planning.plans, planning.limiter)
	seats := seat.NewKeys(mediaService.Secret())
	captureLimiter := middleware.NewRateLimiter()
	capturing := capture.NewService(db, roomsService, seats, captureLimiter, cfg.RecordingsDir)
	webhooks.New(db, troncenv.String("LIVEKIT_API_KEY", ""), mediaService.Secret()).
		NotifyWith(forwarder).
		NotifyWith(planning.limiter).
		NotifyWith(roomsService).
		NotifyWith(capturing).
		RegisterRoutes(router)
	planning.Mount(router)
	erasure := setupDeletions(cfg, db, log)
	if erasure != nil {
		deletions.RegisterRoutes(router, erasure)
		go erasure.Run(shutdown)
	}
	router.Route("/api", func(r chi.Router) {
		r.Use(extendWriteDeadline("/summary", summaryWriteBudget))
		r.Use(extendWriteDeadline("/file", recordingWriteBudget))
		authKit.Mount(r)
		rooms.RegisterRoutes(r, roomsService, authKit.service, authKit.requireAuth, middleware.NewRateLimiter())
		recording.RegisterRoutes(r, recording.NewRecording(roomsService, mediaService), authKit.requireAuth)
		historyService := history.NewService(db, roomsService, summarize.NewFromEnv(), cfg.RecordingsDir)
		history.RegisterRoutes(r, historyService, authKit.requireAuth)
		transcripts.RegisterRoutes(r, transcripts.NewService(db, roomsService.RequireAI), seats, captureLimiter)
		capture.RegisterRoutes(r, capturing, authKit.requireAuth)
		integrations.RegisterRoutes(r, integrations.NewService(db, mediaService, integrationKeys))
	})

	go sweepSessions(shutdown, authKit.sessions, log)
	go planning.Run(shutdown, log)
	go capturing.Run(shutdown)

	mountClient(router, clientDir, cfg.EmbedOrigins, log)

	return serve(router, cfg.Port, shutdown.Done(), log)
}

func openDatabase(url string, log *slog.Logger) (orm *gorm.DB, close func(), err error) {
	db, err := database.Open(url)
	if err != nil {
		return nil, nil, err
	}
	if err := schemas.Migrate(db); err != nil {
		return nil, nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	return db, func() {
		if err := sqlDB.Close(); err != nil {
			log.Error("failed to close database", slog.Any("error", err))
		}
	}, nil
}

// summaryWriteBudget is what one route is allowed to spend writing its
// response. It has to clear the summarizer's own upstream timeout with room
// to spare: the server's 30s WriteTimeout used to kill a slow summary as a
// network error for the owner, after the row was committed and the model
// billed.
const summaryWriteBudget = 3 * time.Minute

// recordingWriteBudget is what streaming one recording may take. A meeting
// runs for an hour or more and a viewer on a slow line pulls it at their own
// pace, so the short server deadline would cut playback and downloads mid-file.
const recordingWriteBudget = 2 * time.Hour

// extendWriteDeadline lifts the server's WriteTimeout for the routes whose
// path ends in suffix, and leaves every other route on the short one. The
// deadline is per-connection, so raising it here costs nothing elsewhere.
func extendWriteDeadline(suffix string, budget time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, suffix) {
				if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(budget)); err != nil {
					slog.Debug("write deadline not settable", slog.Any("error", err))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func serve(router http.Handler, port int, shutdown <-chan struct{}, log *slog.Logger) int {
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		<-shutdown
		log.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Error("graceful shutdown failed", slog.Any("error", err))
		}
	}()

	log.Info("echo api listening", slog.Int("port", port))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server exited", slog.Any("error", err))
		return 1
	}
	return 0
}

func sweepSessions(ctx context.Context, sessions *session.Manager, logger *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		deleted, err := sessions.Sweep(ctx)
		if err != nil {
			if ctx.Err() == nil {
				logger.Error("session sweep failed", slog.Any("error", err))
			}
		} else if deleted > 0 {
			logger.Info("session sweep deleted expired sessions", slog.Int64("deleted", deleted))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
