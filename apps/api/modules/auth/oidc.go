package auth

import (
	"context"
	stderrors "errors"
	"net/mail"
	"strings"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte"
	"github.com/FacileStudio/tronc/errors"

	"gorm.io/gorm"
)

// UserStore resolves an OIDC callback to a row in Echo's own users table.
// porte offers UserStore as the escape hatch for exactly this: users carries
// is_admin — which porte transports no opinion about — and the first account
// created here becomes the administrator.
type UserStore struct {
	orm *gorm.DB
}

// NewUserStore builds a UserStore over the given database.
func NewUserStore(orm *gorm.DB) *UserStore {
	return &UserStore{orm: orm}
}

var (
	_ porte.UserStore         = (*UserStore)(nil)
	_ porte.PasswordUserStore = (*UserStore)(nil)
)

const registrationLockKey = 0x4563686F

// UpsertFromOIDC matches on (provider, subject) first, falls back to a
// verified email, and creates the account when neither finds anything.
//
// Matching an existing account on the address alone is an account-takeover
// primitive when the provider lets a user claim any address without proving
// it, so an unverified claim is refused. Authentik returns email_verified
// hardcoded false, so on this IdP the email fallback never fires and adoption
// must go through backfilled identities rather than this branch. The first
// account ever created becomes admin.
func (s *UserStore) UpsertFromOIDC(ctx context.Context, claims porte.Claims) (int64, error) {
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return 0, errors.Invalid("the identity provider returned no usable email")
	}
	name := claims.DisplayName()

	var userID int64
	txErr := s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(registrationLockKey)).Error; err != nil {
			return errors.Internal("failed to acquire registration lock", err)
		}

		if err := s.linkedIdentity(tx, claims, &userID); err != nil {
			return err
		}
		if userID != 0 {
			return refreshProfile(tx, userID, email, name)
		}
		if err := s.adoptByEmail(tx, claims, email, name, &userID); err != nil {
			return err
		}
		if userID != 0 {
			return nil
		}
		return createAccount(tx, email, name, &userID)
	})
	if txErr != nil {
		return 0, txErr
	}
	return userID, nil
}

func (s *UserStore) linkedIdentity(tx *gorm.DB, claims porte.Claims, userID *int64) error {
	var linked int64
	err := tx.Raw(
		`SELECT user_id FROM porte_identities WHERE provider = ? AND subject = ?`,
		claims.Provider, claims.Subject,
	).Scan(&linked).Error
	if err != nil {
		return errors.Internal("failed to resolve the identity", err)
	}
	*userID = linked
	return nil
}

func (s *UserStore) adoptByEmail(tx *gorm.DB, claims porte.Claims, email, name string, userID *int64) error {
	var existing schemas.User
	err := tx.Where("email = ?", email).First(&existing).Error
	switch {
	case err == nil:
		if !claims.EmailVerified {
			return errors.Conflict("an account with this email already exists and the identity provider did not verify the address")
		}
		*userID = existing.ID
		return refreshProfile(tx, existing.ID, email, name)
	case !stderrors.Is(err, gorm.ErrRecordNotFound):
		return errors.Internal("failed to look up the account", err)
	}
	return nil
}

func createAccount(tx *gorm.DB, email, name string, userID *int64) error {
	var count int64
	if err := tx.Model(&schemas.User{}).Count(&count).Error; err != nil {
		return errors.Internal("failed to count users", err)
	}
	user := schemas.User{Email: email, Name: name, PasswordHash: "", IsAdmin: count == 0}
	if err := tx.Create(&user).Error; err != nil {
		return errors.Internal("failed to create the account", err)
	}
	*userID = user.ID
	return nil
}

func refreshProfile(tx *gorm.DB, userID int64, email, name string) error {
	updates := map[string]any{"email": email}
	if name != "" {
		updates["name"] = name
	}
	if err := tx.Model(&schemas.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return errors.Internal("failed to update the account", err)
	}
	return nil
}

func (s *UserStore) CreateFromPassword(ctx context.Context, email, name string) (int64, error) {
	var userID int64
	txErr := s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(registrationLockKey)).Error; err != nil {
			return errors.Internal("failed to acquire registration lock", err)
		}
		var count int64
		if err := tx.Model(&schemas.User{}).Count(&count).Error; err != nil {
			return errors.Internal("failed to count users", err)
		}
		var existing int64
		if err := tx.Model(&schemas.User{}).Where("email = ?", email).Count(&existing).Error; err != nil {
			return errors.Internal("failed to check email", err)
		}
		if existing > 0 {
			return errors.Conflict("an account with this email already exists")
		}
		user := schemas.User{Email: email, Name: name, PasswordHash: "", IsAdmin: count == 0}
		if err := tx.Create(&user).Error; err != nil {
			if stderrors.Is(err, gorm.ErrDuplicatedKey) {
				return errors.Conflict("an account with this email already exists")
			}
			return errors.Internal("failed to create the account", err)
		}
		userID = user.ID
		return nil
	})
	if txErr != nil {
		return 0, txErr
	}
	return userID, nil
}

func (s *UserStore) FindByEmail(ctx context.Context, email string) (int64, error) {
	var user schemas.User
	err := s.orm.WithContext(ctx).Select("id").Where("email = ?", email).First(&user).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return 0, porte.ErrNotFound
	}
	if err != nil {
		return 0, errors.Internal("failed to look up the account", err)
	}
	return user.ID, nil
}

func (s *UserStore) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	if err := s.orm.WithContext(ctx).Model(&schemas.User{}).Count(&count).Error; err != nil {
		return 0, errors.Internal("failed to count users", err)
	}
	return count, nil
}

// ConfigExtra adds allow_registration to GET /auth/config. porte serves
// sso_only and oidc_enabled there itself.
func ConfigExtra(allowRegistration bool) func() map[string]any {
	return func() map[string]any {
		return map[string]any{"allow_registration": allowRegistration}
	}
}
