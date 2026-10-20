package models

import (
	"time"

	"gorm.io/gorm"
)

type PaymentReportStatus string

const (
	PaymentReportStatusAwaiting  PaymentReportStatus = "awaiting_confirmation"
	PaymentReportStatusConfirmed PaymentReportStatus = "confirmed"
	PaymentReportStatusRejected  PaymentReportStatus = "rejected"
)

type PaymentReport struct {
	gorm.Model

	PayerMobile    string              `gorm:"size:15;index:idx_pay_report_payer;not null" json:"payer_mobile"`
	ReceiverMobile string              `gorm:"size:15;index:idx_pay_report_receiver;not null" json:"receiver_mobile"`
	PayerName      string              `gorm:"size:100" json:"payer_name"`
	ReceiverName   string              `gorm:"size:100" json:"receiver_name"`
	Amount         float64             `gorm:"type:decimal(12,2);not null" json:"amount"`
	Currency       string              `gorm:"type:char(3);not null;default:'INR'" json:"currency"`
	UTR            string              `gorm:"size:100" json:"utr"`
	Note           string              `gorm:"type:text" json:"note"`
	Status         PaymentReportStatus `gorm:"type:varchar(30);default:'awaiting_confirmation';index:idx_pay_report_status" json:"status"`
	ExpenseID      *uint               `json:"expense_id,omitempty"`
	TransactionID  *uint               `gorm:"index" json:"transaction_id,omitempty"`

	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	RejectedAt  *time.Time `json:"rejected_at,omitempty"`
}
