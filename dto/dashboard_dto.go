package dto

import (
	"split-udhar-apis/models"
	"time"
)

type TransactionSummaryResponse struct {
	Mobile              string               `json:"mobile"`
	ContactName         string               `json:"contact_name"`
	RegisteredName      string               `json:"registered_name,omitempty"`
	IsRegistered        bool                 `json:"is_registered"`
	Balance             float64              `json:"balance"`
	TotalTransactions   int64                `json:"total_transactions"`
	LastTransactionDate time.Time            `json:"last_transaction_date"`
	Transactions        []models.Transaction `json:"transactions"`
}
