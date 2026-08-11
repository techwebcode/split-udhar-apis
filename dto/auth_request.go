package dto

type SignupRequest struct {
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Mobile   string `json:"mobile" binding:"required,len=10,numeric"`
}

type CheckEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type GoogleAuthRequest struct {
	IDToken string `json:"id_token" binding:"required"`
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

// SetMPINRequest sets the MPIN for the *authenticated* caller. Email is accepted
// for backwards compatibility with older clients but is deliberately ignored:
// the target account is taken from the JWT, never from the request body.
type SetMPINRequest struct {
	Email string `json:"email"`
	MPIN  string `json:"mpin" binding:"required,len=4,numeric"`
}

type VerifyMPINRequest struct {
	Email string `json:"email" binding:"required,email"`
	MPIN  string `json:"mpin" binding:"required,len=4,numeric"`
}
