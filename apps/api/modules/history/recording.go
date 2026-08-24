package history

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	troncerrors "github.com/FacileStudio/tronc/errors"
)

// errNoRecording is the single answer to every reason a recording cannot be
// served — missing root, escaping path, absent file. The caller has no
// business learning which, and a 404 that varies by cause is a probe oracle.
var errNoRecording = troncerrors.NotFound("this recording is not available on this node")

// resolveRecordingPath joins a stored recording path onto root, refusing
// anything that escapes it. The stored path comes from a LiveKit webhook, so
// it is user-influenced data reaching the filesystem: treat it as hostile.
func resolveRecordingPath(root, stored string) (string, error) {
	if root == "" || stored == "" {
		return "", errNoRecording
	}
	if filepath.IsAbs(stored) || strings.Contains(stored, "..") {
		return "", errNoRecording
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", errNoRecording
	}
	full := filepath.Join(absRoot, filepath.Clean("/"+stored))
	if full != absRoot && !strings.HasPrefix(full, absRoot+string(os.PathSeparator)) {
		return "", errNoRecording
	}
	return full, nil
}

// RecordingFile opens the MP4 of an owned call for streaming. The returned
// file is the caller's to close.
func (s *Service) RecordingFile(ctx context.Context, callID string, callerID int64) (*os.File, string, error) {
	call, err := s.ownedCall(ctx, callID, callerID)
	if err != nil {
		return nil, "", err
	}
	full, err := resolveRecordingPath(s.recordingsDir, call.RecordingPath)
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(full)
	if err != nil {
		return nil, "", errNoRecording
	}
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		_ = file.Close()
		return nil, "", errNoRecording
	}
	return file, downloadName(call.ID.String(), full), nil
}

// downloadName is the Content-Disposition filename: the call id keeps it
// unique and meaningful once several land in a downloads folder.
func downloadName(callID, full string) string {
	extension := filepath.Ext(full)
	if extension == "" {
		extension = ".mp4"
	}
	return "echo-" + callID + extension
}
