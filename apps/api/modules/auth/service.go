package auth

import (
	"context"
	stderrors "errors"

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
