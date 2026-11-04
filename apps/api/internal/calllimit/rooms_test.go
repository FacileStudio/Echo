package calllimit

import (
	"context"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
)

func TestARoomTimerEndsASessionNoWebhookReported(t *testing.T) {
	db := testdb.Migrated(t)
	addRoom(t, db, "silent")
	live := newFakeLive(media.LiveRoom{Name: "silent", SID: "RM_silent", CreatedAt: time.Now().Add(-41 * time.Minute)})
	limiter := New(context.Background(), db, freePlans(t, db), live, nil)
	limiter.ArmRoom("silent", time.Now().Add(-time.Minute))
	live.waitEnded(t, "silent")
}

func TestARoomTimerFollowsLiveKitsCreationTime(t *testing.T) {
	db := testdb.Migrated(t)
	addRoom(t, db, "young")
	created := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	live := newFakeLive(media.LiveRoom{Name: "young", SID: "RM_young", CreatedAt: created})
	limiter := New(context.Background(), db, freePlans(t, db), live, nil)
	limiter.evaluateRoom(context.Background(), "young")
	if got, ok := limiter.RoomDeadline("young"); !ok || !got.Equal(created.Add(40*time.Minute)) {
		t.Fatalf("room deadline = %v (%v), want creation + 40 min", got, ok)
	}
}

func TestReconcileClosesRowsLiveKitForgotAndEndsOverdueSessions(t *testing.T) {
	db := testdb.Migrated(t)
	ghost := openCall(t, db, "ghost", time.Now().Add(-time.Hour))
	if err := db.Create(&schemas.CallParticipant{CallID: ghost.ID, Identity: "guest-1", JoinedAt: ghost.StartedAt}).Error; err != nil {
		t.Fatalf("participant: %v", err)
	}
	fresh := openCall(t, db, "fresh", time.Now())
	addRoom(t, db, "unrecorded")
	live := newFakeLive(
		media.LiveRoom{Name: "fresh-elsewhere", SID: "RM_other", CreatedAt: time.Now()},
		media.LiveRoom{Name: "unrecorded", SID: "RM_unrecorded", CreatedAt: time.Now().Add(-50 * time.Minute)},
	)
	limiter := New(context.Background(), db, freePlans(t, db), live, nil)
	if err := limiter.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var reloaded schemas.Call
	db.First(&reloaded, "id = ?", ghost.ID)
	var left schemas.CallParticipant
	db.First(&left, "call_id = ?", ghost.ID)
	if reloaded.EndedAt == nil || left.LeftAt == nil {
		t.Fatal("the call row of a session LiveKit forgot stayed open")
	}
	var young schemas.Call
	if err := db.First(&young, "id = ?", fresh.ID).Error; err != nil || young.EndedAt != nil {
		t.Fatal("a call row younger than the race window was closed")
	}
	live.waitEnded(t, "unrecorded")
}
