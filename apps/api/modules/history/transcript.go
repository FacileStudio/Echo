package history

import (
	"context"
	"strings"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	troncerrors "github.com/FacileStudio/tronc/errors"
	"github.com/google/uuid"
)

const maxQueryBytes = 200

var errNoTimings = troncerrors.Conflict("this transcript has no timings, so it cannot be exported as WebVTT")

type transcriptLineOut struct {
	SpeakerID string `json:"speaker_id"`
	Speaker   string `json:"speaker"`
	Text      string `json:"text"`
	OffsetMS  *int64 `json:"offset_ms,omitempty"`
}

type transcriptOut struct {
	Total int                 `json:"total"`
	Lines []transcriptLineOut `json:"lines"`
}

// Transcript returns the call's transcript for its owner or a participant,
// optionally narrowed to the lines whose text or speaker contains query.
func (s *Service) Transcript(ctx context.Context, callID string, callerID int64, query string) (*transcriptOut, error) {
	got, err := s.accessible(ctx, callID, callerID)
	if err != nil {
		return nil, err
	}
	if err := s.rooms.RequireAI(ctx, got.room); err != nil {
		return nil, err
	}
	if len(query) > maxQueryBytes {
		return nil, troncerrors.Invalid("search text is too long")
	}
	lines, err := s.linesOf(ctx, got.call.ID)
	if err != nil {
		return nil, err
	}
	return &transcriptOut{Total: len(lines), Lines: filterLines(lines, query)}, nil
}

// linesOf reads the timed lines of a call. A call transcribed before lines
// were stored has only the concatenated text, which is split back into
// untimed lines so it stays searchable.
func (s *Service) linesOf(ctx context.Context, callID uuid.UUID) ([]transcriptLineOut, error) {
	var rows []schemas.TranscriptLine
	if err := s.orm.WithContext(ctx).Where("call_id = ?", callID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	names, err := s.speakerNames(ctx, callID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return s.legacyLines(ctx, callID, names)
	}
	out := make([]transcriptLineOut, 0, len(rows))
	for _, row := range rows {
		offset := row.OffsetMS
		out = append(out, transcriptLineOut{
			SpeakerID: row.Speaker, Speaker: names[row.Speaker], Text: row.Text, OffsetMS: &offset,
		})
	}
	return out, nil
}

func (s *Service) legacyLines(ctx context.Context, callID uuid.UUID, names map[string]string) ([]transcriptLineOut, error) {
	transcript, err := s.transcriptOf(ctx, callID)
	if err != nil || transcript == nil {
		return []transcriptLineOut{}, err
	}
	out := []transcriptLineOut{}
	for _, raw := range strings.Split(transcript.Content, "\n") {
		speaker, text, found := strings.Cut(raw, ": ")
		if found && text != "" {
			out = append(out, transcriptLineOut{SpeakerID: speaker, Speaker: names[speaker], Text: text})
		}
	}
	return out, nil
}

// speakerNames maps each identity of the call to the name it joined with.
func (s *Service) speakerNames(ctx context.Context, callID uuid.UUID) (map[string]string, error) {
	var rows []schemas.CallParticipant
	err := s.orm.WithContext(ctx).Select("identity", "name").Where("call_id = ?", callID).Find(&rows).Error
	names := make(map[string]string, len(rows))
	for _, row := range rows {
		names[row.Identity] = row.Name
	}
	return names, err
}

func filterLines(lines []transcriptLineOut, query string) []transcriptLineOut {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return lines
	}
	out := []transcriptLineOut{}
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line.Text), needle) || strings.Contains(strings.ToLower(line.Speaker), needle) {
			out = append(out, line)
		}
	}
	return out
}

// Export renders the whole transcript as plain text or WebVTT.
func (s *Service) Export(ctx context.Context, callID string, callerID int64, format string) (string, error) {
	got, err := s.accessible(ctx, callID, callerID)
	if err != nil {
		return "", err
	}
	if err := s.rooms.RequireAI(ctx, got.room); err != nil {
		return "", err
	}
	lines, err := s.linesOf(ctx, got.call.ID)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", troncerrors.NotFound("no transcript for this call")
	}
	if format == "vtt" {
		return renderVTT(lines)
	}
	return renderText(lines), nil
}
