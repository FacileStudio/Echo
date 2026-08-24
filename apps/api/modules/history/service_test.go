package history

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
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

func seedCall(t *testing.T, db *gorm.DB, ownerID *int64, recordingPath string) schemas.Call {
	t.Helper()
	room := schemas.Room{ID: uuid.New(), Slug: "standup", Name: "Standup", OwnerID: ownerID}
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

func seedOwnedCall(t *testing.T, db *gorm.DB, recordingPath string) schemas.Call {
	t.Helper()
	ownerID := owner
	return seedCall(t, db, &ownerID, recordingPath)
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

// An unowned room has no owner to be, so ownedCall fails closed: nobody reads
// its calls, including the person who created it as a guest. Pinned because
// the alternative reading — nil owner means public — is a data leak.
func TestUnownedRoomIsRefusedEverySide(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	call := seedCall(t, db, nil, "")

	if _, err := s.Detail(context.Background(), call.ID.String(), owner); !forbidden(err) {
		t.Fatalf("Detail on an unowned room = %v, want forbidden", err)
	}
}

// Detail's participants come back in join order and carry every field the
// client renders, left_at included when the participant has gone.
func TestDetailListsParticipantsInJoinOrder(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	call := seedOwnedCall(t, db, "")
	base := time.Now().UTC().Truncate(time.Second)
	left := base.Add(3 * time.Minute)
	rows := []schemas.CallParticipant{
		{CallID: call.ID, Identity: "second", Name: "Second", JoinedAt: base.Add(time.Minute)},
		{CallID: call.ID, Identity: "first", Name: "First", JoinedAt: base, LeftAt: &left},
	}
	for _, row := range rows {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed participant: %v", err)
		}
	}

	detail, err := s.Detail(context.Background(), call.ID.String(), owner)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if len(detail.Participants) != 2 {
		t.Fatalf("participants = %+v, want two", detail.Participants)
	}
	if detail.Participants[0].Identity != "first" || detail.Participants[1].Identity != "second" {
		t.Fatalf("participants = %+v, want join order", detail.Participants)
	}
	got := detail.Participants[0]
	if got.Name != "First" || got.JoinedAt != base.Format(time.RFC3339) || got.LeftAt != left.Format(time.RFC3339) {
		t.Fatalf("first participant = %+v, want every field mapped", got)
	}
	if detail.Participants[1].LeftAt != "" {
		t.Fatalf("a participant still in the call reported left_at = %q", detail.Participants[1].LeftAt)
	}
}

// storeSummary is an upsert on call_id, so regenerating replaces the row
// instead of racing a second one past the unique index.
func TestStoreSummaryReplacesTheExistingRow(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	call := seedOwnedCall(t, db, "")
	ctx := context.Background()

	if _, err := s.storeSummary(ctx, call.ID, "first pass"); err != nil {
		t.Fatalf("first storeSummary: %v", err)
	}
	payload, err := s.storeSummary(ctx, call.ID, "second pass")
	if err != nil {
		t.Fatalf("second storeSummary: %v", err)
	}
	if payload.Content != "second pass" {
		t.Fatalf("payload = %+v, want the regenerated content", payload)
	}
	var count int64
	if err := db.Model(&schemas.Summary{}).Where("call_id = ?", call.ID).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("summaries for the call = %d, want exactly one", count)
	}
}

// The wire carries whether a recording exists, never where it lives.
func TestResponseReportsHasRecordingNotThePath(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	seedOwnedCall(t, db, "/output/standup-1.mp4")

	calls, err := s.List(context.Background(), "standup", owner)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(calls) != 1 || !calls[0].HasRecording {
		t.Fatalf("List = %+v, want one call flagged as recorded", calls)
	}
	encoded, err := json.Marshal(calls[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "/output/") {
		t.Fatalf("the response leaks a server path: %s", encoded)
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
	if err := os.WriteFile(filepath.Join(root, "standup-1.mp4"), []byte("mp4"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	s, db := newTestService(t, root)
	call := seedOwnedCall(t, db, "/output/standup-1.mp4")

	file, name, err := s.RecordingFile(context.Background(), call.ID.String(), owner)
	if err != nil {
		t.Fatalf("RecordingFile: %v", err)
	}
	defer file.Close()
	if name != "echo-"+call.ID.String()+".mp4" {
		t.Fatalf("download name = %q", name)
	}
}

// A call whose recording never landed on this node is a NotFound, not a 500.
func TestRecordingFileIsNotFoundWhenTheFileIsMissing(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	call := seedOwnedCall(t, db, "/output/never-written.mp4")

	_, _, err := s.RecordingFile(context.Background(), call.ID.String(), owner)
	envelope := new(troncerrors.Error)
	if !stderrors.As(err, &envelope) || envelope.Code != "not_found" {
		t.Fatalf("RecordingFile = %v, want a not_found envelope", err)
	}
}
