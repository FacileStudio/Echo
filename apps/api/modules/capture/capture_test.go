package capture

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
)

func TestOnlyThePlanThatIncludesItCanTurnCaptureOn(t *testing.T) {
	w := newWorld(t)
	w.open(w.free, "user-"+strconv.FormatInt(w.free, 10))
	free := call{session: w.free}

	w.status(put(free, `{"transcription":true}`), http.StatusPaymentRequired)
	w.status(put(free, `{"recording":true}`), http.StatusPaymentRequired)
	if on := w.state(free); on.Transcription || on.Recording {
		t.Fatalf("state = %+v, want nothing on", on)
	}
}

func TestSwitchesAreOwnerOnlyAndSessionOnly(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10), w.guest)

	w.status(put(call{guest: w.guest}, `{"transcription":true}`), http.StatusUnauthorized)
	w.status(put(call{session: w.free}, `{"transcription":true}`), http.StatusForbidden)
	w.status(put(w.owner(), `{"transcription":true}`), http.StatusNoContent)
	if got := w.state(call{guest: w.guest}); got.Owner || got.People != nil || !got.Transcription {
		t.Fatalf("guest view = %+v, want flags only and no owner data", got)
	}
}

func TestNobodyIsCapturedUntilTheyAnswerYes(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10), w.guest)
	guest := call{guest: w.guest}

	w.status(put(with(guest, "/consent"), `{"transcribe":true}`), http.StatusConflict)
	w.status(put(w.owner(), `{"transcription":true,"recording":true}`), http.StatusNoContent)

	mine := w.state(guest).You
	if mine.Transcribe != nil || mine.Record != nil {
		t.Fatalf("a guest who was never asked already answers %+v", mine)
	}
	w.status(put(with(guest, "/consent"), `{"transcribe":true,"record":false}`), http.StatusNoContent)
	mine = w.state(guest).You
	if mine.Transcribe == nil || !*mine.Transcribe || mine.Record == nil || *mine.Record {
		t.Fatalf("answers = %+v, want transcribe yes and record no", mine)
	}

	people := w.state(w.owner()).People
	if len(people) != 2 || people[0].Record == nil || !*people[0].Record || people[1].Record == nil || *people[1].Record {
		t.Fatalf("people = %+v, want the owner in and the guest out of the recording", people)
	}
}

func TestSwitchingOnAgainAsksEveryoneAgain(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10), w.guest)
	guest := call{guest: w.guest}

	w.status(put(w.owner(), `{"transcription":true}`), http.StatusNoContent)
	w.status(put(with(guest, "/consent"), `{"transcribe":true}`), http.StatusNoContent)
	w.status(put(w.owner(), `{"transcription":false}`), http.StatusNoContent)
	w.status(put(w.owner(), `{"transcription":true}`), http.StatusNoContent)

	if again := w.state(guest).You; again.Transcribe != nil {
		t.Fatalf("answer = %+v, want it forgotten after the feature was off", again)
	}
}

func TestSomeoneWhoLeftOrNeverJoinedHasNoSay(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10))
	w.status(put(w.owner(), `{"transcription":true}`), http.StatusNoContent)

	stranger := call{guest: "guest-deadbeef"}
	w.status(put(with(stranger, "/consent"), `{"transcribe":true}`), http.StatusUnauthorized)
	w.status(call{method: http.MethodGet, guest: "guest-deadbeef"}, http.StatusUnauthorized)
	w.status(call{method: http.MethodGet}, http.StatusUnauthorized)
}

func TestRecordingNeedsTheSwitchTheOwnerAndAFormat(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10), w.guest)
	start := func(c call, body string) call {
		c.method, c.body, c.path = http.MethodPost, body, "/recording"
		return c
	}

	w.status(start(w.owner(), `{"content_type":"audio/webm"}`), http.StatusConflict)
	w.status(put(w.owner(), `{"recording":true}`), http.StatusNoContent)
	w.status(start(w.owner(), `{"content_type":"video/mp4"}`), http.StatusBadRequest)
	w.status(start(call{session: w.free}, `{"content_type":"audio/webm"}`), http.StatusForbidden)
	w.status(start(call{guest: w.guest}, `{"content_type":"audio/webm"}`), http.StatusUnauthorized)
	w.status(start(w.owner(), `{"content_type":"audio/webm"}`), http.StatusCreated)
}

