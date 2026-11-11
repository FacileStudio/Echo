package mcp

// listRoomsInput is echo_list_rooms's input: it takes none.
type listRoomsInput struct{}

// createRoomInput is echo_create_room's input.
type createRoomInput struct {
	Name string `json:"name" jsonschema:"the room's display name, such as Weekly sync"`
	Slug string `json:"slug,omitempty" jsonschema:"the room's address, lowercase letters, digits and hyphens; derived from name when left out"`
}

// renameRoomInput is echo_rename_room's input.
type renameRoomInput struct {
	Room string `json:"room" jsonschema:"the slug of one of your rooms, from echo_list_rooms"`
	Name string `json:"name" jsonschema:"the new display name; the slug and the link stay the same"`
}

// roomInput names one of the caller's rooms.
type roomInput struct {
	Room string `json:"room" jsonschema:"the slug of one of your rooms, from echo_list_rooms"`
}

// listMeetingsInput is echo_list_meetings's input.
type listMeetingsInput struct {
	Limit  int    `json:"limit,omitempty" jsonschema:"meetings per page, 1 to 50. Default 20"`
	Cursor string `json:"cursor,omitempty" jsonschema:"next_cursor from the previous page"`
}

// meetingInput names one meeting.
type meetingInput struct {
	MeetingID string `json:"meeting_id" jsonschema:"a meeting id, from echo_list_meetings"`
}

// transcriptInput is echo_read_transcript's input.
type transcriptInput struct {
	MeetingID string `json:"meeting_id" jsonschema:"a meeting id, from echo_list_meetings"`
	Query     string `json:"query,omitempty" jsonschema:"keep only the lines whose text or speaker contains this, ignoring case"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"next_cursor from the previous page"`
}

// shareInput is echo_share_recording's input.
type shareInput struct {
	RecordingID string `json:"recording_id" jsonschema:"a recording id, from echo_get_meeting"`
	Shared      bool   `json:"shared" jsonschema:"true lets the people who sat in the call watch it, false takes it back"`
}

// roomOut is one room. JoinURL carries the room key only when Echo just made
// the room: a key is never stored on the server.
type roomOut struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	JoinURL   string `json:"join_url"`
	CreatedAt string `json:"created_at"`
}

// roomsOutput answers echo_list_rooms.
type roomsOutput struct {
	Rooms []roomOut `json:"rooms"`
	Note  string    `json:"note"`
}

// roomOutput answers the tools that change one room.
type roomOutput struct {
	Status     string   `json:"status"`
	Room       *roomOut `json:"room,omitempty"`
	ConfirmURL string   `json:"confirm_url,omitempty"`
	Message    string   `json:"message,omitempty"`
}

// personOut is one person who sat in a meeting.
type personOut struct {
	Name     string `json:"name"`
	Guest    bool   `json:"guest,omitempty"`
	JoinedAt string `json:"joined_at"`
	LeftAt   string `json:"left_at,omitempty"`
}

// meetingOut is one meeting in a list.
type meetingOut struct {
	ID              string      `json:"id"`
	Room            string      `json:"room"`
	RoomName        string      `json:"room_name"`
	Role            string      `json:"role"`
	StartedAt       string      `json:"started_at"`
	EndedAt         string      `json:"ended_at,omitempty"`
	DurationMinutes *int64      `json:"duration_minutes,omitempty"`
	HasRecording    bool        `json:"has_recording"`
	HasTranscript   bool        `json:"has_transcript"`
	People          []personOut `json:"people"`
}

// meetingsOutput answers echo_list_meetings.
type meetingsOutput struct {
	Meetings   []meetingOut `json:"meetings"`
	Total      int64        `json:"total"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

// summaryOut is a meeting's AI summary.
type summaryOut struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
}

// recordingOut is one recording the caller can watch.
type recordingOut struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Shared    bool   `json:"shared"`
	CanShare  bool   `json:"can_share"`
}

// meetingOutput answers echo_get_meeting.
type meetingOutput struct {
	ID            string         `json:"id"`
	Room          string         `json:"room"`
	RoomName      string         `json:"room_name"`
	Role          string         `json:"role"`
	StartedAt     string         `json:"started_at"`
	EndedAt       string         `json:"ended_at,omitempty"`
	HasTranscript bool           `json:"has_transcript"`
	Summary       *summaryOut    `json:"summary,omitempty"`
	People        []personOut    `json:"people"`
	Recordings    []recordingOut `json:"recordings"`
	URL           string         `json:"url"`
}

// lineOut is one transcript line.
type lineOut struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
	At      string `json:"at,omitempty"`
}

// transcriptOutput answers echo_read_transcript.
type transcriptOutput struct {
	Total      int       `json:"total"`
	Matched    int       `json:"matched"`
	Lines      []lineOut `json:"lines"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

// summaryOutput answers echo_summarize_meeting.
type summaryOutput struct {
	Status  string     `json:"status"`
	Summary summaryOut `json:"summary"`
}

// recordingOutput answers echo_share_recording.
type recordingOutput struct {
	Status    string       `json:"status"`
	Recording recordingOut `json:"recording"`
}
