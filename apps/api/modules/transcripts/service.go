package transcripts

import (
	"context"
	"errors"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
func (s *Service) Append(ctx context.Context, slug, speaker, text string) error {
	line, err := transcriptLine(speaker, text)
	if err != nil {
		return err
	}
	return s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		call, err := lockOpenCall(tx, slug)
		if err != nil {
			return err
		}
		return appendLine(tx, call.ID, line)
	})
}

// lockOpenCall locks the call row for the duration of the transaction, which
// serialises concurrent appends for one call and keeps a call that is being
// closed from accepting a line on the way out.
func lockOpenCall(tx *gorm.DB, slug string) (schemas.Call, error) {
	var call schemas.Call
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("livekit_room_name = ? AND ended_at IS NULL", slug).
		Order("started_at DESC, id").
		First(&call).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return call, troncerrors.NotFound("no open call for this room")
	}
	return call, err
}

// appendLine concatenates one line onto the call's transcript, or starts the
// transcript when this is the first line. It refuses outright past the
// ceiling: a silent truncation would leave a record that reads complete.
func appendLine(tx *gorm.DB, callID uuid.UUID, line string) error {
	var transcript schemas.Transcript
	err := tx.Where("call_id = ?", callID).Order("created_at, id").First(&transcript).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&schemas.Transcript{ID: uuid.New(), CallID: callID, Content: line}).Error
	}
	if err != nil {
		return err
	}
	if len(transcript.Content)+len(line) > maxTranscriptBytes {
		return troncerrors.Conflict("this call's transcript has reached its maximum size")
	}
	return tx.Model(&transcript).Update("content", gorm.Expr("content || ?", line)).Error
}
