package rooms

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/authcontext"
	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte"
	"github.com/FacileStudio/tronc/errors"
	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

// Handler serves the rooms HTTP endpoints.
type Handler struct {
	service  *Service
	resolver middleware.IdentityResolver
}

func newHandler(service *Service, resolver middleware.IdentityResolver) *Handler {
	return &Handler{service: service, resolver: resolver}
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

	ownerID, err := h.callerID(r.Context())
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
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
	viewerID, err := h.callerID(r.Context())
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, toRoomResponse(*room, viewerID))
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
	if authenticated, ok := porte.From(r.Context()); ok {
		identity, err := h.resolver.IdentityForUser(r.Context(), authenticated.UserID)
		if err != nil {
			httpjson.WriteError(w, err)
			return
		}
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

// callerID resolves the logged-in caller on a route that may be public.
// It reads porte's session context, NOT authcontext, which only RequireAuth
// hydrates: a public route reading authcontext always sees an anonymous
// caller, which is how every room was created without an owner. Routes
// behind RequireAuth carry the porte context too, so this works on both.
func (h *Handler) callerID(ctx context.Context) (*int64, error) {
	if identity, ok := authcontext.From(ctx); ok {
		return &identity.UserID, nil
	}
	authenticated, ok := porte.From(ctx)
	if !ok {
		return nil, nil
	}
	identity, err := h.resolver.IdentityForUser(ctx, authenticated.UserID)
	if err != nil {
		return nil, err
	}
	return &identity.UserID, nil
}

func toRoomResponse(room schemas.Room, viewerID *int64) RoomResponse {
	return RoomResponse{
		ID:        room.ID.String(),
		Slug:      room.Slug,
		Name:      room.Name,
		Owned:     viewerID != nil && room.OwnerID != nil && *room.OwnerID == *viewerID,
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
