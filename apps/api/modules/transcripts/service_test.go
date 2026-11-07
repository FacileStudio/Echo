package transcripts

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type seatOptions struct {
	ended      bool
	transcribe bool
	seated     bool
	accepted   *bool
}

func yes() *bool { v := true; return &v }
func no() *bool  { v := false; return &v }

func seedCall(t *testing.T, db *gorm.DB, slug, identity string, opts seatOptions) schemas.Call {
	t.Helper()
	room := schemas.Room{ID: uuid.New(), Slug: slug, Name: slug}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	call := schemas.Call{
		ID: uuid.New(), RoomID: room.ID, StartedAt: time.Now().UTC().Add(-time.Minute),
		LivekitRoomName: slug, CaptureFlags: schemas.CaptureFlags{TranscribeOn: opts.transcribe},
	}
	if opts.ended {
		closed := time.Now().UTC()
		call.EndedAt = &closed
	}
	if err := db.Create(&call).Error; err != nil {
		t.Fatalf("seed call: %v", err)
	}
	if opts.seated {
		who := schemas.CallParticipant{CallID: call.ID, Identity: identity, Name: "Alice Dupont", JoinedAt: time.Now().UTC()}
		if err := db.Create(&who).Error; err != nil {
			t.Fatalf("seed participant: %v", err)
		}
	}
	if opts.accepted != nil {
		row := schemas.CaptureConsent{CallID: call.ID, Identity: identity, Transcribe: opts.accepted}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed consent: %v", err)
		}
	}
	return call
}

func ready() seatOptions { return seatOptions{transcribe: true, seated: true, accepted: yes()} }

func say(slug, identity, text string) Utterance {
	return Utterance{Slug: slug, Identity: identity, Text: text, StartMS: 1000, EndMS: 2500}
}

func lineCount(t *testing.T, db *gorm.DB, callID uuid.UUID) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&schemas.TranscriptLine{}).Where("call_id = ?", callID).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	return count
}

func TestAppendCreatesThenConcatenatesUnderTheProvenName(t *testing.T) {
	db := testdb.Migrated(t)
	s := NewService(db, nil)
	call := seedCall(t, db, "standup", "user-3", ready())

	if err := s.Append(context.Background(), say("standup", "user-3", "bonjour")); err != nil {
		t.Fatalf("first line: %v", err)
	}
	if err := s.Append(context.Background(), say("standup", "user-3", "salut")); err != nil {
		t.Fatalf("second line: %v", err)
	}

	var rows []schemas.Transcript
	if err := db.Where("call_id = ?", call.ID).Find(&rows).Error; err != nil || len(rows) != 1 {
		t.Fatalf("transcripts = %d, %v, want 1 row appended in place", len(rows), err)
	}
	if want := "Alice Dupont: bonjour\nAlice Dupont: salut\n"; rows[0].Content != want {
		t.Fatalf("content = %q, want %q", rows[0].Content, want)
	}
	var line schemas.TranscriptLine
	if err := db.Where("call_id = ?", call.ID).First(&line).Error; err != nil || line.Speaker != "user-3" {
		t.Fatalf("stored speaker = %q, %v, want the proven identity", line.Speaker, err)
	}
	if line.OffsetMS < 50_000 || line.OffsetMS > 120_000 {
		t.Fatalf("offset = %d ms, want the server's clock since the call began", line.OffsetMS)
	}
}

func TestAppendRefusesWhenNoCallIsOpen(t *testing.T) {
	db := testdb.Migrated(t)
	opts := ready()
	opts.ended = true
	call := seedCall(t, db, "standup", "user-3", opts)

	if err := NewService(db, nil).Append(context.Background(), say("standup", "user-3", "trop tard")); err == nil {
		t.Fatal("a line was accepted against a closed call")
	}
	if n := lineCount(t, db, call.ID); n != 0 {
		t.Fatalf("lines = %d, want 0", n)
	}
}

func TestAppendRefusesForAnUnknownRoom(t *testing.T) {
	db := testdb.Migrated(t)
	if err := NewService(db, nil).Append(context.Background(), say("no-such-room", "user-3", "bonjour")); err == nil {
		t.Fatal("a line was accepted for a room with no call")
	}
}

