package webhooks

import (
	"context"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// newHooks returns a receiver whose clock advances a minute per call, so
// rows created in one test have a deterministic, distinguishable order.
func newHooks(t *testing.T) (*Webhooks, *gorm.DB) {
	t.Helper()
	db := testdb.Migrated(t)
	tick := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	w := New(db, testSecret)
	w.now = func() time.Time {
		tick = tick.Add(time.Minute)
		return tick
	}
	return w, db
}

func seedRoom(t *testing.T, db *gorm.DB, slug string) schemas.Room {
	t.Helper()
	room := schemas.Room{ID: uuid.New(), Slug: slug, Name: slug}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	return room
}

func send(t *testing.T, w *Webhooks, p *payload) {
	t.Helper()
	if err := w.dispatch(context.Background(), p); err != nil {
		t.Fatalf("dispatch %s: %v", p.Event, err)
	}
}

func lkRoom(slug string) *roomInfo { return &roomInfo{Name: slug} }

func TestRoomStartedCreatesOneCallAndIsIdempotent(t *testing.T) {
	w, db := newHooks(t)
	seedRoom(t, db, "standup")

	send(t, w, &payload{Event: eventRoomStarted, Room: lkRoom("standup")})
	send(t, w, &payload{Event: eventRoomStarted, Room: lkRoom("standup")})

	var calls int64
	if err := db.Model(&schemas.Call{}).Count(&calls).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 — a retried room_started opened a second call", calls)
	}
}

// LiveKit accepts any room name, so events arrive for rooms Echo never
// created. Those are deliberately dropped, not errors.
func TestRoomStartedIgnoresAnUnknownSlug(t *testing.T) {
	w, db := newHooks(t)

	send(t, w, &payload{Event: eventRoomStarted, Room: lkRoom("ad-hoc-room")})

	var calls int64
	if err := db.Model(&schemas.Call{}).Count(&calls).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want 0 for a room with no rooms row", calls)
	}
}

func TestParticipantJoinAndLeaveStampTheRow(t *testing.T) {
	w, db := newHooks(t)
	seedRoom(t, db, "standup")
	send(t, w, &payload{Event: eventRoomStarted, Room: lkRoom("standup")})

	alice := &who{Identity: "user-7", Name: "Alice"}
	send(t, w, &payload{Event: eventParticipantJoined, Room: lkRoom("standup"), Participant: alice})

	var row schemas.CallParticipant
	if err := db.Where("identity = ?", "user-7").First(&row).Error; err != nil {
		t.Fatalf("participant row missing after join: %v", err)
	}
	if row.Name != "Alice" || row.JoinedAt.IsZero() {
		t.Fatalf("participant = %+v, want a name and a joined_at", row)
	}
	if row.LeftAt != nil {
		t.Fatal("left_at was set on join")
	}

	send(t, w, &payload{Event: eventParticipantLeft, Room: lkRoom("standup"), Participant: alice})

	if err := db.Where("identity = ?", "user-7").First(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.LeftAt == nil {
		t.Fatal("left_at is still null after participant_left")
	}
}

func TestRoomFinishedClosesTheCall(t *testing.T) {
	w, db := newHooks(t)
	seedRoom(t, db, "standup")
	send(t, w, &payload{Event: eventRoomStarted, Room: lkRoom("standup")})
	send(t, w, &payload{Event: eventRoomFinished, Room: lkRoom("standup")})

	var call schemas.Call
	if err := db.First(&call).Error; err != nil {
		t.Fatalf("load call: %v", err)
	}
	if call.EndedAt == nil {
		t.Fatal("ended_at is still null after room_finished")
	}
}

// The regression this guards: GORM cannot apply ORDER BY to a bulk UPDATE on
// Postgres, so an ordered Update stamped every unrecorded call of the room.
func TestEgressEndedStampsOnlyTheLatestCall(t *testing.T) {
	w, db := newHooks(t)
	seedRoom(t, db, "standup")

	send(t, w, &payload{Event: eventRoomStarted, Room: lkRoom("standup")})
	send(t, w, &payload{Event: eventRoomFinished, Room: lkRoom("standup")})
	send(t, w, &payload{Event: eventRoomStarted, Room: lkRoom("standup")})

	send(t, w, &payload{
		Event:  eventEgressEnded,
		Room:   lkRoom("standup"),
		Egress: &egressInfo{EgressID: "EG_1", FileResults: []fileResult{{Filename: "recordings/standup-2.mp4"}}},
	})

	var calls []schemas.Call
	if err := db.Order("started_at ASC").Find(&calls).Error; err != nil {
		t.Fatalf("load calls: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[0].RecordingPath != "" {
		t.Fatalf("the earlier call was stamped with %q — the update hit the wrong row", calls[0].RecordingPath)
	}
	if calls[1].RecordingPath != "recordings/standup-2.mp4" {
		t.Fatalf("latest call recording_path = %q, want the egress filename", calls[1].RecordingPath)
	}
}
