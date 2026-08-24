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
type Call struct {
	ID              uuid.UUID  `json:"id" gorm:"column:id;type:uuid;primaryKey"`
	RoomID          uuid.UUID  `json:"room_id" gorm:"column:room_id;type:uuid;not null"`
	StartedAt       time.Time  `json:"started_at" gorm:"column:started_at;not null"`
	EndedAt         *time.Time `json:"ended_at,omitempty" gorm:"column:ended_at"`
	LivekitRoomName string     `json:"livekit_room_name" gorm:"column:livekit_room_name;not null"`
	CreatedAt       time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`

	Room Room `json:"-" gorm:"foreignKey:RoomID;constraint:OnDelete:CASCADE"`
}

func (Call) TableName() string { return "calls" }

// Transcript holds the raw transcription text for a call.
type Transcript struct {
	ID        uuid.UUID `json:"id" gorm:"column:id;type:uuid;primaryKey"`
	CallID    uuid.UUID `json:"call_id" gorm:"column:call_id;type:uuid;not null"`
	Content   string    `json:"content" gorm:"column:content;type:text"`
	Language  string    `json:"language" gorm:"column:language;not null;default:'fr'"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`

	Call Call `json:"-" gorm:"foreignKey:CallID;constraint:OnDelete:CASCADE"`
}

func (Transcript) TableName() string { return "transcripts" }

// Summary holds an AI-generated summary of a call transcript.
type Summary struct {
	ID        uuid.UUID `json:"id" gorm:"column:id;type:uuid;primaryKey"`
	CallID    uuid.UUID `json:"call_id" gorm:"column:call_id;type:uuid;not null"`
	Content   string    `json:"content" gorm:"column:content;type:text"`
	Model     string    `json:"model" gorm:"column:model;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`

	Call Call `json:"-" gorm:"foreignKey:CallID;constraint:OnDelete:CASCADE"`
}

func (Summary) TableName() string { return "summaries" }
