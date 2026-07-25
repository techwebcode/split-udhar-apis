package dto

type CreateGroupRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Members     []GroupMemberReq `json:"members"`
}

type GroupMemberReq struct {
	Mobile string `json:"mobile" binding:"required"`
	Name   string `json:"name"`
}

type AddGroupExpenseRequest struct {
	Amount      float64  `json:"amount" binding:"required"`
	Description string   `json:"description" binding:"required"`
	PayerMobile string   `json:"payer_mobile" binding:"required"`
	SplitWith   []string `json:"split_with"` // List of member mobiles
}

type SettleGroupRequest struct {
	PayerMobile    string  `json:"payer_mobile" binding:"required"`
	ReceiverMobile string  `json:"receiver_mobile" binding:"required"`
	Amount         float64 `json:"amount" binding:"required"`
}
