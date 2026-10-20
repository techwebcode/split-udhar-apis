package dto

type CreatePaymentReportRequest struct {
	ReceiverMobile string  `json:"receiver_mobile" binding:"required"`
	ReceiverName   string  `json:"receiver_name"`
	Amount         float64 `json:"amount" binding:"required,gt=0"`
	UTR            string  `json:"utr"`
	Note           string  `json:"note"`
	ExpenseID      *uint   `json:"expense_id"`
}
