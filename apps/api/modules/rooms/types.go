package rooms

import "time"

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
// the room joined. CallDeadline is when the server ends the call under the
// owner's plan, RFC 3339 in UTC, and absent when call length is unlimited.
type TokenResponse struct {
	Token        string       `json:"token"`
	URL          string       `json:"url"`
	Room         RoomResponse `json:"room"`
	CallDeadline string       `json:"call_deadline,omitempty"`
}

func toTokenResponse(admission Admission, url string) TokenResponse {
	response := TokenResponse{Token: admission.Token, URL: url}
	if admission.Deadline != nil {
		response.CallDeadline = admission.Deadline.UTC().Format(time.RFC3339)
	}
	return response
}

// DeadlineResponse is the body of GET /rooms/{slug}/deadline: when the
// running call must end, absent when nothing limits it.
type DeadlineResponse struct {
	CallDeadline string `json:"call_deadline,omitempty"`
}

func toDeadlineResponse(deadline *time.Time) DeadlineResponse {
	if deadline == nil {
		return DeadlineResponse{}
	}
	return DeadlineResponse{CallDeadline: deadline.UTC().Format(time.RFC3339)}
}
