package dto

type CreateTransactionRequest struct {
	Mobile      string  `json:"mobile" binding:"required,len=10,numeric"`
	ContactName string  `json:"contact_name" binding:"required,min=2,max=100"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	Type        string  `json:"type" binding:"required,oneof=give receive"`
	Note        string  `json:"note"`
}

type UpdateTransactionRequest struct {
	TransactionID uint    `json:"transaction_id"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	Type          string  `json:"type,omitempty"`
	Note          string  `json:"note"`
}

type TransactionHistoryResponse struct {
	Mobile        string      `json:"mobile"`
	TotalGiven    float64     `json:"total_given"`
	TotalReceived float64     `json:"total_received"`
	Balance       float64     `json:"balance"`
	Transactions  interface{} `json:"transactions"`
}
