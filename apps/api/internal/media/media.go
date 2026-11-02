package media

import (
	"errors"
	"time"

	"github.com/FacileStudio/tronc/env"
	"github.com/livekit/protocol/auth"
)

const tokenTTL = 6 * time.Hour

// Grant is the LiveKit permission set issued with a join token.
type Grant struct {
	CanPublish     bool
	CanPublishData bool
	CanSubscribe   bool
	RoomAdmin      bool
}

// Service signs LiveKit access tokens from the configured API key pair.
type Service struct {
	apiKey    string
	apiSecret string
	url       string
}

// NewServiceFromEnv builds a Service from the LIVEKIT_* environment
// variables.
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

func (s *Service) Issue(room, identity, name string, grant Grant) (string, error) {
	video := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	video.SetCanPublish(grant.CanPublish)
	video.SetCanPublishData(grant.CanPublishData)
	video.SetCanSubscribe(grant.CanSubscribe)
	video.RoomAdmin = grant.RoomAdmin

	token := auth.NewAccessToken(s.apiKey, s.apiSecret).
		SetIdentity(identity).
		SetName(name).
		SetValidFor(tokenTTL).
		SetVideoGrant(video)
	return token.ToJWT()
}

func (s *Service) URL() string {
	return s.url
}

// Secret exposes the API secret for webhook signature verification.
func (s *Service) Secret() string {
	return s.apiSecret
}
