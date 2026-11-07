package main

import (
	"context"
	"net/http"

	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/internal/env"
	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/modules/auth"
	"github.com/FacileStudio/porte/oidc"
	"github.com/FacileStudio/porte/session"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// authKit is porte's session manager and OIDC kit plus Echo's account service.
type authKit struct {
	sessions    *session.Manager
	kit         *oidc.Kit
	service     *auth.Service
	requireAuth func(http.Handler) http.Handler
}

// Mount wires the auth routes and, first, porte's Optional middleware over
// the whole group. Without it porte's identity exists only inside RequireAuth,
// so every public route that serves both a guest and a signed-in caller (the
// room token endpoint, room create, room get) sees everyone as anonymous.
// That is what minted guest grants for logged-in users and created every room
// without an owner. Optional must be registered before any route on this
// router: chi refuses a Use after the first handler.
func (k *authKit) Mount(r chi.Router) {
	r.Use(k.sessions.Optional)
	k.sessions.Mount(r)
	k.kit.Mount(r)
	auth.RegisterRoutes(r, k.service, k.requireAuth)
}

// setupAuth builds porte's session manager and OIDC kit. Sign-in goes through
// Registre only: there is no password kit, so with no OIDC_ISSUER nobody can
// sign in and every room is a guest room.
func setupAuth(shutdown context.Context, cfg env.Config, db *gorm.DB, log *slog.Logger) (*authKit, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	identityStore, err := newPorteStore(sqlDB)
	if err != nil {
		return nil, err
	}
	resealInBackground(shutdown, identityStore, log)

	sessions, err := session.New(cfg.Porte, session.Deps{
		Sessions: identityStore.Sessions(),
		Logger:   log,
	})
	if err != nil {
		return nil, err
	}

	kit, err := oidc.New(shutdown, cfg.Porte, oidc.Deps{
		Users:      auth.NewUserStore(db),
		Identities: identityStore.Identities(),
		Sessions:   sessions,
		Codes:      identityStore.LoginCodes(),
		Logger:     log,
	})
	if err != nil {
		return nil, err
	}
	if kit.Enabled() {
		log.Info("single sign-on enabled", slog.String("issuer", cfg.Porte.Issuer))
	} else {
		log.Warn("OIDC_ISSUER is not set: nobody can sign in, and every room is a guest room")
	}

	service := auth.NewService(db)
	return &authKit{
		sessions:    sessions,
		kit:         kit,
		service:     service,
		requireAuth: middleware.RequireAuth(sessions, service),
	}, nil
}
