package env

import (
	"fmt"

	"github.com/FacileStudio/porte"
	troncenv "github.com/FacileStudio/tronc/env"
)

type Config struct {
	troncenv.Core

	AllowRegistration bool
	Porte             porte.Config
}

func Load() (Config, error) {
	core, err := troncenv.LoadCore()
	if err != nil {
		return Config{}, err
	}
	if core.Port < 1 || core.Port > 65535 {
		return Config{}, fmt.Errorf("PORT must be a valid TCP port")
	}

	cfg := Config{Core: core}

	if cfg.AllowRegistration, err = troncenv.Bool("ALLOW_REGISTRATION", true); err != nil {
		return Config{}, err
	}

	cfg.Porte = porte.Config{
		ClaimsScope:  troncenv.String("OIDC_CLAIMS_SCOPE", ""),
		Issuer:       troncenv.String("OIDC_ISSUER", ""),
		ClientID:     troncenv.String("OIDC_CLIENT_ID", ""),
		ClientSecret: troncenv.String("OIDC_CLIENT_SECRET", ""),
		RedirectURL:  troncenv.String("OIDC_REDIRECT_URL", ""),
		SuccessURL:   troncenv.String("OIDC_SUCCESS_URL", ""),
	}
	if cfg.Porte.SSOOnly, err = troncenv.Bool("SSO_ONLY", false); err != nil {
		return Config{}, err
	}
	if cfg.Porte.SSOOnly && !cfg.Porte.Enabled() {
		return Config{}, fmt.Errorf("SSO_ONLY=true with no OIDC_ISSUER leaves no way to sign in")
	}
	if err := cfg.Porte.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
