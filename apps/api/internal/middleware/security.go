package middleware

import (
	"net/http"
	"net/url"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
)

// SecurityConfig extends tronc's security headers with what a call needs: the
// media permissions (tronc denies camera and microphone unless they are listed), a connect-src for the LiveKit server, a media-src for the
// recording player's blob URL, the blob worker and WebAssembly that the on-device
// speech recognizer runs in, and the script hashes of the built client.
// liveKitURL is the public LiveKit address; a value that does not parse adds
// nothing, and the policy only reports until Echo enforces it.
func SecurityConfig(liveKitURL string, scriptSources []string) troncmiddleware.SecurityConfig {
	sources := map[string][]string{
		"connect-src": append(liveKitOrigins(liveKitURL), "blob:"),
		"media-src":   {"'self'", "blob:"},
		"worker-src":  {"'self'", "blob:"},
		"script-src":  append([]string{"'wasm-unsafe-eval'"}, scriptSources...),
	}
	return troncmiddleware.SecurityConfig{Allow: []string{"camera", "microphone", "display-capture"}, CSPSources: sources}
}

func liveKitOrigins(raw string) []string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return nil
	}
	switch parsed.Scheme {
	case "wss", "https":
		return []string{"wss://" + parsed.Host, "https://" + parsed.Host}
	case "ws", "http":
		return []string{"ws://" + parsed.Host, "http://" + parsed.Host}
	}
	return nil
}

// EmbedFrameable lets the given origins frame the route it is mounted on and
// nobody else: it extends the chassis frame-ancestors, which is 'none', and
// drops X-Frame-Options only because an origin is now allowed. With no origins
// it changes nothing, so the embed stays unframeable.
func EmbedFrameable(origins []string) func(http.Handler) http.Handler {
	if len(origins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return troncmiddleware.SecurityOverride(troncmiddleware.SecurityOverrideConfig{
		CSPSources: map[string][]string{"frame-ancestors": origins},
	})
}
