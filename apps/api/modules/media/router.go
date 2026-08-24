package media

import (
	"errors"
	"net/http"

	"github.com/FacileStudio/tronc/httpjson"
	"github.com/go-chi/chi/v5"
)

type tokenRequest struct {
	Identity    string `json:"identity"`
	DisplayName string `json:"displayName"`
	Guest       bool   `json:"guest"`
}

type tokenResponse struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

func RegisterRoutes(router chi.Router, service *Service) {
	router.Post("/rooms/{slug}/token", func(w http.ResponseWriter, r *http.Request) {
		var req tokenRequest
		if err := httpjson.DecodeJSON(w, r, &req); err != nil {
			httpjson.WriteError(w, err)
			return
		}
		if req.Identity == "" {
			httpjson.WriteError(w, errors.New("identity is required"))
			return
		}
		slug := chi.URLParam(r, "slug")
		token, err := service.Issue(slug, req.Identity, req.DisplayName, req.Guest)
		if err != nil {
			httpjson.WriteError(w, err)
			return
		}
		httpjson.WriteJSON(w, http.StatusOK, tokenResponse{Token: token, URL: service.URL()})
	})
}
