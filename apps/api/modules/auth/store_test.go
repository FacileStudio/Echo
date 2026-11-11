package auth

import (
	"context"
	"log/slog"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/porte"
	"github.com/FacileStudio/porte/kit"
	"gorm.io/gorm"
)

const issuer = "https://sso.example"

func newStore(t *testing.T) (*kit.Store, *gorm.DB) {
	t.Helper()
	db := testdb.Migrated(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(context.Background(), sqlDB, slog.New(slog.DiscardHandler), nil)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store, db
}

func signIn(t *testing.T, store *kit.Store, db *gorm.DB, subject, email string) int64 {
	t.Helper()
	claims := porte.Claims{Provider: issuer, Subject: subject, Email: email, EmailVerified: true, Name: "Ada", Locale: "fr"}
	id, err := store.UpsertFromOIDC(context.Background(), claims)
	if err != nil {
		t.Fatalf("sign in %s: %v", subject, err)
	}
	err = db.Exec(`INSERT INTO porte_identities (user_id, provider, subject) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
		id, issuer, subject).Error
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// Every nullable column without a default is one the kit's insert leaves
// NULL. Echo accepts none, so a new nullable column fails here first.
func TestTheKitLeavesNoColumnUnwritten(t *testing.T) {
	store, _ := newStore(t)
	if got := store.Unwritten(); len(got) != 0 {
		t.Fatalf("unwritten = %v, want none", got)
	}
}

// A kit sign-in fills every column an account created before the kit has
// filled, makes the first account admin and only the first.
func TestAKitSignInFillsEveryColumnAndOnlyTheFirstIsAdmin(t *testing.T) {
	store, db := newStore(t)
	first := signIn(t, store, db, "sub-1", "Ada@Example.com")
	second := signIn(t, store, db, "sub-2", "bob@example.com")
	var nulls int64
	err := db.Raw(`SELECT count(*) FROM users WHERE email IS NULL OR name IS NULL OR password_hash IS NULL
		OR picture IS NULL OR locale IS NULL OR is_admin IS NULL OR created_at IS NULL`).Scan(&nulls).Error
	if err != nil || nulls != 0 {
		t.Fatalf("accounts with a NULL column = %d, %v", nulls, err)
	}
	var admins []int64
	if err := db.Raw(`SELECT id FROM users WHERE is_admin`).Scan(&admins).Error; err != nil {
		t.Fatal(err)
	}
	if len(admins) != 1 || admins[0] != first || first == second {
		t.Fatalf("admins = %v, want only %d", admins, first)
	}
	identity, err := store.Identity(context.Background(), first)
	if err != nil || identity.Email != "ada@example.com" || identity.Name != "Ada" || !identity.IsAdmin {
		t.Fatalf("identity = %+v, %v", identity, err)
	}
}

// An account made before single sign-on links by verified email at its
// first Registre sign-in and keeps its rooms.
func TestAVerifiedSignInLinksAnExistingAccount(t *testing.T) {
	store, db := newStore(t)
	var legacy int64
	if err := db.Raw(`INSERT INTO users (email, name) VALUES ('owner@example.com', 'Owner') RETURNING id`).Scan(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if got := signIn(t, store, db, "sub-owner", "Owner@Example.com"); got != legacy {
		t.Fatalf("signed in as %d, want the existing account %d", got, legacy)
	}
}
