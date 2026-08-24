package transcripts

import (
	"errors"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service persists final transcript lines for the open call of a room.
type Service struct {
	orm *gorm.DB
}

// NewService builds a Service over the given database.
func NewService(orm *gorm.DB) *Service { return &Service{orm: orm} }

// Append adds one final utterance to the room's open call transcript. The
// call must still be open — after room_finished the record is history and no
// late captions may rewrite it.
func (s *Service) Append(slug, speaker, text string) error {
	var call schemas.Call
	err := s.orm.Where("livekit_room_name = ? AND ended_at IS NULL", slug).First(&call).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return troncerrors.NotFound("no open call for this room")
	}
	if err != nil {
		return err
	}

	line := speaker + ": " + text + "\n"
	var transcript schemas.Transcript
	err = s.orm.Where("call_id = ?", call.ID).First(&transcript).Error
	if err == nil {
		return s.orm.Model(&transcript).Update("content", gorm.Expr("content || ?", line)).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.orm.Create(&schemas.Transcript{
		ID:      uuid.New(),
		CallID:  call.ID,
		Content: line,
	}).Error
}
