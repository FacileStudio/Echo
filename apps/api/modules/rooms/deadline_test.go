package rooms

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/forfait"
)

func TestTheDeadlineComesFromLiveKitsSessionStart(t *testing.T) {
	s := newTestService(t)
	created := time.Now().Add(-5 * time.Minute).Truncate(time.Second).UTC()
	s.live = &fakeLive{room: dummyLive("RM_unrecorded", created)}
	deadlines := s.deadlines.(*fakeDeadlines)
	owner := addOwner(t, s, "sub-free")
	pushPlan(t, s, "sub-free", callMinutes(`40`))
	if _, err := s.Create(context.Background(), "free-call", "free-call", &owner); err != nil {
		t.Fatalf("create: %v", err)
	}
	admission, err := s.Token(context.Background(), JoinRequest{Slug: "free-call", DisplayName: "Guest"})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	want := created.Add(40 * time.Minute)
	if admission.Deadline == nil || !admission.Deadline.Equal(want) {
		t.Fatalf("deadline = %v, want creation + 40 min = %v", admission.Deadline, want)
	}
	if got := toTokenResponse(admission, "", "", "").CallDeadline; got != want.Format(time.RFC3339) {
		t.Fatalf("call_deadline = %q", got)
	}
	if armed, ok := deadlines.armed["free-call"]; !ok || !armed.Equal(want) {
		t.Fatalf("no room timer for a session without a call row: %v", deadlines.armed)
	}
}

func TestAFreeCallHasNoDeadline(t *testing.T) {
	s := newTestService(t)
	s.live = &fakeLive{room: dummyLive("RM_free", time.Now().Add(-90*time.Minute))}
	owner := addOwner(t, s, "sub-unlimited")
	pushPlan(t, s, "someone-else", nil)
	if _, err := s.Create(context.Background(), "unlimited-call", "unlimited-call", &owner); err != nil {
		t.Fatalf("create: %v", err)
	}
	admission, err := s.Token(context.Background(), JoinRequest{Slug: "unlimited-call", DisplayName: "Guest"})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if admission.Deadline != nil || toTokenResponse(admission, "", "", "").CallDeadline != "" {
		t.Fatalf("a free owner's call got a deadline: %v", admission.Deadline)
	}
	if len(s.deadlines.(*fakeDeadlines).armed) != 0 {
		t.Fatal("a free owner's call armed a timer")
	}
}

func TestARecordedSessionLeavesTheTimerToItsCall(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-recorded")
	pushPlan(t, s, "sub-recorded", callMinutes(`40`))
	call := openCallWith(t, s, "recorded", owner)
	s.live = &fakeLive{room: dummyLive(call.LivekitRoomSID, call.StartedAt)}
	admission, err := s.Token(context.Background(), JoinRequest{Slug: "recorded", DisplayName: "Guest"})
	if err != nil || admission.Deadline == nil || !admission.Deadline.Equal(call.StartedAt.Add(40*time.Minute)) {
		t.Fatalf("deadline = %v, %v", admission.Deadline, err)
	}
	if len(s.deadlines.(*fakeDeadlines).armed) != 0 {
		t.Fatal("a session its call row records got a second, room-keyed timer")
	}
}

func TestACutRoomCoolsDownBeforeANewCall(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-cut")
	pushPlan(t, s, "sub-cut", callMinutes(`40`))
	call := openCallWith(t, s, "cut", owner)
	ended := call.StartedAt.Add(40*time.Minute + 15*time.Second)
	if err := s.orm.Model(&schemas.Call{}).Where("id = ?", call.ID).Update("ended_at", ended).Error; err != nil {
		t.Fatalf("end: %v", err)
	}
	s.now = func() time.Time { return ended.Add(time.Minute) }
	err := guestJoin(s, "cut")
	var refusal *forfait.LimitError
	if !stderrors.As(err, &refusal) || refusal.Key != "echo.max_call_minutes" || refusal.Limit != 40 {
		t.Fatalf("join one minute after the cut = %v, want a max_call_minutes refusal", err)
	}
	s.now = func() time.Time { return ended.Add(3 * time.Minute) }
	if err := guestJoin(s, "cut"); err != nil {
		t.Fatalf("join after the cooldown refused: %v", err)
	}
}

func TestACutNoWebhookReportedStillCoolsDown(t *testing.T) {
	s := newTestService(t)
	owner := addOwner(t, s, "sub-silent-cut")
	pushPlan(t, s, "sub-silent-cut", callMinutes(`40`))
	if _, err := s.Create(context.Background(), "silent-cut", "silent-cut", &owner); err != nil {
		t.Fatalf("create: %v", err)
	}
	s.deadlines.(*fakeDeadlines).cuts = map[string]time.Time{"silent-cut": time.Now().Add(-time.Minute)}
	if err := guestJoin(s, "silent-cut"); !refusedOn(err, "echo.max_call_minutes") {
		t.Fatalf("join right after a cut the webhooks never reported = %v, want a refusal", err)
	}
	pushPlan(t, s, "sub-silent-cut", map[string]json.RawMessage{"echo.max_call_minutes": json.RawMessage(`-1`)})
	if err := guestJoin(s, "silent-cut"); err != nil {
		t.Fatalf("an unlimited plan was held to the cooldown: %v", err)
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
