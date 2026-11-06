package calllimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/directory"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
)

type fakeLive struct {
	mu       sync.Mutex
	rooms    []media.LiveRoom
	ended    chan string
	limiter  *Limiter
	cutFirst bool
}

func newFakeLive(rooms ...media.LiveRoom) *fakeLive {
	return &fakeLive{rooms: rooms, ended: make(chan string, 8)}
}

func (f *fakeLive) EndRoom(_ context.Context, room string) error {
	if f.limiter != nil {
		_, f.cutFirst = f.limiter.CutAt(room)
	}
	f.ended <- room
	return nil
}

func (f *fakeLive) Rooms(_ context.Context, names ...string) ([]media.LiveRoom, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []media.LiveRoom{}
	for _, room := range f.rooms {
		if len(names) == 0 || names[0] == room.Name {
			out = append(out, room)
		}
	}
	return out, nil
}

func (f *fakeLive) waitEnded(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-f.ended:
		if got != want {
			t.Fatalf("ended %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("%s was not ended", want)
	}
}

func freePlans(t *testing.T, db *gorm.DB) *plans.Plans {
	t.Helper()
	p := plans.New(db, "https://sso.example.test", nil)
	if err := p.Apply(context.Background(), directory.Snapshot{Version: 1, EntitlementsRevision: 1}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return p
}

func addRoom(t *testing.T, db *gorm.DB, slug string) schemas.Room {
	t.Helper()
	room := schemas.Room{ID: uuid.New(), Slug: slug, Name: slug}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("room: %v", err)
	}
	return room
}

func openCall(t *testing.T, db *gorm.DB, slug string, startedAt time.Time) schemas.Call {
	t.Helper()
	room := addRoom(t, db, slug)
	call := schemas.Call{ID: uuid.New(), RoomID: room.ID, StartedAt: startedAt.Truncate(time.Second).UTC(),
		LivekitRoomName: slug, LivekitRoomSID: "RM_" + slug}
	if err := db.Create(&call).Error; err != nil {
		t.Fatalf("call: %v", err)
	}
	return call
}

func TestARestartReArmsTheDeadlineFromStartedAt(t *testing.T) {
	db := testdb.Migrated(t)
	call := openCall(t, db, "running", time.Now().Add(-15*time.Minute))
	limiter := New(context.Background(), db, freePlans(t, db), newFakeLive(), nil)
	if err := limiter.Rearm(context.Background()); err != nil {
		t.Fatalf("rearm: %v", err)
	}
	got, ok := limiter.Deadline(call.ID)
	if want := call.StartedAt.Add(40 * time.Minute); !ok || !got.Equal(want) {
		t.Fatalf("deadline = %v (%v), want started_at + 40 min = %v", got, ok, want)
	}
	if err := db.Model(&schemas.Call{}).Where("id = ?", call.ID).Update("ended_at", time.Now()).Error; err != nil {
		t.Fatalf("end: %v", err)
	}
	limiter.Notify(context.Background(), &livekit.WebhookEvent{Event: "room_finished", Room: &livekit.Room{Sid: "RM_running"}})
	if _, ok := limiter.Deadline(call.ID); ok {
		t.Fatal("a finished call kept its timer")
	}
}

func TestAnOverdueCallIsEnded(t *testing.T) {
	db := testdb.Migrated(t)
	openCall(t, db, "overdue", time.Now().Add(-time.Hour))
	live := newFakeLive(media.LiveRoom{Name: "overdue", SID: "RM_overdue"})
	limiter := New(context.Background(), db, freePlans(t, db), live, nil)
	live.limiter = limiter
	if err := limiter.Rearm(context.Background()); err != nil {
		t.Fatalf("rearm: %v", err)
	}
	live.waitEnded(t, "overdue")
	if !live.cutFirst {
		t.Fatal("the cut was recorded after the room closed, so a racing join could start a new call")
	}
	if _, cut := limiter.CutAt("overdue"); !cut {
		t.Fatal("the cut was not remembered for the cooldown")
	}
}

func TestAStaleCallRowNeverEndsTheNewerSession(t *testing.T) {
	db := testdb.Migrated(t)
	call := openCall(t, db, "reused", time.Now().Add(-time.Hour))
	live := newFakeLive(media.LiveRoom{Name: "reused", SID: "RM_newer", CreatedAt: time.Now()})
	limiter := New(context.Background(), db, freePlans(t, db), live, nil)
	limiter.evaluate(context.Background(), call.ID)
	if len(live.ended) != 0 {
		t.Fatal("a call row LiveKit no longer runs ended the newer session of its room")
	}
}

func TestNoTimerBeforeTheFirstSnapshotOrForASupersededCall(t *testing.T) {
	db := testdb.Migrated(t)
	stale := openCall(t, db, "stale", time.Now().Add(-2*time.Hour))
	unsynced := New(context.Background(), db, plans.New(db, "", nil), newFakeLive(), nil)
	if err := unsynced.Rearm(context.Background()); err != nil {
		t.Fatalf("rearm: %v", err)
	}
	if _, ok := unsynced.Deadline(stale.ID); ok {
		t.Fatal("a store that never synced armed a deadline")
	}
	newer := schemas.Call{ID: uuid.New(), RoomID: stale.RoomID, StartedAt: time.Now().UTC(),
		LivekitRoomName: "stale", LivekitRoomSID: "RM_stale_2"}
	if err := db.Create(&newer).Error; err != nil {
		t.Fatalf("newer call: %v", err)
	}
	live := newFakeLive()
	limiter := New(context.Background(), db, freePlans(t, db), live, nil)
	limiter.Notify(context.Background(), &livekit.WebhookEvent{Event: "room_started", Room: &livekit.Room{Sid: "RM_stale"}})
	if _, ok := limiter.Deadline(stale.ID); ok || len(live.ended) != 0 {
		t.Fatal("a superseded call was armed or ended, which would cut the newer meeting")
	}
}
