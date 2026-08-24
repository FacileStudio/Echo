package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/database"
	"github.com/FacileStudio/Echo/apps/api/internal/env"
	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/modules/auth"
	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/local"
	"github.com/FacileStudio/porte/oidc"
	portepg "github.com/FacileStudio/porte/pg"
	"github.com/FacileStudio/porte/session"

	"github.com/FacileStudio/tronc/health"
	"github.com/FacileStudio/tronc/healthcheck"
	"github.com/FacileStudio/tronc/httpx"
	"github.com/FacileStudio/tronc/logger"
	troncmiddleware "github.com/FacileStudio/tronc/middleware"
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

	router := httpx.NewRouter(httpx.Config{
		Logger: log,
		CORS: troncmiddleware.CORSConfig{
			AllowedOrigins:   cfg.CORSAllowedOrigins,
			AllowCredentials: true,
		},
	})
	health.Mount(router)
	router.Route("/api", func(r chi.Router) {
		authKit.Mount(r, cfg.Porte.SSOOnly)
		roomsService := rooms.NewService(db, mediaService)
		rooms.RegisterRoutes(r, roomsService, authKit.service, authKit.requireAuth)
	})

	go sweepSessions(shutdown, authKit.sessions, log)

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

type authKit struct {
	sessions    *session.Manager
	kit         *oidc.Kit
	service     *auth.Service
	requireAuth func(http.Handler) http.Handler
}

func (k *authKit) Mount(r chi.Router, ssoOnly bool) {
	k.sessions.Mount(r)
	k.kit.Mount(r)
	auth.RegisterRoutes(r, k.service, ssoOnly, k.requireAuth)
}

func setupAuth(shutdown context.Context, cfg env.Config, db *gorm.DB, log *slog.Logger) (*authKit, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	identityStore := portepg.New(sqlDB)
	users := auth.NewUserStore(db)

	sessions, err := session.New(cfg.Porte, session.Deps{
		Sessions: identityStore.Sessions(),
		Logger:   log,
	})
	if err != nil {
		return nil, err
	}

	kit, err := oidc.New(shutdown, cfg.Porte, oidc.Deps{
		Users:       users,
		Identities:  identityStore.Identities(),
		Sessions:    sessions,
		Codes:       identityStore.LoginCodes(),
		Logger:      log,
		ConfigExtra: auth.ConfigExtra(cfg.AllowRegistration),
	})
	if err != nil {
		return nil, err
	}

	passwords, err := local.New(local.Config{AllowRegistration: cfg.AllowRegistration}, local.Deps{
		Users:      users,
		Identities: identityStore.Identities(),
		Sessions:   sessions,
		Logger:     log,
		Count:      users.CountUsers,
	})
	if err != nil {
		return nil, err
	}
	if kit.Enabled() {
		log.Info("single sign-on enabled",
			slog.String("issuer", cfg.Porte.Issuer),
			slog.Bool("sso_only", cfg.Porte.SSOOnly))
	}

	service := auth.NewService(db, passwords)
	return &authKit{
		sessions:    sessions,
		kit:         kit,
		service:     service,
		requireAuth: middleware.RequireAuth(sessions, service),
	}, nil
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
