package deletions

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/modules/auth"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/directory"
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
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	users, err := auth.NewStore(context.Background(), sqlDB, slog.Default(), EraseData)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{db: db, dir: dir, erased: New(users, directory.Client{Issuer: issuer}, nil, dir, slog.Default())}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
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
	f.room(t, "gone-room", &gone, "/output/gone.mp4")
	keptRecording := filepath.Join(f.dir, "kept.mp4")
	if err := os.WriteFile(keptRecording, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	keptCall := f.room(t, "kept-room", &kept, "/output/kept.mp4")
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
	if _, err := os.Stat(keptRecording); err != nil {
		t.Fatalf("another person's recording was touched: %v", err)
	}
	if f.count(t, "users", "id = ?", kept) != 1 || f.count(t, "rooms", "owner_id = ?", kept) != 1 {
		t.Fatal("another person's data was touched")
	}
	if f.count(t, "call_participants", "identity = ?", seat.Identity) != 0 {
		t.Fatal("the user-<id> identity of an erased person survived on a seat in someone else's call")
	}
	if f.count(t, "call_participants", "id = ? AND name = '' AND user_id IS NULL AND identity = ?", seat.ID, "erased-"+itoa(seat.ID)) != 1 {
		t.Fatal("the seat in someone else's call kept the name or the account link")
	}
}

func TestErasureKeepsAJoinedMeetingButNamesNoOne(t *testing.T) {
	f := newFixture(t)
	gone := f.person(t, "gone@example.com", "sub-gone")
	kept := f.person(t, "kept@example.com", "sub-kept")
	call := f.room(t, "shared-room", &kept, "")
	identity := "user-" + itoa(gone)
	seats := []schemas.CallParticipant{
		{CallID: call, Identity: identity, UserID: &gone, Name: "Gone", JoinedAt: time.Now()},
		{CallID: call, Identity: "user-" + itoa(kept), UserID: &kept, Name: "Kept", JoinedAt: time.Now()},
	}
	for i := range seats {
		if err := f.db.Create(&seats[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	text := identity + ": bonjour\nuser-" + itoa(kept) + ": salut\n" + identity + ": a demain\n"
	if err := f.db.Create(&schemas.Transcript{ID: uuid.New(), CallID: call, Content: text}).Error; err != nil {
		t.Fatal(err)
	}
	lines := []schemas.TranscriptLine{
		{CallID: call, Speaker: identity, Text: "bonjour"},
		{CallID: call, Speaker: "user-" + itoa(kept), Text: "salut"},
	}
	if err := f.db.Create(&lines).Error; err != nil {
		t.Fatal(err)
	}

	answer := true
	consents := []schemas.CaptureConsent{{CallID: call, Identity: identity, Transcribe: &answer}, {CallID: call, Identity: "user-" + itoa(kept), Transcribe: &answer}}
	if err := f.db.Create(&consents).Error; err != nil {
		t.Fatal(err)
	}

	if err := f.erased.ErasePerson(context.Background(), issuer, "sub-gone"); err != nil {
		t.Fatal(err)
	}
	if f.count(t, "capture_consents", "identity = ?", identity) != 0 || f.count(t, "capture_consents", "call_id = ?", call) != 1 {
		t.Fatal("the erased person's capture answers were kept, or the other person's were not")
	}

	var content string
	if err := f.db.Raw(`SELECT content FROM transcripts WHERE call_id = ?`, call).Scan(&content).Error; err != nil {
		t.Fatal(err)
	}
	erased := "erased-" + itoa(seats[0].ID)
	want := erased + ": bonjour\nuser-" + itoa(kept) + ": salut\n" + erased + ": a demain\n"
	if content != want {
		t.Fatalf("transcript = %q, want %q", content, want)
	}
	if f.count(t, "transcript_lines", "speaker = ?", identity) != 0 || f.count(t, "transcript_lines", "speaker = ?", erased) != 1 {
		t.Fatal("the erased person's transcript lines still carry their identity")
	}
	if f.count(t, "transcript_lines", "call_id = ?", call) != 2 {
		t.Fatal("a joined meeting lost its transcript lines")
	}
	if f.count(t, "call_participants", "id = ? AND user_id = ?", seats[1].ID, kept) != 1 {
		t.Fatal("the other participant's seat was touched")
	}
}

func TestErasureDeletesOwnedTranscriptsLinesAndRecordings(t *testing.T) {
	f := newFixture(t)
	gone := f.person(t, "gone@example.com", "sub-gone")
	call := f.room(t, "gone-room", &gone, "")
	rows := []any{
		&schemas.Transcript{ID: uuid.New(), CallID: call, Content: "user-1: x\n"},
		&schemas.TranscriptLine{CallID: call, Speaker: "user-1", Text: "x"},
		&schemas.Recording{ID: uuid.New(), CallID: call, Filename: "/output/a.mp4"},
	}
	for _, row := range rows {
		if err := f.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.dir, "a.mp4"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := f.erased.ErasePerson(context.Background(), issuer, "sub-gone"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"transcripts", "transcript_lines", "recordings"} {
		if f.count(t, table, "call_id = ?", call) != 0 {
			t.Fatalf("%s of an erased owner survived", table)
		}
	}
	if _, err := os.Stat(filepath.Join(f.dir, "a.mp4")); !os.IsNotExist(err) {
		t.Fatalf("the recording file survived: %v", err)
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
