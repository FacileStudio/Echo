package rooms

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/authcontext"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func newHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := httpjson.DecodeJSON(w, r, &req); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Name)
	}
	if slug == "" {
		httpjson.WriteError(w, errors.Invalid("a name or a slug is required"))
		return
	}

	ownerID := callerID(r.Context())
	room, err := h.service.Create(r.Context(), slug, req.Name, ownerID)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, toRoomResponse(*room, ownerID))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	room, err := h.service.BySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	if room == nil {
		httpjson.WriteError(w, errors.NotFound("room not found"))
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, toRoomResponse(*room, callerID(r.Context())))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	identity, ok := authcontext.From(r.Context())
	if !ok {
		httpjson.WriteError(w, errors.Unauthorized("not authenticated"))
		return
	}
	rooms, err := h.service.OwnedBy(r.Context(), identity.UserID)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	ownerID := identity.UserID
	response := make([]RoomResponse, 0, len(rooms))
	for _, room := range rooms {
		response = append(response, toRoomResponse(room, &ownerID))
	}
	httpjson.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) rename(w http.ResponseWriter, r *http.Request) {
	var req RenameRequest
	if err := httpjson.DecodeJSON(w, r, &req); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	identity, ok := authcontext.From(r.Context())
	if !ok {
		httpjson.WriteError(w, errors.Unauthorized("not authenticated"))
		return
	}
	room, err := h.service.Rename(r.Context(), chi.URLParam(r, "slug"), req.Name, identity.UserID)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	ownerID := identity.UserID
	httpjson.WriteJSON(w, http.StatusOK, toRoomResponse(*room, &ownerID))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	identity, ok := authcontext.From(r.Context())
	if !ok {
		httpjson.WriteError(w, errors.Unauthorized("not authenticated"))
		return
	}
	if err := h.service.Delete(r.Context(), chi.URLParam(r, "slug"), identity.UserID); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) token(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if err := httpjson.DecodeJSON(w, r, &req); err != nil {
		httpjson.WriteError(w, err)
		return
	}

	var userID *int64
	email := ""
	if identity, ok := authcontext.From(r.Context()); ok {
		userID = &identity.UserID
		email = identity.Email
	}

	token, err := h.service.Token(r.Context(), chi.URLParam(r, "slug"), userID, email, req.DisplayName)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, TokenResponse{
		Token: token,
		URL:   h.service.media.URL(),
	})
}

func callerID(ctx context.Context) *int64 {
	identity, ok := authcontext.From(ctx)
	if !ok {
		return nil
	}
	return &identity.UserID
}

func toRoomResponse(room schemas.Room, callerID *int64) RoomResponse {
	return RoomResponse{
		ID:        room.ID.String(),
		Slug:      room.Slug,
		Name:      room.Name,
		Owned:     callerID != nil && room.OwnerID != nil && *room.OwnerID == *callerID,
		CreatedAt: room.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func slugify(name string) string {
	lowered := strings.ToLower(name)
	var builder strings.Builder
	for _, r := range lowered {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == ' ', r == '-', r == '_':
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}
