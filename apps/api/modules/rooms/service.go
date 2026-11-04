package rooms

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var validSlugRunes = func(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-'
}

// ValidSlug reports whether slug is a legal room slug: 1-128 chars of
// lowercase letters, digits and hyphens.
func ValidSlug(slug string) bool {
	return len(slug) >= 1 && len(slug) <= 128 && strings.IndexFunc(slug, func(r rune) bool { return !validSlugRunes(r) }) < 0
}

// Service owns room persistence and mints join tokens through the media
// service.
type Service struct {
	orm       *gorm.DB
	media     *media.Service
	plans     *plans.Plans
	provision func(ctx context.Context, slug string, settings media.RoomSettings) error
}

// NewService builds a Service over the given database, media service and the
// plans that cap who may join.
func NewService(orm *gorm.DB, media *media.Service, plans *plans.Plans) *Service {
	return &Service{orm: orm, media: media, plans: plans, provision: media.EnsureRoom}
}

func (s *Service) Create(ctx context.Context, slug, name string, ownerID *int64) (*schemas.Room, error) {
	if name == "" {
		name = slug
	}
	if !ValidSlug(slug) {
		return nil, troncerrors.Invalid("slug must be lowercase alphanumeric characters and hyphens")
	}
	existing, err := s.BySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, troncerrors.Conflict("room already exists")
	}
	room := &schemas.Room{ID: uuid.New(), Slug: slug, Name: name, OwnerID: ownerID}
	if err := s.orm.WithContext(ctx).Create(room).Error; err != nil {
		if isDuplicateSlug(err) {
			return nil, troncerrors.Conflict("room already exists")
		}
		return nil, err
	}
	return room, nil
}

func (s *Service) BySlug(ctx context.Context, slug string) (*schemas.Room, error) {
	var room schemas.Room
	err := s.orm.WithContext(ctx).Where("slug = ?", slug).First(&room).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (s *Service) OwnedBy(ctx context.Context, ownerID int64) ([]schemas.Room, error) {
	var rooms []schemas.Room
	if err := s.orm.WithContext(ctx).Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&rooms).Error; err != nil {
		return nil, err
	}
	return rooms, nil
}

func (s *Service) Rename(ctx context.Context, slug, name string, callerID int64) (*schemas.Room, error) {
	room, err := s.bySlugOwned(ctx, slug, callerID)
	if err != nil {
		return nil, err
	}
	room.Name = name
	if err := s.orm.WithContext(ctx).Save(room).Error; err != nil {
		return nil, err
	}
	return room, nil
}

func (s *Service) Delete(ctx context.Context, slug string, callerID int64) error {
	room, err := s.bySlugOwned(ctx, slug, callerID)
	if err != nil {
		return err
	}
	return s.orm.WithContext(ctx).Delete(room).Error
}

// RequireOwner fails unless slug names an existing room owned by callerID.
func (s *Service) RequireOwner(ctx context.Context, slug string, callerID int64) error {
	_, err := s.bySlugOwned(ctx, slug, callerID)
	return err
}

func (s *Service) bySlugOwned(ctx context.Context, slug string, callerID int64) (*schemas.Room, error) {
	room, err := s.BySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, troncerrors.NotFound("room not found")
	}
	if room.OwnerID == nil || *room.OwnerID != callerID {
		return nil, troncerrors.Forbidden("only the room owner can do this")
	}
	return room, nil
}

func (s *Service) Join(ctx context.Context, slug string, ownerID *int64) (*schemas.Room, error) {
	room, err := s.BySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if room != nil {
		return room, nil
	}
	if !ValidSlug(slug) {
		return nil, troncerrors.Invalid("slug must be lowercase alphanumeric characters and hyphens")
	}
	room = &schemas.Room{ID: uuid.New(), Slug: slug, Name: slug, OwnerID: ownerID}
	if err := s.orm.WithContext(ctx).Create(room).Error; err != nil {
		if isDuplicateSlug(err) {
			return s.BySlug(ctx, slug)
		}
		return nil, err
	}
	return room, nil
}

func isDuplicateSlug(err error) bool {
	var pgErr *pgconn.PgError
	return stderrors.As(err, &pgErr) && pgErr.Code == "23505"
}

var moderatorGrant = media.Grant{
	CanPublish:     true,
	CanPublishData: true,
	CanSubscribe:   true,
	RoomAdmin:      true,
}

var guestGrant = media.Grant{
	CanPublish:     true,
	CanPublishData: false,
	CanSubscribe:   true,
	RoomAdmin:      false,
}

// Token admits the caller into the room's call under the owner's plan and
// mints their LiveKit join token. A full call answers a *forfait.LimitError.
func (s *Service) Token(ctx context.Context, slug string, userID *int64, email, displayName string) (Admission, error) {
	room, err := s.Join(ctx, slug, userID)
	if err != nil {
		return Admission{}, err
	}
	if room.IntegrationApp != "" {
		return Admission{}, troncerrors.Forbidden("this call is joined from the app that created it")
	}
	identity, name, grant := "guest-"+uuid.NewString()[:8], displayName, guestGrant
	if userID != nil {
		identity, name, grant = fmt.Sprintf("user-%d", *userID), displayNameOr(email, displayName), moderatorGrant
	} else if displayName == "" {
		return Admission{}, troncerrors.Invalid("display_name is required for guests")
	}
	deadline, err := s.admit(ctx, room, identity)
	if err != nil {
		return Admission{}, err
	}
	token, err := s.media.Issue(room.Slug, identity, name, grant)
	return Admission{Token: token, Deadline: deadline}, err
}

func displayNameOr(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
