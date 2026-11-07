package main

import (
	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/internal/env"
	"github.com/FacileStudio/Echo/apps/api/modules/deletions"
	"github.com/FacileStudio/porte/directory"
	"gorm.io/gorm"
)

// setupDeletions follows Registre's GDPR deletions with the same client
// credentials as the plan snapshot. It returns nil without OIDC: there is no
// Registre to follow. Echo runs one replica, so this is the one poller.
func setupDeletions(cfg env.Config, db *gorm.DB, log *slog.Logger) *deletions.Deletions {
	if !cfg.Porte.Enabled() {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Error("deletion receiver not started", slog.Any("error", err))
		return nil
	}
	store, err := newPorteStore(sqlDB)
	if err != nil {
		log.Error("deletion receiver not started", slog.Any("error", err))
		return nil
	}
	client := directory.Client{
		Issuer:       cfg.Porte.Issuer,
		ClientID:     cfg.Porte.ClientID,
		ClientSecret: cfg.Porte.ClientSecret,
	}
	return deletions.New(db, client, store.EraseIdentity, cfg.RecordingsDir, log)
}
