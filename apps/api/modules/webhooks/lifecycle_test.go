package webhooks

import (
	"testing"
	"time"

	"github.com/livekit/protocol/livekit"
)

func alice(sid string, at time.Time) *livekit.ParticipantInfo {
	return &livekit.ParticipantInfo{Sid: sid, Identity: "user-7", Name: "Alice", JoinedAt: at.Unix()}
}

// LiveKit accepts any room name, so events arrive for rooms Echo never
// created. Those are dropped, not errors.
func TestRoomStartedIgnoresAnUnknownSlug(t *testing.T) {
	h := newHarness(t)

	h.send(roomEvent(eventRoomStarted, "ad-hoc-room", "RM_1", base))

	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("calls = %d, want 0 for a room with no rooms row", len(calls))
	}
}

// Finding 1: a retried room_started must not open a second call. The insert
// conflicts on the room sid and does nothing.
func TestARetriedRoomStartedOpensOneCall(t *testing.T) {
	h := newHarness(t)
	h.seedRoom("standup")

	h.send(roomEvent(eventRoomStarted, "standup", "RM_1", base))
	h.send(roomEvent(eventRoomStarted, "standup", "RM_1", base))

	if calls := h.calls(); len(calls) != 1 {
		t.Fatalf("calls = %d, want 1 — a retried room_started opened a second call", len(calls))
	}
}

// Finding 2: a dropped room_finished used to poison the room forever, because
// the next meeting was written onto the still-open call. With the sid as the
// key, the second meeting is simply a second row.
func TestALostRoomFinishedDoesNotMergeTheNextMeeting(t *testing.T) {
	h := newHarness(t)
	h.seedRoom("standup")

	h.send(roomEvent(eventRoomStarted, "standup", "RM_1", base))
	h.send(peopleEvent(eventParticipantJoined, "standup", "RM_1", alice("PA_1", base), base))
	h.send(roomEvent(eventRoomStarted, "standup", "RM_2", base.Add(24*time.Hour)))
	h.send(peopleEvent(eventParticipantJoined, "standup", "RM_2", alice("PA_2", base.Add(24*time.Hour)),
		base.Add(24*time.Hour)))

	calls := h.calls()
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2 — the second day's meeting merged into the first", len(calls))
	}
	rows := h.participants()
	if len(rows) != 2 || rows[0].CallID == rows[1].CallID {
		t.Fatalf("both days' attendance landed on one call: %+v", rows)
	}
}

// Finding 3: room_finished must close the participants too, in the same
// transaction. They used to show as never having left.
func TestRoomFinishedClosesTheParticipantsStillConnected(t *testing.T) {
	h := newHarness(t)
	h.seedRoom("standup")
	h.send(roomEvent(eventRoomStarted, "standup", "RM_1", base))
	h.send(peopleEvent(eventParticipantJoined, "standup", "RM_1", alice("PA_1", base), base))

	ended := base.Add(time.Hour)
	h.send(roomEvent(eventRoomFinished, "standup", "RM_1", ended))

	calls := h.calls()
	if len(calls) != 1 || calls[0].EndedAt == nil {
		t.Fatalf("ended_at is still null after room_finished: %+v", calls)
	}
	rows := h.participants()
	if len(rows) != 1 || rows[0].LeftAt == nil {
		t.Fatalf("a connected participant survived room_finished with left_at null: %+v", rows)
	}
	if !rows[0].LeftAt.UTC().Equal(ended) {
		t.Fatalf("left_at = %v, want the room_finished event time %v", rows[0].LeftAt.UTC(), ended)
	}
}

// Finding 4a: LiveKit routinely delivers the trailing participant_left after
// room_finished. Resolving by "still open" dropped it and left_at stayed null.
func TestParticipantLeftAfterRoomFinishedStillLands(t *testing.T) {
	h := newHarness(t)
	h.seedRoom("standup")
	h.send(roomEvent(eventRoomStarted, "standup", "RM_1", base))
	h.send(peopleEvent(eventParticipantJoined, "standup", "RM_1", alice("PA_1", base), base))
	h.send(roomEvent(eventRoomFinished, "standup", "RM_1", base.Add(time.Hour)))

	h.send(peopleEvent(eventParticipantLeft, "standup", "RM_1", alice("PA_1", base), base.Add(2*time.Hour)))

	rows := h.participants()
	if len(rows) != 1 || rows[0].LeftAt == nil {
		t.Fatalf("the trailing participant_left was dropped: %+v", rows)
	}
}

