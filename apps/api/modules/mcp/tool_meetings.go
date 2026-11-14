package mcp

import (
	"context"

	mcpkit "github.com/FacileStudio/tronc/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	statusSummarized = "summarized"
	statusShared     = "shared"
	statusUnshared   = "unshared"
	guestPrefix      = "guest-"
	maxLinesPerPage  = 500
)

// listMeetings is echo_list_meetings: the history page's list, one page at a
// time.
func (t *toolset) listMeetings(ctx context.Context, req *sdk.CallToolRequest, in listMeetingsInput) (*sdk.CallToolResult, meetingsOutput, error) {
	userID, err := t.userOf(ctx, req)
	if err != nil {
		return nil, meetingsOutput{}, err
	}
	page := 1
	if err := mcpkit.DecodeCursor(in.Cursor, &page); err != nil {
		return nil, meetingsOutput{}, err
	}
	found, err := t.History.Meetings(ctx, userID, page, in.Limit)
	if err != nil {
		return nil, meetingsOutput{}, mcpkit.Failure(err, hintMeeting)
	}
	out := meetingsOutput{Meetings: make([]meetingOut, 0, len(found.Items)), Total: found.Total}
	for _, m := range found.Items {
		item := meetingOut{
			ID: m.ID, Room: m.RoomSlug, RoomName: m.RoomName, Role: m.Role, StartedAt: m.StartedAt, EndedAt: m.EndedAt,
			HasRecording: m.HasRecording, HasTranscript: m.HasTranscript, People: people(m.Participants),
		}
		if m.DurationSeconds != nil {
			minutes := *m.DurationSeconds / 60
			item.DurationMinutes = &minutes
		}
		out.Meetings = append(out.Meetings, item)
	}
	if int64(found.Page*found.PageSize) < found.Total {
		out.NextCursor = mcpkit.EncodeCursor(found.Page + 1)
	}
	return nil, out, nil
}

// getMeeting is echo_get_meeting: the meeting page without the media.
func (t *toolset) getMeeting(ctx context.Context, req *sdk.CallToolRequest, in meetingInput) (*sdk.CallToolResult, meetingOutput, error) {
	userID, err := t.userOf(ctx, req)
	if err != nil {
		return nil, meetingOutput{}, err
	}
	detail, err := t.History.Detail(ctx, in.MeetingID, userID)
	if err != nil {
		return nil, meetingOutput{}, mcpkit.Failure(err, hintMeeting)
	}
	recordings, err := t.History.Recordings(ctx, in.MeetingID, userID)
	if err != nil {
		return nil, meetingOutput{}, mcpkit.Failure(err, hintMeeting)
	}
	out := meetingOutput{
		ID: detail.ID, Room: detail.RoomSlug, RoomName: detail.RoomName, Role: detail.Role, StartedAt: detail.StartedAt,
		EndedAt: detail.EndedAt, HasTranscript: detail.HasTranscript, People: people(detail.Participants),
		Recordings: make([]recordingOut, 0, len(recordings)), URL: t.origin + "/calls/" + detail.ID,
	}
	if s := detail.Summary; s != nil {
		out.Summary = &summaryOut{Content: s.Content, Model: s.Model, CreatedAt: s.CreatedAt}
	}
	for _, r := range recordings {
		out.Recordings = append(out.Recordings, recordingOut{ID: r.ID, CreatedAt: r.CreatedAt, Shared: r.Shared, CanShare: r.CanShare})
	}
	return nil, out, nil
}

// readTranscript is echo_read_transcript: the transcript's lines from the
// cursor on, as many as fit one result.
func (t *toolset) readTranscript(ctx context.Context, req *sdk.CallToolRequest, in transcriptInput) (*sdk.CallToolResult, transcriptOutput, error) {
	userID, err := t.userOf(ctx, req)
	if err != nil {
		return nil, transcriptOutput{}, err
	}
	start, err := mcpkit.Offset(in.Cursor)
	if err != nil {
		return nil, transcriptOutput{}, err
	}
	found, err := t.History.Transcript(ctx, in.MeetingID, userID, in.Query)
	if err != nil {
		return nil, transcriptOutput{}, mcpkit.Failure(err, hintMeeting)
	}
	lines := make([]lineOut, 0, len(found.Lines))
	for _, line := range found.Lines {
		lines = append(lines, lineOut{Speaker: speakerOf(line.Speaker, line.SpeakerID), Text: line.Text, At: offsetOf(line.OffsetMS)})
	}
	return nil, page(lines, start, found.Total), nil
}

// page is the transcript lines from start on that fit one result.
func page(lines []lineOut, start, total int) transcriptOutput {
	rest := lines[min(start, len(lines)):]
	rest = rest[:min(len(rest), maxLinesPerPage)]
	out := transcriptOutput{Total: total, Matched: len(lines)}
	n := mcpkit.FitCount(len(rest), func(n int) int {
		out.Lines = rest[:n]
		return mcpkit.EncodedSize(out)
	})
	out.Lines = rest[:n]
	out.NextCursor = mcpkit.Cursor(start+max(n, 1), len(lines))
	return out
}

// summarize is echo_summarize_meeting.
func (t *toolset) summarize(ctx context.Context, req *sdk.CallToolRequest, in meetingInput) (*sdk.CallToolResult, summaryOutput, error) {
	userID, err := t.userOf(ctx, req)
	if err != nil {
		return nil, summaryOutput{}, err
	}
	s, err := t.History.Summarize(ctx, in.MeetingID, userID)
	if err != nil {
		return nil, summaryOutput{}, mcpkit.Failure(err, hintMeeting)
	}
	return nil, summaryOutput{Status: statusSummarized, Summary: summaryOut{Content: s.Content, Model: s.Model, CreatedAt: s.CreatedAt}}, nil
}

// shareRecording is echo_share_recording.
func (t *toolset) shareRecording(ctx context.Context, req *sdk.CallToolRequest, in shareInput) (*sdk.CallToolResult, recordingOutput, error) {
	userID, err := t.userOf(ctx, req)
	if err != nil {
		return nil, recordingOutput{}, err
	}
	r, err := t.History.SetShared(ctx, in.RecordingID, userID, in.Shared)
	if err != nil {
		return nil, recordingOutput{}, mcpkit.Failure(err, hintRecording)
	}
	status := statusUnshared
	if r.Shared {
		status = statusShared
	}
	return nil, recordingOutput{Status: status, Recording: recordingOut{ID: r.ID, CreatedAt: r.CreatedAt, Shared: r.Shared, CanShare: r.CanShare}}, nil
}
