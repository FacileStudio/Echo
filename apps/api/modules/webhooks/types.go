package webhooks

const (
	eventRoomStarted       = "room_started"
	eventRoomFinished      = "room_finished"
	eventParticipantJoined = "participant_joined"
	eventParticipantLeft   = "participant_left"
	eventEgressEnded       = "egress_ended"
)

type payload struct {
	Event       string      `json:"event"`
	Room        *roomInfo   `json:"room,omitempty"`
	Participant *who        `json:"participant,omitempty"`
	Egress      *egressInfo `json:"egress,omitempty"`
}

type roomInfo struct {
	Name string `json:"name"`
}

type who struct {
	Identity string `json:"identity"`
	Name     string `json:"name"`
}

type egressInfo struct {
	EgressID    string       `json:"egressId"`
	FileResults []fileResult `json:"fileResults,omitempty"`
}

type fileResult struct {
	Filename string `json:"filename"`
}