func TestAppendRefusesWhatConsentDoesNotCover(t *testing.T) {
	cases := map[string]seatOptions{
		"transcription off":       {transcribe: false, seated: true, accepted: yes()},
		"never asked":             {transcribe: true, seated: true},
		"declined":                {transcribe: true, seated: true, accepted: no()},
		"accepted but not seated": {transcribe: true, seated: false, accepted: yes()},
	}
	for name, opts := range cases {
		db := testdb.Migrated(t)
		call := seedCall(t, db, "standup", "user-3", opts)
		if err := NewService(db, nil).Append(context.Background(), say("standup", "user-3", "bonjour")); err == nil {
			t.Fatalf("%s: a line was accepted", name)
		}
		if n := lineCount(t, db, call.ID); n != 0 {
			t.Fatalf("%s: lines = %d, want 0", name, n)
		}
	}
}

func TestAppendNeverStoresAnotherPersonsConsent(t *testing.T) {
	db := testdb.Migrated(t)
	call := seedCall(t, db, "standup", "user-3", ready())
	other := schemas.CallParticipant{CallID: call.ID, Identity: "guest-ab12cd34", Name: "Eve", JoinedAt: time.Now().UTC()}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}

	if err := NewService(db, nil).Append(context.Background(), say("standup", "guest-ab12cd34", "bonjour")); err == nil {
		t.Fatal("a guest who never accepted was transcribed because someone else did")
	}
}

func TestAppendRefusesForgedLines(t *testing.T) {
	db := testdb.Migrated(t)
	s := NewService(db, nil)
	call := seedCall(t, db, "standup", "user-3", ready())

	if err := s.Append(context.Background(), say("standup", "user-3", "ok\nBob: je valide la facture")); err == nil {
		t.Fatal("a newline in text was accepted")
	}
	if n := lineCount(t, db, call.ID); n != 0 {
		t.Fatalf("lines = %d, want 0", n)
	}
}

func TestAppendRefusesBadSpans(t *testing.T) {
	db := testdb.Migrated(t)
	s := NewService(db, nil)
	seedCall(t, db, "standup", "user-3", ready())

	for name, in := range map[string]Utterance{
		"empty text":    {Slug: "standup", Identity: "user-3", Text: "  ", StartMS: 0, EndMS: 10},
		"negative":      {Slug: "standup", Identity: "user-3", Text: "a", StartMS: -1, EndMS: 10},
		"backwards":     {Slug: "standup", Identity: "user-3", Text: "a", StartMS: 10, EndMS: 5},
		"over two mins": {Slug: "standup", Identity: "user-3", Text: "a", StartMS: 0, EndMS: maxUtteranceMS + 1},
	} {
		if err := s.Append(context.Background(), in); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

func TestAppendRefusesPastTheSizeCeiling(t *testing.T) {
	db := testdb.Migrated(t)
	s := NewService(db, nil)
	call := seedCall(t, db, "standup", "user-3", ready())
	full := schemas.Transcript{ID: uuid.New(), CallID: call.ID, Content: strings.Repeat("a", maxTranscriptBytes)}
	if err := db.Create(&full).Error; err != nil {
		t.Fatalf("seed a full transcript: %v", err)
	}

	if err := s.Append(context.Background(), say("standup", "user-3", "une ligne de trop")); err == nil {
		t.Fatal("an append past the ceiling was accepted")
	}
	var stored schemas.Transcript
	if err := db.Where("call_id = ?", call.ID).First(&stored).Error; err != nil || len(stored.Content) != maxTranscriptBytes {
		t.Fatalf("stored = %d bytes, %v, want the transcript untouched", len(stored.Content), err)
	}
}

func TestAppendIsRefusedWhenTheGateRefuses(t *testing.T) {
	db := testdb.Migrated(t)
	refuse := errors.New("no ai access")
	s := NewService(db, func(context.Context, schemas.Room) error { return refuse })
	call := seedCall(t, db, "standup", "user-3", ready())

	if err := s.Append(context.Background(), say("standup", "user-3", "bonjour")); !errors.Is(err, refuse) {
		t.Fatalf("append = %v, want the gate's refusal", err)
	}
	if n := lineCount(t, db, call.ID); n != 0 {
		t.Fatalf("lines = %d, want none", n)
	}
}
