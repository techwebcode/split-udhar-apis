package dto

type AuthResponse struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}
