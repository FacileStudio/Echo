package main

import (
	"context"
	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/internal/bus"
	"github.com/FacileStudio/Echo/apps/api/internal/env"
	"github.com/FacileStudio/Echo/apps/api/internal/pool"
	"gorm.io/gorm"
)

// setupPublishing returns the call outbox and the loop that drains it to the
// Antenne pool. Without ANTENNE_URL there is neither and Echo publishes nothing.
func setupPublishing(cfg env.Config, db *gorm.DB, log *slog.Logger) (*bus.Outbox, []func(context.Context), error) {
	if cfg.Antenne == nil {
		log.Info("pool: ANTENNE_URL is not set, call events are not published")
		return nil, nil, nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	client := pool.NewClient(*cfg.Antenne, sqlDB, log)
	return bus.New(cfg.PublicURL), []func(context.Context){client.Run}, nil
}
