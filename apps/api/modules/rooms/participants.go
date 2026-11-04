package rooms

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

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
func (s *Service) present(ctx context.Context, slug string, call *schemas.Call) []string {
	identities, err := s.live.Participants(ctx, slug)
	if err != nil {
		slog.Warn("livekit participant list failed, counting call rows", slog.String("room", slug), slog.Any("error", err))
		identities = s.connectedRows(ctx, call)
	}
	people := identities[:0:0]
	for _, identity := range identities {
		if isPerson(identity) {
			people = append(people, identity)
		}
	}
	return people
}

// connectedRows lists the identities the open call's rows show as connected.
func (s *Service) connectedRows(ctx context.Context, call *schemas.Call) []string {
	if call == nil || call.EndedAt != nil {
		return nil
	}
	var identities []string
	err := s.orm.WithContext(ctx).Model(&schemas.CallParticipant{}).
		Where("call_id = ? AND left_at IS NULL", call.ID).
		Pluck("identity", &identities).Error
	if err != nil {
		slog.Warn("call participant rows unreadable", slog.Any("error", err))
	}
	return identities
}

func isPerson(identity string) bool {
	return strings.HasPrefix(identity, "user-") || strings.HasPrefix(identity, "guest-")
}

// ledger remembers, per room, the tokens admitted that have not connected
// yet. An entry lives as long as the token can open a connection, and leaves
// as soon as its identity shows up connected.
type ledger struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	pending map[string]map[string]time.Time
}

func newLedger(ttl time.Duration, now func() time.Time) *ledger {
	return &ledger{ttl: ttl, now: now, pending: map[string]map[string]time.Time{}}
}

// admit counts the people present plus the pending admissions, except
// identity itself, asks check whether one more fits, and records identity as
// pending when it does. Counting and recording happen under one lock, so two
// concurrent joins cannot both take the last seat.
func (l *ledger) admit(slug, identity string, present []string, check func(int64) error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	seen := map[string]bool{}
	for _, who := range present {
		seen[who] = true
	}
	room := l.pending[slug]
	for who, expires := range room {
		if seen[who] || !now.Before(expires) {
			delete(room, who)
			continue
		}
		seen[who] = true
	}
	if len(room) == 0 {
		delete(l.pending, slug)
		room = nil
	}
	delete(seen, identity)
	if err := check(int64(len(seen))); err != nil {
		return err
	}
	if room == nil {
		room = map[string]time.Time{}
		l.pending[slug] = room
	}
	room[identity] = now.Add(l.ttl)
	return nil
}
