package summarize

import (
	"strings"
	"testing"
)

func TestPromptKeepsAShortTranscriptWhole(t *testing.T) {
	prompt := buildPrompt("Alice: bonjour\nBob: salut\n")

	if !strings.Contains(prompt, "Alice: bonjour") {
		t.Fatal("the transcript was not included")
	}
	if strings.Contains(prompt, truncationNotice) {
		t.Fatal("a short transcript was announced as truncated")
	}
}

func TestPromptTruncatesTheOldestContent(t *testing.T) {
	old := strings.Repeat("Alice: vieux bavardage\n", 20000)
	transcript := old + "Bob: décision finale\n"
	if len(transcript) <= maxTranscriptBytes {
		t.Fatalf("fixture is %d bytes, needs to exceed %d", len(transcript), maxTranscriptBytes)
	}

	prompt := buildPrompt(transcript)

	if len(prompt) > maxTranscriptBytes+len(instructions)+len(truncationNotice)+len(transcriptHeader) {
		t.Fatalf("prompt = %d bytes, want the transcript capped at %d", len(prompt), maxTranscriptBytes)
	}
	if !strings.Contains(prompt, "Bob: décision finale") {
		t.Fatal("the newest line was dropped: truncation must keep the tail")
	}
	if !strings.Contains(prompt, truncationNotice) {
		t.Fatal("the model was not told the transcript was cut")
	}
}

func TestNilSummarizerIsSafe(t *testing.T) {
	var s *Summarizer

	if got := s.Model(); got != defaultModel {
		t.Fatalf("Model() = %q, want %q", got, defaultModel)
	}
	if _, err := s.Summarize(t.Context(), "Alice: bonjour"); err == nil {
		t.Fatal("a nil Summarizer returned no error")
	}
}
