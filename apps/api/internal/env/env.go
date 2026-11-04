package env

import (
	"fmt"
	"net/netip"
	"os"
	"strings"

	"github.com/FacileStudio/porte"
	troncenv "github.com/FacileStudio/tronc/env"
)

// Config carries the tronc core settings plus Echo auth configuration.
type Config struct {
	troncenv.Core

	AllowRegistration bool
	Porte             porte.Config

	// TranscriberToken authenticates the transcriber worker against the
	// transcript ingestion endpoint. It is required: an open ingestion
	// endpoint is worse than a refused boot.
	TranscriberToken string

	// RecordingsDir is the directory the egress volume is mounted at, and
	// the root every stored recording path is resolved against.
	RecordingsDir string

	// IntegrationKeys maps each suite app allowed on the integration API to
	// its shared secret. Empty means the integration API refuses everyone.
	IntegrationKeys map[string]string

	// EmbedOrigins are the origins allowed to frame /embed/{room}.
	EmbedOrigins []string

	// TrustedProxies are the hops whose X-Forwarded-For tronc's RealIP
	// believes, from TRUSTED_PROXIES.
	TrustedProxies []netip.Prefix
}

// defaultTrustedProxies is what an unset TRUSTED_PROXIES means here.
// echo.facile.studio is served through Cloudflare and then Traefik, so the
// forwarded chain is [visitor, cloudflare edge] and both sets must be
// trusted for RemoteAddr to name the visitor rather than the edge.
const defaultTrustedProxies = "private,cloudflare"

// trustedProxies reads TRUSTED_PROXIES with tronc's grammar: CIDR blocks,
// addresses, the names private and cloudflare, or none.
func trustedProxies() ([]netip.Prefix, error) {
	if strings.TrimSpace(os.Getenv("TRUSTED_PROXIES")) != "" {
		return troncenv.TrustedProxies()
	}
	var prefixes []netip.Prefix
	for _, name := range strings.Split(defaultTrustedProxies, ",") {
		prefixes = append(prefixes, troncenv.TrustedProxySets[name]...)
	}
	return prefixes, nil
}

// minTranscriberToken is the shortest token the API will boot with. The
// ingestion endpoint is unthrottled, so a constant-time compare is worth
// nothing unless the search space is out of reach of an online guess.
const minTranscriberToken = 32

// checkTranscriberToken refuses a token nobody would have to guess.
func checkTranscriberToken(token string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("TRANSCRIBER_TOKEN must be set: it authenticates transcript ingestion")
	}
	if len(token) < minTranscriberToken {
		return fmt.Errorf(
			"TRANSCRIBER_TOKEN must be at least %d characters; generate one with: openssl rand -hex 32",
			minTranscriberToken,
		)
	}
	return nil
}

// Load reads and validates every environment variable the API needs.
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
	if err := cfg.loadServiceSecrets(); err != nil {
		return Config{}, err
	}
	cfg.RecordingsDir = troncenv.String("RECORDINGS_DIR", "/recordings")
	if cfg.Porte.SSOOnly && !cfg.Porte.Enabled() {
		return Config{}, fmt.Errorf("SSO_ONLY=true with no OIDC_ISSUER leaves no way to sign in")
	}
	if err := cfg.Porte.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// loadServiceSecrets reads what other services present to Echo: the
// transcriber's token, the integrated apps' secrets, the origins allowed to
// frame the embed route, and the proxies whose forwarding headers count.
func (cfg *Config) loadServiceSecrets() error {
	cfg.TranscriberToken = troncenv.String("TRANSCRIBER_TOKEN", "")
	if err := checkTranscriberToken(cfg.TranscriberToken); err != nil {
		return err
	}
	keys, err := ParseIntegrationKeys(troncenv.String("ECHO_INTEGRATION_KEYS", ""))
	if err != nil {
		return err
	}
	origins, err := ParseEmbedOrigins(troncenv.String("ECHO_EMBED_ORIGINS", ""))
	if err != nil {
		return err
	}
	cfg.IntegrationKeys, cfg.EmbedOrigins = keys, origins
	cfg.TrustedProxies, err = trustedProxies()
	return err
}
