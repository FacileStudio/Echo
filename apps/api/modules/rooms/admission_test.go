package rooms

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/directory"
	"github.com/FacileStudio/porte/forfait"
	"github.com/google/uuid"
)

const testIssuer = "https://sso.example.test"

func addOwner(t *testing.T, s *Service, subject string) int64 {
	t.Helper()
	user := schemas.User{Email: subject + "@example.test", Name: subject}
	if err := s.orm.Create(&user).Error; err != nil {
		t.Fatalf("create the owner: %v", err)
	}
	err := s.orm.Exec(`INSERT INTO porte_identities (user_id, provider, subject) VALUES (?, ?, ?)`,
		user.ID, testIssuer, subject).Error
	if err != nil {
		t.Fatalf("link the owner to Registre: %v", err)
	}
	return user.ID
}

func pushPlan(t *testing.T, s *Service, subject string, entitlements map[string]json.RawMessage) {
	t.Helper()
	snapshot := directory.Snapshot{Version: 1, EntitlementsRevision: 1,
		People: []directory.Person{{Subject: subject, Entitlements: entitlements}}}
	if err := s.plans.Apply(context.Background(), snapshot); err != nil {
		t.Fatalf("apply: %v", err)
	}
}

func openCallWith(t *testing.T, s *Service, slug string, ownerID int64, identities ...string) schemas.Call {
	t.Helper()
	room, err := s.Create(context.Background(), slug, slug, &ownerID)
	if err != nil {
		t.Fatalf("create the room: %v", err)
	}
	call := schemas.Call{ID: uuid.New(), RoomID: room.ID, StartedAt: time.Now().Add(-10 * time.Minute).Truncate(time.Second).UTC(),
		LivekitRoomName: slug, LivekitRoomSID: "RM_" + slug}
	if err := s.orm.Create(&call).Error; err != nil {
		t.Fatalf("open the call: %v", err)
	}
	for _, identity := range identities {
		row := schemas.CallParticipant{CallID: call.ID, Identity: identity, JoinedAt: call.StartedAt}
		if err := s.orm.Create(&row).Error; err != nil {
			t.Fatalf("join %s: %v", identity, err)
		}
	}
	return call
}

func TestAFullCallRefusesANewParticipant(t *testing.T) {
	s := newTestService(t)
	var ceilings []int64
	s.provision = func(_ context.Context, _ string, settings media.RoomSettings) error {
		ceilings = append(ceilings, settings.MaxParticipants)
		return nil
	}
	owner := addOwner(t, s, "sub-owner")
	pushPlan(t, s, "sub-owner", map[string]json.RawMessage{"echo.max_participants": json.RawMessage(`2`)})
	openCallWith(t, s, "full-call", owner, "user-77", "guest-aaaa1111", "agent-captions")

	_, err := s.Token(context.Background(), "full-call", nil, "", "Third")
	var refusal *forfait.LimitError
	if !stderrors.As(err, &refusal) || refusal.Key != "echo.max_participants" || refusal.Limit != 2 {
		t.Fatalf("third participant = %v, want a max_participants refusal at 2", err)
	}
	if refusal.UpgradeURL != testIssuer+"/spaces" {
		t.Fatalf("upgrade url = %q", refusal.UpgradeURL)
	}
	rejoining := int64(77)
	if _, err := s.Token(context.Background(), "full-call", &rejoining, "x@example.test", ""); err != nil {
		t.Fatalf("a participant reloading their tab was refused: %v", err)
	}
	if len(ceilings) != 1 || ceilings[0] != 2 {
		t.Fatalf("livekit room ceilings = %v, want [2]", ceilings)
	}
}

func TestEveryoneJoinsBeforeTheFirstSnapshot(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-early")
	openCallWith(t, s, "early-call", owner, "guest-1", "guest-2", "guest-3", "guest-4", "guest-5")
	admission, err := s.Token(context.Background(), "early-call", nil, "", "Sixth")
	if err != nil {
		t.Fatalf("fail-open refused a join: %v", err)
	}
	if admission.Deadline != nil {
		t.Fatalf("fail-open set a call deadline %v", admission.Deadline)
	}
}

func TestTheTokenCarriesTheCallDeadline(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-free")
	pushPlan(t, s, "someone-else", nil)
	call := openCallWith(t, s, "free-call", owner)
	admission, err := s.Token(context.Background(), "free-call", nil, "", "Guest")
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	want := call.StartedAt.Add(40 * time.Minute)
	if admission.Deadline == nil || !admission.Deadline.Equal(want) {
		t.Fatalf("deadline = %v, want started_at + 40 min = %v", admission.Deadline, want)
	}
	if got := toTokenResponse(admission, "").CallDeadline; got != want.UTC().Format(time.RFC3339) {
		t.Fatalf("call_deadline = %q", got)
	}
}

func TestRecordingFollowsThePlan(t *testing.T) {
	s := newTestService(t)
	free, paid := addOwner(t, s, "sub-solo"), addOwner(t, s, "sub-pro")
	pushPlan(t, s, "sub-pro", map[string]json.RawMessage{"echo.recording": json.RawMessage(`true`)})
	openCallWith(t, s, "solo-room", free)
	openCallWith(t, s, "pro-room", paid)

	if err := s.RequireRecording(context.Background(), "solo-room", free); !stderrors.Is(err, forfait.ErrLimit) {
		t.Fatalf("free plan recording = %v, want a plan refusal", err)
	}
	if err := s.RequireRecording(context.Background(), "pro-room", paid); err != nil {
		t.Fatalf("paid plan recording refused: %v", err)
	}
	if err := s.RequireRecording(context.Background(), "pro-room", free); !troncCode(err, "permission_denied") {
		t.Fatalf("a stranger recording = %v, want forbidden", err)
	}
}
