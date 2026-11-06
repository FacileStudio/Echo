package rooms

import (
	"context"
	"slices"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
)

type fakeLive struct {
	room            media.LiveRoom
	ensureErr       error
	participants    []string
	participantsErr error
	ceilings        []int64
	onEnsure        func()
	joined          map[string]time.Time
	removed         []string
}

func (f *fakeLive) EnsureRoom(_ context.Context, name string, settings media.RoomSettings) (media.LiveRoom, error) {
	f.ceilings = append(f.ceilings, settings.MaxParticipants)
	if f.onEnsure != nil {
		f.onEnsure()
	}
	if f.ensureErr != nil {
		return media.LiveRoom{}, f.ensureErr
	}
	room := f.room
	room.Name = name
	return room, nil
}

func (f *fakeLive) Participants(context.Context, string) ([]media.Connected, error) {
	connected := []media.Connected{}
	for _, identity := range f.participants {
		if !slices.Contains(f.removed, identity) {
			connected = append(connected, media.Connected{Identity: identity, JoinedAt: f.joined[identity]})
		}
	}
	return connected, f.participantsErr
}

func (f *fakeLive) RemoveParticipant(_ context.Context, _, identity string) error {
	f.removed = append(f.removed, identity)
	return nil
}

func (f *fakeLive) Rooms(context.Context, ...string) ([]media.LiveRoom, error) {
	if f.room.SID == "" {
		return nil, nil
	}
	return []media.LiveRoom{f.room}, nil
}

type fakeDeadlines struct {
	armed map[string]time.Time
	cuts  map[string]time.Time
}

func (f *fakeDeadlines) ArmRoom(slug string, deadline time.Time) {
	if f.armed == nil {
		f.armed = map[string]time.Time{}
	}
	f.armed[slug] = deadline
}

func (f *fakeDeadlines) CutAt(slug string) (time.Time, bool) {
	at, ok := f.cuts[slug]
	return at, ok
}

func dummyLive(sid string, created time.Time) media.LiveRoom {
	return media.LiveRoom{SID: sid, CreatedAt: created}
}
