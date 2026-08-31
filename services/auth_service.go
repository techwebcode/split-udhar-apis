package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"
	"split-udhar-apis/utils"

	"gorm.io/gorm"
)

const (
	// otpValidity must stay in step with the "valid for N minutes" copy in
	// templates/*.html and utils.ParseOTPTemplate's fallback body.
	otpValidity = 10 * time.Minute

	// maxMPINAttempts failed tries triggers a lockout of mpinLockoutWindow.
	maxMPINAttempts   = 5
	mpinLockoutWindow = 15 * time.Minute
)

type AuthService struct {
	UserRepo     *repositories.UserRepository
	OTPRepo      *repositories.OTPRepository
	PendingRepo  *repositories.PendingUserRepository
	EmailService *EmailService
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{
		UserRepo:     repositories.NewUserRepository(db),
		OTPRepo:      repositories.NewOTPRepository(db),
		PendingRepo:  repositories.NewPendingUserRepository(db),
		EmailService: NewEmailService(),
	}
}

// -------------------- GOOGLE AUTH --------------------

type GoogleTokenInfo struct {
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Sub           string `json:"sub"`
	Audience      string `json:"aud"`
	Error         string `json:"error_description"`
}

type GoogleAuthResult struct {
	ProfileComplete bool         `json:"profile_complete"`
	Token           string       `json:"token,omitempty"`
	RefreshToken    string       `json:"refresh_token,omitempty"`
	Email           string       `json:"email"`
	FullName        string       `json:"full_name"`
	GoogleID        string       `json:"google_id"`
	User            *models.User `json:"user,omitempty"`
}

func (s *AuthService) GoogleAuth(idToken string) (*GoogleAuthResult, error) {
	idToken = strings.TrimSpace(idToken)
	if idToken == "" {
		return nil, errors.New("google id_token is required")
	}

	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken)
	if err != nil {
		return nil, errors.New("failed to verify google token with identity provider")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("invalid or expired google token")
	}

	var info GoogleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, errors.New("failed to parse google token claims")
	}

	if info.Email == "" {
		return nil, errors.New("google token does not contain a valid email")
	}

	email := strings.ToLower(strings.TrimSpace(info.Email))
	googleID := strings.TrimSpace(info.Sub)
	fullName := strings.TrimSpace(info.Name)
	if fullName == "" {
		fullName = strings.Split(email, "@")[0]
	}

	// 1. Check if user exists by GoogleID or Email
	var user *models.User
	if googleID != "" {
		user, _ = s.UserRepo.GetByGoogleID(googleID)
	}

	if user == nil {
		existingUser, err := s.UserRepo.GetByEmail(email)
		if err == nil && existingUser != nil {
			user = existingUser
		}
	}

	if user != nil {
		// Auto-link GoogleID if missing
		if user.GoogleID == "" {
			user.GoogleID = googleID
			user.IsVerified = true
			_ = s.UserRepo.Update(user)
		}

		// Check if profile is complete (needs valid mobile AND mpin)
		isComplete := strings.TrimSpace(user.Mobile) != "" &&
			strings.TrimSpace(user.Mobile) != "0000000000" &&
			strings.TrimSpace(user.MPIN) != ""

		if isComplete {
			token, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
			if err != nil {
				return nil, fmt.Errorf("failed to generate authentication token: %v", err)
			}
			refreshToken, _ := utils.GenerateToken(user.ID, user.Email, user.Mobile)

			return &GoogleAuthResult{
				ProfileComplete: true,
				Token:           token,
				RefreshToken:    refreshToken,
				Email:           user.Email,
				FullName:        user.FullName,
				GoogleID:        googleID,
				User:            user,
			}, nil
		}

		// Account exists but profile is incomplete
		return &GoogleAuthResult{
			ProfileComplete: false,
			Email:           user.Email,
			FullName:        user.FullName,
			GoogleID:        googleID,
			User:            user,
		}, nil
	}

	// New Google user (identity verified, profile incomplete)
	return &GoogleAuthResult{
		ProfileComplete: false,
		Email:           email,
		FullName:        fullName,
		GoogleID:        googleID,
	}, nil
}

