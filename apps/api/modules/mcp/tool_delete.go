package mcp

import (
	"context"
	"fmt"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	mcpkit "github.com/FacileStudio/tronc/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const statusDeleted = "deleted"

// deleteRoom is echo_delete_room: the room goes once the user confirms.
func (t *toolset) deleteRoom(ctx context.Context, req *sdk.CallToolRequest, in roomInput) (*sdk.CallToolResult, roomOutput, error) {
	if mcpkit.Retrying(req) {
		return t.resumeDelete(ctx, req)
	}
	userID, err := t.userOf(ctx, req)
	if err != nil {
		return nil, roomOutput{}, err
	}
	room, meetings, err := t.owned(ctx, userID, in.Room)
	if err != nil {
		return nil, roomOutput{}, mcpkit.Failure(err, hintRoom)
	}
	result, outcome, err := t.confirm.Request(ctx, req, kindDelete, room.Slug, deleteQuestion(room, meetings))
	if err != nil {
		return nil, roomOutput{}, mcpkit.Failure(err, hintRoom)
	}
	if result != nil {
		return result, roomOutput{}, nil
	}
	out := t.room(*room, "")
	return nil, roomOutput{Status: outcome.Status, Room: &out, ConfirmURL: outcome.ConfirmURL, Message: outcome.Message}, nil
}

// resumeDelete handles the retry that carries the person's answer.
func (t *toolset) resumeDelete(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, roomOutput, error) {
	if _, err := t.userOf(ctx, req); err != nil {
		return nil, roomOutput{}, err
	}
	outcome, err := t.confirm.Resume(ctx, req, kindDelete)
	if err != nil {
		return nil, roomOutput{}, mcpkit.Failure(err, "Call echo_delete_room again to ask the user afresh.")
	}
	if outcome.Status == mcpkit.StatusDeclined {
		return nil, roomOutput{Status: mcpkit.StatusDeclined, Message: outcome.Message}, nil
	}
	return nil, roomOutput{Status: statusDeleted, Message: "The user confirmed and the room " + outcome.Action.Target + " is deleted."}, nil
}

// deleteQuestion is what the person is asked before a room goes.
func deleteQuestion(room *schemas.Room, meetings int) string {
	return fmt.Sprintf("Delete the room %q (%s) with its %d meetings, their transcripts, summaries and recordings? "+
		"This cannot be undone.", room.Name, room.Slug, meetings)
}
