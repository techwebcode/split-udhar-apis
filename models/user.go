package models

import "time"

type User struct {
	BaseModel
	FullName   string `gorm:"size:100;not null" json:"full_name"`
	Email      string `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Mobile     string `gorm:"size:15;uniqueIndex;not null" json:"mobile"`
	IsVerified bool   `gorm:"default:false" json:"is_verified"`
	MPIN       string `gorm:"size:255" json:"-"` // bcrypt hash; wide enough for the 60-char digest

	// A 4-digit MPIN only has 10,000 combinations, so failed attempts are
	// counted and the account is locked out temporarily to make online
	// guessing impractical.
	MPINFailedAttempts int        `gorm:"default:0" json:"-"`
	MPINLockedUntil    *time.Time `json:"-"`
}
