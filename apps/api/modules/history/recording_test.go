package history

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeRecording(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// The stored path arrives from a LiveKit webhook, so it is attacker-influenced
// data reaching the filesystem. Nothing may resolve outside the root.
func TestOpenRecordingRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	writeRecording(t, root, "standup-1.mp4", "mp4")

	hostile := []string{
		"../../etc/passwd",
		"recordings/../../etc/passwd",
		"..",
		"/etc/passwd",
		"recordings/../../../../../../etc/passwd",
	}
	for _, stored := range hostile {
		file, err := openRecording(root, stored)
		if err == nil {
			_ = file.Close()
			t.Fatalf("openRecording(%q) opened a file, want a refusal", stored)
		}
	}
}

// The egress container mounts the recordings volume read-write and runs
// headless Chrome under SYS_ADMIN, so a symlink inside the root is a real
// threat, not a theoretical one. A lexical check passes it; os.OpenInRoot
// resolves through the filesystem and refuses it.
func TestOpenRecordingRejectsASymlinkOutOfTheRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("classified"), 0o600); err != nil {
		t.Fatalf("write the target: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "leak.mp4")); err != nil {
		t.Fatalf("plant the symlink: %v", err)
	}

	file, err := openRecording(root, "/output/leak.mp4")
	if err == nil {
		body, _ := io.ReadAll(file)
		_ = file.Close()
		t.Fatalf("the symlink was followed out of the root and served %q", body)
	}
}

// Egress writes flat into its own mount of the volume and reports an absolute
// path from inside that container. The API resolves the basename against its
// own mount, so both ends land on the same file.
func TestOpenRecordingAcceptsTheEgressPath(t *testing.T) {
	root := t.TempDir()
	writeRecording(t, root, "standup-1756000000.mp4", "mp4-bytes")

	file, err := openRecording(root, "/output/standup-1756000000.mp4")
	if err != nil {
		t.Fatalf("a legitimate egress path was refused: %v", err)
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil || string(body) != "mp4-bytes" {
		t.Fatalf("read = %q, %v, want the seeded bytes", body, err)
	}
}

// A dot in a filename is not traversal. The old lexical check rejected these.
func TestOpenRecordingAcceptsDottedNames(t *testing.T) {
	root := t.TempDir()
	writeRecording(t, root, "sprint..review.mp4", "mp4")

	file, err := openRecording(root, "/output/sprint..review.mp4")
	if err != nil {
		t.Fatalf("sprint..review.mp4 was refused: %v", err)
	}
	_ = file.Close()
}

// An unset RECORDINGS_DIR, an unrecorded call and a directory are all "not
// available here", not a server error and not a served handle.
func TestOpenRecordingRefusesNonFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "adir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := openRecording("", "/output/a.mp4"); err == nil {
		t.Fatal("an unset recordings root was accepted")
	}
	if _, err := openRecording(root, ""); err == nil {
		t.Fatal("an empty stored path was accepted")
	}
	if _, err := openRecording(root, "/output/adir"); err == nil {
		t.Fatal("a directory was accepted")
	}
	if _, err := openRecording(root, "/output/absent.mp4"); err == nil {
		t.Fatal("a missing file was accepted")
	}
}

// The stored path never reaches Content-Disposition: a name like `a.mp4";x="`
// would split the header parameter.
func TestDownloadNameIsTheCallIDAndAFixedExtension(t *testing.T) {
	name := downloadName("11111111-2222-3333-4444-555555555555")
	if name != "echo-11111111-2222-3333-4444-555555555555.mp4" {
		t.Fatalf("downloadName = %q", name)
	}
}
