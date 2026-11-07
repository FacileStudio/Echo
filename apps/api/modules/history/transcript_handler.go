package history

import (
	"net/http"

	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

var exportTypes = map[string]string{
	"txt": "text/plain; charset=utf-8",
	"vtt": "text/vtt; charset=utf-8",
}

// transcript answers the searchable JSON by default, and the whole transcript
// as a .txt or .vtt attachment when format asks for one.
func (h handler) transcript(w http.ResponseWriter, r *http.Request) {
	identity, ok := caller(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	format := r.URL.Query().Get("format")
	if format == "" || format == "json" {
		out, err := h.service.Transcript(r.Context(), id, identity.UserID, r.URL.Query().Get("q"))
		if err != nil {
			plans.WriteError(w, err)
			return
		}
		httpjson.WriteJSON(w, http.StatusOK, out)
		return
	}
	contentType, known := exportTypes[format]
	if !known {
		httpjson.WriteError(w, troncerrors.Invalid("format must be json, txt or vtt"))
		return
	}
	body, err := h.service.Export(r.Context(), id, identity.UserID, format)
	if err != nil {
		plans.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="echo-`+id+`.`+format+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(body))
}
