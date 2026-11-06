package rooms

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/schemas"
)

// latestCall is the room's most recent call, open or ended, or nil.
func (s *Service) latestCall(ctx context.Context, room *schemas.Room) (*schemas.Call, error) {
	var calls []schemas.Call
	err := s.orm.WithContext(ctx).
		Where("room_id = ?", room.ID).
		Order("started_at DESC").Limit(1).
		Find(&calls).Error
	if err != nil || len(calls) == 0 {
		return nil, err
	}
	return &calls[0], nil
}

// present lists the people connected to the room. LiveKit's own participant
// list is the truth; the webhook-fed rows of the open call are the fallback
// when LiveKit cannot be asked. Recorders and agents are not people.
func (s *Service) present(ctx context.Context, slug string, call *schemas.Call) []media.Connected {
	connected, err := s.live.Participants(ctx, slug)
	if err != nil {
		slog.Warn("livekit participant list failed, counting call rows", slog.String("room", slug), slog.Any("error", err))
		connected = s.connectedRows(ctx, call)
	}
	people := connected[:0:0]
	for _, who := range connected {
		if isPerson(who.Identity) {
			people = append(people, who)
		}
	}
	return people
}

// connectedRows lists who the open call's rows show as connected.
func (s *Service) connectedRows(ctx context.Context, call *schemas.Call) []media.Connected {
	if call == nil || call.EndedAt != nil {
		return nil
	}
	var rows []schemas.CallParticipant
	err := s.orm.WithContext(ctx).Where("call_id = ? AND left_at IS NULL", call.ID).Find(&rows).Error
	if err != nil {
		slog.Warn("call participant rows unreadable", slog.Any("error", err))
	}
	connected := make([]media.Connected, 0, len(rows))
	for _, row := range rows {
		connected = append(connected, media.Connected{Identity: row.Identity, JoinedAt: row.JoinedAt})
	}
	return connected
}

func identitiesOf(connected []media.Connected) []string {
	identities := make([]string, 0, len(connected))
	for _, who := range connected {
		identities = append(identities, who.Identity)
	}
	return identities
}

// makeRoomForOwner disconnects the most recently joined guests when the
// owner arrives at a room already at its ceiling, so LiveKit does not refuse
// the owner's connection. Tokens minted earlier stay valid for a short while
// after their pending seat lapses, so guests can fill a room past Echo's own
// count; the owner's seat is taken back here. Signed-in people other than
// the owner are disconnected only once no guest is left.
func (s *Service) makeRoomForOwner(ctx context.Context, slug string, ceiling int64, owner string, call *schemas.Call) {
	if ceiling < 0 {
		return
	}
	others := slices.DeleteFunc(s.present(ctx, slug, call), func(who media.Connected) bool { return who.Identity == owner })
	slices.SortStableFunc(others, func(a, b media.Connected) int {
		if guestA, guestB := strings.HasPrefix(a.Identity, "guest-"), strings.HasPrefix(b.Identity, "guest-"); guestA != guestB {
			if guestA {
				return -1
			}
			return 1
		}
		return b.JoinedAt.Compare(a.JoinedAt)
	})
	for excess := int64(len(others)) - ceiling + 1; excess > 0 && len(others) > 0; excess-- {
		if err := s.live.RemoveParticipant(ctx, slug, others[0].Identity); err != nil {
			slog.Warn("making room for the owner failed", slog.String("room", slug), slog.Any("error", err))
			return
		}
		others = others[1:]
	}
}

func isPerson(identity string) bool {
	return strings.HasPrefix(identity, "user-") || strings.HasPrefix(identity, "guest-")
}
