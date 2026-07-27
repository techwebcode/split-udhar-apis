package dto

type SaveFCMTokenRequest struct {
	FCMToken string `json:"fcm_token" binding:"required"`
	Platform string `json:"platform"`
	DeviceID string `json:"device_id"`
}

type DeleteFCMTokenRequest struct {
	FCMToken string `json:"fcm_token"`
	DeviceID string `json:"device_id"`
}
