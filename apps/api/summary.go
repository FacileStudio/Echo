package main

import (
	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/internal/env"
	"github.com/FacileStudio/Echo/apps/api/internal/summarize"
	"github.com/FacileStudio/porte/exchange"
)

// newSummarizer builds the AI summary client when SSO is on and BULLE_URL is
// set, and nil otherwise. Echo calls Bulle as the room owner, with a token
// Registre exchanges from the owner's stored grant for audience bulle.
func newSummarizer(cfg env.Config, auth *authKit, log *slog.Logger) (*summarize.Summarizer, error) {
	if !cfg.Porte.Enabled() || cfg.BulleURL == "" {
		log.Warn("AI summaries are off: they need OIDC_ISSUER and BULLE_URL")
		return nil, nil
	}
	exchanger, err := exchange.New(exchange.Config{
		Issuer:       cfg.Porte.Issuer,
		ClientID:     cfg.Porte.ClientID,
		ClientSecret: cfg.Porte.ClientSecret,
		Identities:   auth.identities,
		Logger:       log,
	})
	if err != nil {
		return nil, err
	}
	return summarize.New(cfg.BulleURL, exchanger, log)
}
