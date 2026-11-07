package schemas

import (
	"time"

	"github.com/google/uuid"
)

// Recording is one file a recorder produced for a call. A call can carry
// several, one per egress job. Shared is the owner's per-recording choice to
// let the call's other participants watch it; it is off until the owner turns
// it on. A browser recording is uploaded in chunks: it is Uploading, hidden
// from every list, until it is finalized as Complete; Size and Chunks say how
// much of it the volume holds. Filename is what egress reported, and only its basename is ever
// resolved against the recordings volume.
type Recording struct {
	ID        uuid.UUID `json:"id" gorm:"column:id;type:uuid;primaryKey"`
	CallID    uuid.UUID `json:"call_id" gorm:"column:call_id;type:uuid;not null;index"`
	EgressID  string    `json:"-" gorm:"column:egress_id;not null;default:''"`
	Filename  string    `json:"-" gorm:"column:filename;not null"`
	Shared    bool      `json:"shared" gorm:"column:shared;not null;default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`

	ContentType string    `json:"-" gorm:"column:content_type;not null;default:'video/mp4'"`
	Status      string    `json:"-" gorm:"column:status;not null;default:'complete'"`
	Size        int64     `json:"-" gorm:"column:size;not null;default:0"`
	Chunks      int       `json:"-" gorm:"column:chunks;not null;default:0"`
	UpdatedAt   time.Time `json:"-" gorm:"column:updated_at;autoUpdateTime"`

	Call Call `json:"-" gorm:"foreignKey:CallID;constraint:OnDelete:CASCADE"`
}

const (
	RecordingComplete  = "complete"
	RecordingUploading = "uploading"
)

func (Recording) TableName() string { return "recordings" }

// CaptureConsent is one participant's answer, for one call, to the organizer's
// request to transcribe or record them. A nil answer means not asked yet, which counts as
// a refusal: nothing of that person is transcribed or recorded.
type CaptureConsent struct {
	ID         int64     `json:"-" gorm:"column:id;primaryKey;autoIncrement"`
	CallID     uuid.UUID `json:"-" gorm:"column:call_id;type:uuid;not null;uniqueIndex:idx_capture_consent_identity"`
	Identity   string    `json:"-" gorm:"column:identity;not null;uniqueIndex:idx_capture_consent_identity"`
	Transcribe *bool     `json:"-" gorm:"column:transcribe"`
	Record     *bool     `json:"-" gorm:"column:record"`
	UpdatedAt  time.Time `json:"-" gorm:"column:updated_at;autoUpdateTime"`

	Call Call `json:"-" gorm:"foreignKey:CallID;constraint:OnDelete:CASCADE"`
}

func (CaptureConsent) TableName() string { return "capture_consents" }

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
