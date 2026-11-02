package webhooks

// The LiveKit event names Echo reacts to. Every other event is accepted and
// ignored: LiveKit sends track_*, ingress_* and egress_started too, and a 4xx
// on those would make the server retry an event nothing will ever record.
const (
	eventRoomStarted       = "room_started"
	eventRoomFinished      = "room_finished"
	eventParticipantJoined = "participant_joined"
	eventParticipantLeft   = "participant_left"
	eventEgressEnded       = "egress_ended"
)
