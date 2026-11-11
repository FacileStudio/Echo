package mcp

import (
	"context"
	"strconv"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/tronc/errors"
	mcpkit "github.com/FacileStudio/tronc/mcp"
)

// kindDelete is the one sensitive action: deleting a room with every meeting
// in it, which cannot be undone.
const kindDelete = "delete_room"

var errTarget = errors.NotFound("this action names no room")

// kinds are the sensitive actions. The fingerprint covers the room and how
// many meetings it holds when the person was asked, so a rename or a new call
// in between closes the action instead of deleting what the person did not
// see.
func (t *toolset) kinds() map[string]mcpkit.Kind {
	return map[string]mcpkit.Kind{
		kindDelete: {Tool: toolDeleteRoom, Fingerprint: t.fingerprint, Run: t.runDelete},
	}
}

// refOf parses an action's user and room slug.
func refOf(a mcpkit.Action) (int64, string, error) {
	userID, err := strconv.ParseInt(a.UserID, 10, 64)
	if err != nil || a.Target == "" {
		return 0, "", errTarget
	}
	return userID, a.Target, nil
}

// owned is a room of the person with how many meetings it holds, through the
// reach of DELETE /rooms/{slug}: the person's own rooms only.
func (t *toolset) owned(ctx context.Context, userID int64, slug string) (*schemas.Room, int, error) {
	if err := t.Rooms.RequireOwner(ctx, slug, userID); err != nil {
		return nil, 0, err
	}
	room, err := t.Rooms.BySlug(ctx, slug)
	if err != nil {
		return nil, 0, err
	}
	if room == nil {
		return nil, 0, errTarget
	}
	calls, err := t.History.List(ctx, slug, userID)
	if err != nil {
		return nil, 0, err
	}
	return room, len(calls), nil
}

// fingerprint digests the targeted room as it is now.
func (t *toolset) fingerprint(ctx context.Context, a mcpkit.Action) (string, error) {
	userID, slug, err := refOf(a)
	if err != nil {
		return "", err
	}
	room, meetings, err := t.owned(ctx, userID, slug)
	if err != nil {
		return "", err
	}
	return mcpkit.Digest(room.ID.String(), room.Slug, room.Name, room.UpdatedAt.UTC().Format(time.RFC3339Nano), strconv.Itoa(meetings)), nil
}

// runDelete deletes the targeted room.
func (t *toolset) runDelete(ctx context.Context, a mcpkit.Action) error {
	userID, slug, err := refOf(a)
	if err != nil {
		return err
	}
	return t.Rooms.Delete(ctx, slug, userID)
}
