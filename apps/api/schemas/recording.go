package schemas

import (
	"time"

	"github.com/google/uuid"
)

// Recording is one file a recorder produced for a call. A call can carry
// several, one per egress job. Shared is the owner's per-recording choice to
// let the call's other participants watch it; it is off until the owner turns
// it on. Filename is what egress reported, and only its basename is ever
// resolved against the recordings volume.
type Recording struct {
	ID        uuid.UUID `json:"id" gorm:"column:id;type:uuid;primaryKey"`
	CallID    uuid.UUID `json:"call_id" gorm:"column:call_id;type:uuid;not null;index"`
	EgressID  string    `json:"-" gorm:"column:egress_id;not null;default:''"`
	Filename  string    `json:"-" gorm:"column:filename;not null"`
	Shared    bool      `json:"shared" gorm:"column:shared;not null;default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`

	Call Call `json:"-" gorm:"foreignKey:CallID;constraint:OnDelete:CASCADE"`
}

func (Recording) TableName() string { return "recordings" }

// TranscriptLine is one final utterance of a call, timed from the call start.
// The transcript row keeps the concatenated text for the summariser; these
// rows are what search and the .vtt export read. Speaker is the LiveKit
// identity, resolved to a display name through call_participants at read time
// so an erased person's lines carry no name.
type TranscriptLine struct {
	ID       int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	CallID   uuid.UUID `json:"call_id" gorm:"column:call_id;type:uuid;not null;index"`
	Speaker  string    `json:"speaker" gorm:"column:speaker;not null"`
	Text     string    `json:"text" gorm:"column:text;type:text;not null"`
	OffsetMS int64     `json:"offset_ms" gorm:"column:offset_ms;not null;default:0"`

	Call Call `json:"-" gorm:"foreignKey:CallID;constraint:OnDelete:CASCADE"`
}

func (TranscriptLine) TableName() string { return "transcript_lines" }
