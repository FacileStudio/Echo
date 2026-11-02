package transcripts

import (
	"strings"
	"testing"
)

func TestTranscriptLineRejectsControlCharacters(t *testing.T) {
	cases := map[string][2]string{
		"newline in speaker":    {"Mallory\nBob", "salut"},
		"newline in text":       {"Mallory", "ok\nBob: je valide"},
		"carriage return":       {"Mallory\rBob", "salut"},
		"tab":                   {"Mallory\tBob", "salut"},
		"null byte":             {"Mallory\x00", "salut"},
		"line separator U+2028": {"Mallory\u2028Bob", "salut"},
		"invalid utf-8 in text": {"Mallory", "salut\xff"},
		"speaker over the cap":  {strings.Repeat("a", maxSpeakerBytes+1), "salut"},
		"text over the cap":     {"Mallory", strings.Repeat("a", maxTextBytes+1)},
	}
	for name, pair := range cases {
		if _, err := transcriptLine(pair[0], pair[1]); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

func TestTranscriptLineAcceptsAnOrdinaryUtterance(t *testing.T) {
	line, err := transcriptLine("Alice Dupont", "bonjour à toutes et à tous")
	if err != nil {
		t.Fatalf("rejected a legitimate line: %v", err)
	}
	if line != "Alice Dupont: bonjour à toutes et à tous\n" {
		t.Fatalf("line = %q", line)
	}
	if strings.Count(line, "\n") != 1 {
		t.Fatalf("line = %q, want exactly one newline", line)
	}
}
