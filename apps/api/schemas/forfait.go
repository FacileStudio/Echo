package schemas

import "time"

// ForfaitStateID is the primary key of the only ForfaitState row.
const ForfaitStateID = 1

// ForfaitState holds the last entitlements document porte/forfait exported,
// so a restart restores the plans Registre last pushed instead of failing open
// again. It is one row, rewritten after every applied directory snapshot.
// Document is kept as text, byte for byte, because forfait's Import rejects
// anything it did not write itself.
type ForfaitState struct {
	ID        int       `json:"id" gorm:"column:id;primaryKey;autoIncrement:false"`
	Document  string    `json:"-" gorm:"column:document;type:text;not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (ForfaitState) TableName() string { return "forfait_state" }
