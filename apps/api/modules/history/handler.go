package history

import (
	"net/http"

	"github.com/FacileStudio/Echo/apps/api/internal/authcontext"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

type handler struct{ service *Service }

// caller resolves the authenticated identity, writing the 401 itself when
// there is none so each handler stays a straight line.
func caller(w http.ResponseWriter, r *http.Request) (authcontext.Identity, bool) {
	identity, ok := authcontext.From(r.Context())
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
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, summary)
}

// recording streams the call's MP4. ServeContent handles range requests, so
// a browser can seek without pulling the whole file.
func (h handler) recording(w http.ResponseWriter, r *http.Request) {
	identity, ok := caller(w, r)
	if !ok {
		return
	}
	file, name, err := h.service.RecordingFile(r.Context(), chi.URLParam(r, "id"), identity.UserID)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		httpjson.WriteError(w, errNoRecording)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, name, info.ModTime(), file)
}
