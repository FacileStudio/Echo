package transcripts

import (
	"crypto/subtle"
	"net/http"
	"strings"

	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

const bearerPrefix = "Bearer "

type appendRequest struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

type handler struct {
	service *Service
	token   string
}

// RegisterRoutes mounts the transcriber ingestion endpoint behind a static
// bearer token — the transcriber is infrastructure, not a user, so it carries
// no session and no CSRF header.
func RegisterRoutes(router chi.Router, service *Service, token string) {
	router.Post("/rooms/{slug}/transcript", handler{service: service, token: token}.append)
}

// authorized compares the bearer token in constant time.
func (h handler) authorized(r *http.Request) bool {
	given := ""
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, bearerPrefix) {
		given = auth[len(bearerPrefix):]
	}
	return subtle.ConstantTimeCompare([]byte(given), []byte(h.token)) == 1
}

func (h handler) append(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		httpjson.WriteError(w, troncerrors.Unauthorized("invalid transcriber token"))
		return
	}
	var req appendRequest
	if err := httpjson.DecodeJSON(w, r, &req); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	if req.Text == "" {
		httpjson.WriteError(w, troncerrors.Invalid("text is required"))
		return
	}
	speaker := req.Speaker
	if speaker == "" {
		speaker = "unknown"
	}
	if err := h.service.Append(r.Context(), chi.URLParam(r, "slug"), speaker, req.Text); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
