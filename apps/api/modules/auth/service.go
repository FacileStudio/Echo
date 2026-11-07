package auth

import (
	"context"
	stderrors "errors"

	"github.com/FacileStudio/Echo/apps/api/internal/authcontext"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/tronc/errors"

	"gorm.io/gorm"
)

// Service reads Echo's accounts. Sign-in itself belongs to porte's OIDC kit.
type Service struct {
	orm *gorm.DB
}

// NewService builds the auth service over Echo's user table.
func NewService(orm *gorm.DB) *Service {
	return &Service{orm: orm}
}

// IdentityForUser turns the user id porte authenticated into the identity the
// rest of Echo reads. porte hands the middleware a session and a user id and
// stops there: it holds no email and no is_admin, because what a role may do
// is the app's business. A session pointing at a user that no longer exists
// (the foreign key cascades a delete, so this is a race rather than a leak) is
// still not an authenticated request and is refused.
func (s *Service) IdentityForUser(ctx context.Context, userID int64) (authcontext.Identity, error) {
	var out struct {
		ID      int64
		Email   string
		Name    string
		Picture string
		IsAdmin bool
	}
	err := s.orm.WithContext(ctx).
		Model(&schemas.User{}).
		Select("id", "email", "name", "picture", "is_admin").
		Where("id = ?", userID).
		Scan(&out).Error
	if err != nil {
		return authcontext.Identity{}, errors.Internal("failed to load the account", err)
	}
	if out.ID == 0 {
		return authcontext.Identity{}, errors.Unauthorized("invalid auth token")
	}
	return authcontext.Identity{UserID: out.ID, Email: out.Email, Name: out.Name, Picture: out.Picture, IsAdmin: out.IsAdmin}, nil
}

// UserByID loads one account.
func (s *Service) UserByID(ctx context.Context, id int64) (*schemas.User, error) {
	var user schemas.User
	if err := s.orm.WithContext(ctx).First(&user, id).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.NotFound("user not found")
		}
		return nil, errors.Internal("failed to load user", err)
	}
	return &user, nil
}
