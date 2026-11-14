package mcp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	stderrors "errors"
	"net/url"
	"strconv"

	"github.com/FacileStudio/porte/mcpauth"
	mcpkit "github.com/FacileStudio/tronc/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolListRooms    = "echo_list_rooms"
	toolCreateRoom   = "echo_create_room"
	toolRenameRoom   = "echo_rename_room"
	toolDeleteRoom   = "echo_delete_room"
	toolMeetings     = "echo_list_meetings"
	toolMeeting      = "echo_get_meeting"
	toolTranscript   = "echo_read_transcript"
	toolSummarize    = "echo_summarize_meeting"
	toolShare        = "echo_share_recording"
	hintRoom         = "Call echo_list_rooms for the slugs of your rooms; only rooms you own can be changed."
	hintMeeting      = "Call echo_list_meetings for the ids of the meetings you own or sat in."
	hintRecording    = "Call echo_get_meeting for the ids of a meeting's recordings."
	createScope      = "mcp-room-create"
	createsPerMinute = 30
	roomKeyBytes     = 32
)

var (
	errNoCaller = stderrors.New("this request carries no valid access token. Reconnect Echo")
	errBusy     = stderrors.New("too many rooms created in the last minute. Wait a minute and try again")
)

// toolset holds what the tool handlers share.
type toolset struct {
	Deps
	confirm *mcpkit.Confirmations
	origin  string
}

// userOf reads the Echo user id from the verified token, refused the way
// RequireAuth refuses it: a deleted account never acts.
func (t *toolset) userOf(ctx context.Context, req *sdk.CallToolRequest) (int64, error) {
	account, err := mcpauth.AccountExists(ctx, req.Extra.TokenInfo, t.Accounts.Identity)
	if err != nil {
		return 0, errNoCaller
	}
	return account.UserID, nil
}

// roomURL is the link to a room, with its key after #key= when there is one.
func (t *toolset) roomURL(slug, key string) string {
	link := t.origin + "/room/" + url.PathEscape(slug)
	if key == "" {
		return link
	}
	return link + "#key=" + key
}

// newRoomKey is 32 random bytes as base64url, the format of the room keys the
// web app makes. It is handed to the caller and never stored.
func newRoomKey() (string, error) {
	raw := make([]byte, roomKeyBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// userKey is the limiter key of a person.
func userKey(userID int64) string {
	return strconv.FormatInt(userID, 10)
}
