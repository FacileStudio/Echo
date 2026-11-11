package capture

import (
	"net/http"
	"strconv"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/porte/kit"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

const (
	pollsPerMinute  = 60
	chunksPerMinute = 120
	writesPerMinute = 30
	maxBodyBytes    = 4 << 10
)

type handler struct{ service *Service }

type startRequest struct {
	ContentType string `json:"content_type"`
}

// RegisterRoutes mounts the capture endpoints. Reading the state and
// answering the request are for anyone seated, a session or a seat key; the
// organizer's switches and the recording upload need a signed-in owner.
func RegisterRoutes(router chi.Router, service *Service, requireAuth func(http.Handler) http.Handler) {
	h := handler{service: service}
	router.Get("/rooms/{slug}/capture", h.state)
	router.Put("/rooms/{slug}/capture/consent", h.answer)
	router.With(requireAuth).Put("/rooms/{slug}/capture", h.flags)
	router.With(requireAuth).Post("/rooms/{slug}/capture/recording", h.start)
	router.With(requireAuth).Put("/rooms/{slug}/capture/recording/{id}/chunks/{index}", h.chunk)
	router.With(requireAuth).Post("/rooms/{slug}/capture/recording/{id}/finalize", h.finalize)
}

func (h handler) seated(w http.ResponseWriter, r *http.Request, scope string, perMinute int) (slug, identity string, ok bool) {
	slug = chi.URLParam(r, "slug")
	identity, err := h.service.keys.Identity(r, slug)
	if err != nil {
		httpjson.WriteError(w, err)
		return "", "", false
	}
	if !h.service.limiter.AllowKey(scope, slug+"|"+identity, perMinute) {
		middleware.Refuse(w)
		return "", "", false
	}
	return slug, identity, true
}

func (h handler) owner(w http.ResponseWriter, r *http.Request, scope string, perMinute int) (int64, bool) {
	who, ok := kit.FromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, troncerrors.Unauthorized("not authenticated"))
		return 0, false
	}
	key := chi.URLParam(r, "slug") + "|" + identityOf(who.UserID)
	if !h.service.limiter.AllowKey(scope, key, perMinute) {
		middleware.Refuse(w)
		return 0, false
	}
	return who.UserID, true
}

func (h handler) state(w http.ResponseWriter, r *http.Request) {
	slug, identity, ok := h.seated(w, r, "capture-state", pollsPerMinute)
	if !ok {
		return
	}
	state, err := h.service.State(r.Context(), slug, identity)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, state)
}

func (h handler) answer(w http.ResponseWriter, r *http.Request) {
	slug, identity, ok := h.seated(w, r, "capture-consent", writesPerMinute)
	if !ok {
		return
	}
	var req Answer
	if !decode(w, r, &req) {
		return
	}
	if err := h.service.SetAnswer(r.Context(), slug, identity, req); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handler) flags(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.owner(w, r, "capture-flags", writesPerMinute)
	if !ok {
		return
	}
	var req Flags
	if !decode(w, r, &req) {
		return
	}
	if err := h.service.SetFlags(r.Context(), chi.URLParam(r, "slug"), ownerID, req); err != nil {
		plans.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handler) start(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.owner(w, r, "capture-start", writesPerMinute)
	if !ok {
		return
	}
	var req startRequest
	if !decode(w, r, &req) {
		return
	}
	upload, err := h.service.StartUpload(r.Context(), chi.URLParam(r, "slug"), ownerID, req.ContentType)
	if err != nil {
		plans.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, upload)
}

func (h handler) chunk(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.owner(w, r, "capture-chunk", chunksPerMinute)
	if !ok {
		return
	}
	index, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil || index < 0 {
		httpjson.WriteError(w, troncerrors.Invalid("index must be a chunk number"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxChunkBytes+1)
	upload, err := h.service.PutChunk(r.Context(), Chunk{Slug: chi.URLParam(r, "slug"), OwnerID: ownerID, ID: chi.URLParam(r, "id"), Index: index}, r.Body)
	if gap, isGap := Gap(err); isGap {
		httpjson.WriteJSON(w, http.StatusConflict, gap)
		return
	}
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, upload)
}

func (h handler) finalize(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.owner(w, r, "capture-finalize", writesPerMinute)
	if !ok {
		return
	}
	if err := h.service.Finalize(r.Context(), chi.URLParam(r, "slug"), ownerID, chi.URLParam(r, "id")); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := httpjson.DecodeJSON(w, r, into); err != nil {
		httpjson.WriteError(w, troncerrors.Invalid("the body must be a small JSON object"))
		return false
	}
	return true
}
