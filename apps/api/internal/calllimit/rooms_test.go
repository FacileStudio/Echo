package calllimit

import (
	"context"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/livekit/protocol/livekit"
)

func TestARoomTimerEndsASessionNoWebhookReported(t *testing.T) {
	db := testdb.Migrated(t)
	addRoom(t, db, "silent")
	live := newFakeLive(media.LiveRoom{Name: "silent", SID: "RM_silent", CreatedAt: time.Now().Add(-41 * time.Minute)})
	limiter := New(context.Background(), db, cappedPlans(t, db), live, nil)
	limiter.ArmRoom("silent", time.Now().Add(-time.Minute))
	live.waitEnded(t, "silent")
}

func TestARoomTimerFollowsLiveKitsCreationTime(t *testing.T) {
	db := testdb.Migrated(t)
	addRoom(t, db, "young")
	created := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	live := newFakeLive(media.LiveRoom{Name: "young", SID: "RM_young", CreatedAt: created})
	limiter := New(context.Background(), db, cappedPlans(t, db), live, nil)
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
	limiter := New(context.Background(), db, cappedPlans(t, db), live, nil)
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

func TestReconcileReportsTheCallsItClosesToIntegrations(t *testing.T) {
	db := testdb.Migrated(t)
	ghost := openCall(t, db, "ghost", time.Now().Add(-time.Hour))
	limiter := New(context.Background(), db, cappedPlans(t, db), newFakeLive(), nil)
	heard := &recorder{}
	limiter.NotifyWith(heard)
	if err := limiter.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(heard.events) != 1 || heard.events[0].GetEvent() != "room_finished" ||
		heard.events[0].GetRoom().GetName() != "ghost" || heard.events[0].GetRoom().GetSid() != ghost.LivekitRoomSID {
		t.Fatalf("integrations heard %v, want one room_finished for the forgotten call", heard.events)
	}
}

type recorder struct{ events []*livekit.WebhookEvent }

func (r *recorder) Notify(_ context.Context, event *livekit.WebhookEvent) {
	r.events = append(r.events, event)
}

func TestOldCutsAreForgotten(t *testing.T) {
	limiter := New(context.Background(), nil, nil, newFakeLive(), nil)
	clock := time.Now()
	limiter.now = func() time.Time { return clock }
	limiter.recordCut("old")
	clock = clock.Add(cutMemory + time.Minute)
	limiter.recordCut("new")
	if _, kept := limiter.CutAt("old"); kept {
		t.Fatal("a cut older than cutMemory was kept")
	}
	if _, kept := limiter.CutAt("new"); !kept {
		t.Fatal("the fresh cut was dropped")
	}
}

func TestAStaleRowOfARoomStillRunningIsClosedSilently(t *testing.T) {
	db := testdb.Migrated(t)
	stale := openCall(t, db, "prisme-room", time.Now().Add(-time.Hour))
	live := newFakeLive(media.LiveRoom{Name: "prisme-room", SID: "RM_newer_session", CreatedAt: time.Now()})
	limiter := New(context.Background(), db, cappedPlans(t, db), live, nil)
	heard := &recorder{}
	limiter.NotifyWith(heard)
	if err := limiter.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var closed schemas.Call
	if err := db.First(&closed, "id = ?", stale.ID).Error; err != nil || closed.EndedAt == nil {
		t.Fatal("the stale row of a superseded session stayed open")
	}
	if len(heard.events) != 0 {
		t.Fatalf("integrations heard %v while a newer session of the room is live: Prisme would end that call", heard.events)
	}
}
