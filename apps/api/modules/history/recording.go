package history

import (
	"os"
	"path/filepath"

	troncerrors "github.com/FacileStudio/tronc/errors"
)

// errNoRecording is the single answer to every reason a recording cannot be
// served — missing root, escaping path, absent file. The caller has no
// business learning which, and a 404 that varies by cause is a probe oracle.
var errNoRecording = troncerrors.NotFound("this recording is not available on this node")

// extensions maps the content types a recording can have to the extension a
// download gets. The extension is looked up and never read off the stored
// path: that path is webhook data, and a name like `a.mp4";x="` fed into
// Content-Disposition would split the header parameter. Anything unknown is
// the egress default.
var extensions = map[string]string{
	"audio/webm": ".webm",
	"audio/ogg":  ".ogg",
	"audio/mp4":  ".m4a",
}

func recordingExtension(contentType string) string {
	if ext, ok := extensions[contentType]; ok {
		return ext
	}
	return ".mp4"
}

func servedType(contentType string) string {
	if _, ok := extensions[contentType]; ok {
		return contentType
	}
	return "video/mp4"
}

// openRecording opens a call's recording inside root, or refuses.
//
// The stored path is what LiveKit reported over the egress_ended webhook: an
// absolute path under the egress container's own mount of the recordings
// volume. The API mounts that same volume elsewhere, so only the basename
// crosses between them, and egress writes flat into the volume root, which
// makes the basename the volume-relative name.
//
// os.OpenInRoot resolves that name against root through the filesystem rather
// than lexically, so a symlink planted inside the volume cannot point out of
// it — the egress container mounts the volume read-write, runs headless
// Chrome under SYS_ADMIN on the host network and holds the LiveKit secret, so
// it is exactly the thing that would plant one. It also resolves and opens in
// one call, leaving no window between the check and the open.
func openRecording(root, stored string) (*os.File, error) {
	if root == "" || stored == "" {
		return nil, errNoRecording
	}
	file, err := os.OpenInRoot(root, filepath.Base(stored))
	if err != nil {
		return nil, errNoRecording
	}
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		_ = file.Close()
		return nil, errNoRecording
	}
	return file, nil
}

// downloadName is the Content-Disposition filename: the call id keeps it
// unique and meaningful once several land in a downloads folder, and it is a
// UUID, so nothing attacker-controlled reaches the header.
func downloadName(id, contentType string) string {
	return "echo-" + id + recordingExtension(contentType)
}
