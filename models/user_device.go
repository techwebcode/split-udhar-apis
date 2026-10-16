package models

import (
	"gorm.io/gorm"
)

type UserDevice struct {
	gorm.Model
	UserID   uint   `gorm:"index:idx_user_devices_user_deleted,priority:1;index;not null" json:"user_id"`
	Mobile   string `gorm:"size:15;index:idx_user_devices_mobile_deleted,priority:1;index;not null" json:"mobile"`
	FCMToken string `gorm:"size:255;uniqueIndex;not null" json:"fcm_token"`
	Platform string `gorm:"size:20;default:'android'" json:"platform"`
}
