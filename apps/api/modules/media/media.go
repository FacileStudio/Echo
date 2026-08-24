package media

import (
	"errors"
	"time"

	"github.com/FacileStudio/tronc/env"
	"github.com/livekit/protocol/auth"
)

const tokenTTL = 6 * time.Hour

type Service struct {
	apiKey    string
	apiSecret string
	url       string
}

func NewServiceFromEnv() (*Service, error) {
	key, err := env.Required("LIVEKIT_API_KEY")
	if err != nil {
		return nil, err
	}
	secret, err := env.Required("LIVEKIT_API_SECRET")
	if err != nil {
		return nil, err
	}
	url := env.String("LIVEKIT_URL", "")
	if url == "" {
		return nil, errors.New("LIVEKIT_URL must be set")
	}
	return &Service{apiKey: key, apiSecret: secret, url: url}, nil
}

func IssueToken(apiKey, apiSecret, room, identity string, canPublish bool) (string, error) {
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	grant.SetCanPublish(canPublish)
	grant.SetCanSubscribe(true)

	token := auth.NewAccessToken(apiKey, apiSecret).
		SetIdentity(identity).
		SetValidFor(tokenTTL).
		SetVideoGrant(grant)
	return token.ToJWT()
}

func (s *Service) Issue(room, identity, displayName string, guest bool) (string, error) {
	name := displayName
	if name == "" {
		name = identity
	}
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	grant.SetCanPublish(!guest)
	grant.SetCanSubscribe(true)

	token := auth.NewAccessToken(s.apiKey, s.apiSecret).
		SetIdentity(identity).
		SetName(name).
		SetValidFor(tokenTTL).
		SetVideoGrant(grant)
	return token.ToJWT()
}

func (s *Service) URL() string {
	return s.url
}
