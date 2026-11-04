package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/calllimit"
	"github.com/FacileStudio/Echo/apps/api/internal/env"
	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/porte/directory"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// planKit is the plan enforcement Echo runs: the entitlements, the timers
// that end calls at their deadline, and the Registre syncer feeding both.
// syncer is nil when OIDC is off, and the plans then fail open forever.
type planKit struct {
	plans   *plans.Plans
	limiter *calllimit.Limiter
	syncer  *directory.Syncer
}

// setupPlans restores the persisted entitlements, re-arms the deadline of
// every open call from its stored started_at, and builds the syncer. Every
// applied snapshot re-evaluates the open calls, so a plan change reaches a
// call already running.
func setupPlans(ctx context.Context, cfg env.Config, db *gorm.DB, mediaService *media.Service, log *slog.Logger) (*planKit, error) {
	kit := &planKit{plans: plans.New(db, cfg.Porte.Issuer, log)}
	if err := kit.plans.Load(ctx); err != nil {
		return nil, err
	}
	kit.limiter = calllimit.New(ctx, db, kit.plans, mediaService, log)
	if err := kit.limiter.Rearm(ctx); err != nil {
		return nil, err
	}
	if !cfg.Porte.Enabled() {
		return kit, nil
	}
	kit.syncer = kit.plans.NewSyncer(plans.Credentials{
		Issuer:       cfg.Porte.Issuer,
		ClientID:     cfg.Porte.ClientID,
		ClientSecret: cfg.Porte.ClientSecret,
	})
	kit.syncer.Apply = func(ctx context.Context, snapshot directory.Snapshot) error {
		if err := kit.plans.Apply(ctx, snapshot); err != nil {
			return err
		}
		return kit.limiter.Rearm(ctx)
	}
	return kit, nil
}

// Mount serves Registre's sync nudge, outside /api like every suite app.
func (k *planKit) Mount(router chi.Router) {
	if k.syncer != nil {
		router.Method(http.MethodPost, directory.SyncPath, k.syncer.Handler())
	}
}

// reconcileEvery is how often open calls are compared with LiveKit's rooms.
const reconcileEvery = 5 * time.Minute

// Run pulls Registre's snapshot and reconciles calls with LiveKit until ctx
// ends.
func (k *planKit) Run(ctx context.Context, log *slog.Logger) {
	go k.reconcile(ctx, log)
	if k.syncer != nil {
		k.syncer.Run(ctx)
	}
}

// reconcile runs the limiter's reconcile at boot and then periodically, so
// sessions no webhook reported still get their deadline and call rows a lost
// room_finished left open get closed.
func (k *planKit) reconcile(ctx context.Context, log *slog.Logger) {
	ticker := time.NewTicker(reconcileEvery)
	defer ticker.Stop()
	for {
		if err := k.limiter.Reconcile(ctx); err != nil && ctx.Err() == nil {
			log.Warn("call limit: reconcile with livekit failed", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
