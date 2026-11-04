package rooms

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"testing"
	"time"

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

func capAt(n string) map[string]json.RawMessage {
	return map[string]json.RawMessage{"echo.max_participants": json.RawMessage(n)}
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

func guestJoin(s *Service, slug string) error {
	_, err := s.Token(context.Background(), slug, nil, "", "Guest")
	return err
}

func refusedOn(err error, key string) bool {
	var refusal *forfait.LimitError
	return stderrors.As(err, &refusal) && refusal.Key == key
}

func TestAFullCallRefusesANewParticipant(t *testing.T) {
	s := newTestService(t)
	live := &fakeLive{participants: []string{"user-77", "guest-aaaa1111", "agent-captions", "EG_recorder"}}
	s.live = live
	owner := addOwner(t, s, "sub-owner")
	pushPlan(t, s, "sub-owner", capAt(`2`))
	openCallWith(t, s, "full-call", owner)

	err := guestJoin(s, "full-call")
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
	for _, ceiling := range live.ceilings {
		if ceiling != 2 {
			t.Fatalf("livekit room ceilings = %v, want 2 each time", live.ceilings)
		}
	}
}

func TestABurstOfTokensCannotOutrunTheCap(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-burst")
	pushPlan(t, s, "sub-burst", capAt(`2`))
	if _, err := s.Create(context.Background(), "burst", "burst", &owner); err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := range 2 {
		if err := guestJoin(s, "burst"); err != nil {
			t.Fatalf("guest %d refused: %v", i, err)
		}
	}
	if err := guestJoin(s, "burst"); !refusedOn(err, "echo.max_participants") {
		t.Fatalf("third token while two are still unused = %v, want a refusal", err)
	}
	later := time.Now().Add(11 * time.Minute)
	s.ledger.now = func() time.Time { return later }
	if err := guestJoin(s, "burst"); err != nil {
		t.Fatalf("a seat held by an expired token stayed taken: %v", err)
	}
}

func TestCallRowsCountWhenLiveKitCannotBeAsked(t *testing.T) {
	s := newTestService(t)
	s.live = &fakeLive{participantsErr: stderrors.New("livekit down")}
	owner := addOwner(t, s, "sub-rows")
	pushPlan(t, s, "sub-rows", capAt(`2`))
	openCallWith(t, s, "rows-call", owner, "user-1", "guest-2")
	if err := guestJoin(s, "rows-call"); !refusedOn(err, "echo.max_participants") {
		t.Fatalf("join with two rows connected = %v, want a refusal", err)
	}
}

func TestALimitedRoomIsNotJoinedWithoutItsCeiling(t *testing.T) {
	s := newTestService(t)
	s.live = &fakeLive{ensureErr: stderrors.New("livekit down")}
	owner := addOwner(t, s, "sub-down")
	if _, err := s.Create(context.Background(), "down", "down", &owner); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := guestJoin(s, "down"); err != nil {
		t.Fatalf("an unlimited room refused a join when provisioning failed: %v", err)
	}
	pushPlan(t, s, "sub-down", capAt(`4`))
	if err := guestJoin(s, "down"); !troncCode(err, "unavailable") {
		t.Fatalf("a capped room without its ceiling = %v, want 503 unavailable", err)
	}
}

func TestEveryoneJoinsBeforeTheFirstSnapshot(t *testing.T) {
	s := newTestService(t)
	s.live = &fakeLive{participants: []string{"guest-1", "guest-2", "guest-3", "guest-4", "guest-5"}}
	owner := addOwner(t, s, "sub-early")
	openCallWith(t, s, "early-call", owner)
	admission, err := s.Token(context.Background(), "early-call", nil, "", "Sixth")
	if err != nil {
		t.Fatalf("fail-open refused a join: %v", err)
	}
	if admission.Deadline != nil {
		t.Fatalf("fail-open set a call deadline %v", admission.Deadline)
	}
}
