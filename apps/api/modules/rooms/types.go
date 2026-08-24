package rooms

// CreateRequest is the body of POST /rooms.
type CreateRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// RenameRequest is the body of PATCH /rooms/{slug}.
type RenameRequest struct {
	Name string `json:"name"`
}

// TokenRequest is the body of POST /rooms/{slug}/token; DisplayName is
// required only for guests.
type TokenRequest struct {
	DisplayName string `json:"display_name"`
}

// RoomResponse is the wire shape of a room; Owned is relative to the caller.
type RoomResponse struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Owned     bool   `json:"owned"`
	CreatedAt string `json:"created_at"`
}

// TokenResponse carries the LiveKit token, the server URL to connect to and
// the room joined.
type TokenResponse struct {
	Token string       `json:"token"`
	URL   string       `json:"url"`
	Room  RoomResponse `json:"room"`
}
