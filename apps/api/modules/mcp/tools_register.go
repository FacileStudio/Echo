package mcp

import (
	mcpkit "github.com/FacileStudio/tronc/mcp"
)

// list is the curated tool set. Meetings are open world: participant names,
// transcripts and summaries come from the other people in a call. Deleting a
// room is the sensitive tool.
func (t *toolset) list() []mcpkit.Tool {
	return append(append(append(t.roomReads(), t.meetingReads()...), t.writes()...), t.sensitive()...)
}

// roomReads is the read-only room tool.
func (t *toolset) roomReads() []mcpkit.Tool {
	return []mcpkit.Tool{
		mcpkit.NewTool(mcpkit.ToolSpec{
			Name: toolListRooms, Title: "List rooms",
			Description: "List the rooms the user owns, newest first, with slug, name and link. The links carry no " +
				"room key, which only the user's browser keeps: to invite people, tell the user to copy the link from " +
				"the room in their browser, or create a new room with echo_create_room.",
			Annotations: &mcpkit.Annotations{ReadOnly: true, Idempotent: true},
		}, t.listRooms),
	}
}

// meetingReads are the read-only meeting tools.
func (t *toolset) meetingReads() []mcpkit.Tool {
	return []mcpkit.Tool{
		mcpkit.NewTool(mcpkit.ToolSpec{
			Name: toolMeetings, Title: "List meetings",
			Description: "List the past and running calls the user owns or sat in, newest first, with room, role " +
				"(owner or participant), times, duration, who came, and whether a recording or a transcript exists. " +
				"Pass next_cursor back as cursor for the next page.",
			Annotations: &mcpkit.Annotations{ReadOnly: true, Idempotent: true, OpenWorld: true},
		}, t.listMeetings),
		mcpkit.NewTool(mcpkit.ToolSpec{
			Name: toolMeeting, Title: "Get a meeting",
			Description: "Show one meeting: room, times, who came, the AI summary when there is one, and the " +
				"recordings the user can watch, with a link to the meeting page. Recordings are watched in Echo, not " +
				"through this tool.",
			Annotations: &mcpkit.Annotations{ReadOnly: true, Idempotent: true, OpenWorld: true},
		}, t.getMeeting),
		mcpkit.NewTool(mcpkit.ToolSpec{
			Name: toolTranscript, Title: "Read a transcript",
			Description: "Read what was said in a meeting, line by line with the speaker and the time into the call. " +
				"Pass query to keep only the lines that mention something. Long transcripts come in pages: pass " +
				"next_cursor back as cursor. Needs AI on the room owner's plan.",
			Annotations: &mcpkit.Annotations{ReadOnly: true, Idempotent: true, OpenWorld: true},
		}, t.readTranscript),
	}
}

// writes are the tools that create or change things without losing any.
func (t *toolset) writes() []mcpkit.Tool {
	return []mcpkit.Tool{
		mcpkit.NewTool(mcpkit.ToolSpec{
			Name: toolCreateRoom, Title: "Create a room",
			Description: "Create a room the user owns and return its invite link. The link carries a new room key " +
				"after #key=, which Echo does not keep: give everyone this exact link. A slug already taken is refused.",
			Annotations: &mcpkit.Annotations{},
		}, t.createRoom),
		mcpkit.NewTool(mcpkit.ToolSpec{
			Name: toolRenameRoom, Title: "Rename a room",
			Description: "Change the display name of one of the user's rooms. The slug and every link stay the same.",
			Annotations: &mcpkit.Annotations{Destructive: true, Idempotent: true},
		}, t.renameRoom),
		mcpkit.NewTool(mcpkit.ToolSpec{
			Name: toolSummarize, Title: "Summarize a meeting",
			Description: "Write, or write again, the AI summary of a meeting the user owns, from its transcript, and " +
				"return it. It replaces the summary already there. Needs AI on the user's plan and a transcript.",
			Annotations: &mcpkit.Annotations{Destructive: true, OpenWorld: true},
		}, t.summarize),
		mcpkit.NewTool(mcpkit.ToolSpec{
			Name: toolShare, Title: "Share a recording",
			Description: "Let the signed-in people who sat in the call watch one of the user's recordings, or take " +
				"that back with shared false. Only the room owner can do this, and sharing needs recording on their plan.",
			Annotations: &mcpkit.Annotations{Idempotent: true},
		}, t.shareRecording),
	}
}

// sensitive are the tools the user confirms.
func (t *toolset) sensitive() []mcpkit.Tool {
	return []mcpkit.Tool{
		mcpkit.NewTool(mcpkit.ToolSpec{
			Name: toolDeleteRoom, Title: "Delete a room",
			Description: "Delete one of the user's rooms for good, with every meeting in it: transcripts, summaries " +
				"and recordings. The user always confirms: in your client when it can ask, otherwise the result has " +
				"status pending_confirmation and a confirm_url to give the user. Nothing is deleted before they confirm.",
			Annotations: &mcpkit.Annotations{Destructive: true, Idempotent: true},
		}, t.deleteRoom),
	}
}
