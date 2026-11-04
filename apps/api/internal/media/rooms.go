package media

import (
	"context"
	"time"

	"github.com/livekit/protocol/auth"
)

// The LiveKit room timeouts Echo asks for. An empty room closes two minutes
// after its last participant leaves, and a room whose participants all left
// at once closes twenty seconds later, so a call cut at its plan deadline
// does not linger as an open session.
const (
	roomEmptyTimeout     = 2 * time.Minute
	roomDepartureTimeout = 20 * time.Second
)

// RoomSettings are the limits a LiveKit room is created with. A
// MaxParticipants of 0 or less is unlimited.
type RoomSettings struct {
	MaxParticipants int64
}

// EnsureRoom creates the LiveKit room before anyone joins it, with the
// participant ceiling and timeouts Echo wants. LiveKit returns a room that
// already exists unchanged, so a limit set here holds for the whole session.
func (s *Service) EnsureRoom(ctx context.Context, roomName string, settings RoomSettings) error {
	token, err := s.roomServiceToken(roomName)
	if err != nil {
		return err
	}
	body := map[string]any{
		"name":              roomName,
		"empty_timeout":     uint32(roomEmptyTimeout / time.Second),
		"departure_timeout": uint32(roomDepartureTimeout / time.Second),
		"max_participants":  uint32(max(0, min(settings.MaxParticipants, 1<<31))),
	}
	return s.twirp(ctx, token, "livekit.RoomService/CreateRoom", body, nil)
}

// EndRoom closes a LiveKit room and disconnects everyone in it. LiveKit then
// posts room_finished, which closes the call row.
func (s *Service) EndRoom(ctx context.Context, roomName string) error {
	token, err := s.roomServiceToken(roomName)
	if err != nil {
		return err
	}
	return s.twirp(ctx, token, "livekit.RoomService/DeleteRoom", map[string]any{"room": roomName}, nil)
}

// roomServiceToken is the server-only credential for creating and deleting
// one room. It never reaches a client.
func (s *Service) roomServiceToken(roomName string) (string, error) {
	video := &auth.VideoGrant{RoomCreate: true, Room: roomName}
	return auth.NewAccessToken(s.apiKey, s.apiSecret).
		SetIdentity("echo-api").
		SetValidFor(time.Minute).
		SetVideoGrant(video).
		ToJWT()
}