// Finding 4b: a participant_joined arriving after room_finished used to
// create no row at all, so the attendee vanished from the record.
func TestParticipantJoinedAfterRoomFinishedStillLands(t *testing.T) {
	h := newHarness(t)
	h.seedRoom("standup")
	h.send(roomEvent(eventRoomStarted, "standup", "RM_1", base))
	h.send(roomEvent(eventRoomFinished, "standup", "RM_1", base.Add(time.Hour)))

	h.send(peopleEvent(eventParticipantJoined, "standup", "RM_1", alice("PA_1", base), base.Add(2*time.Hour)))

	if rows := h.participants(); len(rows) != 1 {
		t.Fatalf("participants = %d, want 1 — an attendee vanished from the record", len(rows))
	}
}

// Finding 5: a redelivered participant_joined carries the sid that already
// left. Clearing left_at unconditionally resurrected them.
func TestARedeliveredParticipantJoinedDoesNotResurrect(t *testing.T) {
	h := newHarness(t)
	h.seedRoom("standup")
	h.send(roomEvent(eventRoomStarted, "standup", "RM_1", base))
	h.send(peopleEvent(eventParticipantJoined, "standup", "RM_1", alice("PA_1", base), base))
	h.send(peopleEvent(eventParticipantLeft, "standup", "RM_1", alice("PA_1", base), base.Add(time.Hour)))

	h.send(peopleEvent(eventParticipantJoined, "standup", "RM_1", alice("PA_1", base), base.Add(2*time.Hour)))

	rows := h.participants()
	if len(rows) != 1 || rows[0].LeftAt == nil {
		t.Fatalf("a retried join resurrected a participant who had left: %+v", rows)
	}
}

// The other half of finding 5: a genuine rejoin arrives with a new
// participant sid and a later join time, and must clear left_at.
func TestAGenuineRejoinClearsLeftAt(t *testing.T) {
	h := newHarness(t)
	h.seedRoom("standup")
	h.send(roomEvent(eventRoomStarted, "standup", "RM_1", base))
	h.send(peopleEvent(eventParticipantJoined, "standup", "RM_1", alice("PA_1", base), base))
	h.send(peopleEvent(eventParticipantLeft, "standup", "RM_1", alice("PA_1", base), base.Add(time.Hour)))

	later := base.Add(2 * time.Hour)
	h.send(peopleEvent(eventParticipantJoined, "standup", "RM_1", alice("PA_2", later), later))

	rows := h.participants()
	if len(rows) != 1 || rows[0].LeftAt != nil {
		t.Fatalf("a genuine rejoin did not reopen the row: %+v", rows)
	}
}

// Finding 6: a retried egress_ended used to stamp a second, unrelated call
// with the same filename, so two owners downloaded the same MP4.
func TestARetriedEgressEndedStampsOnlyItsOwnCall(t *testing.T) {
	h := newHarness(t)
	h.seedRoom("standup")
	h.send(roomEvent(eventRoomStarted, "standup", "RM_1", base))
	h.send(roomEvent(eventRoomFinished, "standup", "RM_1", base.Add(time.Hour)))
	h.send(roomEvent(eventRoomStarted, "standup", "RM_2", base.Add(2*time.Hour)))

	ended := egressEvent("RM_1", "standup", "EG_1", "recordings/standup-1.mp4", base.Add(3*time.Hour))
	h.send(ended)
	h.send(ended)

	calls := h.calls()
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[0].RecordingPath != "recordings/standup-1.mp4" || calls[0].EgressID != "EG_1" {
		t.Fatalf("the egress did not stamp the call it names: %+v", calls[0])
	}
	if calls[1].RecordingPath != "" {
		t.Fatalf("the retry stamped an unrelated call with %q", calls[1].RecordingPath)
	}
}
