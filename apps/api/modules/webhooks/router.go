package webhooks

import (
	"encoding/json"
	"io"
	"net/http"

	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

const maxBodyBytes = 1 << 20

type handler struct{ webhooks *Webhooks }

// RegisterRoutes mounts the LiveKit webhook endpoint. It sits OUTSIDE the
// /api group on purpose: no cookies, no CSRF, only LiveKit's signed JWT.
func (w *Webhooks) RegisterRoutes(router chi.Router) {
	router.Post("/livekit/webhook", handler{webhooks: w}.receive)
}

// receive handles a LiveKit webhook POST.
func (h handler) receive(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		httpjson.WriteError(w, troncerrors.Invalid("unreadable body"))
		return
	}
	if err := h.webhooks.verify(r.Header.Get("Authorization"), body); err != nil {
		httpjson.WriteError(w, troncerrors.Unauthorized("invalid webhook signature"))
		return
	}
	var p payload
	if err := json.Unmarshal(body, &p); err != nil || p.Event == "" {
		httpjson.WriteError(w, troncerrors.Invalid("malformed webhook event"))
		return
	}
	if err := h.webhooks.dispatch(r.Context(), &p); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
