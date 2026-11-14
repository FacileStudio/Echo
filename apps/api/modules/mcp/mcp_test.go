package mcp

import (
	"net/http"
	"strings"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/modules/deletions"
	"github.com/FacileStudio/tronc/mcp/mcptest"
	"github.com/google/uuid"
)

// TestMCPConformance runs tronc's conformance kit, with deleting a room as
// the sensitive tool.
func TestMCPConformance(t *testing.T) {
	f := newFixture(t)
	mcptest.Run(t, mcptest.Config{
		URL:     f.resource,
		Token:   f.token,
		Confirm: []mcptest.ConfirmCase{{Tool: toolDeleteRoom, Setup: f.freshRoom}},
	})
}

// freshRoom makes a room to delete and counts how many times it was deleted.
func (f *fixture) freshRoom(tb testing.TB) (map[string]any, func() int) {
	slug := "doomed-" + uuid.NewString()[:8]
	f.room(tb, slug, "owner")
	return map[string]any{"room": slug}, func() int { return 1 - f.exists(slug) }
}

// TestRooms creates a room with a keyed link, lists and renames it.
func TestRooms(t *testing.T) {
	f := newFixture(t)
	owner := f.connect(t, "owner")
	var created roomOutput
	call(t, owner, toolCreateRoom, map[string]any{"name": "Weekly Sync"}, &created)
	if created.Status != statusCreated || created.Room.Slug != "weekly-sync" || !strings.Contains(created.Room.JoinURL, "/room/weekly-sync#key=") {
		t.Fatalf("want a weekly-sync room with a keyed link, got %+v", created)
	}
	if key := created.Room.JoinURL[strings.Index(created.Room.JoinURL, "#key=")+5:]; len(key) != 43 {
		t.Fatalf("want a 32-byte base64url key, got %q", key)
	}
	if text := refused(t, owner, toolCreateRoom, map[string]any{"name": "Weekly Sync"}); !strings.Contains(text, "already exists") {
		t.Fatalf("a taken slug must be refused, got %q", text)
	}
	var listed roomsOutput
	call(t, owner, toolListRooms, nil, &listed)
	if len(listed.Rooms) != 1 || strings.Contains(listed.Rooms[0].JoinURL, "#key=") {
		t.Fatalf("want the one room without a key, got %+v", listed)
	}
	var renamed roomOutput
	call(t, owner, toolRenameRoom, map[string]any{"room": "weekly-sync", "name": "Monday sync"}, &renamed)
	if renamed.Room.Name != "Monday sync" || renamed.Room.Slug != "weekly-sync" {
		t.Fatalf("want the new name on the same slug, got %+v", renamed)
	}
}

// TestStrangerCannotTouchARoom keeps a stranger from renaming a room or even
// asking to delete it.
func TestStrangerCannotTouchARoom(t *testing.T) {
	f := newFixture(t)
	f.room(t, "weekly-sync", "owner")
	stranger := f.connect(t, "stranger")
	if text := refused(t, stranger, toolRenameRoom, map[string]any{"room": "weekly-sync", "name": "Mine"}); !strings.Contains(text, "owner") {
		t.Fatalf("a stranger must not rename the room, got %q", text)
	}
	if text := refused(t, stranger, toolDeleteRoom, map[string]any{"room": "weekly-sync"}); !strings.Contains(text, "owner") {
		t.Fatalf("a stranger must not even ask to delete the room, got %q", text)
	}
}

// TestMeetings walks the history as the owner and as a teammate who sat in,
// and keeps a stranger out.
func TestMeetings(t *testing.T) {
	f := newFixture(t)
	meeting := f.meeting(t, f.room(t, "standup", "owner"))
	teammate := f.connect(t, "teammate")
	var list meetingsOutput
	call(t, teammate, toolMeetings, nil, &list)
	if list.Total != 1 || list.Meetings[0].Role != "participant" || len(list.Meetings[0].People) != 3 || !list.Meetings[0].People[2].Guest {
		t.Fatalf("the teammate must see the meeting they sat in with its people, got %+v", list)
	}
	var detail meetingOutput
	call(t, teammate, toolMeeting, map[string]any{"meeting_id": meeting.ID.String()}, &detail)
	if detail.Room != "standup" || !strings.HasSuffix(detail.URL, "/calls/"+meeting.ID.String()) {
		t.Fatalf("want the meeting with its page link, got %+v", detail)
	}
	var transcript transcriptOutput
	call(t, teammate, toolTranscript, map[string]any{"meeting_id": meeting.ID.String(), "query": "BUDGET"}, &transcript)
	if transcript.Total != 3 || len(transcript.Lines) != 1 || transcript.Lines[0].Speaker != "Visitor" || transcript.Lines[0].At != "00:01:01" {
		t.Fatalf("want the one budget line, got %+v", transcript)
	}
	if text := refused(t, teammate, toolSummarize, map[string]any{"meeting_id": meeting.ID.String()}); !strings.Contains(text, "owner") {
		t.Fatalf("only the owner summarizes, got %q", text)
	}
	if text := refused(t, f.connect(t, "owner"), toolSummarize, map[string]any{"meeting_id": meeting.ID.String()}); !strings.Contains(text, "not configured") {
		t.Fatalf("a deployment without AI must say so, got %q", text)
	}
	stranger := f.connect(t, "stranger")
	if text := refused(t, stranger, toolMeeting, map[string]any{"meeting_id": meeting.ID.String()}); !strings.Contains(text, "not part of this call") {
		t.Fatalf("a stranger must not read the meeting, got %q", text)
	}
}

