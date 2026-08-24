package env

import "testing"

func TestSSOOnlyWithoutIssuerRefusesToLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("SSO_ONLY", "true")
	t.Setenv("OIDC_ISSUER", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when SSO_ONLY is set with no OIDC_ISSUER")
	}
}

func TestDisabledOIDCIsValid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
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
}
