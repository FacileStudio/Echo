package webhooks

import (
	"net/http"

	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
	"github.com/livekit/protocol/webhook"
)

const maxBodyBytes = 1 << 20

type handler struct{ webhooks *Webhooks }

// RegisterRoutes mounts the LiveKit webhook endpoint. It sits OUTSIDE the
// /api group on purpose: no cookies, no CSRF, only LiveKit's signed JWT.
func (w *Webhooks) RegisterRoutes(router chi.Router) {
	router.Post("/livekit/webhook", handler{webhooks: w}.receive)
}

// receive handles a LiveKit webhook POST.
//
// Reading, verifying and decoding are all the protocol SDK's job. It resolves
// the secret from the token's issuer, checks the signature and the sha256
// body claim in constant time, and unmarshals with protojson — which is the
// only decoder that agrees with the wire format LiveKit actually sends.
//
// The body cap goes on before the call, because the SDK reads the whole body
// itself.
func (h handler) receive(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	event, err := webhook.ReceiveWebhookEvent(r, h.webhooks.keys)
	if err != nil {
		httpjson.WriteError(w, troncerrors.Unauthorized("invalid webhook request"))
		return
	}
	if event.GetEvent() == "" {
		httpjson.WriteError(w, troncerrors.Invalid("malformed webhook event"))
		return
	}
	if err := h.webhooks.dispatch(r.Context(), event); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
