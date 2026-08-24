package transcripts

import (
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// openCall seeds a room plus one call, ended or still running.
func openCall(t *testing.T, db *gorm.DB, slug string, ended bool) schemas.Call {
	t.Helper()
	room := schemas.Room{ID: uuid.New(), Slug: slug, Name: slug}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	call := schemas.Call{
		ID:              uuid.New(),
		RoomID:          room.ID,
		StartedAt:       time.Now().UTC(),
		LivekitRoomName: slug,
	}
	if ended {
		closed := time.Now().UTC()
		call.EndedAt = &closed
	}
	if err := db.Create(&call).Error; err != nil {
		t.Fatalf("seed call: %v", err)
	}
	return call
}

func TestAppendCreatesThenConcatenates(t *testing.T) {
	db := testdb.Migrated(t)
	s := NewService(db)
	call := openCall(t, db, "standup", false)

	if err := s.Append("standup", "Alice", "bonjour"); err != nil {
		t.Fatalf("first line: %v", err)
	}
	if err := s.Append("standup", "Bob", "salut"); err != nil {
		t.Fatalf("second line: %v", err)
	}

	var rows []schemas.Transcript
	if err := db.Where("call_id = ?", call.ID).Find(&rows).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("transcripts = %d, want 1 row appended in place", len(rows))
	}
	want := "Alice: bonjour\nBob: salut\n"
	if rows[0].Content != want {
		t.Fatalf("content = %q, want %q", rows[0].Content, want)
	}
}

// After room_finished the record is history: a late caption must not rewrite it.
func TestAppendRefusesWhenNoCallIsOpen(t *testing.T) {
	db := testdb.Migrated(t)
	s := NewService(db)
	openCall(t, db, "standup", true)

	if err := s.Append("standup", "Alice", "trop tard"); err == nil {
		t.Fatal("a caption was accepted against a closed call")
	}

	var count int64
	if err := db.Model(&schemas.Transcript{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("transcripts = %d, want 0", count)
	}
}

func TestAppendRefusesForAnUnknownRoom(t *testing.T) {
	db := testdb.Migrated(t)

	if err := NewService(db).Append("no-such-room", "Alice", "bonjour"); err == nil {
		t.Fatal("a caption was accepted for a room with no call")
	}
}
