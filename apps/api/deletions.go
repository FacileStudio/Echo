package main

import (
	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/internal/env"
	"github.com/FacileStudio/Echo/apps/api/modules/deletions"
	"github.com/FacileStudio/porte/directory"
	"github.com/FacileStudio/porte/kit"
	portepg "github.com/FacileStudio/porte/pg"
)

// setupDeletions follows Registre's GDPR deletions with the same client
// credentials as the plan snapshot. It returns nil without OIDC: there is no
// Registre to follow. Echo runs one replica, so this is the one poller. It is
// built before the OIDC kit, which asks it whether a subject was erased, and
// its Client.Issuer is OIDC_ISSUER, the provider porte/oidc files identities
// under.
func setupDeletions(cfg env.Config, users *kit.Store, store *portepg.Store, log *slog.Logger) *deletions.Deletions {
	if !cfg.Porte.Enabled() {
		return nil
	}
	client := directory.Client{
		Issuer:       cfg.Porte.Issuer,
		ClientID:     cfg.Porte.ClientID,
		ClientSecret: cfg.Porte.ClientSecret,
	}
	identity := deletions.Identity{Erase: store.EraseIdentity, Holds: store.HoldsIdentity}
	return deletions.New(users, client, identity, cfg.RecordingsDir, log)
}
