package history

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var errNotShared = troncerrors.Forbidden("this recording is not shared with you")

type recordingOut struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Shared    bool   `json:"shared"`
	CanShare  bool   `json:"can_share"`
}

// visible is the whole recording policy: the room owner sees every recording
// of their calls, everyone else sees only the ones the owner shared.
func (a access) visible(rec schemas.Recording) bool { return a.owner() || rec.Shared }

// Recordings lists the recordings of a call the caller can see, newest first.
func (s *Service) Recordings(ctx context.Context, callID string, callerID int64) ([]recordingOut, error) {
	got, err := s.accessible(ctx, callID, callerID)
	if err != nil {
		return nil, err
	}
	var rows []schemas.Recording
	err = s.orm.WithContext(ctx).Where("call_id = ? AND status = ?", got.call.ID, schemas.RecordingComplete).Order("created_at DESC, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]recordingOut, 0, len(rows))
	for _, rec := range rows {
		if got.visible(rec) {
			out = append(out, toRecording(rec, got.owner()))
		}
	}
	return out, nil
}

func toRecording(rec schemas.Recording, canShare bool) recordingOut {
	return recordingOut{
		ID:        rec.ID.String(),
		CreatedAt: rec.CreatedAt.UTC().Format(time.RFC3339),
		Shared:    rec.Shared,
		CanShare:  canShare,
	}
}

func (s *Service) hasVisibleRecording(ctx context.Context, got *access) (bool, error) {
	query := s.orm.WithContext(ctx).Model(&schemas.Recording{}).Where("call_id = ? AND status = ?", got.call.ID, schemas.RecordingComplete)
	if !got.owner() {
		query = query.Where("shared")
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

// recordingFor loads a recording and the caller's access to its call. A
// recording that is not visible to the caller answers the same refusal as one
// the caller has no business with.
func (s *Service) recordingFor(ctx context.Context, recordingID string, callerID int64) (*schemas.Recording, *access, error) {
	id, err := uuid.Parse(recordingID)
	if err != nil {
		return nil, nil, troncerrors.NotFound("recording not found")
	}
	var rec schemas.Recording
	if err := s.orm.WithContext(ctx).First(&rec, "id = ? AND status = ?", id, schemas.RecordingComplete).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, troncerrors.NotFound("recording not found")
		}
		return nil, nil, err
	}
	got, err := s.accessible(ctx, rec.CallID.String(), callerID)
	if err != nil {
		return nil, nil, err
	}
	if !got.visible(rec) {
		return nil, nil, errNotShared
	}
	return &rec, got, nil
}

// RecordingFile opens a recording the caller may watch for streaming. The
// returned file is the caller's to close.
func (s *Service) RecordingFile(ctx context.Context, recordingID string, callerID int64) (*RecordingFile, error) {
	rec, _, err := s.recordingFor(ctx, recordingID, callerID)
	if err != nil {
		return nil, err
	}
	file, err := openRecording(s.recordingsDir, rec.Filename)
	if err != nil {
		return nil, err
	}
	return &RecordingFile{File: file, Download: downloadName(rec.ID.String(), rec.ContentType), Type: rec.ContentType}, nil
}

// RecordingFile is an opened recording with the name and content type it is
// served under.
type RecordingFile struct {
	*os.File
	Download string
	Type     string
}

// SetShared lets the owner share a recording with the call's participants, or
// take it back. Turning sharing on needs the owner's plan to include
// echo.recording; turning it off never does, so a lapsed plan cannot pin a
// recording open to other people.
func (s *Service) SetShared(ctx context.Context, recordingID string, callerID int64, shared bool) (*recordingOut, error) {
	rec, got, err := s.recordingFor(ctx, recordingID, callerID)
	if err != nil {
		return nil, err
	}
	if !got.owner() {
		return nil, troncerrors.Forbidden("only the room owner can do this")
	}
	if shared {
		if err := s.rooms.RequireRecording(ctx, got.room.Slug, callerID); err != nil {
			return nil, err
		}
	}
	err = s.orm.WithContext(ctx).Model(&schemas.Recording{}).Where("id = ?", rec.ID).Update("shared", shared).Error
	if err != nil {
		return nil, err
	}
	rec.Shared = shared
	out := toRecording(*rec, true)
	return &out, nil
}
