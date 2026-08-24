package summarize

import (
	"errors"
	"strings"
	"testing"
)

func block(text string) messagesResponse {
	return messagesResponse{
		Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{{Type: "text", Text: text}},
	}
}

func TestTextRefusesATruncatedOrRefusedAnswer(t *testing.T) {
	for _, reason := range []string{"max_tokens", "refusal"} {
		decoded := block("un début de résumé")
		decoded.StopReason = reason

		got, err := text(decoded, defaultModel)
		if err == nil {
			t.Fatalf("stop_reason %q returned %q as a finished summary", reason, got)
		}
	}
}

func TestTextAcceptsACompleteAnswer(t *testing.T) {
	decoded := block("résumé complet")
	decoded.StopReason = "end_turn"

	got, err := text(decoded, defaultModel)
	if err != nil {
		t.Fatalf("end_turn: %v", err)
	}
	if got != "résumé complet" {
		t.Fatalf("text = %q", got)
	}
}

func TestRedactRemovesTheAPIKey(t *testing.T) {
	s := &Summarizer{apiKey: "sk-ant-secret"}

	got := s.redact(errors.New("post failed with x-api-key sk-ant-secret"))

	if strings.Contains(got.Error(), "sk-ant-secret") {
		t.Fatalf("the key survived redaction: %v", got)
	}
}
