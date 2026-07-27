package models

import (
	"gorm.io/gorm"
)

type UserDevice struct {
	gorm.Model
	UserID   uint   `gorm:"index;not null" json:"user_id"`
	Mobile   string `gorm:"size:15;index;not null" json:"mobile"`
	FCMToken string `gorm:"size:500;uniqueIndex;not null" json:"fcm_token"`
	Platform string `gorm:"size:20;default:'android'" json:"platform"`
	DeviceID string `gorm:"size:100;index" json:"device_id"`
}
