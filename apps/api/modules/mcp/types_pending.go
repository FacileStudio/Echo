package mcp

import (
	mcpkit "github.com/FacileStudio/tronc/mcp"
)

// PendingItem is one action waiting on the confirm page.
type PendingItem struct {
	mcpkit.PendingItem
	Room *PendingRoom `json:"room,omitempty"`
}

// PendingRoom is the room an action targets, as it is now.
type PendingRoom struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Meetings int    `json:"meetings"`
}
