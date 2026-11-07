package history

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	defaultPageSize = 20
	maxPageSize     = 50
	maxPage         = 1_000_000
)

type meetingRow struct {
	ID            uuid.UUID
	StartedAt     time.Time
	EndedAt       *time.Time
	Slug          string
	RoomName      string
	Owner         bool
	HasRecording  bool
	HasTranscript bool
}

type meetingOut struct {
	ID              string           `json:"id"`
	RoomSlug        string           `json:"room_slug"`
	RoomName        string           `json:"room_name"`
	Role            string           `json:"role"`
	StartedAt       string           `json:"started_at"`
	EndedAt         string           `json:"ended_at,omitempty"`
	DurationSeconds *int64           `json:"duration_seconds,omitempty"`
	HasRecording    bool             `json:"has_recording"`
	HasTranscript   bool             `json:"has_transcript"`
	Participants    []participantOut `json:"participants"`
}

type meetingsPage struct {
	Items    []meetingOut `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Total    int64        `json:"total"`
}

const (
	meetingsScope = `
  FROM calls c JOIN rooms r ON r.id = c.room_id
 WHERE r.owner_id = @me
    OR EXISTS (SELECT 1 FROM call_participants p WHERE p.call_id = c.id AND p.user_id = @me)`

	selectMeetings = `
SELECT c.id, c.started_at, c.ended_at, r.slug, r.name AS room_name,
       COALESCE(r.owner_id = @me, false) AS owner,
       EXISTS (SELECT 1 FROM recordings x WHERE x.call_id = c.id AND x.status = 'complete' AND (x.shared OR r.owner_id = @me)) AS has_recording,
       EXISTS (SELECT 1 FROM transcripts t WHERE t.call_id = c.id AND t.content <> '') AS has_transcript` +
		meetingsScope + `
 ORDER BY c.started_at DESC, c.id DESC LIMIT @limit OFFSET @offset`

	countMeetings = `SELECT count(*)` + meetingsScope
)

// clampPaging keeps the page and its size inside what the endpoint serves.
func clampPaging(page, size int) (int, int) {
	if size < 1 {
		size = defaultPageSize
	}
	return min(max(page, 1), maxPage), min(size, maxPageSize)
}

// Meetings lists the calls the caller owns or sat in, newest first. The scope
// is the caller's own id in the query, so another person's calls cannot appear
// in any page, and a joined call shows a recording only once it is shared.
func (s *Service) Meetings(ctx context.Context, callerID int64, page, size int) (*meetingsPage, error) {
	page, size = clampPaging(page, size)
	args := map[string]any{"me": callerID, "limit": size, "offset": (page - 1) * size}
	var total int64
	if err := s.orm.WithContext(ctx).Raw(countMeetings, args).Scan(&total).Error; err != nil {
		return nil, err
	}
	var rows []meetingRow
	if err := s.orm.WithContext(ctx).Raw(selectMeetings, args).Scan(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	people, err := s.participantsOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	items := make([]meetingOut, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.out(people[row.ID]))
	}
	return &meetingsPage{Items: items, Page: page, PageSize: size, Total: total}, nil
}

func (row meetingRow) out(people []participantOut) meetingOut {
	role := roleParticipant
	if row.Owner {
		role = roleOwner
	}
	if people == nil {
		people = []participantOut{}
	}
	item := meetingOut{
		ID: row.ID.String(), RoomSlug: row.Slug, RoomName: row.RoomName, Role: role,
		StartedAt:    row.StartedAt.UTC().Format(time.RFC3339),
		HasRecording: row.HasRecording, HasTranscript: row.HasTranscript, Participants: people,
	}
	if row.EndedAt != nil {
		seconds := max(0, int64(row.EndedAt.Sub(row.StartedAt).Seconds()))
		item.EndedAt = row.EndedAt.UTC().Format(time.RFC3339)
		item.DurationSeconds = &seconds
	}
	return item
}
