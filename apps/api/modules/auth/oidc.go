package auth

import (
	"context"
	stderrors "errors"
	"net/mail"
	"net/url"
	"strings"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte"
	"github.com/FacileStudio/tronc/errors"

	"gorm.io/gorm"
)

// UserStore resolves an OIDC callback to a row in Echo's own users table.
// porte offers UserStore as the escape hatch for exactly this: users carries
// is_admin, which porte transports no opinion about, and the first account
// created here becomes the administrator.
type UserStore struct {
	orm *gorm.DB
}

// NewUserStore builds a UserStore over the given database.
func NewUserStore(orm *gorm.DB) *UserStore {
	return &UserStore{orm: orm}
}

var _ porte.UserStore = (*UserStore)(nil)

const registrationLockKey = 0x4563686F

// UpsertFromOIDC matches on (provider, subject) first, falls back to a
// verified email, and creates the account when neither finds anything.
//
// Matching an existing account on the address alone is an account-takeover
// primitive when the provider lets a user claim any address without proving
// it, so an unverified claim is refused. A verified address links only onto
// an account that holds no identity from this issuer yet: an account already
// linked to another subject of the same issuer belongs to somebody else, so a
// recycled address is refused rather than handed over. The first account ever
// created becomes admin.
func (s *UserStore) UpsertFromOIDC(ctx context.Context, claims porte.Claims) (int64, error) {
	claims.Email = strings.ToLower(strings.TrimSpace(claims.Email))
	if _, err := mail.ParseAddress(claims.Email); err != nil {
		return 0, errors.Invalid("the identity provider returned no usable email")
	}
	var userID int64
	err := s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(registrationLockKey)).Error; err != nil {
			return errors.Internal("failed to acquire registration lock", err)
		}
		id, err := resolve(tx, claims)
		userID = id
		return err
	})
	return userID, err
}

func resolve(tx *gorm.DB, claims porte.Claims) (int64, error) {
	linked, err := linkedAccount(tx, claims)
	if err != nil {
		return 0, err
	}
	if linked != 0 {
		return linked, refreshProfile(tx, linked, claims)
	}
	var existing schemas.User
	err = tx.Where("email = ?", claims.Email).First(&existing).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return createAccount(tx, claims)
	}
	if err != nil {
		return 0, errors.Internal("failed to look up the account", err)
	}
	if err := checkLinkable(tx, existing.ID, claims); err != nil {
		return 0, err
	}
	return existing.ID, refreshProfile(tx, existing.ID, claims)
}

func linkedAccount(tx *gorm.DB, claims porte.Claims) (int64, error) {
	var linked int64
	err := tx.Raw(
		`SELECT user_id FROM porte_identities WHERE provider = ? AND subject = ?`,
		claims.Provider, claims.Subject,
	).Scan(&linked).Error
	if err != nil {
		return 0, errors.Internal("failed to resolve the identity", err)
	}
	return linked, nil
}

func checkLinkable(tx *gorm.DB, userID int64, claims porte.Claims) error {
	if !claims.EmailVerified {
		return errors.Conflict("an account with this email already exists and the identity provider did not verify the address")
	}
	var held int64
	err := tx.Raw(
		`SELECT count(*) FROM porte_identities WHERE user_id = ? AND provider = ?`,
		userID, claims.Provider,
	).Scan(&held).Error
	if err != nil {
		return errors.Internal("failed to check the account's identities", err)
	}
	if held > 0 {
		return errors.Conflict("this email already belongs to another identity of this provider")
	}
	return nil
}

func createAccount(tx *gorm.DB, claims porte.Claims) (int64, error) {
	var count int64
	if err := tx.Model(&schemas.User{}).Count(&count).Error; err != nil {
		return 0, errors.Internal("failed to count users", err)
	}
	user := schemas.User{
		Email:   claims.Email,
		Name:    claims.DisplayName(),
		Picture: pictureOf(claims),
		Locale:  porte.NormalizeLocale(claims.Locale),
		IsAdmin: count == 0,
	}
	if err := tx.Create(&user).Error; err != nil {
		return 0, errors.Internal("failed to create the account", err)
	}
	return user.ID, nil
}

func refreshProfile(tx *gorm.DB, userID int64, claims porte.Claims) error {
	updates := map[string]any{}
	if claims.EmailVerified {
		updates["email"] = claims.Email
	}
	if name := claims.DisplayName(); name != "" {
		updates["name"] = name
	}
	if picture := pictureOf(claims); picture != "" {
		updates["picture"] = picture
	}
	if claims.Locale != "" {
		updates["locale"] = porte.NormalizeLocale(claims.Locale)
	}
	if len(updates) == 0 {
		return nil
	}
	if err := tx.Model(&schemas.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return errors.Internal("failed to update the account", err)
	}
	return nil
}

func pictureOf(claims porte.Claims) string {
	for _, raw := range []string{claims.AvatarURL, claims.Picture} {
		parsed, err := url.Parse(raw)
		if err == nil && parsed.Host != "" && (parsed.Scheme == "https" || parsed.Scheme == "http") {
			return raw
		}
	}
	return ""
}
