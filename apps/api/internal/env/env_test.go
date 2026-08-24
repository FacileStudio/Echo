package env

import "testing"

func baseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("TRANSCRIBER_TOKEN", "test-token")
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
