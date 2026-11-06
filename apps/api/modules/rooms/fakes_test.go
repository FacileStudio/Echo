package rooms

import (
	"context"
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

func (f *fakeLive) Participants(context.Context, string) ([]string, error) {
	return f.participants, f.participantsErr
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
