package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The stored path arrives from a LiveKit webhook, so it is attacker-influenced
// data reaching the filesystem. Nothing may resolve outside the root.
func TestResolveRecordingPathRejectsTraversal(t *testing.T) {
	root := t.TempDir()

	hostile := []string{
		"../../etc/passwd",
		"recordings/../../etc/passwd",
		"..",
		"/etc/passwd",
		"recordings/../../../../../../etc/passwd",
	}
	for _, stored := range hostile {
		full, err := resolveRecordingPath(root, stored)
		if err == nil {
			t.Fatalf("resolveRecordingPath(%q) escaped to %q, want a refusal", stored, full)
		}
	}
}

func TestResolveRecordingPathAcceptsAStoredPath(t *testing.T) {
	root := t.TempDir()

	full, err := resolveRecordingPath(root, "recordings/standup-1756000000.mp4")
	if err != nil {
		t.Fatalf("a legitimate path was refused: %v", err)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if !strings.HasPrefix(full, absRoot+string(os.PathSeparator)) {
		t.Fatalf("resolved %q, want it under %q", full, absRoot)
	}
	if filepath.Base(full) != "standup-1756000000.mp4" {
		t.Fatalf("resolved basename = %q", filepath.Base(full))
	}
}

// An unset RECORDINGS_DIR or an unrecorded call are both "not available here",
// not a server error.
func TestResolveRecordingPathRefusesEmptyInputs(t *testing.T) {
	if _, err := resolveRecordingPath("", "recordings/a.mp4"); err == nil {
		t.Fatal("an unset recordings root was accepted")
	}
	if _, err := resolveRecordingPath(t.TempDir(), ""); err == nil {
		t.Fatal("an empty stored path was accepted")
	}
}
