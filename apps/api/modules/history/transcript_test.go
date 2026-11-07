package history

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
)

func ptr(v int64) *int64 { return &v }

func TestVTTCuesAreOrderedEscapedAndNeverOverlapBackwards(t *testing.T) {
	lines := []transcriptLineOut{
		{Speaker: "Alice", Text: "bonjour a tous", OffsetMS: ptr(3000)},
		{Speaker: "Bob\nforged: x", Text: "a --> b <script>", OffsetMS: ptr(3100)},
	}
	out, err := renderVTT(lines)
	if err != nil {
		t.Fatal(err)
	}
	want := "WEBVTT\n\n" +
		"00:00:02.000 --> 00:00:03.000\nAlice: bonjour a tous\n\n" +
		"00:00:03.000 --> 00:00:03.500\nBob forged: x: a --&gt; b &lt;script&gt;\n\n"
	if out != want {
		t.Fatalf("vtt =\n%q\nwant\n%q", out, want)
	}
	if _, err := renderVTT([]transcriptLineOut{{Speaker: "A", Text: "x"}}); err == nil {
		t.Fatal("an untimed transcript was exported as WebVTT")
	}
}

func TestTextExportCarriesClockTimesAndNames(t *testing.T) {
	out := renderText([]transcriptLineOut{
		{Speaker: "Alice", Text: "hello", OffsetMS: ptr(3_723_000)},
		{Speaker: "", Text: "legacy"},
	})
	if out != "[01:02:03] Alice: hello\n?: legacy\n" {
		t.Fatalf("text = %q", out)
	}
}

func seedLines(t *testing.T, s *Service, callID uuid.UUID, rows ...schemas.TranscriptLine) {
	t.Helper()
	for i := range rows {
		rows[i].CallID = callID
	}
	if err := s.orm.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
}

func TestTranscriptSearchAndExportForOwnerAndParticipantOnly(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	alice := seedUser(t, db, "alice@example.test", "Alice", "")
	bob := seedUser(t, db, "bob@example.test", "Bob", "")
	eve := seedUser(t, db, "eve@example.test", "Eve", "")
	call := seedRoomCall(t, db, "standup", &alice, time.Now().UTC())
	seat(t, db, call, alice, "Alice")
	seat(t, db, call, bob, "Bob")
	seedLines(t, s, call.ID,
		schemas.TranscriptLine{Speaker: "user-" + strconv.FormatInt(alice, 10), Text: "Le budget est valide", OffsetMS: 5000},
		schemas.TranscriptLine{Speaker: "user-" + strconv.FormatInt(bob, 10), Text: "Parfait, merci", OffsetMS: 9000},
	)
	id := call.ID.String()

	found, err := s.Transcript(t.Context(), id, bob, "BUDGET")
	if err != nil || found.Total != 2 || len(found.Lines) != 1 || found.Lines[0].Speaker != "Alice" {
		t.Fatalf("search = %+v, %v, want only Alice's budget line out of two", found, err)
	}
	byName, err := s.Transcript(t.Context(), id, alice, "bob")
	if err != nil || len(byName.Lines) != 1 || byName.Lines[0].Text != "Parfait, merci" {
		t.Fatalf("search by speaker = %+v, %v", byName, err)
	}
	if _, err := s.Transcript(t.Context(), id, eve, ""); !forbidden(err) {
		t.Fatalf("a stranger read the transcript: %v", err)
	}
	if _, err := s.Export(t.Context(), id, eve, "txt"); !forbidden(err) {
		t.Fatalf("a stranger exported the transcript: %v", err)
	}
	vtt, err := s.Export(t.Context(), id, bob, "vtt")
	if err != nil || !strings.HasPrefix(vtt, "WEBVTT") || !strings.Contains(vtt, "Bob: Parfait, merci") {
		t.Fatalf("vtt = %q, %v", vtt, err)
	}
}

func TestTranscriptOverHTTPSetsDownloadHeaders(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	alice := seedUser(t, db, "alice@example.test", "Alice", "")
	call := seedRoomCall(t, db, "standup", &alice, time.Now().UTC())
	seat(t, db, call, alice, "Alice")
	seedLines(t, s, call.ID, schemas.TranscriptLine{Speaker: "user-" + strconv.FormatInt(alice, 10), Text: "salut", OffsetMS: 2000})
	owner := strconv.FormatInt(alice, 10)
	target := "/calls/" + call.ID.String() + "/transcript"

	vtt := get(t, s, target+"?format=vtt", owner)
	if vtt.Code != http.StatusOK || !strings.HasPrefix(vtt.Header().Get("Content-Disposition"), "attachment;") ||
		!strings.HasPrefix(vtt.Header().Get("Content-Type"), "text/vtt") {
		t.Fatalf("vtt = %d %v", vtt.Code, vtt.Header())
	}
	if bad := get(t, s, target+"?format=exe", owner); bad.Code != http.StatusBadRequest {
		t.Fatalf("unknown format = %d, want 400", bad.Code)
	}
	if anon := get(t, s, target, ""); anon.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d, want 401", anon.Code)
	}
}

func TestAnOlderTranscriptWithoutTimingsStaysReadableButHasNoVTT(t *testing.T) {
	s, db := newTestService(t, t.TempDir())
	alice := seedUser(t, db, "alice@example.test", "Alice", "")
	call := seedRoomCall(t, db, "standup", &alice, time.Now().UTC())
	seat(t, db, call, alice, "Alice")
	id := "user-" + strconv.FormatInt(alice, 10)
	row := schemas.Transcript{ID: uuid.New(), CallID: call.ID, Content: id + ": ancien texte\n"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}

	out, err := s.Transcript(t.Context(), call.ID.String(), alice, "ancien")
	if err != nil || len(out.Lines) != 1 || out.Lines[0].Speaker != "Alice" || out.Lines[0].OffsetMS != nil {
		t.Fatalf("legacy = %+v, %v", out, err)
	}
	if txt, err := s.Export(t.Context(), call.ID.String(), alice, "txt"); err != nil || txt != "Alice: ancien texte\n" {
		t.Fatalf("txt = %q, %v", txt, err)
	}
	if _, err := s.Export(t.Context(), call.ID.String(), alice, "vtt"); err == nil {
		t.Fatal("an untimed transcript produced a WebVTT file")
	}
}
