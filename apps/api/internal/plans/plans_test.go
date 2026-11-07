package plans

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/directory"
	"github.com/FacileStudio/porte/forfait"
	"github.com/FacileStudio/tronc/errors"
)

const issuer = "https://sso.example.test"

func linkedUser(t *testing.T, p *Plans, email, provider, subject string) int64 {
	t.Helper()
	user := schemas.User{Email: email}
	if err := p.orm.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if provider != "" {
		err := p.orm.Exec(`INSERT INTO porte_identities (user_id, provider, subject) VALUES (?, ?, ?)`,
			user.ID, provider, subject).Error
		if err != nil {
			t.Fatalf("link identity: %v", err)
		}
	}
	return user.ID
}

func proSnapshot() directory.Snapshot {
	return directory.Snapshot{Version: 3, EntitlementsRevision: 2, People: []directory.Person{{
		Subject:      "sub-pro",
		Entitlements: map[string]json.RawMessage{KeyMaxParticipants: json.RawMessage(`25`), KeyRecording: json.RawMessage(`true`)},
	}}}
}

func TestTheOwnerReadsTheirRegistreSubject(t *testing.T) {
	p := New(testdb.Migrated(t), issuer, nil)
	sso := linkedUser(t, p, "sso@example.test", issuer, "sub-pro")
	local := linkedUser(t, p, "local@example.test", "local", "2")
	other := linkedUser(t, p, "other@example.test", "https://elsewhere.test", "sub-pro")
	for userID, want := range map[int64]string{sso: "sub-pro", local: "", other: ""} {
		got, err := p.SubjectOf(context.Background(), userID)
		if err != nil || got != want {
			t.Errorf("SubjectOf(%d) = %q, %v; want %q", userID, got, err, want)
		}
	}
	if err := p.Apply(context.Background(), proSnapshot()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	owned, err := p.ForRoom(context.Background(), schemas.Room{OwnerID: &sso})
	if err != nil || p.Limit(owned, KeyMaxParticipants) != 25 {
		t.Fatalf("owned room limit = %d, %v; want the owner's 25", p.Limit(owned, KeyMaxParticipants), err)
	}
	orphan, _ := p.ForRoom(context.Background(), schemas.Room{})
	if got := p.Limit(orphan, KeyMaxParticipants); got != 4 {
		t.Fatalf("unowned room limit = %d, want the free 4", got)
	}
	integration, _ := p.ForRoom(context.Background(), schemas.Room{IntegrationApp: "prisme"})
	if got := p.Limit(integration, KeyMaxCallMinutes); got != -1 || p.Require(integration, KeyRecording) != nil {
		t.Fatalf("integration room is limited (%d minutes)", got)
	}
}

func TestEntitlementsSurviveARestart(t *testing.T) {
	db := testdb.Migrated(t)
	before := New(db, issuer, nil)
	if err := before.Load(context.Background()); err != nil {
		t.Fatalf("load with no row: %v", err)
	}
	if !before.store.Enabled(forfait.Scope{Subject: "anyone"}, KeyRecording) {
		t.Fatal("a store that never synced is not failing open")
	}
	if err := before.Apply(context.Background(), proSnapshot()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	after := New(db, issuer, nil)
	if err := after.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	if after.store.Enabled(forfait.Scope{Subject: "anyone"}, KeyRecording) {
		t.Fatal("the restarted store failed open again")
	}
	if got := after.store.Limit(forfait.Scope{Subject: "sub-pro"}, KeyMaxParticipants); got != 25 {
		t.Fatalf("restored limit = %d, want 25", got)
	}
}

func TestABlankedRowFailsClosed(t *testing.T) {
	for _, document := range []string{"", "  \n"} {
		db := testdb.Migrated(t)
		if err := db.Create(&schemas.ForfaitState{ID: schemas.ForfaitStateID, Document: document}).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
		p := New(db, issuer, nil)
		if err := p.Load(context.Background()); err != nil {
			t.Fatalf("a blank row refused the boot: %v", err)
		}
		if p.store.Enabled(forfait.Scope{}, KeyRecording) || p.store.Limit(forfait.Scope{}, KeyMaxParticipants) != 4 {
			t.Fatalf("a blank row %q failed open: forfait imports empty input as a no-op", document)
		}
	}
}

func TestACorruptRowFailsClosedUntilTheNextSnapshot(t *testing.T) {
	db := testdb.Migrated(t)
	if err := db.Create(&schemas.ForfaitState{ID: schemas.ForfaitStateID, Document: `{"version":9}`}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	p := New(db, issuer, nil)
	if err := p.Load(context.Background()); err != nil {
		t.Fatalf("a corrupt row refused the boot: %v", err)
	}
	if p.store.Enabled(forfait.Scope{}, KeyRecording) || p.store.Limit(forfait.Scope{}, KeyMaxParticipants) != 4 {
		t.Fatal("a corrupt row failed open instead of enforcing the free defaults")
	}
	if err := p.Apply(context.Background(), proSnapshot()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := p.store.Limit(forfait.Scope{Subject: "sub-pro"}, KeyMaxParticipants); got != 25 {
		t.Fatalf("a fresh snapshot after a corrupt row reads %d, want 25", got)
	}
}

func TestARefusalIsA402WithTheUpgradeLink(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteError(recorder, &forfait.LimitError{Key: KeyMaxParticipants, Limit: 4, UpgradeURL: issuer + "/spaces"})
	if recorder.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", recorder.Code)
	}
	var body struct{ Error refusalBody }
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := refusalBody{Code: "payment_required", Message: body.Error.Message, Key: KeyMaxParticipants,
		Limit: 4, UpgradeURL: issuer + "/spaces"}
	if body.Error != want || body.Error.Message == "" {
		t.Fatalf("body = %+v, want %+v", body.Error, want)
	}
	for _, unsafe := range []string{"javascript:alert(1)", "http://sso.example.test/spaces", "https://u:p@evil.test/"} {
		recorder = httptest.NewRecorder()
		WriteError(recorder, &forfait.LimitError{Key: KeyRecording, UpgradeURL: unsafe})
		var unsafeBody struct{ Error refusalBody }
		if json.Unmarshal(recorder.Body.Bytes(), &unsafeBody) != nil || unsafeBody.Error.UpgradeURL != "" {
			t.Errorf("unsafe upgrade url %q reached the client", unsafe)
		}
	}
	recorder = httptest.NewRecorder()
	WriteError(recorder, errors.NotFound("room not found"))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("a plain error answered %d", recorder.Code)
	}
}

func TestTheDeadlineFollowsTheLimit(t *testing.T) {
	p := New(nil, issuer, nil)
	testStart := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	if _, limited := p.Deadline(Policy{}, testStart); limited {
		t.Fatal("a store that never synced limited call length")
	}
	p.store.Apply(directory.Snapshot{EntitlementsRevision: 1})
	if got, limited := p.Deadline(Policy{}, testStart); !limited || !got.Equal(testStart.Add(40*time.Minute)) {
		t.Fatalf("free deadline = %v, %v", got, limited)
	}
	if _, limited := p.Deadline(Policy{Unlimited: true}, testStart); limited {
		t.Fatal("an unlimited policy got a deadline")
	}
}

func TestAIAccessFailsOpenOnlyBeforeTheFirstRevision(t *testing.T) {
	p := New(testdb.Migrated(t), issuer, nil)
	owner := forfait.Scope{Subject: "sub-pro"}
	if err := p.Require(Policy{Scope: owner}, KeyAIAccess); err != nil {
		t.Fatalf("before the first revision ai.access = %v, want open", err)
	}

	if err := p.Apply(context.Background(), proSnapshot()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := p.Require(Policy{Scope: owner}, KeyAIAccess); err == nil {
		t.Fatal("an owner whose entitlements lack ai.access was allowed after the first revision")
	}
	if err := p.Require(Policy{}, KeyAIAccess); err == nil {
		t.Fatal("a room with no linked owner was allowed ai.access")
	}

	comped := proSnapshot()
	comped.EntitlementsRevision++
	comped.People[0].Entitlements[KeyAIAccess] = json.RawMessage(`true`)
	if err := p.Apply(context.Background(), comped); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := p.Require(Policy{Scope: owner}, KeyAIAccess); err != nil {
		t.Fatalf("a comped owner once the catalog publishes ai.access = %v, want allowed", err)
	}
	if err := p.Require(Policy{Scope: forfait.Scope{Subject: "sub-other"}}, KeyAIAccess); err == nil {
		t.Fatal("another owner inherited ai.access")
	}
}
