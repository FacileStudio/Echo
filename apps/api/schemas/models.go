package schemas

import (
	"time"

	"github.com/google/uuid"
)

// Room is a persistent named meeting room; OwnerID is nil for unowned rooms.
type Room struct {
	ID        uuid.UUID `json:"id" gorm:"column:id;type:uuid;primaryKey"`
	Slug      string    `json:"slug" gorm:"column:slug;uniqueIndex;not null"`
	Name      string    `json:"name" gorm:"column:name;not null"`
	OwnerID   *int64    `json:"owner_id,omitempty" gorm:"column:owner_id;index"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (Room) TableName() string { return "rooms" }

// Call is one session in a room, from first participant to last out.
//
// LivekitRoomSID is LiveKit's per-session room id and is the identity every
// webhook handler keys on: a new meeting always carries a new sid, so a lost
// room_finished cannot merge two meetings into one row. LivekitRoomName is
// Echo's room slug, kept for display and for lookups by slug — the LiveKit
// room name and the slug are the same string by construction.
//
// EgressID is the recorder job that produced RecordingPath. It makes
// egress_ended idempotent: a retried delivery finds its own id already
// stored and stops.
type Call struct {
	ID              uuid.UUID  `json:"id" gorm:"column:id;type:uuid;primaryKey"`
	RoomID          uuid.UUID  `json:"room_id" gorm:"column:room_id;type:uuid;not null;index"`
	StartedAt       time.Time  `json:"started_at" gorm:"column:started_at;not null"`
	EndedAt         *time.Time `json:"ended_at,omitempty" gorm:"column:ended_at"`
	LivekitRoomName string     `json:"livekit_room_name" gorm:"column:livekit_room_name;not null;index"`
	LivekitRoomSID  string     `json:"livekit_room_sid" gorm:"column:livekit_room_sid;not null;default:''"`
	EgressID        string     `json:"egress_id" gorm:"column:egress_id;not null;default:''"`
	RecordingPath   string     `json:"recording_path" gorm:"column:recording_path;not null;default:''"`
	CreatedAt       time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`

	Room Room `json:"-" gorm:"foreignKey:RoomID;constraint:OnDelete:CASCADE"`
}

func (Call) TableName() string { return "calls" }

// Transcript holds the raw transcription text for a call. A call has at most
// one: the unique index is what makes the ingestion path's read-then-write
// safe under two concurrent requests.
type Transcript struct {
	ID        uuid.UUID `json:"id" gorm:"column:id;type:uuid;primaryKey"`
	CallID    uuid.UUID `json:"call_id" gorm:"column:call_id;type:uuid;not null;uniqueIndex"`
	Content   string    `json:"content" gorm:"column:content;type:text"`
	Language  string    `json:"language" gorm:"column:language;not null;default:'fr'"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`

	Call Call `json:"-" gorm:"foreignKey:CallID;constraint:OnDelete:CASCADE"`
}

func (Transcript) TableName() string { return "transcripts" }

// Summary holds an AI-generated summary of a call transcript. A call has at
// most one, enforced by the unique index rather than by the handler's
// read-then-write.
type Summary struct {
	ID        uuid.UUID `json:"id" gorm:"column:id;type:uuid;primaryKey"`
	CallID    uuid.UUID `json:"call_id" gorm:"column:call_id;type:uuid;not null;uniqueIndex"`
	Content   string    `json:"content" gorm:"column:content;type:text"`
	Model     string    `json:"model" gorm:"column:model;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`

	Call Call `json:"-" gorm:"foreignKey:CallID;constraint:OnDelete:CASCADE"`
}

func (Summary) TableName() string { return "summaries" }

// CallParticipant tracks one LiveKit identity's presence in a call. SID is
// the participant's per-connection id: a redelivered participant_joined
// carries the sid that already left, which is how a rejoin is told apart
// from a retry.
type CallParticipant struct {
	ID       int64      `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	CallID   uuid.UUID  `json:"call_id" gorm:"column:call_id;type:uuid;not null;uniqueIndex:idx_call_participant_identity"`
	Identity string     `json:"identity" gorm:"column:identity;not null;uniqueIndex:idx_call_participant_identity"`
	SID      string     `json:"sid" gorm:"column:sid;not null;default:''"`
	Name     string     `json:"name" gorm:"column:name;not null;default:''"`
	JoinedAt time.Time  `json:"joined_at" gorm:"column:joined_at;not null"`
	LeftAt   *time.Time `json:"left_at,omitempty" gorm:"column:left_at"`

	Call Call `json:"-" gorm:"foreignKey:CallID;constraint:OnDelete:CASCADE"`
}

func (CallParticipant) TableName() string { return "call_participants" }
