package recording

import (
	"net/http"

	"github.com/FacileStudio/Echo/apps/api/internal/authcontext"
	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

type handler struct{ recording *Recording }

type egressResponse struct {
	EgressID string `json:"egressId"`
	Status   string `json:"status,omitempty"`
}

// RegisterRoutes mounts the record endpoints. Both mutate a room's state and
// cost real disk on the host, so they sit behind requireAuth plus the
// service-level owner check.
func RegisterRoutes(router chi.Router, recording *Recording, requireAuth func(http.Handler) http.Handler) {
	h := handler{recording: recording}
	router.With(requireAuth).Post("/rooms/{slug}/record/start", h.start)
	router.With(requireAuth).Post("/rooms/{slug}/record/stop", h.stop)
}

func (h handler) start(w http.ResponseWriter, r *http.Request) {
	identity, ok := authcontext.From(r.Context())
	if !ok {
		httpjson.WriteError(w, errors.Unauthorized("not authenticated"))
		return
	}
	info, err := h.recording.Start(r.Context(), chi.URLParam(r, "slug"), identity.UserID)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, toResponse(info))
}

func (h handler) stop(w http.ResponseWriter, r *http.Request) {
	identity, ok := authcontext.From(r.Context())
	if !ok {
		httpjson.WriteError(w, errors.Unauthorized("not authenticated"))
		return
	}
	info, err := h.recording.Stop(r.Context(), chi.URLParam(r, "slug"), identity.UserID)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, toResponse(info))
}

func toResponse(info media.EgressInfo) egressResponse {
	return egressResponse{EgressID: info.EgressID, Status: info.Status}
}
