package integrations

// CreateRoomRequest is the body of POST /integrations/rooms.
type CreateRoomRequest struct {
	ExternalID  string `json:"external_id"`
	Title       string `json:"title"`
	CallbackURL string `json:"callback_url"`
}

// RoomCreated answers POST /integrations/rooms.
type RoomCreated struct {
	RoomID  string `json:"room_id"`
	JoinURL string `json:"join_url"`
}

// TokenRequest is the body of POST /integrations/rooms/{id}/tokens.
type TokenRequest struct {
	Identity  string `json:"identity"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// TokenResponse carries a LiveKit join token, the server to use it on, and
// the embed URL that opens the call with it.
type TokenResponse struct {
	LivekitURL string `json:"livekit_url"`
	Token      string `json:"token"`
	EmbedURL   string `json:"embed_url"`
}

// Participant is one person connected to a call, keyed by the identity the
// app supplied when it minted their token.
type Participant struct {
	Identity string `json:"identity"`
	Name     string `json:"name"`
}

// RoomState answers GET /integrations/rooms/{id}.
type RoomState struct {
	RoomID       string        `json:"room_id"`
	Title        string        `json:"title"`
	Participants []Participant `json:"participants"`
}

// CallbackEvent is the JSON body forwarded to an app's callback_url.
// Participant is set on participant_joined and participant_left only.
type CallbackEvent struct {
	ID          string       `json:"id"`
	Event       string       `json:"event"`
	RoomID      string       `json:"room_id"`
	ExternalID  string       `json:"external_id"`
	At          string       `json:"at"`
	Participant *Participant `json:"participant,omitempty"`
}
