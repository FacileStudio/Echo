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
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/session"
	"github.com/FacileStudio/tronc/healthcheck"
	"github.com/FacileStudio/tronc/logger"
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

	app, err := buildServer(shutdown, cfg, db, log)
	if err != nil {
		log.Error("startup", slog.Any("error", err))
		return 1
	}
	app.start(shutdown)

	return serve(app.router, cfg.Port, shutdown.Done(), log)
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
