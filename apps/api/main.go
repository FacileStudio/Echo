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
	"github.com/FacileStudio/Echo/apps/api/internal/summarize"
	"github.com/FacileStudio/Echo/apps/api/modules/auth"
	"github.com/FacileStudio/Echo/apps/api/modules/history"
	"github.com/FacileStudio/Echo/apps/api/modules/recording"
	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	"github.com/FacileStudio/Echo/apps/api/modules/transcripts"
	"github.com/FacileStudio/Echo/apps/api/modules/webhooks"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/local"
	"github.com/FacileStudio/porte/oidc"
	portepg "github.com/FacileStudio/porte/pg"
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

	router := httpx.NewRouter(httpx.Config{
		Logger: log,
		CORS: troncmiddleware.CORSConfig{
			AllowedOrigins:   cfg.CORSAllowedOrigins,
			AllowCredentials: true,
		},
	})
	health.Mount(router)
	webhooks.New(db, troncenv.String("LIVEKIT_API_KEY", ""), mediaService.Secret()).RegisterRoutes(router)
	router.Route("/api", func(r chi.Router) {
		r.Use(extendWriteDeadline("/summary", summaryWriteBudget))
		authKit.Mount(r, cfg.Porte.SSOOnly)
		roomsService := rooms.NewService(db, mediaService)
		rooms.RegisterRoutes(r, roomsService, authKit.service, authKit.requireAuth)
		recording.RegisterRoutes(r, recording.NewRecording(roomsService, mediaService), authKit.requireAuth)
		historyService := history.NewService(db, roomsService, summarize.NewFromEnv(), cfg.RecordingsDir)
		history.RegisterRoutes(r, historyService, authKit.requireAuth)
		transcripts.RegisterRoutes(r, transcripts.NewService(db), cfg.TranscriberToken)
	})

	go sweepSessions(shutdown, authKit.sessions, log)

	// SPA catch-all must stay LAST — anything registered after it is unreachable.
	clientDir := spa.DirFromEnv()
	if spa.Available(clientDir) {
		router.Handle("/*", spa.Handler(spa.Config{Dir: clientDir}))
		log.Info("serving client", slog.String("dir", clientDir))
	}

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

// summaryWriteBudget is what one route is allowed to spend writing its
// response. It has to clear the summarizer's own upstream timeout with room
// to spare: the server's 30s WriteTimeout used to kill a slow summary as a
// network error for the owner, after the row was committed and the model
// billed.
const summaryWriteBudget = 3 * time.Minute

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
