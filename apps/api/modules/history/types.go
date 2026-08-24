package history

import (
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
)

// callResponse is one call on the wire. It carries whether a recording
// exists, never where it lives: the stored path is a server filesystem path
// and the client only ever needs to decide whether to show the download link.
type callResponse struct {
	ID           string `json:"id"`
	StartedAt    string `json:"started_at"`
	EndedAt      string `json:"ended_at,omitempty"`
	HasRecording bool   `json:"has_recording"`
}

type summaryPayload struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
}

type participantOut struct {
	Identity string `json:"identity"`
	Name     string `json:"name"`
	JoinedAt string `json:"joined_at"`
	LeftAt   string `json:"left_at,omitempty"`
}

type callDetail struct {
	callResponse
	Transcript   string           `json:"transcript,omitempty"`
	Summary      *summaryPayload  `json:"summary,omitempty"`
	Participants []participantOut `json:"participants"`
}

func toResponse(call schemas.Call) callResponse {
	resp := callResponse{
		ID:           call.ID.String(),
		StartedAt:    call.StartedAt.UTC().Format(time.RFC3339),
		HasRecording: call.RecordingPath != "",
	}
	if call.EndedAt != nil {
		resp.EndedAt = call.EndedAt.UTC().Format(time.RFC3339)
	}
	return resp
}

func toSummary(summary schemas.Summary) summaryPayload {
	return summaryPayload{
		Content:   summary.Content,
		Model:     summary.Model,
		CreatedAt: summary.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toParticipants(rows []schemas.CallParticipant) []participantOut {
	out := make([]participantOut, 0, len(rows))
	for _, row := range rows {
		left := ""
		if row.LeftAt != nil {
			left = row.LeftAt.UTC().Format(time.RFC3339)
		}
		out = append(out, participantOut{
			Identity: row.Identity,
			Name:     row.Name,
			JoinedAt: row.JoinedAt.UTC().Format(time.RFC3339),
			LeftAt:   left,
		})
	}
	return out
}
