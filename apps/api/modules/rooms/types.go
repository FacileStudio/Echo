package rooms

type CreateRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type RenameRequest struct {
	Name string `json:"name"`
}

type TokenRequest struct {
	DisplayName string `json:"display_name"`
}

type RoomResponse struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Owned     bool   `json:"owned"`
	CreatedAt string `json:"created_at"`
}

type TokenResponse struct {
	Token string       `json:"token"`
	URL   string       `json:"url"`
	Room  RoomResponse `json:"room"`
}