func (s *AuthService) CompleteGoogleSignup(req dto.CompleteGoogleSignupRequest) (string, string, *models.User, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	googleID := strings.TrimSpace(req.GoogleID)
	fullName := strings.TrimSpace(req.FullName)
	mobile := strings.TrimSpace(req.Mobile)
	mpin := strings.TrimSpace(req.MPIN)

	if email == "" || googleID == "" || fullName == "" || len(mobile) != 10 || len(mpin) != 4 {
		return "", "", nil, errors.New("all fields (name, email, mobile, 4-digit mpin) are required")
	}

	// Hash MPIN
	hashedMPIN, err := bcrypt.GenerateFromPassword([]byte(mpin), bcrypt.DefaultCost)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to process MPIN: %v", err)
	}

	var user *models.User
	if googleID != "" {
		user, _ = s.UserRepo.GetByGoogleID(googleID)
	}
	if user == nil {
		user, _ = s.UserRepo.GetByEmail(email)
	}

	if user != nil {
		user.FullName = fullName
		user.Mobile = mobile
		user.GoogleID = googleID
		user.AuthProvider = "google"
		user.IsVerified = true
		user.MPIN = string(hashedMPIN)
		user.MPINFailedAttempts = 0
		user.MPINLockedUntil = nil

		if err := s.UserRepo.Update(user); err != nil {
			return "", "", nil, fmt.Errorf("failed to update user profile: %v", err)
		}
	} else {
		newUser := models.User{
			FullName:     fullName,
			Email:        email,
			Mobile:       mobile,
			GoogleID:     googleID,
			AuthProvider: "google",
			IsVerified:   true,
			MPIN:         string(hashedMPIN),
		}

		if err := s.UserRepo.Create(&newUser); err != nil {
			return "", "", nil, fmt.Errorf("failed to create user profile: %v", err)
		}
		user = &newUser
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate token: %v", err)
	}
	refreshToken, _ := utils.GenerateToken(user.ID, user.Email, user.Mobile)

	return token, refreshToken, user, nil
}

// -------------------- CHECK EMAIL --------------------

func (s *AuthService) CheckEmail(email string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return false, errors.New("email is required")
	}

	existingUser, err := s.UserRepo.GetByEmail(normalized)
	if err == nil && existingUser != nil {
		return true, nil
	}
	return false, nil
}

// -------------------- SIGNUP --------------------

func (s *AuthService) Signup(req dto.SignupRequest) error {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Mobile = strings.TrimSpace(req.Mobile)

	if existingUser, err := s.UserRepo.GetByEmail(req.Email); err == nil && existingUser != nil {
		return errors.New("email already registered")
	}

	if existingUser, err := s.UserRepo.GetByMobile(req.Mobile); err == nil && existingUser != nil {
		return errors.New("mobile already registered")
	}

	otp := utils.GenerateOTP()

	pending := models.PendingUser{
		FullName: req.FullName,
		Email:    req.Email,
		Mobile:   req.Mobile,
	}

	if err := s.PendingRepo.CreateOrUpdate(&pending); err != nil {
		return err
	}

	verification := models.OTPVerification{
		Email:     req.Email,
		OTP:       otp,
		Purpose:   "signup",
		ExpiresAt: time.Now().Add(otpValidity),
	}

	if err := s.OTPRepo.CreateOrUpdate(&verification); err != nil {
		return err
	}

	if err := s.EmailService.SendOTP(req.Email, otp); err != nil {
		return err
	}

	return nil
}

// -------------------- VERIFY SIGNUP --------------------

