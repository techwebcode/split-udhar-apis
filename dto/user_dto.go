package dto

type UpdateProfileRequest struct {
	FullName          string `json:"full_name" binding:"omitempty,min=2,max=100"`
	Mobile            string `json:"mobile" binding:"omitempty"`
	PreferredCurrency string `json:"preferred_currency" binding:"omitempty,oneof=INR USD EUR GBP AED CAD AUD SGD JPY CHF"`
}

type ProfileImageResponse struct {
	ProfileImageKey string `json:"profile_image_key"`
	ProfileImageURL string `json:"profile_image_url"`
}

type PaymentSettingsResponse struct {
	UpiID string `json:"upi_id"`
	IsSet bool   `json:"is_set"`
}

type UpdatePaymentSettingsRequest struct {
	UpiID string `json:"upi_id"`
}

type RecipientUpiResponse struct {
	Mobile string `json:"mobile"`
	Name   string `json:"name"`
	UpiID  string `json:"upi_id"`
	HasUpi bool   `json:"has_upi"`
}
