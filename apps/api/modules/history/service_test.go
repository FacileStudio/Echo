package history

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	owner    = int64(1)
	stranger = int64(2)
)

// newTestService builds a Service with a nil summarizer — the
// no-ANTHROPIC_API_KEY deployment, which every path must survive rather than
// panic on.
func newTestService(t *testing.T, recordingsDir string) (*Service, *gorm.DB) {
	t.Helper()
	t.Setenv("LIVEKIT_API_KEY", "devkey")
	t.Setenv("LIVEKIT_API_SECRET", "secret")
	t.Setenv("LIVEKIT_URL", "ws://localhost:7880")
	mediaService, err := media.NewServiceFromEnv()
	if err != nil {
		t.Fatalf("media service: %v", err)
	}
	db := testdb.Migrated(t)
	return NewService(db, rooms.NewService(db, mediaService), nil, recordingsDir), db
}

func seedOwnedCall(t *testing.T, db *gorm.DB, recordingPath string) schemas.Call {
	t.Helper()
	ownerID := owner
	room := schemas.Room{ID: uuid.New(), Slug: "standup", Name: "Standup", OwnerID: &ownerID}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	call := schemas.Call{
		ID:              uuid.New(),
		RoomID:          room.ID,
		StartedAt:       time.Now().UTC(),
		LivekitRoomName: "standup",
		RecordingPath:   recordingPath,
	}
	if err := db.Create(&call).Error; err != nil {
		t.Fatalf("seed call: %v", err)
	}
	return call
}

func forbidden(err error) bool {
	envelope := new(troncerrors.Error)
	return stderrors.As(err, &envelope) && envelope.Code == "permission_denied"
}

// Every read path is owner-gated: a call is the room owner's business record.
func TestNonOwnerIsRefusedEverywhere(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	call := seedOwnedCall(t, db, "recordings/standup-1.mp4")
	ctx := context.Background()
	id := call.ID.String()

	if _, err := s.List(ctx, "standup", stranger); !forbidden(err) {
		t.Fatalf("List = %v, want forbidden", err)
	}
	if _, err := s.Detail(ctx, id, stranger); !forbidden(err) {
		t.Fatalf("Detail = %v, want forbidden", err)
	}
	if _, err := s.Summarize(ctx, id, stranger); !forbidden(err) {
		t.Fatalf("Summarize = %v, want forbidden", err)
	}
	if _, _, err := s.RecordingFile(ctx, id, stranger); !forbidden(err) {
		t.Fatalf("RecordingFile = %v, want forbidden", err)
	}
}

func TestOwnerSeesTheCall(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	call := seedOwnedCall(t, db, "")
	ctx := context.Background()

	calls, err := s.List(ctx, "standup", owner)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(calls) != 1 || calls[0].ID != call.ID.String() {
		t.Fatalf("List = %+v, want the seeded call", calls)
	}

	detail, err := s.Detail(ctx, call.ID.String(), owner)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if detail.Participants == nil {
		t.Fatal("participants is null, want an empty array so the client can iterate it")
	}
	if detail.Summary != nil || detail.Transcript != "" {
		t.Fatalf("detail = %+v, want no transcript or summary yet", detail)
	}
}

// The nil summarizer must degrade to a 503, not a panic.
func TestSummarizeWithoutAnAPIKeyIsUnavailable(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	call := seedOwnedCall(t, db, "")

	_, err := s.Summarize(context.Background(), call.ID.String(), owner)
	envelope := new(troncerrors.Error)
	if !stderrors.As(err, &envelope) || envelope.Code != "unavailable" {
		t.Fatalf("Summarize = %v, want an unavailable envelope", err)
	}
}

func TestRecordingFileServesTheOwnersMP4(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "recordings"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "recordings", "standup-1.mp4"), []byte("mp4"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	s, db := newTestService(t, root)
	call := seedOwnedCall(t, db, "recordings/standup-1.mp4")

	file, name, err := s.RecordingFile(context.Background(), call.ID.String(), owner)
	if err != nil {
		t.Fatalf("RecordingFile: %v", err)
	}
	defer file.Close()
	if filepath.Ext(name) != ".mp4" {
		t.Fatalf("download name = %q, want an .mp4", name)
	}
}

// A call whose recording never landed on this node is a NotFound, not a 500.
func TestRecordingFileIsNotFoundWhenTheFileIsMissing(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	call := seedOwnedCall(t, db, "recordings/never-written.mp4")

	_, _, err := s.RecordingFile(context.Background(), call.ID.String(), owner)
	envelope := new(troncerrors.Error)
	if !stderrors.As(err, &envelope) || envelope.Code != "not_found" {
		t.Fatalf("RecordingFile = %v, want a not_found envelope", err)
	}
}
