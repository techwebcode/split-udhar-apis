package models

import "time"

type OTPVerification struct {
	ID uint `gorm:"primaryKey"`

	FullName string `gorm:"size:100"`
	Email    string `gorm:"size:255;index"`
	Mobile   string `gorm:"size:15"`

	OTP string `gorm:"size:6"`

	Purpose string `gorm:"size:20"`

	Verified bool

	ExpiresAt time.Time

	CreatedAt time.Time
}
