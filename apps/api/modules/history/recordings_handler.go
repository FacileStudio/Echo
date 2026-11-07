package history

import (
	"net/http"

	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

type shareRequest struct {
	Shared bool `json:"shared"`
}

func (h handler) recordings(w http.ResponseWriter, r *http.Request) {
	identity, ok := caller(w, r)
	if !ok {
		return
	}
	list, err := h.service.Recordings(r.Context(), chi.URLParam(r, "id"), identity.UserID)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, list)
}

// recordingFile streams one recording behind the session. ServeContent
// answers range requests, so a browser can seek without pulling the whole
// file, and ?download=1 turns the same bytes into an attachment. The response
// is private and uncacheable: the bytes are only ever for the person asking.
func (h handler) recordingFile(w http.ResponseWriter, r *http.Request) {
	identity, ok := caller(w, r)
	if !ok {
		return
	}
	file, err := h.service.RecordingFile(r.Context(), chi.URLParam(r, "id"), identity.UserID)
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
	disposition := "inline"
	if r.URL.Query().Get("download") == "1" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", servedType(file.Type))
	w.Header().Set("Content-Disposition", disposition+`; filename="`+file.Download+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, file.Download, info.ModTime(), file)
}

func (h handler) share(w http.ResponseWriter, r *http.Request) {
	identity, ok := caller(w, r)
	if !ok {
		return
	}
	var req shareRequest
	if err := httpjson.DecodeJSON(w, r, &req); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	out, err := h.service.SetShared(r.Context(), chi.URLParam(r, "id"), identity.UserID, req.Shared)
	if err != nil {
		plans.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, out)
}