func (s *AuthService) VerifySignup(req dto.SignupVerifyRequest) (string, string, *models.User, error) {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.OTP = strings.TrimSpace(req.OTP)

	record, err := s.OTPRepo.Get(req.Email, "signup", req.OTP)
	if err != nil {
		return "", "", nil, errors.New("invalid otp")
	}

	if time.Now().After(record.ExpiresAt) {
		return "", "", nil, errors.New("otp expired")
	}

	pending, err := s.PendingRepo.GetByEmail(req.Email)
	if err != nil {
		return "", "", nil, err
	}

	user := models.User{
		FullName:   pending.FullName,
		Email:      pending.Email,
		Mobile:     pending.Mobile,
		IsVerified: true,
	}

	if err := s.UserRepo.Create(&user); err != nil {
		return "", "", nil, err
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
	if err != nil {
		return "", "", nil, err
	}
	refreshToken, _ := utils.GenerateToken(user.ID, user.Email, user.Mobile)

	_ = s.OTPRepo.Delete(record.ID)
	_ = s.PendingRepo.Delete(pending.ID)

	return token, refreshToken, &user, nil
}

// -------------------- LOGIN --------------------

func (s *AuthService) Login(req dto.LoginRequest) (bool, error) {

	user, err := s.UserRepo.GetByEmail(req.Email)
	if err != nil || user == nil {
		return false, errors.New("user not found")
	}

	// If MPIN is set, return hasMPIN = true without generating/sending OTP
	if user.MPIN != "" {
		return true, nil
	}

	otp := utils.GenerateOTP()

	verification := models.OTPVerification{
		Email:     user.Email,
		OTP:       otp,
		Purpose:   "login",
		ExpiresAt: time.Now().Add(otpValidity),
	}

	if err := s.OTPRepo.CreateOrUpdate(&verification); err != nil {
		return false, err
	}

	if err := s.EmailService.SendOTP(user.Email, otp); err != nil {
		return false, err
	}

	return false, nil
}

// -------------------- SEND FORGOT PASSCODE OTP --------------------

func (s *AuthService) SendForgotPasscodeOTP(email string) error {
	user, err := s.UserRepo.GetByEmail(email)
	if err != nil || user == nil {
		return errors.New("no registered account found with this email address")
	}

	otp := utils.GenerateOTP()

	verification := models.OTPVerification{
		Email:     user.Email,
		OTP:       otp,
		Purpose:   "login",
		ExpiresAt: time.Now().Add(otpValidity),
	}

	if err := s.OTPRepo.CreateOrUpdate(&verification); err != nil {
		return err
	}

	if err := s.EmailService.SendForgotPasscodeOTP(user.Email, otp); err != nil {
		return err
	}

	return nil
}

// -------------------- VERIFY LOGIN --------------------

func (s *AuthService) VerifyLogin(req dto.LoginVerifyRequest) (string, string, *models.User, error) {

	record, err := s.OTPRepo.Get(req.Email, "login", req.OTP)
	if err != nil {
		return "", "", nil, errors.New("invalid otp")
	}

	if time.Now().After(record.ExpiresAt) {
		return "", "", nil, errors.New("otp expired")
	}

	user, err := s.UserRepo.GetByEmail(req.Email)
	if err != nil {
		return "", "", nil, err
	}
	if user == nil {
		return "", "", nil, errors.New("user not found")
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
	if err != nil {
		return "", "", nil, err
	}
	refreshToken, _ := utils.GenerateToken(user.ID, user.Email, user.Mobile)

	_ = s.OTPRepo.Delete(record.ID)

	return token, refreshToken, user, nil
}

// -------------------- SET MPIN --------------------

// SetMPIN updates the MPIN of the authenticated user identified by userID. The
// account is resolved from the JWT rather than from the request body, so a
// caller can only ever change their own MPIN.
func (s *AuthService) SetMPIN(userID uint, req dto.SetMPINRequest) error {
	if userID == 0 {
		return errors.New("unauthorized")
	}

	user, err := s.UserRepo.GetByID(userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	hashed, err := utils.HashMPIN(req.MPIN)
	if err != nil {
		return err
	}

	user.MPIN = hashed
	return s.UserRepo.Update(user)
}

// -------------------- VERIFY MPIN --------------------

func (s *AuthService) VerifyMPIN(req dto.VerifyMPINRequest) (string, string, *models.User, error) {
	user, err := s.UserRepo.GetByEmail(req.Email)
	if err != nil || user == nil {
		return "", "", nil, errors.New("user not found")
	}

	if user.MPIN == "" {
		return "", "", nil, errors.New("MPIN not set for this account")
	}

	if user.MPINLockedUntil != nil && time.Now().Before(*user.MPINLockedUntil) {
		remaining := int(time.Until(*user.MPINLockedUntil).Minutes()) + 1
		return "", "", nil, fmt.Errorf(
			"too many incorrect attempts, please try again in %d minute(s) or sign in with an OTP",
			remaining,
		)
	}

	if !utils.CheckMPIN(user.MPIN, req.MPIN) {
		user.MPINFailedAttempts++
		if user.MPINFailedAttempts >= maxMPINAttempts {
			lockedUntil := time.Now().Add(mpinLockoutWindow)
			user.MPINLockedUntil = &lockedUntil
			user.MPINFailedAttempts = 0
		}
		_ = s.UserRepo.Update(user)
		return "", "", nil, errors.New("incorrect MPIN")
	}

	// Successful login clears any accumulated failures.
	if user.MPINFailedAttempts != 0 || user.MPINLockedUntil != nil {
		user.MPINFailedAttempts = 0
		user.MPINLockedUntil = nil
	}

	// Transparently upgrade accounts still holding a pre-hashing plaintext MPIN.
	if !utils.IsHashedMPIN(user.MPIN) {
		if hashed, hashErr := utils.HashMPIN(req.MPIN); hashErr == nil {
			user.MPIN = hashed
		}
	}
	_ = s.UserRepo.Update(user)

	token, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
	if err != nil {
		return "", "", nil, err
	}
	refreshToken, _ := utils.GenerateToken(user.ID, user.Email, user.Mobile)

	return token, refreshToken, user, nil
}

// -------------------- REFRESH TOKEN --------------------

func (s *AuthService) RefreshToken(refreshTokenStr string) (string, string, *models.User, error) {
	refreshTokenStr = strings.TrimSpace(refreshTokenStr)
	if refreshTokenStr == "" {
		return "", "", nil, errors.New("refresh_token is required")
	}

	claims := &utils.Claims{}
	token, err := jwt.ParseWithClaims(
		refreshTokenStr,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		},
	)
	if err != nil || !token.Valid {
		return "", "", nil, errors.New("invalid or expired refresh token")
	}

	user, err := s.UserRepo.GetByID(claims.UserID)
	if err != nil || user == nil {
		return "", "", nil, errors.New("user account not found")
	}

	newToken, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate access token: %v", err)
	}

	newRefreshToken, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
	if err != nil {
		newRefreshToken = refreshTokenStr
	}

	return newToken, newRefreshToken, user, nil
}
