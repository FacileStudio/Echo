package schemas

import "time"

// User is the local identity row: the account Registre's sign-in resolves to.
// The password hash is a legacy column that stays empty. It is kept NOT NULL
// so a future accidental write of a hash here cannot silently become a second
// source of truth for credentials. Locale is the last fr, en, es or de
// Registre reported.
type User struct {
	ID           int64     `json:"id" gorm:"column:id;primaryKey"`
	Email        string    `json:"email" gorm:"column:email;uniqueIndex;not null"`
	Name         string    `json:"name" gorm:"column:name"`
	PasswordHash string    `json:"-" gorm:"column:password_hash;not null;default:''"`
	Picture      string    `json:"picture" gorm:"column:picture;not null;default:''"`
	Locale       string    `json:"locale" gorm:"column:locale;not null;default:en"`
	IsAdmin      bool      `json:"is_admin" gorm:"column:is_admin;default:false"`
	CreatedAt    time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

func (User) TableName() string { return "users" }
