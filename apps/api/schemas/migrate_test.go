package schemas_test

import (
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"

	"github.com/google/uuid"
)

func TestMigrationsApplyCleanly(t *testing.T) {
	db := testdb.Migrated(t)

	room := schemas.Room{ID: uuid.New(), Slug: "team-standup", Name: "Team Standup"}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("create an unowned room: %v", err)
	}

	var owner int64 = 7
	owned := schemas.Room{ID: uuid.New(), Slug: "client-review", Name: "Client Review", OwnerID: &owner}
	if err := db.Create(&owned).Error; err != nil {
		t.Fatalf("create an owned room: %v", err)
	}

	var unownedCount, ownedCount int64
	if err := db.Model(&schemas.Room{}).Where("owner_id IS NULL").Count(&unownedCount).Error; err != nil {
		t.Fatalf("count unowned rooms: %v", err)
	}
	if err := db.Model(&schemas.Room{}).Where("owner_id IS NOT NULL").Count(&ownedCount).Error; err != nil {
		t.Fatalf("count owned rooms: %v", err)
	}
	if unownedCount != 1 || ownedCount != 1 {
		t.Fatalf("rooms = %d unowned / %d owned, want 1/1", unownedCount, ownedCount)
	}

	call := schemas.Call{ID: uuid.New(), RoomID: room.ID, LivekitRoomName: "echo_team-standup"}
	if err := db.Create(&call).Error; err != nil {
		t.Fatalf("create a call: %v", err)
	}

	transcript := schemas.Transcript{ID: uuid.New(), CallID: call.ID, Content: "bonjour tout le monde"}
	if err := db.Create(&transcript).Error; err != nil {
		t.Fatalf("create a transcript: %v", err)
	}

	var language string
	if err := db.Raw(`SELECT language FROM transcripts WHERE id = ?`, transcript.ID).Scan(&language).Error; err != nil {
		t.Fatalf("read the transcript language: %v", err)
	}
	if language != "fr" {
		t.Fatalf("language = %q, want the default 'fr'", language)
	}

	summary := schemas.Summary{ID: uuid.New(), CallID: call.ID, Content: "résumé", Model: "claude-test"}
	if err := db.Create(&summary).Error; err != nil {
		t.Fatalf("create a summary: %v", err)
	}
}

func TestDeletingARoomCascadesThroughCalls(t *testing.T) {
	db := testdb.Migrated(t)

	room := schemas.Room{ID: uuid.New(), Slug: "doomed", Name: "Doomed"}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	call := schemas.Call{ID: uuid.New(), RoomID: room.ID, LivekitRoomName: "echo_doomed"}
	if err := db.Create(&call).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	transcript := schemas.Transcript{ID: uuid.New(), CallID: call.ID, Language: "fr"}
	if err := db.Create(&transcript).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	summary := schemas.Summary{ID: uuid.New(), CallID: call.ID, Model: "claude-test"}
	if err := db.Create(&summary).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := db.Delete(&room).Error; err != nil {
		t.Fatalf("delete the room: %v", err)
	}

	for _, table := range []string{"calls", "transcripts", "summaries"} {
		var remaining int64
		if err := db.Table(table).Count(&remaining).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if remaining != 0 {
			t.Fatalf("%s survived the room it belongs to", table)
		}
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	db := testdb.Open(t)

	if err := schemas.Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := schemas.Migrate(db); err != nil {
		t.Fatalf("migrate is not idempotent: %v", err)
	}
}

// The deployed database is populated, so a re-migration has to be a no-op
// over existing rows and not just over an empty schema. The rows here carry
// the empty livekit_room_sid a pre-webhook call would have, which is exactly
// the case the partial unique index has to tolerate.
func TestASecondMigrateLeavesExistingRowsAlone(t *testing.T) {
	db := testdb.Migrated(t)

	room := schemas.Room{ID: uuid.New(), Slug: "legacy", Name: "Legacy"}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	for range 2 {
		call := schemas.Call{ID: uuid.New(), RoomID: room.ID, LivekitRoomName: "legacy"}
		if err := db.Create(&call).Error; err != nil {
			t.Fatalf("seed a call with no room sid: %v", err)
		}
	}

	if err := schemas.Migrate(db); err != nil {
		t.Fatalf("re-migrating a populated database failed: %v", err)
	}

	var calls int64
	if err := db.Model(&schemas.Call{}).Count(&calls).Error; err != nil {
		t.Fatalf("count calls: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want the 2 seeded rows to survive", calls)
	}
}

// Two live sessions cannot share a room sid, whatever a retried or duplicated
// delivery says. The constraint is in the database, not in the handler.
func TestTheRoomSIDIsUniqueAcrossCalls(t *testing.T) {
	db := testdb.Migrated(t)

	room := schemas.Room{ID: uuid.New(), Slug: "standup", Name: "Standup"}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	first := schemas.Call{ID: uuid.New(), RoomID: room.ID, LivekitRoomName: "standup", LivekitRoomSID: "RM_1"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("seed the first call: %v", err)
	}

	second := schemas.Call{ID: uuid.New(), RoomID: room.ID, LivekitRoomName: "standup", LivekitRoomSID: "RM_1"}
	if err := db.Create(&second).Error; err == nil {
		t.Fatal("a second call took the same LiveKit room sid")
	}
}
