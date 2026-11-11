package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/FacileStudio/Echo/apps/api/modules/history"
)

// people renders the participants of a meeting, guests marked.
func people(participants []history.Participant) []personOut {
	out := make([]personOut, 0, len(participants))
	for _, p := range participants {
		out = append(out, personOut{
			Name: p.Name, Guest: strings.HasPrefix(p.Identity, guestPrefix), JoinedAt: p.JoinedAt, LeftAt: p.LeftAt,
		})
	}
	return out
}

// speakerOf names a transcript line's speaker: the name they joined with,
// else their seat.
func speakerOf(name, seat string) string {
	if name != "" {
		return name
	}
	return seat
}

// offsetOf renders the time into the call a line was said at.
func offsetOf(ms *int64) string {
	if ms == nil {
		return ""
	}
	d := time.Duration(*ms) * time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}
