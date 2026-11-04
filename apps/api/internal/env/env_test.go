package env

import (
	"net/netip"
	"strings"
	"testing"
)

func baseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("TRANSCRIBER_TOKEN", strings.Repeat("t", minTranscriberToken))
}

func TestSSOOnlyWithoutIssuerRefusesToLoad(t *testing.T) {
	baseEnv(t)
	t.Setenv("SSO_ONLY", "true")
	t.Setenv("OIDC_ISSUER", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when SSO_ONLY is set with no OIDC_ISSUER")
	}
}

func TestDisabledOIDCIsValid(t *testing.T) {
	baseEnv(t)
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("SSO_ONLY", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected a valid config, got %v", err)
	}
	if cfg.Porte.Enabled() {
		t.Fatal("expected OIDC to be disabled")
	}
	if !cfg.AllowRegistration {
		t.Fatal("expected ALLOW_REGISTRATION to default to true")
	}
	if cfg.RecordingsDir != "/recordings" {
		t.Fatalf("RECORDINGS_DIR = %q, want the /recordings default", cfg.RecordingsDir)
	}
}

func TestMissingTranscriberTokenRefusesToLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("TRANSCRIBER_TOKEN", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("SSO_ONLY", "false")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when TRANSCRIBER_TOKEN is unset: ingestion would be unauthenticated")
	}
}

func TestShortTranscriberTokenRefusesToLoad(t *testing.T) {
	baseEnv(t)
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("SSO_ONLY", "false")
	t.Setenv("TRANSCRIBER_TOKEN", strings.Repeat("x", minTranscriberToken-1))

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error: a guessable token makes the constant-time compare pointless")
	}
	if !strings.Contains(err.Error(), "openssl rand") {
		t.Fatalf("error = %q, want it to say how to generate a token", err)
	}
}

func TestTrustedProxiesDefaultToPrivateAndCloudflare(t *testing.T) {
	baseEnv(t)
	t.Setenv("TRUSTED_PROXIES", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	trusts := func(addr string) bool {
		for _, prefix := range cfg.TrustedProxies {
			if prefix.Contains(netip.MustParseAddr(addr)) {
				return true
			}
		}
		return false
	}
	if !trusts("10.0.1.5") || !trusts("162.158.1.2") || !trusts("2606:4700::1") || trusts("203.0.113.9") {
		t.Fatal("the default does not trust exactly the private ranges and Cloudflare")
	}
	t.Setenv("TRUSTED_PROXIES", "none")
	if cfg, err = Load(); err != nil || len(cfg.TrustedProxies) != 0 {
		t.Fatalf("TRUSTED_PROXIES=none = %v, %v; want no proxy trusted", cfg.TrustedProxies, err)
	}
	t.Setenv("TRUSTED_PROXIES", "not-a-range")
	if _, err := Load(); err == nil {
		t.Fatal("a malformed TRUSTED_PROXIES was accepted")
	}
}