func chunk(w *world, c call, id string, index int, data string) *httptest.ResponseRecorder {
	c.method, c.path, c.raw = http.MethodPut, "/recording/"+id+"/chunks/"+strconv.Itoa(index), []byte(data)
	return w.do(c)
}

func (w *world) startRecording() Upload {
	w.t.Helper()
	w.status(put(w.owner(), `{"recording":true}`), http.StatusNoContent)
	rec := w.do(call{method: http.MethodPost, path: "/recording", session: w.pro, body: `{"content_type":"audio/webm"}`})
	var up Upload
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &up) != nil {
		w.t.Fatalf("start = %d %s", rec.Code, rec.Body.String())
	}
	return up
}

func (w *world) stored(id string) schemas.Recording {
	w.t.Helper()
	var rec schemas.Recording
	if err := w.db.First(&rec, "id = ?", id).Error; err != nil {
		w.t.Fatal(err)
	}
	return rec
}

func TestChunksAppendInOrderAndResumeAfterAGap(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10))
	up := w.startRecording()
	owner := w.owner()

	for i, data := range []string{"aaa", "bbbb"} {
		if rec := chunk(w, owner, up.ID, i, data); rec.Code != http.StatusOK {
			t.Fatalf("chunk %d = %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if rec := chunk(w, owner, up.ID, 1, "bbbb"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"next_chunk":2`) {
		t.Fatalf("a retried chunk = %d %s, want an acknowledgement", rec.Code, rec.Body.String())
	}
	rec := chunk(w, owner, up.ID, 4, "zzz")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"next_chunk":2`) || !strings.Contains(rec.Body.String(), `"size":7`) {
		t.Fatalf("a gap = %d %s, want a conflict naming where to resume", rec.Code, rec.Body.String())
	}
	if rec := chunk(w, owner, up.ID, 2, "cc"); rec.Code != http.StatusOK {
		t.Fatalf("resumed chunk = %d %s", rec.Code, rec.Body.String())
	}

	data, err := os.ReadFile(filepath.Join(w.dir, w.stored(up.ID).Filename))
	if err != nil || string(data) != "aaabbbbcc" {
		t.Fatalf("file = %q, %v, want the chunks once each and in order", data, err)
	}
}

func TestATornWriteIsCutBackBeforeTheNextChunk(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10))
	up := w.startRecording()
	path := filepath.Join(w.dir, w.stored(up.ID).Filename)
	chunk(w, w.owner(), up.ID, 0, "aaa")
	if err := os.WriteFile(path, []byte("aaaTORN"), 0o600); err != nil {
		t.Fatal(err)
	}
	chunk(w, w.owner(), up.ID, 1, "bbb")
	if data, _ := os.ReadFile(path); string(data) != "aaabbb" {
		t.Fatalf("file = %q, want the torn bytes gone", data)
	}
}

func TestOnlyTheOwnerOfTheCallUploads(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10), w.guest)
	up := w.startRecording()

	if rec := chunk(w, call{session: w.free}, up.ID, 0, "x"); rec.Code != http.StatusNotFound {
		t.Fatalf("another account's chunk = %d, want not found", rec.Code)
	}
	if rec := chunk(w, call{guest: w.guest}, up.ID, 0, "x"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a guest's chunk = %d, want unauthenticated", rec.Code)
	}
	if rec := chunk(w, w.owner(), uuid.NewString(), 0, "x"); rec.Code != http.StatusNotFound {
		t.Fatalf("an unknown recording = %d", rec.Code)
	}
}

func TestChunksAreBounded(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10))
	up := w.startRecording()

	if rec := chunk(w, w.owner(), up.ID, 0, strings.Repeat("a", maxChunkBytes+1)); rec.Code < 400 {
		t.Fatalf("an oversized chunk = %d", rec.Code)
	}
	if rec := chunk(w, w.owner(), up.ID, 0, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("an empty chunk = %d", rec.Code)
	}
	if err := w.db.Model(&schemas.Recording{}).Where("id = ?", up.ID).Update("size", maxRecordingBytes-1).Error; err != nil {
		t.Fatal(err)
	}
	if rec := chunk(w, w.owner(), up.ID, 0, "ab"); rec.Code < 400 {
		t.Fatalf("a chunk past the recording ceiling = %d", rec.Code)
	}
}

func TestChunksStopWhenRecordingIsSwitchedOff(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10))
	up := w.startRecording()
	chunk(w, w.owner(), up.ID, 0, "aaa")
	w.status(put(w.owner(), `{"recording":false}`), http.StatusNoContent)

	if rec := chunk(w, w.owner(), up.ID, 1, "bbb"); rec.Code < 400 {
		t.Fatalf("a chunk after recording was switched off = %d", rec.Code)
	}
	if got := w.stored(up.ID); got.Status != schemas.RecordingComplete || got.Size != 3 {
		t.Fatalf("recording = %+v, want what was uploaded kept and finalized", got)
	}
}

func TestAClosedTabStillLeavesItsRecording(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10))
	up := w.startRecording()
	chunk(w, w.owner(), up.ID, 0, "aaa")
	if got := w.stored(up.ID).Status; got != schemas.RecordingUploading {
		t.Fatalf("status = %q, want it hidden while uploading", got)
	}

	w.svc.Notify(context.Background(), &livekit.WebhookEvent{Event: "room_finished", Room: &livekit.Room{Sid: "RM_1"}})

	if got := w.stored(up.ID); got.Status != schemas.RecordingComplete {
		t.Fatalf("status = %q after the call ended, want complete", got.Status)
	}
}

func TestAStalledUploadIsFinalizedAndAnEmptyOneDropped(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10))
	full := w.startRecording()
	chunk(w, w.owner(), full.ID, 0, "aaa")
	empty := w.startRecording()
	if w.stored(full.ID).Status != schemas.RecordingComplete {
		t.Fatal("starting a new recorder did not finalize the earlier one")
	}
	old := w.startRecordingAgain()
	chunk(w, w.owner(), old.ID, 0, "bbb")

	if err := w.svc.FinalizeStale(context.Background(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if w.stored(old.ID).Status != schemas.RecordingComplete {
		t.Fatal("a stalled upload stayed hidden")
	}
	var count int64
	w.db.Model(&schemas.Recording{}).Where("id = ?", empty.ID).Count(&count)
	if count != 0 {
		t.Fatal("an upload with no bytes was kept")
	}
	if _, err := os.Stat(filepath.Join(w.dir, empty.ID+".webm")); !os.IsNotExist(err) {
		t.Fatalf("the empty file is still on the volume: %v", err)
	}
}

func (w *world) startRecordingAgain() Upload {
	w.t.Helper()
	rec := w.do(call{method: http.MethodPost, path: "/recording", session: w.pro, body: `{"content_type":"audio/webm"}`})
	var up Upload
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &up) != nil {
		w.t.Fatalf("start = %d %s", rec.Code, rec.Body.String())
	}
	return up
}

func TestRecordingNeedsAVolume(t *testing.T) {
	w := newWorld(t)
	w.svc.dir = ""
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10))
	w.status(put(w.owner(), `{"recording":true}`), http.StatusConflict)
}

func TestIntegrationRoomsAreLeftAlone(t *testing.T) {
	w := newWorld(t)
	w.open(w.pro, "user-"+strconv.FormatInt(w.pro, 10))
	if err := w.db.Model(&schemas.Room{}).Where("slug = ?", w.slug).Update("integration_app", "prisme").Error; err != nil {
		t.Fatal(err)
	}
	w.status(put(w.owner(), `{"transcription":true}`), http.StatusForbidden)
}
