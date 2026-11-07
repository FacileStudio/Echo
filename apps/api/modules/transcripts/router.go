package transcripts

import (
	"net/http"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/internal/seat"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

const (
	maxBodyBytes     = 16 << 10
	linesPerMinute   = 90
	lineLimiterScope = "transcript-line"
)

type appendRequest struct {
	Text    string `json:"text"`
	StartMS int64  `json:"start_ms"`
	EndMS   int64  `json:"end_ms"`
}

type handler struct {
	service *Service
	keys    *seat.Keys
	limiter *middleware.RateLimiter
}

// RegisterRoutes mounts the transcript ingestion endpoint. The caller is a
// browser transcribing its own microphone: a session identifies a signed-in
// person, a seat key identifies a guest, and either way the speaker is the
// identity the server proved, never a field of the body.
func RegisterRoutes(router chi.Router, service *Service, keys *seat.Keys, limiter *middleware.RateLimiter) {
	router.Post("/rooms/{slug}/transcript", handler{service: service, keys: keys, limiter: limiter}.append)
}

func (h handler) append(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	identity, err := h.keys.Identity(r, slug)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	if !h.limiter.AllowKey(lineLimiterScope, slug+"|"+identity, linesPerMinute) {
		middleware.Refuse(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req appendRequest
	if err := httpjson.DecodeJSON(w, r, &req); err != nil {
		httpjson.WriteError(w, troncerrors.Invalid("the body must be a small JSON object"))
		return
	}
	err = h.service.Append(r.Context(), Utterance{
		Slug: slug, Identity: identity, Text: req.Text, StartMS: req.StartMS, EndMS: req.EndMS,
	})
	if err != nil {
		plans.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
