package models

import (
	"fmt"
	"math/rand"
	"time"

	"gorm.io/gorm"
)

type TransactionType string

const (
	TransactionGive    TransactionType = "give"
	TransactionReceive TransactionType = "receive"
)

type ExpenseType string

const (
	ExpensePersonal ExpenseType = "personal"
	ExpenseGroup    ExpenseType = "group"
)

type TransactionStatus string

const (
	StatusPending TransactionStatus = "pending"
	StatusSettled TransactionStatus = "settled"
)

type Transaction struct {
	gorm.Model

	ReferenceID string `gorm:"size:50;uniqueIndex" json:"reference_id"`

	FromMobile string `gorm:"size:15;index;not null" json:"from_mobile"`
	ToMobile   string `gorm:"size:15;index;not null" json:"to_mobile"`

	Type TransactionType `gorm:"type:varchar(10);not null" json:"type"`

	Amount float64 `gorm:"type:decimal(12,2);not null" json:"amount"`

	Note string `gorm:"type:text" json:"note"`

	ContactName string `gorm:"size:100" json:"contact_name"`

	ExpenseType ExpenseType `gorm:"type:varchar(20);default:'personal'" json:"expense_type"`

	GroupID *uint `json:"group_id,omitempty"`

	Status TransactionStatus `gorm:"type:varchar(20);default:'pending'" json:"status"`

	TransactionDate time.Time `json:"transaction_date"`

	CreatedBy string `gorm:"size:50" json:"created_by"`
	UpdatedBy string `gorm:"size:50" json:"updated_by"`

	IsEdited       bool       `gorm:"default:false" json:"is_edited"`
	IsDeleted      bool       `gorm:"default:false" json:"is_deleted"`
	PreviousAmount float64    `gorm:"type:decimal(12,2)" json:"previous_amount,omitempty"`
	PreviousNote   string     `gorm:"type:text" json:"previous_note,omitempty"`
	EditedAt       *time.Time `json:"edited_at,omitempty"`
}

type TransactionEditLog struct {
	gorm.Model
	TransactionID uint      `gorm:"index;not null" json:"transaction_id"`
	OldAmount     float64   `gorm:"type:decimal(12,2)" json:"old_amount"`
	NewAmount     float64   `gorm:"type:decimal(12,2)" json:"new_amount"`
	OldNote       string    `gorm:"type:text" json:"old_note"`
	NewNote       string    `gorm:"type:text" json:"new_note"`
	EditedBy      string    `gorm:"size:50" json:"edited_by"`
	EditedAt      time.Time `json:"edited_at"`
}

func (t *Transaction) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ReferenceID == "" {
		t.ReferenceID = fmt.Sprintf("TXN-%d-%04d", time.Now().UnixNano(), rand.Intn(10000))
	}
	return nil
}
