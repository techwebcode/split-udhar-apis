package models

import (
	"time"

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
	GroupID        uint       `gorm:"not null;index" json:"group_id"`
	Description    string     `gorm:"size:255;not null" json:"description"`
	Amount         float64    `gorm:"type:decimal(12,2);not null" json:"amount"`
	PayerMobile    string     `gorm:"size:15;not null" json:"payer_mobile"`
	PayerName      string     `gorm:"size:100" json:"payer_name"`
	CreatedBy      string     `gorm:"size:15" json:"created_by"`
	ExpenseDate    *time.Time `json:"expense_date"`
	IsEdited       bool       `gorm:"default:false" json:"is_edited"`
	PreviousAmount float64    `gorm:"type:decimal(12,2);default:0" json:"previous_amount"`
	PreviousDesc   string     `gorm:"size:255" json:"previous_desc"`
	EditedAt       *time.Time `json:"edited_at"`
	IsDeleted      bool       `gorm:"default:false" json:"is_deleted"`
	DeletedAtTime  *time.Time `json:"deleted_at_time"`
	DeletedBy      string     `gorm:"size:15" json:"deleted_by"`
}

type GroupExpenseEditLog struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ExpenseID      uint      `gorm:"not null;index" json:"expense_id"`
	GroupID        uint      `gorm:"not null;index" json:"group_id"`
	OldAmount      float64   `gorm:"type:decimal(12,2)" json:"old_amount"`
	NewAmount      float64   `gorm:"type:decimal(12,2)" json:"new_amount"`
	OldDescription string    `gorm:"size:255" json:"old_description"`
	NewDescription string    `gorm:"size:255" json:"new_description"`
	EditedBy       string    `gorm:"size:15" json:"edited_by"`
	EditedAt       time.Time `json:"edited_at"`
}
