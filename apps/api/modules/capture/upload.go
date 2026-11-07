package capture

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/seat"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxChunkBytes     = 8 << 20
	maxRecordingBytes = 2 << 30
	staleAfter        = 15 * time.Minute
)

var contentTypes = map[string]string{
	"audio/webm": ".webm",
	"audio/ogg":  ".ogg",
	"audio/mp4":  ".m4a",
}

// Upload is where a recording stands: how many chunks and bytes the volume
// holds. A client resumes by sending chunk NextChunk.
type Upload struct {
	ID        string `json:"id"`
	NextChunk int    `json:"next_chunk"`
	Size      int64  `json:"size"`
}

func toUpload(rec schemas.Recording) Upload {
	return Upload{ID: rec.ID.String(), NextChunk: rec.Chunks, Size: rec.Size}
}

// StartUpload opens a recording for the owner's open call. Any earlier
// unfinished upload of the call is finalized first: a new recorder cannot
// continue an old file, because each MediaRecorder starts its own container.
func (s *Service) StartUpload(ctx context.Context, slug string, ownerID int64, contentType string) (Upload, error) {
	ext, ok := contentTypes[contentType]
	if !ok {
		return Upload{}, troncerrors.Invalid("content_type must be audio/webm, audio/ogg or audio/mp4")
	}
	room, err := s.ownedRoom(ctx, slug, ownerID)
	if err != nil {
		return Upload{}, err
	}
	recording := true
	if err := s.requirePlan(ctx, room, Flags{Recording: &recording}); err != nil {
		return Upload{}, err
	}
	var out Upload
	err = s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		call, err := seat.OpenCall(tx, slug, true)
		if err != nil {
			return err
		}
		if !call.RecordOn {
			return errRecordingOff
		}
		if err := finalizeCall(tx, s.dir, call.ID); err != nil {
			return err
		}
		rec := schemas.Recording{
			ID: uuid.New(), CallID: call.ID, ContentType: contentType, Status: schemas.RecordingUploading,
		}
		rec.Filename = rec.ID.String() + ext
		if err := touch(s.dir, rec.Filename); err != nil {
			return err
		}
		out = toUpload(rec)
		return tx.Create(&rec).Error
	})
	return out, err
}

func touch(dir, name string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

// Chunk names one chunk of an upload: whose recording, in which room, and which
// number it is.
type Chunk struct {
	Slug    string
	OwnerID int64
	ID      string
	Index   int
}

// PutChunk appends chunk index to the recording. Chunks are strictly ordered:
// the next one is appended, one already stored is acknowledged without being
// written twice, and a gap is a conflict whose body says where to resume. The
// file is cut back to the committed size before each write, so a write torn by
// a crash or a failed commit never leaves bytes the database does not count.
func (s *Service) PutChunk(ctx context.Context, c Chunk, body io.Reader) (Upload, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxChunkBytes+1))
	if err != nil {
		return Upload{}, err
	}
	if len(data) > maxChunkBytes {
		return Upload{}, troncerrors.TooLarge("a chunk is at most 8 MiB")
	}
	var out Upload
	err = s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec, call, err := s.owned(tx, c.Slug, c.OwnerID, c.ID, true)
		if err != nil {
			return err
		}
		if rec.Status != schemas.RecordingUploading {
			return troncerrors.Conflict("this recording is already finalized")
		}
		if call.EndedAt != nil || !call.RecordOn {
			return errRecordingOff
		}
		out = toUpload(rec)
		if done, err := checkChunk(rec, c.Index, len(data)); done || err != nil {
			return err
		}
		if err := s.write(rec, data); err != nil {
			return err
		}
		rec.Size += int64(len(data))
		rec.Chunks++
		out = toUpload(rec)
		return tx.Model(&schemas.Recording{}).Where("id = ?", rec.ID).
			Updates(map[string]any{"size": rec.Size, "chunks": rec.Chunks, "updated_at": time.Now()}).Error
	})
	return out, err
}

func checkChunk(rec schemas.Recording, index, size int) (done bool, err error) {
	switch {
	case index < rec.Chunks:
		return true, nil
	case index > rec.Chunks:
		return false, &gapError{upload: toUpload(rec)}
	case size == 0:
		return false, troncerrors.Invalid("a chunk is not empty")
	case rec.Size+int64(size) > maxRecordingBytes:
		return false, troncerrors.TooLarge("this recording reached its maximum size")
	}
	return false, nil
}

type gapError struct{ upload Upload }

func (e *gapError) Error() string { return "chunk out of order" }

// Gap reports the upload a client must resume from when its chunk skipped one.
func Gap(err error) (Upload, bool) {
	var gap *gapError
	if errors.As(err, &gap) {
		return gap.upload, true
	}
	return Upload{}, false
}

func (s *Service) write(rec schemas.Recording, data []byte) error {
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.OpenFile(rec.Filename, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Truncate(rec.Size); err != nil {
		return err
	}
	if _, err := file.WriteAt(data, rec.Size); err != nil {
		return err
	}
	return file.Sync()
}

// Finalize closes the owner's recording so it appears in the history. An empty
// one is dropped. It works after the call ended, which is how a tab that
// closed mid-call still gets its recording.
func (s *Service) Finalize(ctx context.Context, slug string, ownerID int64, id string) error {
	return s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec, _, err := s.owned(tx, slug, ownerID, id, false)
		if err != nil {
			return err
		}
		return finalizeRows(tx, s.dir, []schemas.Recording{rec})
	})
}

func (s *Service) owned(tx *gorm.DB, slug string, ownerID int64, id string, lock bool) (schemas.Recording, schemas.Call, error) {
	var rec schemas.Recording
	var call schemas.Call
	uid, err := uuid.Parse(id)
	if err != nil {
		return rec, call, errUploadMissing
	}
	query := tx
	if lock {
		query = tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&rec, "id = ?", uid).Error; err != nil {
		return rec, call, notFound(err)
	}
	err = tx.Joins("Room").First(&call, "calls.id = ?", rec.CallID).Error
	if err != nil {
		return rec, call, notFound(err)
	}
	if call.Room.Slug != slug || call.Room.OwnerID == nil || *call.Room.OwnerID != ownerID {
		return rec, call, errUploadMissing
	}
	return rec, call, nil
}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errUploadMissing
	}
	return err
}
