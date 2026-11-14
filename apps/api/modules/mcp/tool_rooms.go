package mcp

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	mcpkit "github.com/FacileStudio/tronc/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	statusCreated = "created"
	statusRenamed = "renamed"
	maxNameBytes  = 200
	roomsNote     = "These links carry no room key. Calls are end-to-end encrypted, so people who open a bare " +
		"link cannot hear each other: share the link from the room in the user's browser, or create a new room."
)

var errName = stderrors.New("name must be 1 to 200 characters")

// listRooms is echo_list_rooms.
func (t *toolset) listRooms(ctx context.Context, req *sdk.CallToolRequest, _ listRoomsInput) (*sdk.CallToolResult, roomsOutput, error) {
	userID, err := t.userOf(ctx, req)
	if err != nil {
		return nil, roomsOutput{}, err
	}
	owned, err := t.Rooms.OwnedBy(ctx, userID)
	if err != nil {
		return nil, roomsOutput{}, mcpkit.Failure(err, hintRoom)
	}
	out := roomsOutput{Rooms: make([]roomOut, 0, len(owned)), Note: roomsNote}
	for _, room := range owned {
		out.Rooms = append(out.Rooms, t.room(room, ""))
	}
	return nil, out, nil
}

// createRoom is echo_create_room: the room is the caller's, and its link
// carries a key made for it.
func (t *toolset) createRoom(ctx context.Context, req *sdk.CallToolRequest, in createRoomInput) (*sdk.CallToolResult, roomOutput, error) {
	userID, err := t.userOf(ctx, req)
	if err != nil {
		return nil, roomOutput{}, err
	}
	name, err := nameOf(in.Name)
	if err != nil {
		return nil, roomOutput{}, err
	}
	if !t.Limiter.AllowKey(createScope, userKey(userID), createsPerMinute) {
		return nil, roomOutput{}, errBusy
	}
	slug := in.Slug
	if slug == "" {
		slug = rooms.Slugify(name)
	}
	room, err := t.Rooms.Create(ctx, slug, name, &userID)
	if err != nil {
		return nil, roomOutput{}, mcpkit.Failure(err, hintRoom)
	}
	key, err := newRoomKey()
	if err != nil {
		return nil, roomOutput{}, mcpkit.Failure(err, hintRoom)
	}
	out := t.room(*room, key)
	return nil, roomOutput{Status: statusCreated, Room: &out, Message: "Give everyone this exact join_url: its key is not kept anywhere else."}, nil
}

// renameRoom is echo_rename_room.
func (t *toolset) renameRoom(ctx context.Context, req *sdk.CallToolRequest, in renameRoomInput) (*sdk.CallToolResult, roomOutput, error) {
	userID, err := t.userOf(ctx, req)
	if err != nil {
		return nil, roomOutput{}, err
	}
	name, err := nameOf(in.Name)
	if err != nil {
		return nil, roomOutput{}, err
	}
	room, err := t.Rooms.Rename(ctx, in.Room, name, userID)
	if err != nil {
		return nil, roomOutput{}, mcpkit.Failure(err, hintRoom)
	}
	out := t.room(*room, "")
	return nil, roomOutput{Status: statusRenamed, Room: &out}, nil
}

// room renders a room with its link.
func (t *toolset) room(room schemas.Room, key string) roomOut {
	return roomOut{
		Slug: room.Slug, Name: room.Name, JoinURL: t.roomURL(room.Slug, key),
		CreatedAt: room.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// nameOf is a room name as given, trimmed, or the refusal for one that is
// empty or too long.
func nameOf(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > maxNameBytes {
		return "", errName
	}
	return name, nil
}
