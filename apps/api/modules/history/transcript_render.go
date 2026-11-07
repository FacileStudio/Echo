package history

import (
	"fmt"
	"html"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	cuePerRune = 70 * time.Millisecond
	cueMinimum = time.Second
	cueMaximum = 12 * time.Second
	cueFloor   = 500 * time.Millisecond
)

// speakerLabel is the name a line is shown under: the participant's name, else
// a neutral label. An erased person has no name left, which reads the same as a
// guest who gave none.
func speakerLabel(line transcriptLineOut) string {
	if name := cleanInline(line.Speaker); name != "" {
		return name
	}
	return "?"
}

func cleanInline(text string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return ' '
		}
		return r
	}, text))
}

func clockSeconds(offset time.Duration) string {
	total := offset.Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d", total/3600000, total/60000%60, total/1000%60)
}

func clock(offset time.Duration) string {
	return fmt.Sprintf("%s.%03d", clockSeconds(offset), offset.Milliseconds()%1000)
}

func renderText(lines []transcriptLineOut) string {
	var out strings.Builder
	for _, line := range lines {
		if line.OffsetMS != nil {
			out.WriteString("[" + clockSeconds(time.Duration(*line.OffsetMS)*time.Millisecond) + "] ")
		}
		out.WriteString(speakerLabel(line) + ": " + cleanInline(line.Text) + "\n")
	}
	return out.String()
}

// cueWindow places a cue ending at end. The stored offset is when the final
// utterance arrived, so the start is estimated from the text length, kept
// after the previous cue and at least half a second long.
func cueWindow(end, previousEnd time.Duration, text string) (time.Duration, time.Duration) {
	length := min(max(time.Duration(utf8.RuneCountInString(text))*cuePerRune, cueMinimum), cueMaximum)
	start := max(end-length, previousEnd, 0)
	return start, max(end, start+cueFloor)
}

func renderVTT(lines []transcriptLineOut) (string, error) {
	var out strings.Builder
	out.WriteString("WEBVTT\n\n")
	var previousEnd time.Duration
	for _, line := range lines {
		if line.OffsetMS == nil {
			return "", errNoTimings
		}
		start, end := cueWindow(time.Duration(*line.OffsetMS)*time.Millisecond, previousEnd, line.Text)
		previousEnd = end
		text := html.EscapeString(speakerLabel(line) + ": " + cleanInline(line.Text))
		out.WriteString(clock(start) + " --> " + clock(end) + "\n" + text + "\n\n")
	}
	return out.String(), nil
}
