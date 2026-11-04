package integrations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	stderrors "errors"
	"strings"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// Service creates integration rooms, mints their join tokens and reports
// who is in them.
type Service struct {
	orm   *gorm.DB
	media *media.Service
	rooms roomCreator
	keys  Keys
}

// roomCreator creates the LiveKit room before a token for it is minted.
// LiveKit's room.auto_create is off in Echo's deployment.
type roomCreator interface {
	EnsureRoom(ctx context.Context, roomName string, settings media.RoomSettings) (media.LiveRoom, error)
}

// NewService builds a Service over the database, the media service and the
// integrated apps' secrets.
func NewService(orm *gorm.DB, media *media.Service, keys Keys) *Service {
	return &Service{orm: orm, media: media, rooms: media, keys: keys}
}

// Ensure returns the app's room for externalID, creating it on first use.
// A repeated call updates the title and callback URL it carries, keeps the
// ones it leaves empty, and reports created=false.
func (s *Service) Ensure(ctx context.Context, app string, req CreateRoomRequest) (*schemas.Room, bool, error) {
	if err := validateCreate(req); err != nil {
		return nil, false, err
	}
	existing, err := s.byExternalID(ctx, app, req.ExternalID)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, s.refresh(ctx, existing, req)
	}
	room, err := newRoom(app, req)
	if err != nil {
		return nil, false, err
	}
	if err := s.orm.WithContext(ctx).Create(room).Error; err != nil {
		if !isUniqueViolation(err) {
			return nil, false, err
		}
		existing, err := s.byExternalID(ctx, app, req.ExternalID)
		if err == nil && existing == nil {
			err = troncerrors.Conflict("room id collision, retry")
		}
		return existing, false, err
	}
	return room, true, nil
}

// Room returns the app's room with the given id, or a 404 when it is missing
// or belongs to another app.
func (s *Service) Room(ctx context.Context, app, roomID string) (*schemas.Room, error) {
	var room schemas.Room
	err := s.orm.WithContext(ctx).
		Where("slug = ? AND integration_app = ? AND integration_app <> ''", roomID, app).
		First(&room).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, troncerrors.NotFound("room not found")
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (s *Service) byExternalID(ctx context.Context, app, externalID string) (*schemas.Room, error) {
	var room schemas.Room
	err := s.orm.WithContext(ctx).
		Where("integration_app = ? AND external_id = ?", app, externalID).
		First(&room).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (s *Service) refresh(ctx context.Context, room *schemas.Room, req CreateRoomRequest) error {
	title, callback := room.Name, room.CallbackURL
	if trimmed := strings.TrimSpace(req.Title); trimmed != "" {
		title = trimmed
	}
	if req.CallbackURL != "" {
		callback = req.CallbackURL
	}
	if room.Name == title && room.CallbackURL == callback {
		return nil
	}
	room.Name, room.CallbackURL = title, callback
	return s.orm.WithContext(ctx).Model(room).
		Updates(map[string]any{"name": title, "callback_url": callback}).Error
}

func newRoom(app string, req CreateRoomRequest) (*schemas.Room, error) {
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return nil, err
	}
	return &schemas.Room{
		ID:             uuid.New(),
		Slug:           app + "-" + hex.EncodeToString(suffix),
		Name:           titleOr(req),
		IntegrationApp: app,
		ExternalID:     req.ExternalID,
		CallbackURL:    req.CallbackURL,
	}, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return stderrors.As(err, &pgErr) && pgErr.Code == "23505"
}
