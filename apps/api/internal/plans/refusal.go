package plans

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/FacileStudio/porte/forfait"
	"github.com/FacileStudio/tronc/httpjson"
)

// refusalBody is the suite error envelope a plan refusal answers with, plus
// what the client needs to offer an upgrade.
type refusalBody struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Key        string `json:"key"`
	Limit      int64  `json:"limit"`
	UpgradeURL string `json:"upgrade_url,omitempty"`
}

// WriteError writes a plan refusal as HTTP 402 and every other error the way
// tronc does. upgrade_url is kept only when it is an https URL with a host and
// no credentials, because the client renders it as a link.
func WriteError(w http.ResponseWriter, err error) {
	var refusal *forfait.LimitError
	if !errors.As(err, &refusal) {
		httpjson.WriteError(w, err)
		return
	}
	body := refusalBody{
		Code:    "payment_required",
		Message: "your plan does not allow this",
		Key:     refusal.Key,
		Limit:   refusal.Limit,
	}
	if safeUpgradeURL(refusal.UpgradeURL) {
		body.UpgradeURL = refusal.UpgradeURL
	}
	httpjson.WriteJSON(w, http.StatusPaymentRequired, map[string]any{"error": body})
}

func safeUpgradeURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return raw != "" && err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}
