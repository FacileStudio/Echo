package media

import (
	"errors"
	"time"

	"github.com/FacileStudio/tronc/env"
	"github.com/livekit/protocol/auth"
)

// TokenTTL bounds how long a join token can open a connection. LiveKit
// refreshes the token of a connected client by itself, so a short lifetime
// costs nothing in a long call, and a token minted past a plan check cannot
// be hoarded to join later.
const TokenTTL = 10 * time.Minute

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

// Participant is who a join token is for and how long it can open a
// connection. A zero TTL means TokenTTL.
type Participant struct {
	Identity string
	Name     string
	Metadata string
	TTL      time.Duration
}

// Issue mints a join token with the default lifetime.
func (s *Service) Issue(room, identity, name string, grant Grant) (string, error) {
	return s.IssueFor(room, Participant{Identity: identity, Name: name}, grant)
}

// IssueFor mints a join token for one participant of one room.
func (s *Service) IssueFor(room string, who Participant, grant Grant) (string, error) {
	video := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	video.SetCanPublish(grant.CanPublish)
	video.SetCanPublishData(grant.CanPublishData)
	video.SetCanSubscribe(grant.CanSubscribe)
	video.RoomAdmin = grant.RoomAdmin

	ttl := who.TTL
	if ttl <= 0 {
		ttl = TokenTTL
	}
	token := auth.NewAccessToken(s.apiKey, s.apiSecret).
		SetIdentity(who.Identity).
		SetName(who.Name).
		SetMetadata(who.Metadata).
		SetValidFor(ttl).
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
