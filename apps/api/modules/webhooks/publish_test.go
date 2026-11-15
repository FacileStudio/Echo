package webhooks

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/bus"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/livekit/protocol/livekit"
)

type queued struct {
	Channel        string
	Payload        string
	IdempotencyKey string
}

func (h *harness) outbox() []queued {
	h.t.Helper()
	var rows []queued
	err := h.db.Raw(`SELECT channel, payload::text AS payload, idempotency_key FROM antenne_outbox ORDER BY id`).Scan(&rows).Error
	if err != nil {
		h.t.Fatalf("read the outbox: %v", err)
	}
	return rows
}

func decodeEvent(t *testing.T, raw string) map[string]any {
	t.Helper()
	var event map[string]any
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return event
}

type wantEvent struct {
	channel string
	action  string
	key     string
	payload map[string]any
}

// runCall plays one call twice over: start, a signed-in join, a guest join
// and the end at ended.
func (h *harness) runCall(ended time.Time) {
	h.t.Helper()
	h.webhooks.PublishWith(bus.New("https://echo.example/"))
	h.seedRoom("standup")
	if err := h.db.Create(&schemas.User{ID: 7, Email: "Alice@Example.com", Name: "Alice"}).Error; err != nil {
		h.t.Fatalf("seed user: %v", err)
	}
	guest := &livekit.ParticipantInfo{Sid: "PA_2", Identity: "guest-1", Name: "Bob", JoinedAt: base.Unix()}
	events := []*livekit.WebhookEvent{
		roomEvent(eventRoomStarted, "standup", "RM_1", base),
		peopleEvent(eventParticipantJoined, "standup", "RM_1", alice("PA_1", base), base),
		peopleEvent(eventParticipantJoined, "standup", "RM_1", guest, base),
		roomEvent(eventRoomFinished, "standup", "RM_1", ended),
	}
	for range 2 {
		for _, event := range events {
			h.send(event)
		}
	}
}

func expectedEvents(callID string, ended time.Time) []wantEvent {
	payload := func(end any, emails ...any) map[string]any {
		return map[string]any{
			"facile_id": callID, "room_slug": "standup", "room_name": "standup",
			"started_at": base.Format(time.RFC3339), "ended_at": end,
			"participant_emails": append([]any{}, emails...), "url": "https://echo.example/calls/" + callID,
		}
	}
	prefix := "echo:call:" + callID + ":"
	return []wantEvent{
		{"call.started", "created", prefix + "started", payload(nil)},
		{"call.updated", "updated", prefix + "joined:7", payload(nil, "alice@example.com")},
		{"call.ended", "updated", prefix + "ended", payload(ended.Format(time.RFC3339), "alice@example.com")},
	}
}

func assertEvent(t *testing.T, row queued, callID string, want wantEvent) {
	t.Helper()
	event := decodeEvent(t, row.Payload)
	if row.Channel != want.channel || row.IdempotencyKey != want.key ||
		event["version"] != float64(1) || event["app"] != "Echo" || event["object"] != "call" ||
		event["action"] != want.action || event["facile_id"] != callID || event["idempotency_key"] != want.key {
		t.Fatalf("%s row = %+v, envelope %v", want.channel, row, event)
	}
	if got := event["payload"]; !reflect.DeepEqual(got, any(want.payload)) {
		t.Fatalf("%s payload = %v, want %v", want.channel, got, want.payload)
	}
}

// A call's start, a signed-in join, a guest join and its end queue exactly
// call.started, call.updated and call.ended, each the full call in the suite
// envelope, with guests left out. Redelivering every event queues nothing.
func TestACallQueuesItsLifecycleOnTheOutbox(t *testing.T) {
	h := newHarness(t)
	ended := base.Add(time.Hour)
	h.runCall(ended)

	rows := h.outbox()
	callID := h.calls()[0].ID.String()
	wants := expectedEvents(callID, ended)
	if len(rows) != len(wants) {
		t.Fatalf("queued %+v, want %d rows", rows, len(wants))
	}
	for i, want := range wants {
		assertEvent(t, rows[i], callID, want)
	}
}
