package calllimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/directory"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
)

type fakeEnder struct {
	mu    sync.Mutex
	ended []string
	done  chan struct{}
}

func (f *fakeEnder) EndRoom(_ context.Context, room string) error {
	f.mu.Lock()
	f.ended = append(f.ended, room)
	f.mu.Unlock()
	f.done <- struct{}{}
	return nil
}

func freePlans(t *testing.T, db *gorm.DB) *plans.Plans {
	t.Helper()
	p := plans.New(db, "https://sso.example.test", nil)
	if err := p.Apply(context.Background(), directory.Snapshot{Version: 1, EntitlementsRevision: 1}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return p
}

func openCall(t *testing.T, db *gorm.DB, slug string, startedAt time.Time) schemas.Call {
	t.Helper()
	room := schemas.Room{ID: uuid.New(), Slug: slug, Name: slug}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("room: %v", err)
	}
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
	limiter := New(context.Background(), db, freePlans(t, db), &fakeEnder{done: make(chan struct{}, 1)}, nil)
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
	ender := &fakeEnder{done: make(chan struct{}, 1)}
	limiter := New(context.Background(), db, freePlans(t, db), ender, nil)
	if err := limiter.Rearm(context.Background()); err != nil {
		t.Fatalf("rearm: %v", err)
	}
	select {
	case <-ender.done:
	case <-time.After(5 * time.Second):
		t.Fatal("an overdue call was not ended")
	}
	if ender.ended[0] != "overdue" {
		t.Fatalf("ended %v", ender.ended)
	}
}

func TestNoTimerBeforeTheFirstSnapshotOrForASupersededCall(t *testing.T) {
	db := testdb.Migrated(t)
	stale := openCall(t, db, "stale", time.Now().Add(-2*time.Hour))
	unsynced := New(context.Background(), db, plans.New(db, "", nil), &fakeEnder{}, nil)
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
	ender := &fakeEnder{done: make(chan struct{}, 1)}
	limiter := New(context.Background(), db, freePlans(t, db), ender, nil)
	limiter.Notify(context.Background(), &livekit.WebhookEvent{Event: "room_started", Room: &livekit.Room{Sid: "RM_stale"}})
	if _, ok := limiter.Deadline(stale.ID); ok || len(ender.ended) != 0 {
		t.Fatal("a superseded call was armed or ended, which would cut the newer meeting")
	}
}
