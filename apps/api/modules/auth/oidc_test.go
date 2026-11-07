package auth

import (
	"context"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte"

	"gorm.io/gorm"
)

const issuer = "https://sso.example"

func claims(subject, email string, verified bool) porte.Claims {
	return porte.Claims{Provider: issuer, Subject: subject, Email: email, EmailVerified: verified, Name: "Ada", Locale: "fr-FR"}
}

func link(t *testing.T, db *gorm.DB, userID int64, subject string) {
	t.Helper()
	err := db.Exec(`INSERT INTO porte_identities (user_id, provider, subject) VALUES (?, ?, ?)`, userID, issuer, subject).Error
	if err != nil {
		t.Fatalf("link identity: %v", err)
	}
}

func upsert(t *testing.T, store *UserStore, c porte.Claims) int64 {
	t.Helper()
	id, err := store.UpsertFromOIDC(context.Background(), c)
	if err != nil {
		t.Fatalf("upsert %s: %v", c.Subject, err)
	}
	link(t, store.orm, id, c.Subject)
	return id
}

func TestFirstLoginCreatesAnAdminWithTheSessionLocale(t *testing.T) {
	store := NewUserStore(testdb.Migrated(t))
	id := upsert(t, store, claims("sub-1", "Ada@Example.com", true))

	var user schemas.User
	if err := store.orm.First(&user, id).Error; err != nil {
		t.Fatal(err)
	}
	if user.Email != "ada@example.com" || user.Locale != "fr" || !user.IsAdmin {
		t.Fatalf("user = %+v, want a lowercased email, locale fr and admin", user)
	}
}

func TestVerifiedEmailLinksOntoAnAccountWithNoIdentityOfTheIssuer(t *testing.T) {
	store := NewUserStore(testdb.Migrated(t))
	legacy := schemas.User{Email: "old@example.com", PasswordHash: ""}
	if err := store.orm.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if got := upsert(t, store, claims("sub-old", "old@example.com", true)); got != legacy.ID {
		t.Fatalf("linked to %d, want the existing account %d", got, legacy.ID)
	}
}

func TestUnverifiedEmailNeverLinksOrRewritesAnAddress(t *testing.T) {
	store := NewUserStore(testdb.Migrated(t))
	owner := upsert(t, store, claims("sub-owner", "owner@example.com", true))

	if _, err := store.UpsertFromOIDC(context.Background(), claims("sub-squat", "owner@example.com", false)); err == nil {
		t.Fatal("an unverified claim linked onto an existing address")
	}
	hijack := claims("sub-owner", "victim@example.com", false)
	if _, err := store.UpsertFromOIDC(context.Background(), hijack); err != nil {
		t.Fatal(err)
	}
	var user schemas.User
	if err := store.orm.First(&user, owner).Error; err != nil {
		t.Fatal(err)
	}
	if user.Email != "owner@example.com" {
		t.Fatalf("email = %q, an unverified claim rewrote it", user.Email)
	}
}

func TestAnAccountHoldingAnotherSubjectOfTheIssuerRefusesTheLink(t *testing.T) {
	store := NewUserStore(testdb.Migrated(t))
	upsert(t, store, claims("sub-first", "shared@example.com", true))

	if _, err := store.UpsertFromOIDC(context.Background(), claims("sub-second", "shared@example.com", true)); err == nil {
		t.Fatal("a second subject of the same issuer took over the address")
	}
}
