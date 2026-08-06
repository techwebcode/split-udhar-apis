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

	// IsArchived is a transient field used to signal to the client that the group is soft-deleted.
	IsArchived bool `gorm:"-" json:"is_archived"`
}

type GroupMember struct {
	gorm.Model
	GroupID    uint    `gorm:"not null;index" json:"group_id"`
	UserMobile string  `gorm:"size:15;not null;index" json:"user_mobile"`
	UserName   string  `gorm:"size:100" json:"user_name"`
	Balance    float64 `gorm:"type:decimal(12,2);default:0" json:"balance"`
}

// GroupExpenseKind distinguishes a shared expense from a settlement payment.
// The two affect member balances differently, so replaying the ledger needs to
// tell them apart without parsing the description text.
type GroupExpenseKind string

const (
	GroupExpenseKindExpense    GroupExpenseKind = "expense"
	GroupExpenseKindSettlement GroupExpenseKind = "settlement"
)

type GroupExpense struct {
	gorm.Model
	GroupID     uint    `gorm:"not null;index" json:"group_id"`
	Description string  `gorm:"size:255;not null" json:"description"`
	Amount      float64 `gorm:"type:decimal(12,2);not null" json:"amount"`
	PayerMobile string  `gorm:"size:15;not null" json:"payer_mobile"`
	PayerName   string  `gorm:"size:100" json:"payer_name"`
	CreatedBy   string  `gorm:"size:15" json:"created_by"`

	Kind GroupExpenseKind `gorm:"type:varchar(20);not null;default:'expense';index" json:"kind"`

	// ReceiverMobile is only set on settlements: the member being paid. Without
	// it a settlement can't be replayed, since the description records display
	// names rather than numbers.
	ReceiverMobile string `gorm:"size:15" json:"receiver_mobile,omitempty"`
	// SplitWith records the exact member mobiles the expense was divided across,
	// comma separated. It must be replayed when reverting balances, otherwise a
	// subset split gets reverted across the whole group. Empty on rows created
	// before this column existed, which are treated as "split across everyone".
	SplitWith      string     `gorm:"type:text" json:"split_with"`
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
