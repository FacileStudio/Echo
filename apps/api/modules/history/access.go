package history

import (
	"context"
	"errors"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	roleOwner       = "owner"
	roleParticipant = "participant"
)

var errNotInCall = troncerrors.Forbidden("you were not part of this call")

// access is a call the caller may open, with the room it belongs to and the
// part the caller played in it.
type access struct {
	call *schemas.Call
	room schemas.Room
	role string
}

func (a access) owner() bool { return a.role == roleOwner }

// accessible loads a call for its room owner or for someone who sat in it as a
// signed-in account. Anyone else is refused, and so is every call of an
// unowned room to a person who never joined it: the check is always the
// caller's own id against the database, never a value the request supplies.
func (s *Service) accessible(ctx context.Context, callID string, callerID int64) (*access, error) {
	id, err := uuid.Parse(callID)
	if err != nil {
		return nil, troncerrors.NotFound("call not found")
	}
	var call schemas.Call
	if err := s.orm.WithContext(ctx).First(&call, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, troncerrors.NotFound("call not found")
		}
		return nil, err
	}
	var room schemas.Room
	if err := s.orm.WithContext(ctx).First(&room, "id = ?", call.RoomID).Error; err != nil {
		return nil, err
	}
	if room.OwnerID != nil && *room.OwnerID == callerID {
		return &access{call: &call, room: room, role: roleOwner}, nil
	}
	var seats int64
	err = s.orm.WithContext(ctx).Model(&schemas.CallParticipant{}).
		Where("call_id = ? AND user_id = ?", call.ID, callerID).Count(&seats).Error
	if err != nil {
		return nil, err
	}
	if seats == 0 {
		return nil, errNotInCall
	}
	return &access{call: &call, room: room, role: roleParticipant}, nil
}
