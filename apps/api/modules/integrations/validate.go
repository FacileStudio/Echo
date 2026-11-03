package integrations

import (
	"net/url"
	"strings"
	"unicode/utf8"

	troncerrors "github.com/FacileStudio/tronc/errors"
)

const (
	maxExternalID = 255
	maxTitle      = 200
	maxIdentity   = 128
	maxName       = 128
	maxURL        = 2048
)

func validateCreate(req CreateRoomRequest) error {
	if strings.TrimSpace(req.ExternalID) == "" || len(req.ExternalID) > maxExternalID {
		return troncerrors.Invalid("external_id is required, at most 255 bytes")
	}
	if utf8.RuneCountInString(req.Title) > maxTitle {
		return troncerrors.Invalid("title is at most 200 characters")
	}
	if req.CallbackURL != "" && !webURL(req.CallbackURL) {
		return troncerrors.Invalid("callback_url must be an absolute http or https URL")
	}
	return nil
}

func validateToken(req TokenRequest) error {
	if strings.TrimSpace(req.Identity) == "" || len(req.Identity) > maxIdentity {
		return troncerrors.Invalid("identity is required, at most 128 bytes")
	}
	if strings.TrimSpace(req.Name) == "" || utf8.RuneCountInString(req.Name) > maxName {
		return troncerrors.Invalid("name is required, at most 128 characters")
	}
	if req.AvatarURL != "" && !webURL(req.AvatarURL) {
		return troncerrors.Invalid("avatar_url must be an absolute http or https URL")
	}
	return nil
}

func webURL(raw string) bool {
	if len(raw) > maxURL {
		return false
	}
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "https" || parsed.Scheme == "http")
}

func titleOr(req CreateRoomRequest) string {
	if title := strings.TrimSpace(req.Title); title != "" {
		return title
	}
	return req.ExternalID
}
