package mcp

import (
	"context"
	"net/http"
	"strconv"

	"github.com/FacileStudio/porte/mcpauth"
	mcpkit "github.com/FacileStudio/tronc/mcp"
)

const webActor = "echo-web"

// pendingUser is the signed-in person of a confirm page request. Only the
// person's own browser session passes: a bearer, CLI or delegated token must
// never confirm what an agent asked for.
func pendingUser(r *http.Request) (string, error) {
	identity, err := mcpauth.RequireOwnSession(r)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(identity.UserID, 10), nil
}

// describe adds the targeted room, as it is now, to a confirm page item.
func (t *toolset) describe(ctx context.Context, item mcpkit.PendingItem, a mcpkit.Action) PendingItem {
	out := PendingItem{PendingItem: item}
	userID, slug, err := refOf(a)
	if err != nil {
		return out
	}
	if room, meetings, err := t.owned(ctx, userID, slug); err == nil {
		out.Room = &PendingRoom{Slug: room.Slug, Name: room.Name, Meetings: meetings}
	}
	return out
}

// done is the confirm page's answer once an action ran: the room is deleted.
func done(context.Context, mcpkit.Outcome) mcpkit.PendingResult {
	return mcpkit.PendingResult{Status: statusDeleted}
}
