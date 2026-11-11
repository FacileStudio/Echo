package history

import (
	"net/http"
	"strconv"

	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/porte/kit"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

type handler struct{ service *Service }

// caller resolves the authenticated identity, writing the 401 itself when
// there is none so each handler stays a straight line.
func caller(w http.ResponseWriter, r *http.Request) (kit.Identity, bool) {
	identity, ok := kit.FromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, troncerrors.Unauthorized("not authenticated"))
	}
	return identity, ok
}

func (h handler) list(w http.ResponseWriter, r *http.Request) {
	identity, ok := caller(w, r)
	if !ok {
		return
	}
	calls, err := h.service.List(r.Context(), chi.URLParam(r, "slug"), identity.UserID)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, calls)
}

func (h handler) detail(w http.ResponseWriter, r *http.Request) {
	identity, ok := caller(w, r)
	if !ok {
		return
	}
	detail, err := h.service.Detail(r.Context(), chi.URLParam(r, "id"), identity.UserID)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, detail)
}

func (h handler) summarize(w http.ResponseWriter, r *http.Request) {
	identity, ok := caller(w, r)
	if !ok {
		return
	}
	summary, err := h.service.Summarize(r.Context(), chi.URLParam(r, "id"), identity.UserID)
	if err != nil {
		plans.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, summary)
}

func (h handler) meetings(w http.ResponseWriter, r *http.Request) {
	identity, ok := caller(w, r)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	meetings, err := h.service.Meetings(r.Context(), identity.UserID, page, size)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, meetings)
}
