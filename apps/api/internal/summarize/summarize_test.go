package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/FacileStudio/porte/exchange"
	"github.com/FacileStudio/porte/forfait"
	troncbulle "github.com/FacileStudio/tronc/bulle"
	troncerrors "github.com/FacileStudio/tronc/errors"
)

const owner = int64(7)

type fakeTokens struct{}

func (fakeTokens) Token(_ context.Context, userID int64, target exchange.Target) (string, error) {
	if target.Audience != "bulle" {
		return "", stderrors.New("wrong audience")
	}
	return "token-for-" + string(rune('0'+userID)), nil
}

func (fakeTokens) Forget(int64, exchange.Target) {}

type fakeBulle struct {
	mu       sync.Mutex
	requests []troncbulle.CompleteRequest
	bodies   []int
	answer   func(troncbulle.CompleteRequest) (int, string)
}

func (f *fakeBulle) serve(t *testing.T) *Summarizer {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(server.Close)
	s, err := New(server.URL, fakeTokens{}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func (f *fakeBulle) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/api/ai/complete" || r.Header.Get("Authorization") != "Bearer token-for-7" {
		http.Error(w, "unexpected request", http.StatusBadRequest)
		return
	}
	raw, err := io.ReadAll(r.Body)
	var body troncbulle.CompleteRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err == nil {
		err = decoder.Decode(&body)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.requests = append(f.requests, body)
	f.bodies = append(f.bodies, len(raw))
	f.mu.Unlock()
	status, payload := f.answer(body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := io.WriteString(w, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func answerText(text string) (int, string) {
	raw, _ := json.Marshal(map[string]string{"text": text, "model": "bulle-model"})
	return http.StatusOK, string(raw)
}

func TestShortTranscriptIsOneCallAsTheOwner(t *testing.T) {
	bulle := &fakeBulle{answer: func(troncbulle.CompleteRequest) (int, string) { return answerText("résumé") }}
	s := bulle.serve(t)

	text, model, err := s.Summarize(t.Context(), owner, "Alice: bonjour\nBob: décision prise\n")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if text != "résumé" || model != "bulle-model" {
		t.Fatalf("Summarize = %q, %q", text, model)
	}
	if len(bulle.requests) != 1 {
		t.Fatalf("%d calls, want 1", len(bulle.requests))
	}
	got := bulle.requests[0]
	if !strings.Contains(got.Input, "Bob: décision prise") || strings.Contains(got.Prompt, "Bob") {
		t.Fatal("the transcript must travel as input, never inside the prompt")
	}
	if got.MaxTokens != summaryTokens || strings.Contains(got.Prompt, truncationNotice) {
		t.Fatalf("request = %+v", got)
	}
}

func TestLongTranscriptIsNotedInPartsThenCombined(t *testing.T) {
	bulle := &fakeBulle{answer: func(body troncbulle.CompleteRequest) (int, string) {
		if strings.HasPrefix(body.Prompt, "Voici la partie") {
			return answerText(strings.Repeat("note ", 5000))
		}
		return answerText("résumé final")
	}}
	s := bulle.serve(t)
	transcript := strings.Repeat("Alice: un long échange <&> sur le budget\n", 12000) + "Bob: décision finale\n"

	text, _, err := s.Summarize(t.Context(), owner, transcript)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if text != "résumé final" {
		t.Fatalf("text = %q", text)
	}
	if len(bulle.requests) != maxParts+1 {
		t.Fatalf("%d calls, want %d notes and one summary", len(bulle.requests), maxParts)
	}
	for i, body := range bulle.requests {
		if len(body.Input) > 64<<10 || len(body.Prompt) > 8<<10 || bulle.bodies[i] > 96<<10 || body.MaxTokens > 4096 {
			t.Fatalf("call %d exceeds a Bulle limit: input %d, prompt %d, body %d", i, len(body.Input), len(body.Prompt), bulle.bodies[i])
		}
	}
	last := bulle.requests[len(bulle.requests)-1]
	if !strings.Contains(last.Prompt, truncationNotice) || !strings.Contains(last.Input, "Partie 3 sur 3") {
		t.Fatal("the combining call must say the start was cut and carry every part's notes")
	}
}

func TestBullePlanRefusalIsEchoPlanError(t *testing.T) {
	bulle := &fakeBulle{answer: func(troncbulle.CompleteRequest) (int, string) {
		return http.StatusPaymentRequired, `{"error":{"code":"payment_required","message":"AI needs a paid plan","upgrade_url":"https://sso.facile.studio/spaces","key":"ai.access"}}`
	}}
	s := bulle.serve(t)

	_, _, err := s.Summarize(t.Context(), owner, "Alice: bonjour\n")
	var refusal *forfait.LimitError
	if !stderrors.As(err, &refusal) || refusal.Key != "ai.access" || refusal.UpgradeURL != "https://sso.facile.studio/spaces" {
		t.Fatalf("Summarize error = %v, want the ai.access plan refusal", err)
	}
}

func TestSplitKeepsTheNewestLines(t *testing.T) {
	transcript := strings.Repeat("Alice: vieux bavardage\n", 20000) + "Bob: décision finale\n"

	parts, truncated := split(transcript)
	if !truncated || len(parts) > maxParts {
		t.Fatalf("split = %d parts, truncated %v", len(parts), truncated)
	}
	for _, part := range parts {
		if len(part) > partBytes || !strings.HasPrefix(part, "Alice") && !strings.HasPrefix(part, "Bob") {
			t.Fatalf("a part is %d bytes or opens mid-line: %.20q", len(part), part)
		}
	}
	if !strings.HasSuffix(parts[len(parts)-1], "Bob: décision finale\n") {
		t.Fatal("the newest line was dropped")
	}
}

func TestNilSummarizerIsUnavailable(t *testing.T) {
	var s *Summarizer
	_, _, err := s.Summarize(t.Context(), owner, "Alice: bonjour")
	var known *troncerrors.Error
	if !stderrors.As(err, &known) || known.Code != "unavailable" {
		t.Fatalf("Summarize = %v, want an unavailable envelope", err)
	}
}

func TestBulleTooManyNamesTheDailyCapOrTheBurst(t *testing.T) {
	cases := map[string]struct {
		payload string
		want    string
	}{
		"daily cap": {`{"error":{"code":"rate_limited","message":"at most 200 completions per day; more tomorrow"}}`, "tomorrow"},
		"burst":     {"rate limit exceeded", "in a minute"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { assertTooMany(t, c.payload, c.want) })
	}
}

func assertTooMany(t *testing.T, payload, want string) {
	t.Helper()
	bulle := &fakeBulle{answer: func(troncbulle.CompleteRequest) (int, string) {
		return http.StatusTooManyRequests, payload
	}}
	_, _, err := bulle.serve(t).Summarize(t.Context(), owner, "Alice: bonjour\n")
	var known *troncerrors.Error
	if !stderrors.As(err, &known) || known.Code != "rate_limited" || !strings.Contains(known.Message, want) {
		t.Fatalf("Summarize = %v, want a rate_limited error saying %q", err, want)
	}
}

func firstFailsOthersWait(release <-chan struct{}) func(troncbulle.CompleteRequest) (int, string) {
	var mu sync.Mutex
	failed := false
	return func(troncbulle.CompleteRequest) (int, string) {
		mu.Lock()
		first := !failed
		failed = true
		mu.Unlock()
		if first {
			return http.StatusBadRequest, `{"error":{"code":"invalid_argument","message":"no"}}`
		}
		<-release
		return answerText("note")
	}
}

func TestOneFailedPartLogsOnce(t *testing.T) {
	release := make(chan struct{})
	bulle := &fakeBulle{answer: firstFailsOthersWait(release)}
	var logs bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(bulle.handle))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	s, err := New(server.URL, fakeTokens{}, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, _, err := s.Summarize(t.Context(), owner, strings.Repeat("Alice: un long échange\n", 6000)); err == nil {
		t.Fatal("Summarize succeeded, want the failed part's error")
	}
	if n := strings.Count(logs.String(), "bulle call failed"); n != 1 {
		t.Fatalf("%d warnings for one failure, want one:\n%s", n, logs.String())
	}
}
