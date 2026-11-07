package history

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func seedRecording(t *testing.T, db *gorm.DB, callID uuid.UUID, filename string, shared bool) schemas.Recording {
	t.Helper()
	rec := schemas.Recording{ID: uuid.New(), CallID: callID, Filename: "/output/" + filename, Shared: shared}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("seed recording: %v", err)
	}
	return rec
}

func TestRecordingFileServesTheOwnersMP4(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "standup-1.mp4"), []byte("mp4"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	s, db := newTestService(t, root)
	call := seedOwnedCall(t, db, "")
	rec := seedRecording(t, db, call.ID, "standup-1.mp4", false)

	file, err := s.RecordingFile(context.Background(), rec.ID.String(), owner)
	if err != nil {
		t.Fatalf("RecordingFile: %v", err)
	}
	defer file.Close()
	if file.Download != "echo-"+rec.ID.String()+".mp4" {
		t.Fatalf("download name = %q", file.Download)
	}
}

// A call whose recording never landed on this node is a NotFound, not a 500.
func TestRecordingFileIsNotFoundWhenTheFileIsMissing(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	call := seedOwnedCall(t, db, "")
	rec := seedRecording(t, db, call.ID, "never-written.mp4", false)

	_, err := s.RecordingFile(context.Background(), rec.ID.String(), owner)
	envelope := new(troncerrors.Error)
	if !stderrors.As(err, &envelope) || envelope.Code != "not_found" {
		t.Fatalf("RecordingFile = %v, want a not_found envelope", err)
	}
}

func TestAnUploadInProgressIsHiddenAndABrowserRecordingKeepsItsType(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.webm"), []byte("webm"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, db := newTestService(t, root)
	call := seedOwnedCall(t, db, "")
	rec := schemas.Recording{
		ID: uuid.New(), CallID: call.ID, Filename: "a.webm", ContentType: "audio/webm", Status: schemas.RecordingUploading,
	}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if list, err := s.Recordings(ctx, call.ID.String(), owner); err != nil || len(list) != 0 {
		t.Fatalf("Recordings = %+v, %v, want an upload in progress hidden", list, err)
	}
	if _, err := s.RecordingFile(ctx, rec.ID.String(), owner); err == nil {
		t.Fatal("an upload in progress was served")
	}
	if err := db.Model(&rec).Update("status", schemas.RecordingComplete).Error; err != nil {
		t.Fatal(err)
	}
	file, err := s.RecordingFile(ctx, rec.ID.String(), owner)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if file.Download != "echo-"+rec.ID.String()+".webm" || file.Type != "audio/webm" {
		t.Fatalf("name = %q, type = %q", file.Download, file.Type)
	}
}
