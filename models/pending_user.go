package models

import "time"

type PendingUser struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	FullName  string    `gorm:"size:100;not null" json:"full_name"`
	Email     string    `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Mobile    string    `gorm:"size:15;uniqueIndex;not null" json:"mobile"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