// TestTranscriptPages reads a long transcript one page after another.
func TestTranscriptPages(t *testing.T) {
	lines := make([]lineOut, 1200)
	for i := range lines {
		lines[i] = lineOut{Speaker: "Visitor", Text: strings.Repeat("word ", 30)}
	}
	first := page(lines, 0, len(lines))
	if len(first.Lines) == 0 || len(first.Lines) > maxLinesPerPage || first.NextCursor == "" {
		t.Fatalf("want a bounded first page with a cursor, got %d lines", len(first.Lines))
	}
	last := page(lines, 1100, len(lines))
	if len(last.Lines) != 100 || last.NextCursor != "" {
		t.Fatalf("want the last 100 lines and no cursor, got %d %q", len(last.Lines), last.NextCursor)
	}
}

// pendingDelete asks to delete a fresh room from a client that cannot
// confirm, and returns the slug and the action id.
func pendingDelete(t *testing.T, f *fixture) (string, string) {
	t.Helper()
	slug := "retro-" + uuid.NewString()[:8]
	f.meeting(t, f.room(t, slug, "owner"))
	var out roomOutput
	call(t, f.connect(t, "owner"), toolDeleteRoom, map[string]any{"room": slug}, &out)
	if out.Status != "pending_confirmation" || f.exists(slug) != 1 {
		t.Fatalf("want a pending action and the room kept, got %+v", out)
	}
	return slug, out.ConfirmURL[strings.LastIndex(out.ConfirmURL, "=")+1:]
}

// TestConfirmPageDeletesOnce checks the person confirms in Echo, the room
// goes once, and an agent cannot confirm its own request.
func TestConfirmPageDeletesOnce(t *testing.T) {
	f := newFixture(t)
	slug, id := pendingDelete(t, f)
	owner := f.users["owner"]
	if got := f.onPage(http.MethodPost, "/"+id+"/confirm", owner, "bulle"); got.Code != http.StatusForbidden || f.exists(slug) != 1 {
		t.Fatalf("a delegated token must not confirm, got %d", got.Code)
	}
	if got := f.onPage(http.MethodPost, "/"+id+"/confirm", owner, "cli"); got.Code != http.StatusForbidden || f.exists(slug) != 1 {
		t.Fatalf("a CLI session must not confirm, got %d", got.Code)
	}
	if got := f.onPage(http.MethodGet, "/", f.users["stranger"], ""); !strings.Contains(got.Body.String(), `"actions":[]`) {
		t.Fatalf("another person must not see the action, got %s", got.Body)
	}
	list := f.onPage(http.MethodGet, "/", owner, "")
	if !strings.Contains(list.Body.String(), `"state":"ready"`) || !strings.Contains(list.Body.String(), `"meetings":1`) {
		t.Fatalf("want one ready action with its room, got %d %s", list.Code, list.Body)
	}
	if got := f.onPage(http.MethodPost, "/"+id+"/confirm", owner, ""); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"status":"deleted"`) || f.exists(slug) != 0 {
		t.Fatalf("want the room deleted, got %d %s", got.Code, got.Body)
	}
	if again := f.onPage(http.MethodPost, "/"+id+"/confirm", owner, ""); again.Code != http.StatusNotFound {
		t.Fatalf("a confirmation is single use, got %d", again.Code)
	}
}

// TestChangedRoomIsNotDeleted checks a rename after the request closes it
// instead of deleting a room the person did not see.
func TestChangedRoomIsNotDeleted(t *testing.T) {
	f := newFixture(t)
	slug, id := pendingDelete(t, f)
	var out roomOutput
	call(t, f.connect(t, "owner"), toolRenameRoom, map[string]any{"room": slug, "name": "Renamed"}, &out)
	if got := f.onPage(http.MethodPost, "/"+id+"/confirm", f.users["owner"], ""); got.Code != http.StatusPreconditionFailed || f.exists(slug) != 1 {
		t.Fatalf("want 412 and the room kept, got %d", got.Code)
	}
}

// TestErasureErasesPendingActions checks an account erasure takes the
// person's open MCP actions with it.
func TestErasureErasesPendingActions(t *testing.T) {
	f := newFixture(t)
	pendingDelete(t, f)
	sqlDB, err := f.db.DB()
	must(t, err)
	tx, err := sqlDB.Begin()
	must(t, err)
	must(t, deletions.EraseData(t.Context(), tx, f.users["owner"]))
	must(t, tx.Commit())
	var left int64
	must(t, f.db.Raw(`SELECT count(*) FROM mcp_actions`).Scan(&left).Error)
	if left != 0 {
		t.Fatalf("want no pending action left, got %d", left)
	}
}
