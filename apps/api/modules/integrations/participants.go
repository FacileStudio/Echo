package integrations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/schemas"
)

// tokenTTL bounds how long a minted token can open a connection. LiveKit
// refreshes the token of a connected client by itself, so a short lifetime
// costs nothing in a long call and stops a leaked token from rejoining later.
// https://docs.livekit.io/frontends/authentication/tokens
const tokenTTL = 10 * time.Minute

const identitySeparator = "#"

var participantGrant = media.Grant{
	CanPublish:     true,
	CanPublishData: true,
	CanSubscribe:   true,
}

// Token mints a LiveKit join token for one participant of the app's room.
//
// The LiveKit identity is the app's identity plus a random suffix, so the
// same person joining from two devices does not make LiveKit disconnect the
// first one. Echo strips the suffix again everywhere it reports an identity.
func (s *Service) Token(ctx context.Context, app, roomID string, req TokenRequest) (string, error) {
	if err := validateToken(req); err != nil {
		return "", err
	}
	room, err := s.Room(ctx, app, roomID)
	if err != nil {
		return "", err
	}
	identity, err := connectionIdentity(req.Identity)
	if err != nil {
		return "", err
	}
	metadata, err := participantMetadata(req.AvatarURL)
	if err != nil {
		return "", err
	}
	return s.media.IssueFor(room.Slug, media.Participant{
		Identity: identity,
		Name:     strings.TrimSpace(req.Name),
		Metadata: metadata,
		TTL:      tokenTTL,
	}, participantGrant)
}

// Participants lists who is connected to the room's latest call while it is
// open, one entry per app identity, read from the call records LiveKit's
// webhooks maintain. Only the latest call counts, so a call whose
// room_finished was lost cannot keep ghosts in the list.
func (s *Service) Participants(ctx context.Context, room *schemas.Room) ([]Participant, error) {
	var rows []schemas.CallParticipant
	err := s.orm.WithContext(ctx).
		Joins("JOIN calls ON calls.id = call_participants.call_id").
		Where("calls.id = (?) AND calls.ended_at IS NULL AND call_participants.left_at IS NULL",
			s.orm.Model(&schemas.Call{}).Select("id").Where("room_id = ?", room.ID).Order("started_at DESC").Limit(1)).
		Order("call_participants.joined_at").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	participants := []Participant{}
	for _, row := range rows {
		identity := AppIdentity(row.Identity)
		if !seen[identity] {
			seen[identity] = true
			participants = append(participants, Participant{Identity: identity, Name: row.Name})
		}
	}
	return participants, nil
}

// AppIdentity turns a LiveKit identity back into the one the app supplied.
func AppIdentity(livekitIdentity string) string {
	if at := strings.LastIndex(livekitIdentity, identitySeparator); at > 0 {
		return livekitIdentity[:at]
	}
	return livekitIdentity
}

func connectionIdentity(identity string) (string, error) {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	return strings.TrimSpace(identity) + identitySeparator + hex.EncodeToString(suffix), nil
}

func participantMetadata(avatarURL string) (string, error) {
	if avatarURL == "" {
		return "", nil
	}
	encoded, err := json.Marshal(map[string]string{"avatar_url": avatarURL})
	return string(encoded), err
}
