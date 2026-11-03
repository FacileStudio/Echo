package integrations

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"sort"
)

// Keys holds each integrated app's shared secret.
type Keys struct {
	apps    []string
	secrets map[string]string
}

// NewKeys builds Keys from the app-to-secret map env.ParseIntegrationKeys
// returns.
func NewKeys(secrets map[string]string) Keys {
	apps := make([]string, 0, len(secrets))
	for app := range secrets {
		apps = append(apps, app)
	}
	sort.Strings(apps)
	return Keys{apps: apps, secrets: secrets}
}

// AppFor returns the app a presented secret belongs to. Every secret is
// compared, in constant time over equal-length digests, so neither the
// answer's timing nor its length tells a caller how close a guess came.
func (k Keys) AppFor(presented string) (string, bool) {
	sum := sha256.Sum256([]byte(presented))
	found := ""
	for _, app := range k.apps {
		want := sha256.Sum256([]byte(k.secrets[app]))
		if subtle.ConstantTimeCompare(sum[:], want[:]) == 1 {
			found = app
		}
	}
	return found, found != ""
}

// Sign returns the lowercase hex HMAC-SHA256 of body under the app's secret,
// the value sent in X-Echo-Signature.
func (k Keys) Sign(app string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(k.secrets[app]))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
