package deletions

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

type person struct {
	id                   int64
	email, name, subject string
	ownCall, keptCall    uuid.UUID
	identity, provider   string
	db                   *gorm.DB
}

func (p person) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if err := p.db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func seedPerson(t *testing.T, db *gorm.DB, provider, email, name, subject string) person {
	t.Helper()
	user := schemas.User{Email: email, Name: name, Picture: "https://sso.example/a/" + subject + ".png"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	p := person{id: user.ID, email: email, name: name, subject: subject, identity: "user-" + itoa(user.ID), provider: provider, db: db}
	p.exec(t, `INSERT INTO porte_identities (user_id, provider, subject, access_token) VALUES (?, ?, ?, ?)`, p.id, provider, subject, "tok-"+subject)
	p.exec(t, `INSERT INTO porte_sessions (token_hash, user_id, label) VALUES (?, ?, ?)`, "hash-"+subject, p.id, name)
	p.exec(t, `INSERT INTO porte_login_codes (code_hash, user_id, expires_at, provider, subject) VALUES (?, ?, now(), ?, ?)`,
		"code-"+subject, p.id, provider, subject)
	return p
}

func (p person) seedCall(t *testing.T, slug string, owner int64) uuid.UUID {
	t.Helper()
	room, call := uuid.New(), uuid.New()
	p.exec(t, `INSERT INTO rooms (id, slug, name, owner_id) VALUES (?, ?, ?, ?)`, room, slug, slug, owner)
	p.exec(t, `INSERT INTO calls (id, room_id, started_at, livekit_room_name) VALUES (?, ?, now(), ?)`, call, room, slug)
	return call
}

func (p person) sitIn(t *testing.T, call uuid.UUID) {
	t.Helper()
	p.exec(t, `INSERT INTO call_participants (call_id, identity, user_id, name, joined_at) VALUES (?, ?, ?, ?, now())`,
		call, p.identity, p.id, p.name)
	p.exec(t, `INSERT INTO transcript_lines (call_id, speaker, text) VALUES (?, ?, 'bonjour')`, call, p.identity)
	p.exec(t, `INSERT INTO transcripts (id, call_id, content) VALUES (?, ?, ?) ON CONFLICT (call_id)
		DO UPDATE SET content = transcripts.content || EXCLUDED.content`, uuid.New(), call, p.identity+": bonjour\n")
	p.exec(t, `INSERT INTO summaries (id, call_id, content, model) VALUES (?, ?, ?, 'm') ON CONFLICT (call_id)
		DO UPDATE SET content = summaries.content || EXCLUDED.content`, uuid.New(), call, p.identity+" a ouvert la séance. ")
	p.exec(t, `INSERT INTO capture_consents (call_id, identity, transcribe) VALUES (?, ?, true)`, call, p.identity)
}

func fakeRegistre(t *testing.T, erased string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/directory/deletions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted": []directory.Deleted{{Subject: erased, DeletedAt: time.Now().Add(-time.Hour)}}})
	}))
	t.Cleanup(server.Close)
	return server
}

func textColumns(t *testing.T, db *gorm.DB) []struct{ Table, Column, Type string } {
	t.Helper()
	var columns []struct{ Table, Column, Type string }
	err := db.Raw(`SELECT table_name AS "table", column_name AS "column", data_type AS "type"
		FROM information_schema.columns WHERE table_schema = current_schema()
		AND data_type IN ('text', 'character varying', 'jsonb', 'json', 'bytea')
		AND table_name <> 'directory_deletion_state' ORDER BY 1, 2`).Scan(&columns).Error
	if err != nil || len(columns) < 20 {
		t.Fatalf("text columns = %d, %v", len(columns), err)
	}
	return columns
}

func mentions(t *testing.T, db *gorm.DB, needle string) []string {
	t.Helper()
	var found []string
	for _, c := range textColumns(t, db) {
		match := `strpos("` + c.Column + `"::text, ?) > 0`
		if c.Type == "bytea" {
			match = `position(convert_to(?, 'UTF8') in "` + c.Column + `") > 0`
		}
		var n int64
		if err := db.Raw(`SELECT count(*) FROM "`+c.Table+`" WHERE `+match, needle).Scan(&n).Error; err != nil {
			t.Fatalf("scan %s.%s: %v", c.Table, c.Column, err)
		}
		if n > 0 {
			found = append(found, c.Table+"."+c.Column)
		}
	}
	return found
}

// A Registre deletion notice, taken through the same poll the receiver runs
// in production, leaves no text column naming the person: not their email,
// their name, their subject or their user-<id> identity, in their own rooms
// or in the calls they joined in other people's rooms. The receiver's own
// record of handled subjects is the one exception, by design.
func TestARegistreDeletionLeavesNoTextColumnNamingThePerson(t *testing.T) {
	db := testdb.Migrated(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	registre := fakeRegistre(t, "sub-gone-7f3a")
	gone := seedPerson(t, db, registre.URL, "gone.7f3a@example.com", "Gonzague Effacé", "sub-gone-7f3a")
	kept := seedPerson(t, db, registre.URL, "kept@example.com", "Kept Person", "sub-kept")
	gone.sitIn(t, gone.seedCall(t, "gone-room", gone.id))
	shared := kept.seedCall(t, "kept-room", kept.id)
	kept.sitIn(t, shared)
	gone.sitIn(t, shared)

	users, err := auth.NewStore(context.Background(), sqlDB, slog.Default(), EraseData)
	if err != nil {
		t.Fatal(err)
	}
	client := directory.Client{Issuer: registre.URL, ClientID: "echo", ClientSecret: "secret"}
	erased := New(users, client, portepg.New(sqlDB).EraseIdentity, t.TempDir(), slog.Default())
	if err := erased.receiver.Poll(context.Background()); err != nil {
		t.Fatalf("poll: %v", err)
	}

	for _, needle := range []string{gone.email, gone.name, gone.subject, gone.identity} {
		if found := mentions(t, db, needle); len(found) > 0 {
			t.Errorf("%q survives in %v", needle, found)
		}
	}
	for _, needle := range []string{kept.email, kept.name, kept.subject, kept.identity} {
		if found := mentions(t, db, needle); len(found) == 0 {
			t.Errorf("another person's %q was erased too", needle)
		}
	}
	var lines int64
	if err := db.Raw(`SELECT count(*) FROM transcript_lines WHERE call_id = ?`, shared).Scan(&lines).Error; err != nil || lines != 2 {
		t.Fatalf("the shared call kept %d lines, %v, want 2", lines, err)
	}
}
