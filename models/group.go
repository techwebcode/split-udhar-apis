package models

import (
	"gorm.io/gorm"
)

type Group struct {
	gorm.Model
	Name        string         `gorm:"size:100;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedBy   string         `gorm:"size:15;not null;index" json:"created_by"`
	Members     []GroupMember  `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"members"`
	Expenses    []GroupExpense `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"expenses"`
}

type GroupMember struct {
	gorm.Model
	GroupID    uint    `gorm:"not null;index" json:"group_id"`
	UserMobile string  `gorm:"size:15;not null;index" json:"user_mobile"`
	UserName   string  `gorm:"size:100" json:"user_name"`
	Balance    float64 `gorm:"type:decimal(12,2);default:0" json:"balance"`
}

type GroupExpense struct {
	gorm.Model
	GroupID     uint    `gorm:"not null;index" json:"group_id"`
	Description string  `gorm:"size:255;not null" json:"description"`
	Amount      float64 `gorm:"type:decimal(12,2);not null" json:"amount"`
	PayerMobile string  `gorm:"size:15;not null" json:"payer_mobile"`
	PayerName   string  `gorm:"size:100" json:"payer_name"`
	CreatedBy   string  `gorm:"size:15" json:"created_by"`
}
