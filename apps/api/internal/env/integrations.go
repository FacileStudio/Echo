package env

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	minIntegrationSecret = 32
	maxAppName           = 32
)

// ParseIntegrationKeys reads ECHO_INTEGRATION_KEYS, a comma-separated list of
// app:secret pairs. An empty value turns the integration API off. A malformed
// one refuses the boot, because a typo here would lock an app out silently.
func ParseIntegrationKeys(raw string) (map[string]string, error) {
	keys := map[string]string{}
	for _, pair := range splitList(raw) {
		app, secret, ok := strings.Cut(pair, ":")
		if !ok || !validAppName(app) {
			return nil, fmt.Errorf("ECHO_INTEGRATION_KEYS: %q is not app:secret with a lowercase app name", app)
		}
		if len(secret) < minIntegrationSecret {
			return nil, fmt.Errorf(
				"ECHO_INTEGRATION_KEYS: the secret for %q must be at least %d characters; "+
					"generate one with: openssl rand -hex 32", app, minIntegrationSecret)
		}
		if _, dup := keys[app]; dup {
			return nil, fmt.Errorf("ECHO_INTEGRATION_KEYS: %q is listed twice", app)
		}
		keys[app] = secret
	}
	return keys, nil
}

// ParseEmbedOrigins reads ECHO_EMBED_ORIGINS, a comma-separated list of
// origins allowed to frame the embed route. Each must be a bare origin, the
// only form CSP frame-ancestors matches the way an operator expects.
func ParseEmbedOrigins(raw string) ([]string, error) {
	var origins []string
	for _, origin := range splitList(raw) {
		parsed, err := url.Parse(origin)
		bare := err == nil && parsed.Host != "" && parsed.Path == "" && parsed.RawQuery == ""
		if !bare || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return nil, fmt.Errorf("ECHO_EMBED_ORIGINS: %q is not an origin such as https://app.example.com", origin)
		}
		origins = append(origins, parsed.Scheme+"://"+parsed.Host)
	}
	return origins, nil
}

func validAppName(app string) bool {
	if app == "" || len(app) > maxAppName {
		return false
	}
	return strings.IndexFunc(app, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) < 0
}

func splitList(raw string) []string {
	var items []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
