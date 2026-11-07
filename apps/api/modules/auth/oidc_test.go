package auth

import (
	"context"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte"
	"github.com/google/uuid"

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

func TestSignInTakesNameAndPictureFromTheClaimsAndRefreshesThem(t *testing.T) {
	store := NewUserStore(testdb.Migrated(t))
	first := claims("sub-pic", "pic@example.com", true)
	first.Picture = "https://sso.example/avatars/1.png"
	id := upsert(t, store, first)

	renamed := claims("sub-pic", "pic@example.com", true)
	renamed.Name = "Ada Lovelace"
	renamed.AvatarURL = "https://sso.example/avatars/2.png"
	if again, err := store.UpsertFromOIDC(context.Background(), renamed); err != nil || again != id {
		t.Fatalf("second login = %d, %v, want the same account %d", again, err, id)
	}
	var user schemas.User
	if err := store.orm.First(&user, id).Error; err != nil {
		t.Fatal(err)
	}
	if user.Name != "Ada Lovelace" || user.Picture != "https://sso.example/avatars/2.png" {
		t.Fatalf("user = %q %q, want the refreshed claims", user.Name, user.Picture)
	}

	unsafe := claims("sub-pic", "pic@example.com", true)
	unsafe.Picture = "javascript:alert(1)"
	if _, err := store.UpsertFromOIDC(context.Background(), unsafe); err != nil {
		t.Fatal(err)
	}
	store.orm.First(&user, id)
	if user.Picture != "https://sso.example/avatars/2.png" {
		t.Fatalf("a non-http picture replaced the stored one: %q", user.Picture)
	}
}

func TestFirstSSOLoginKeepsAnExistingLocalAccountAndItsData(t *testing.T) {
	store := NewUserStore(testdb.Migrated(t))
	local := schemas.User{Email: "owner@example.com", Name: "Owner"}
	if err := store.orm.Create(&local).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.orm.Exec(`INSERT INTO porte_identities (user_id, provider, subject) VALUES (?, 'local', ?)`, local.ID, "owner@example.com").Error; err != nil {
		t.Fatal(err)
	}
	room := schemas.Room{ID: uuid.New(), Slug: "mine", Name: "mine", OwnerID: &local.ID}
	if err := store.orm.Create(&room).Error; err != nil {
		t.Fatal(err)
	}

	id := upsert(t, store, claims("sub-owner", "Owner@Example.com", true))
	if id != local.ID {
		t.Fatalf("sso login created account %d, want the existing %d", id, local.ID)
	}
	var owned int64
	store.orm.Model(&schemas.Room{}).Where("owner_id = ?", local.ID).Count(&owned)
	if owned != 1 {
		t.Fatalf("rooms owned = %d, want 1", owned)
	}
}
