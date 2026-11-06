package rooms

import (
	"context"
	"strings"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/tronc/errors"

	stderrors "errors"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	t.Setenv("LIVEKIT_API_KEY", "devkey")
	t.Setenv("LIVEKIT_API_SECRET", "secret")
	t.Setenv("LIVEKIT_URL", "ws://livekit.invalid")
	m, err := media.NewServiceFromEnv()
	if err != nil {
		t.Fatalf("media service: %v", err)
	}
	db := testdb.Migrated(t)
	s := NewService(db, m, plans.New(db, testIssuer, nil), &fakeDeadlines{})
	s.live = &fakeLive{}
	return s
}

func TestCreateValidatesSlug(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	if _, err := s.Create(ctx, "Bad Slug", "x", nil); err == nil {
		t.Fatal("uppercase and spaces accepted")
	}
	if _, err := s.Create(ctx, "ok-slug", "Ok", nil); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Create(ctx, "ok-slug", "Dup", nil); !troncCode(err, "already_exists") {
		t.Fatalf("duplicate slug = %v, want a conflict", err)
	}
}

func troncCode(err error, code string) bool {
	env := new(errors.Error)
	return stderrors.As(err, &env) && env.Code == code
}

func TestJoinCreatesMissingRoomOnce(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	first, err := s.Join(ctx, "standup", nil)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if first.Name != "standup" || first.OwnerID != nil {
		t.Fatalf("implicit room = %+v, want the slug as name and no owner", first)
	}
	second, err := s.Join(ctx, "standup", nil)
	if err != nil {
		t.Fatalf("second join: %v", err)
	}
	if second.ID != first.ID {
		t.Fatal("joining twice created two rooms")
	}
}

func TestOwnerOnlyMutations(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	owner, stranger := int64(1), int64(2)

	room, err := s.Create(ctx, "private-room", "Private", &owner)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := s.Rename(ctx, room.Slug, "Hijacked", stranger); !troncCode(err, "permission_denied") {
		t.Fatalf("stranger rename = %v, want forbidden", err)
	}
	if _, err := s.Rename(ctx, room.Slug, "Renamed", owner); err != nil {
		t.Fatalf("owner rename: %v", err)
	}
	if err := s.Delete(ctx, room.Slug, stranger); !troncCode(err, "permission_denied") {
		t.Fatalf("stranger delete = %v, want forbidden", err)
	}
	if err := s.Delete(ctx, room.Slug, owner); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if got, _ := s.BySlug(ctx, room.Slug); got != nil {
		t.Fatal("room survived its deletion")
	}
}

func TestOwnedByListsOnlyOwnedRooms(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	me := int64(3)

	for _, slug := range []string{"mine-1", "mine-2"} {
		if _, err := s.Create(ctx, slug, slug, &me); err != nil {
			t.Fatalf("create %s: %v", slug, err)
		}
	}
	if _, err := s.Create(ctx, "theirs", "Theirs", nil); err != nil {
		t.Fatalf("create unowned: %v", err)
	}

	rooms, err := s.OwnedBy(ctx, me)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("owned rooms = %d, want 2", len(rooms))
	}
}

func TestTokenGrants(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	user := int64(9)
	admission, err := s.Token(ctx, JoinRequest{Slug: "grant-check", UserID: &user, Email: "alice@example.com"})
	if err != nil {
		t.Fatalf("user token: %v", err)
	}
	if strings.Count(admission.Token, ".") != 2 {
		t.Fatalf("token = %q, want a JWT", admission.Token)
	}

	guest, err := s.Token(ctx, JoinRequest{Slug: "grant-check", DisplayName: "Bob Guest"})
	if err != nil {
		t.Fatalf("guest token: %v", err)
	}
	if strings.Count(guest.Token, ".") != 2 {
		t.Fatalf("guest token = %q, want a JWT", guest.Token)
	}

	if _, err := s.Token(ctx, JoinRequest{Slug: "grant-check"}); err == nil {
		t.Fatal("guest without a display name was accepted")
	}
}
