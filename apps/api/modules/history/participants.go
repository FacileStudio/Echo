package history

import (
	"context"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var avatarURL = regexp.MustCompile(`^https?://`)

type participantRow struct {
	CallID   uuid.UUID
	Identity string
	Name     string
	UserName string
	Picture  string
	JoinedAt time.Time
	LeftAt   *time.Time
}

const selectParticipants = `
SELECT p.call_id, p.identity, p.name, COALESCE(u.name, '') AS user_name, COALESCE(u.picture, '') AS picture,
       p.joined_at, p.left_at
  FROM call_participants p LEFT JOIN users u ON u.id = p.user_id
 WHERE p.call_id IN ? AND p.identity NOT LIKE 'agent-%'
 ORDER BY p.joined_at, p.id`

// participantsOf returns the people of each call in join order. The name is
// the one LiveKit recorded, else the account's current name, and the avatar is
// the account's Registre photo. A guest has no account and so no avatar, and
// an erased person's seat no longer joins to one.
func (s *Service) participantsOf(ctx context.Context, callIDs []uuid.UUID) (map[uuid.UUID][]participantOut, error) {
	out := make(map[uuid.UUID][]participantOut, len(callIDs))
	if len(callIDs) == 0 {
		return out, nil
	}
	var rows []participantRow
	if err := s.orm.WithContext(ctx).Raw(selectParticipants, callIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.CallID] = append(out[row.CallID], row.out())
	}
	return out, nil
}

func (row participantRow) out() participantOut {
	name := row.Name
	if name == "" {
		name = row.UserName
	}
	picture := ""
	if avatarURL.MatchString(row.Picture) {
		picture = row.Picture
	}
	left := ""
	if row.LeftAt != nil {
		left = row.LeftAt.UTC().Format(time.RFC3339)
	}
	return participantOut{
		Identity:  row.Identity,
		Name:      name,
		AvatarURL: picture,
		JoinedAt:  row.JoinedAt.UTC().Format(time.RFC3339),
		LeftAt:    left,
	}
}
