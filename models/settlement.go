package models

import "time"

type Settlement struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupID        uint64     `gorm:"not null;index:idx_settlement_group_status,priority:1" json:"group_id"`
	PayerUserID    uint64     `gorm:"not null;index:idx_settlement_payer" json:"payer_user_id"`
	ReceiverUserID uint64     `gorm:"not null;index:idx_settlement_receiver" json:"receiver_user_id"`
	Amount         float64    `gorm:"type:decimal(12,2);not null" json:"amount"`
	Currency       string     `gorm:"type:char(3);not null;default:'INR'" json:"currency"`
	PaymentMethod  string     `gorm:"type:varchar(30);not null" json:"payment_method"`
	Status         string     `gorm:"type:varchar(20);not null;default:'pending';index:idx_settlement_group_status,priority:2" json:"status"`
	IdempotencyKey string     `gorm:"type:varchar(100);not null;uniqueIndex:uq_settlement_idempotency" json:"idempotency_key"`
	Note           *string    `gorm:"type:varchar(255)" json:"note,omitempty"`
	CreatedAt      time.Time  `gorm:"not null" json:"created_at"`
	ConfirmedAt    *time.Time `json:"confirmed_at,omitempty"`
	UpdatedAt      time.Time  `gorm:"not null" json:"updated_at"`
}
