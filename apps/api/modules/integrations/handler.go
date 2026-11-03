package integrations

import (
	"net/http"
	"net/url"

	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

type handler struct{ service *Service }

func (h handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRoomRequest
	if err := httpjson.DecodeJSON(w, r, &req); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	room, created, err := h.service.Ensure(r.Context(), appFrom(r.Context()), req)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpjson.WriteJSON(w, status, RoomCreated{RoomID: room.Slug, JoinURL: embedURL(r, room.Slug)})
}

func (h handler) token(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if err := httpjson.DecodeJSON(w, r, &req); err != nil {
		httpjson.WriteError(w, err)
		return
	}
	roomID := chi.URLParam(r, "id")
	token, err := h.service.Token(r.Context(), appFrom(r.Context()), roomID, req)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	livekitURL := h.service.media.URL()
	fragment := url.Values{"token": {token}, "url": {livekitURL}}.Encode()
	httpjson.WriteJSON(w, http.StatusOK, TokenResponse{
		LivekitURL: livekitURL,
		Token:      token,
		EmbedURL:   embedURL(r, roomID) + "#" + fragment,
	})
}

func (h handler) get(w http.ResponseWriter, r *http.Request) {
	room, err := h.service.Room(r.Context(), appFrom(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	participants, err := h.service.Participants(r.Context(), room)
	if err != nil {
		httpjson.WriteError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, RoomState{RoomID: room.Slug, Title: room.Name, Participants: participants})
}

// embedURL builds the public embed address from the request that reached
// Echo through its proxy, so no extra setting has to name Echo's own host.
func embedURL(r *http.Request, roomID string) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	return scheme + "://" + r.Host + "/embed/" + url.PathEscape(roomID)
}
