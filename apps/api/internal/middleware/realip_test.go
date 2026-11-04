package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/env"
	"github.com/FacileStudio/tronc/httpx"
)

func behindCloudflare(t *testing.T) http.Handler {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("TRANSCRIBER_TOKEN", "0123456789abcdef0123456789abcdef")
	t.Setenv("TRUSTED_PROXIES", "")
	cfg, err := env.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	router := httpx.NewRouter(httpx.Config{TrustedProxies: cfg.TrustedProxies})
	router.With(NewRateLimiter(1, 1).Middleware).Post("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return router
}

func viaTraefik(handler http.Handler, peer, forwarded string) int {
	return viaChain(handler, peer, forwarded, "192.0.2.250")
}

func viaChain(handler http.Handler, peer, forwarded, connecting string) int {
	request := httptest.NewRequest(http.MethodPost, "/token", nil)
	request.RemoteAddr = peer
	if forwarded != "" {
		request.Header.Set("X-Forwarded-For", forwarded)
	}
	if connecting != "" {
		request.Header.Set("Cf-Connecting-Ip", connecting)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Code
}

func TestVisitorsBehindOneCloudflareEdgeGetTheirOwnBuckets(t *testing.T) {
	handler := behindCloudflare(t)
	const traefik, edge = "10.0.1.5:443", "162.158.1.2"
	if code := viaTraefik(handler, traefik, "203.0.113.9, "+edge); code != http.StatusNoContent {
		t.Fatalf("first visitor answered %d", code)
	}
	if code := viaTraefik(handler, traefik, "198.51.100.4, "+edge); code != http.StatusNoContent {
		t.Fatalf("a second visitor behind the same edge shared the first one's bucket: %d", code)
	}
	if code := viaTraefik(handler, traefik, "203.0.113.9, 172.70.108.91"); code != http.StatusTooManyRequests {
		t.Fatalf("one visitor moving to another edge got a fresh bucket: %d", code)
	}
}

func TestAForgedChainCannotBuyAFreshBucket(t *testing.T) {
	handler := behindCloudflare(t)
	const traefik, edge = "10.0.1.5:443", "162.158.1.2"
	attacker := "198.51.100.66"
	if code := viaTraefik(handler, traefik, "7.7.7.7, "+attacker+", "+edge); code != http.StatusNoContent {
		t.Fatalf("first request answered %d", code)
	}
	if code := viaTraefik(handler, traefik, "8.8.8.8, "+attacker+", "+edge); code != http.StatusTooManyRequests {
		t.Fatalf("a seeded X-Forwarded-For hop bought a fresh bucket: %d", code)
	}
	if code := viaTraefik(handler, attacker+":5555", "1.1.1.1"); code != http.StatusTooManyRequests {
		t.Fatalf("a direct request with a forged header bought a fresh bucket: %d", code)
	}
}

func TestTheProductionChainKeysOnTheVisitor(t *testing.T) {
	handler := behindCloudflare(t)
	const traefik, edge = "172.18.0.5:43210", "162.158.1.2"
	for _, visitor := range []string{"203.0.113.9", "198.51.100.4", "2001:db8:5::1"} {
		if code := viaChain(handler, traefik, edge, visitor); code != http.StatusNoContent {
			t.Fatalf("visitor %s behind Traefik and one edge answered %d: visitors share a bucket", visitor, code)
		}
	}
	if code := viaChain(handler, traefik, "172.70.108.91", "203.0.113.9"); code != http.StatusTooManyRequests {
		t.Fatalf("a visitor seen through a second edge got a fresh bucket: %d", code)
	}
}

func TestAForgedConnectingIPOutsideTheCloudflareChainIsIgnored(t *testing.T) {
	handler := behindCloudflare(t)
	const traefik, attacker = "172.18.0.5:43210", "198.51.100.66"
	if code := viaChain(handler, traefik, attacker, "192.0.2.1"); code != http.StatusNoContent {
		t.Fatalf("first request answered %d", code)
	}
	if code := viaChain(handler, traefik, attacker, "192.0.2.2"); code != http.StatusTooManyRequests {
		t.Fatalf("a request Traefik received from a non-Cloudflare peer bought a bucket with its own header: %d", code)
	}
	if code := viaChain(handler, attacker+":5555", "162.158.1.2", "192.0.2.3"); code != http.StatusTooManyRequests {
		t.Fatalf("a direct request with a forged Cloudflare chain bought a fresh bucket: %d", code)
	}
	if code := viaChain(handler, traefik, "162.158.1.2", "not-an-ip"); code != http.StatusNoContent {
		t.Fatalf("an unparsable Cf-Connecting-Ip did not fall back to the peer: %d", code)
	}
}
