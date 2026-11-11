package mcp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	stderrors "errors"
	"net/url"
	"strconv"

	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/porte/forfait"
	"github.com/FacileStudio/porte/mcpauth"
	"github.com/FacileStudio/tronc/errors"
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
	identity, ok := mcpauth.Identity(req.Extra.TokenInfo)
	if !ok {
		return 0, errNoCaller
	}
	if _, err := t.Accounts.Identity(ctx, identity.UserID); err != nil {
		return 0, errNoCaller
	}
	return identity.UserID, nil
}

// failure turns a service error into one a model can act on. A plan refusal
// says what the plan lacks and where to upgrade, and a feature this
// deployment does not run says so, instead of a generic retry.
func failure(err error, hint string) error {
	var refusal *forfait.LimitError
	if stderrors.As(err, &refusal) {
		message := "The room owner's plan does not include " + refusal.Key
		if link := plans.UpgradeURL(refusal); link != "" {
			return stderrors.New(message + ". Upgrade at " + link)
		}
		return stderrors.New(message + ".")
	}
	var known *errors.Error
	if stderrors.As(err, &known) && known.Code == "unavailable" {
		return stderrors.New(known.Message)
	}
	return mcpkit.Failure(err, hint)
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
