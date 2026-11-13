package transcripts

import (
	"unicode"
	"unicode/utf8"

	troncerrors "github.com/FacileStudio/tronc/errors"
)

const (
	// maxSpeakerBytes bounds a speaker label. LiveKit participant identities
	// are short; 128 bytes fits any real display name with room to spare.
	maxSpeakerBytes = 128

	// maxTextBytes bounds one final utterance. a recognizer emits a final every few
	// seconds, so 4 KB is orders of magnitude above a real sentence and still
	// small enough that no single request can move the transcript far.
	maxTextBytes = 4096

	// maxUtteranceMS bounds the span of one final utterance. A recognizer
	// closes a result at every pause, so two minutes is far past real speech.
	maxUtteranceMS = 2 * 60 * 1000

	// maxTranscriptBytes is the ceiling for a whole stored transcript. It
	// bounds what Detail returns to a browser; a summary reads at most the
	// newest 168 KiB of it.
	maxTranscriptBytes = 1 << 20
)

// transcriptLine validates one utterance and renders it as the single line it
// will occupy in the stored transcript.
func transcriptLine(speaker, text string) (string, error) {
	if err := checkField("speaker", speaker, maxSpeakerBytes); err != nil {
		return "", err
	}
	if err := checkField("text", text, maxTextBytes); err != nil {
		return "", err
	}
	return speaker + ": " + text + "\n", nil
}

// checkField refuses anything that would forge a line. The transcript is a
// newline-separated blob, so a control character in either field is not a
// formatting nuisance, it is an authorship forgery.
func checkField(name, value string, limit int) error {
	if len(value) > limit {
		return troncerrors.Invalid(name + " is too long")
	}
	if !utf8.ValidString(value) {
		return troncerrors.Invalid(name + " must be valid UTF-8")
	}
	for _, r := range value {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return troncerrors.Invalid(name + " must not contain control characters")
		}
	}
	return nil
}
