package media

import (
	"context"
	"errors"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
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

// LiveRoom is a room LiveKit is running: its name, its per-session id and
// when the session started.
type LiveRoom struct {
	Name      string
	SID       string
	CreatedAt time.Time
}

// EnsureRoom creates the LiveKit room before anyone joins it and returns the
// running session. LiveKit's own room.auto_create is off in Echo's
// deployment, so this is the only way a room comes to exist.
//
// On a room that already exists LiveKit keeps the session and overwrites its
// max_participants and timeouts with every value above 0. An unlimited room
// sends 0 and therefore never lowers a ceiling a limited owner set.
func (s *Service) EnsureRoom(ctx context.Context, roomName string, settings RoomSettings) (LiveRoom, error) {
	token, err := s.roomServiceToken(roomName)
	if err != nil {
		return LiveRoom{}, err
	}
	body := map[string]any{
		"name":              roomName,
		"empty_timeout":     uint32(roomEmptyTimeout / time.Second),
		"departure_timeout": uint32(roomDepartureTimeout / time.Second),
		"max_participants":  uint32(max(0, min(settings.MaxParticipants, 1<<31))),
	}
	room := &livekit.Room{}
	if err := s.twirp(ctx, token, "livekit.RoomService/CreateRoom", body, room); err != nil {
		return LiveRoom{}, err
	}
	return liveRoom(room), nil
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

// Participants lists the identities connected to a room, none when LiveKit
// does not know the room.
func (s *Service) Participants(ctx context.Context, roomName string) ([]string, error) {
	token, err := s.roomServiceToken(roomName)
	if err != nil {
		return nil, err
	}
	response := &livekit.ListParticipantsResponse{}
	err = s.twirp(ctx, token, "livekit.RoomService/ListParticipants", map[string]any{"room": roomName}, response)
	if errors.Is(err, ErrNotFound) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	identities := make([]string, 0, len(response.GetParticipants()))
	for _, participant := range response.GetParticipants() {
		identities = append(identities, participant.GetIdentity())
	}
	return identities, nil
}

// Rooms lists the rooms LiveKit is running, only those named when names are
// given.
func (s *Service) Rooms(ctx context.Context, names ...string) ([]LiveRoom, error) {
	token, err := s.roomServiceToken("")
	if err != nil {
		return nil, err
	}
	response := &livekit.ListRoomsResponse{}
	if err := s.twirp(ctx, token, "livekit.RoomService/ListRooms", map[string]any{"names": names}, response); err != nil {
		return nil, err
	}
	rooms := make([]LiveRoom, 0, len(response.GetRooms()))
	for _, room := range response.GetRooms() {
		rooms = append(rooms, liveRoom(room))
	}
	return rooms, nil
}

// roomServiceToken is the server-only credential for the room service: it
// creates, lists and administers rooms, one room when roomName is set. It
// never reaches a client.
func (s *Service) roomServiceToken(roomName string) (string, error) {
	video := &auth.VideoGrant{RoomCreate: true, RoomList: true, RoomAdmin: true, Room: roomName}
	return auth.NewAccessToken(s.apiKey, s.apiSecret).
		SetIdentity("echo-api").
		SetValidFor(time.Minute).
		SetVideoGrant(video).
		ToJWT()
}

func liveRoom(room *livekit.Room) LiveRoom {
	created := time.Time{}
	if room.GetCreationTime() > 0 {
		created = time.Unix(room.GetCreationTime(), 0).UTC()
	}
	return LiveRoom{Name: room.GetName(), SID: room.GetSid(), CreatedAt: created}
}
