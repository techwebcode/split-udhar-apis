package dto

type SignupRequest struct {
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Mobile   string `json:"mobile" binding:"required,len=10,numeric"`
}

type SignupVerifyRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required,len=6"`
}

type LoginRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type LoginVerifyRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required,len=6"`
}

type SetMPINRequest struct {
	Email string `json:"email" binding:"required,email"`
	MPIN  string `json:"mpin" binding:"required,len=4,numeric"`
}

type VerifyMPINRequest struct {
	Email string `json:"email" binding:"required,email"`
	MPIN  string `json:"mpin" binding:"required,len=4,numeric"`
}
