package history

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func seedUser(t *testing.T, db *gorm.DB, email, name, picture string) int64 {
	t.Helper()
	user := schemas.User{Email: email, Name: name, Picture: picture}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user.ID
}

func seedRoomCall(t *testing.T, db *gorm.DB, slug string, ownerID *int64, startedAt time.Time) schemas.Call {
	t.Helper()
	room := schemas.Room{ID: uuid.New(), Slug: slug, Name: slug, OwnerID: ownerID}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	ended := startedAt.Add(42 * time.Minute)
	call := schemas.Call{ID: uuid.New(), RoomID: room.ID, StartedAt: startedAt, EndedAt: &ended, LivekitRoomName: slug}
	if err := db.Create(&call).Error; err != nil {
		t.Fatalf("seed call: %v", err)
	}
	return call
}

func seat(t *testing.T, db *gorm.DB, call schemas.Call, userID int64, name string) {
	t.Helper()
	id := userID
	row := schemas.CallParticipant{
		CallID: call.ID, Identity: fmt.Sprintf("user-%d", userID), UserID: &id, Name: name, JoinedAt: call.StartedAt,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed seat: %v", err)
	}
}

func meetingIDs(page *meetingsPage) []string {
	ids := []string{}
	for _, item := range page.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestMeetingsAreScopedToTheCaller(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	alice := seedUser(t, db, "alice@example.test", "Alice", "")
	bob := seedUser(t, db, "bob@example.test", "Bob", "")
	carol := seedUser(t, db, "carol@example.test", "Carol", "")
	now := time.Now().UTC()
	alices := seedRoomCall(t, db, "alice-room", &alice, now)
	bobs := seedRoomCall(t, db, "bob-room", &bob, now)
	seat(t, db, alices, alice, "Alice")
	seat(t, db, bobs, bob, "Bob")
	seat(t, db, bobs, alice, "Alice")

	for who, want := range map[int64][]string{
		alice: {alices.ID.String(), bobs.ID.String()},
		bob:   {bobs.ID.String()},
		carol: {},
	} {
		page, err := s.Meetings(context.Background(), who, 1, 20)
		if err != nil {
			t.Fatalf("Meetings(%d): %v", who, err)
		}
		got := map[string]bool{}
		for _, id := range meetingIDs(page) {
			got[id] = true
		}
		if len(got) != len(want) || page.Total != int64(len(want)) {
			t.Fatalf("Meetings(%d) = %v (total %d), want %v", who, meetingIDs(page), page.Total, want)
		}
		for _, id := range want {
			if !got[id] {
				t.Fatalf("Meetings(%d) is missing %s", who, id)
			}
		}
	}
}

func TestMeetingsCarryRoleDurationAndParticipantsWithAvatars(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	alice := seedUser(t, db, "alice@example.test", "Alice", "https://sso.example/alice.png")
	bob := seedUser(t, db, "bob@example.test", "Bob", "javascript:alert(1)")
	call := seedRoomCall(t, db, "standup", &alice, time.Now().UTC())
	seat(t, db, call, alice, "Alice")
	seat(t, db, call, bob, "")
	guest := schemas.CallParticipant{CallID: call.ID, Identity: "guest-ab12cd34", Name: "Guest", JoinedAt: call.StartedAt}
	agent := schemas.CallParticipant{CallID: call.ID, Identity: "agent-x", Name: "agent", JoinedAt: call.StartedAt}
	if err := db.Create(&[]schemas.CallParticipant{guest, agent}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	page, err := s.Meetings(context.Background(), bob, 1, 20)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("Meetings = %+v, %v", page, err)
	}
	item := page.Items[0]
	if item.Role != roleParticipant || item.DurationSeconds == nil || *item.DurationSeconds != 42*60 {
		t.Fatalf("item = %+v, want participant role and a 42 minute duration", item)
	}
	if len(item.Participants) != 3 {
		t.Fatalf("participants = %+v, want alice, bob and the guest but not the agent", item.Participants)
	}
	byName := map[string]participantOut{}
	for _, p := range item.Participants {
		byName[p.Name] = p
	}
	if byName["Alice"].AvatarURL != "https://sso.example/alice.png" {
		t.Fatalf("alice = %+v, want the account photo", byName["Alice"])
	}
	if byName["Bob"].AvatarURL != "" || byName["Guest"].AvatarURL != "" {
		t.Fatalf("avatars = %+v, want none for a non-http photo and for a guest", byName)
	}
	mine, err := s.Meetings(context.Background(), alice, 1, 20)
	if err != nil || mine.Items[0].Role != roleOwner {
		t.Fatalf("owner role = %+v, %v", mine, err)
	}
}

func TestMeetingsAreNewestFirstAndPaginated(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	alice := seedUser(t, db, "alice@example.test", "Alice", "")
	base := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
	var ids []string
	for i := range 5 {
		call := seedRoomCall(t, db, fmt.Sprintf("room-%d", i), &alice, base.Add(time.Duration(i)*time.Hour))
		ids = append(ids, call.ID.String())
	}

	first, err := s.Meetings(context.Background(), alice, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	last, err := s.Meetings(context.Background(), alice, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 5 || first.PageSize != 2 || len(first.Items) != 2 || len(last.Items) != 1 {
		t.Fatalf("pages = %+v / %+v", first, last)
	}
	if first.Items[0].ID != ids[4] || first.Items[1].ID != ids[3] || last.Items[0].ID != ids[0] {
		t.Fatalf("order = %v then %v, want newest first", meetingIDs(first), meetingIDs(last))
	}
	huge, err := s.Meetings(context.Background(), alice, -3, 10_000)
	if err != nil || huge.Page != 1 || huge.PageSize != maxPageSize {
		t.Fatalf("clamped = %+v, %v", huge, err)
	}
}

func TestAParticipantReadsTheCallButNotItsRecordingUntilShared(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	alice := seedUser(t, db, "alice@example.test", "Alice", "")
	bob := seedUser(t, db, "bob@example.test", "Bob", "")
	call := seedRoomCall(t, db, "standup", &alice, time.Now().UTC())
	seat(t, db, call, bob, "Bob")
	rec := seedRecording(t, db, call.ID, "a.mp4", false)
	ctx := context.Background()

	detail, err := s.Detail(ctx, call.ID.String(), bob)
	if err != nil || detail.Role != roleParticipant || detail.HasRecording {
		t.Fatalf("Detail = %+v, %v, want a participant view with no recording", detail, err)
	}
	if list, err := s.Recordings(ctx, call.ID.String(), bob); err != nil || len(list) != 0 {
		t.Fatalf("Recordings = %+v, %v, want none while unshared", list, err)
	}
	if _, err := s.RecordingFile(ctx, rec.ID.String(), bob); !forbidden(err) {
		t.Fatalf("RecordingFile = %v, want forbidden while unshared", err)
	}
	if _, err := s.Summarize(ctx, call.ID.String(), bob); !forbidden(err) {
		t.Fatalf("Summarize = %v, want owner only", err)
	}

	if err := db.Model(&rec).Update("shared", true).Error; err != nil {
		t.Fatal(err)
	}
	list, err := s.Recordings(ctx, call.ID.String(), bob)
	if err != nil || len(list) != 1 || list[0].CanShare {
		t.Fatalf("Recordings = %+v, %v, want one shared recording the participant cannot manage", list, err)
	}
	if _, err := s.SetShared(ctx, rec.ID.String(), bob, false); !forbidden(err) {
		t.Fatalf("SetShared by a participant = %v, want forbidden", err)
	}
}

func TestPagingIsClampedBeforeItBecomesAnOffset(t *testing.T) {
	page, size := clampPaging(math.MaxInt, 50)
	if offset := (page - 1) * size; offset < 0 {
		t.Fatalf("offset %d overflowed", offset)
	}
	if page, size := clampPaging(-3, 0); page != 1 || size != defaultPageSize {
		t.Fatalf("clampPaging(-3, 0) = %d, %d", page, size)
	}
}
