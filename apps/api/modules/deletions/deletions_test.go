package deletions

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	portepg "github.com/FacileStudio/porte/pg"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const issuer = "https://sso.example"

type fixture struct {
	db     *gorm.DB
	dir    string
	erased *Deletions
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db := testdb.Migrated(t)
	dir := t.TempDir()
	return fixture{db: db, dir: dir, erased: &Deletions{orm: db, recordingsDir: dir, logger: slog.Default()}}
}

func (f fixture) person(t *testing.T, email, subject string) int64 {
	t.Helper()
	user := schemas.User{Email: email, Name: email}
	if err := f.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	err := f.db.Exec(`INSERT INTO porte_identities (user_id, provider, subject) VALUES (?, ?, ?)`, user.ID, issuer, subject).Error
	if err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func (f fixture) room(t *testing.T, slug string, owner *int64, recording string) uuid.UUID {
	t.Helper()
	room := schemas.Room{ID: uuid.New(), Slug: slug, Name: slug, OwnerID: owner}
	call := schemas.Call{ID: uuid.New(), RoomID: room.ID, StartedAt: time.Now(), LivekitRoomName: slug, RecordingPath: recording}
	if err := f.db.Create(&room).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&call).Error; err != nil {
		t.Fatal(err)
	}
	return call.ID
}

func (f fixture) count(t *testing.T, table, where string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := f.db.Raw("SELECT count(*) FROM "+table+" WHERE "+where, args...).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestErasingAPersonLeavesOthersUntouchedAndReplaysAsANoOp(t *testing.T) {
	f := newFixture(t)
	gone := f.person(t, "gone@example.com", "sub-gone")
	kept := f.person(t, "kept@example.com", "sub-kept")
	recording := filepath.Join(f.dir, "gone.mp4")
	if err := os.WriteFile(recording, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.room(t, "gone-room", &gone, "gone.mp4")
	keptCall := f.room(t, "kept-room", &kept, "")
	seat := schemas.CallParticipant{CallID: keptCall, Identity: "user-" + itoa(gone), Name: "Gone", JoinedAt: time.Now()}
	if err := f.db.Create(&seat).Error; err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := f.erased.ErasePerson(context.Background(), issuer, "sub-gone"); err != nil {
			t.Fatalf("erase: %v", err)
		}
	}

	if f.count(t, "users", "id = ?", gone) != 0 || f.count(t, "porte_identities", "subject = ?", "sub-gone") != 0 {
		t.Fatal("the account or its identity survived")
	}
	if f.count(t, "rooms", "owner_id = ?", gone) != 0 || f.count(t, "calls", "livekit_room_name = ?", "gone-room") != 0 {
		t.Fatal("the person's rooms or calls survived")
	}
	if _, err := os.Stat(recording); !os.IsNotExist(err) {
		t.Fatalf("the recording survived: %v", err)
	}
	if f.count(t, "users", "id = ?", kept) != 1 || f.count(t, "rooms", "owner_id = ?", kept) != 1 {
		t.Fatal("another person's data was touched")
	}
	if f.count(t, "call_participants", "identity = ? AND name = ''", seat.Identity) != 1 {
		t.Fatal("the name on a seat in someone else's call was not blanked")
	}
}

func TestPorteEraserRunsAgainstEchosSchema(t *testing.T) {
	f := newFixture(t)
	f.person(t, "solo@example.com", "sub-solo")
	sqlDB, err := f.db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := portepg.New(sqlDB).EraseIdentity(context.Background(), issuer, "sub-solo"); err != nil {
		t.Fatalf("EraseIdentity: %v", err)
	}
	if f.count(t, "porte_identities", "subject = ?", "sub-solo") != 0 {
		t.Fatal("the identity survived")
	}
}
