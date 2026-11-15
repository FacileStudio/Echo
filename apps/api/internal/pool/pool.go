// Package pool sends Echo's queued call events to the Antenne pool.
//
// Echo only emits: it declares the call channels when it registers and
// listens to nothing. Queued rows wait in antenne_outbox while the pool is
// down, and the drainer resumes in id order once the connection is back.
package pool

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/internal/bus"
	antenne "github.com/FacileStudio/antenne-client/go"
	"github.com/FacileStudio/antenne-client/go/outbox"
)

// Client keeps the pool connection alive and drains the outbox onto it.
type Client struct {
	client  *antenne.Client
	drainer *outbox.Drainer
	logger  *slog.Logger
}

// NewClient declares the call channels on cfg. Call Run to connect.
func NewClient(cfg antenne.Config, db *sql.DB, logger *slog.Logger) *Client {
	for _, channel := range bus.Channels() {
		cfg.Events.Emit = append(cfg.Events.Emit, string(channel))
	}
	client := antenne.NewClient(&cfg,
		antenne.WithPersistentConnect(),
		antenne.WithOnConnect(func() { logger.Info("pool: connected") }),
		antenne.WithOnError(func(err error) {
			var transient *antenne.TransientError
			if errors.As(err, &transient) {
				logger.Warn("pool: reconnecting", slog.Int("attempt", transient.Attempt), slog.Any("error", transient.Err))
				return
			}
			logger.Error("pool: error", slog.Any("error", err))
		}),
	)
	emit := func(channel string, payload any) error {
		if err := client.EmitNow(channel, payload); err != nil {
			return err
		}
		logger.Info("pool: published", slog.String("channel", channel))
		return nil
	}
	drainer := outbox.NewDrainer(db, emit, outbox.WithOnError(func(err error) {
		if errors.Is(err, antenne.ErrNotConnected) {
			logger.Debug("pool: outbox waits for the connection")
			return
		}
		logger.Warn("pool: outbox drain failed", slog.Any("error", err))
	}))
	return &Client{client: client, drainer: drainer, logger: logger}
}

// Run connects, drains the outbox until ctx ends, then disconnects. The
// antenne client retries the first connection and every later one itself.
func (c *Client) Run(ctx context.Context) {
	if err := c.client.Connect(ctx); err != nil {
		c.logger.Error("pool: connect", slog.Any("error", err))
	}
	c.drainer.Run(ctx)
	c.client.Disconnect()
}
