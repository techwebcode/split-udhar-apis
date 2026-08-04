package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

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

func (s *AuthService) VerifySignup(req dto.SignupVerifyRequest) (string, *models.User, error) {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.OTP = strings.TrimSpace(req.OTP)

	record, err := s.OTPRepo.Get(req.Email, "signup", req.OTP)
	if err != nil {
		return "", nil, errors.New("invalid otp")
	}

	if time.Now().After(record.ExpiresAt) {
		return "", nil, errors.New("otp expired")
	}

	pending, err := s.PendingRepo.GetByEmail(req.Email)
	if err != nil {
		return "", nil, err
	}

	user := models.User{
		FullName:   pending.FullName,
		Email:      pending.Email,
		Mobile:     pending.Mobile,
		IsVerified: true,
	}

	if err := s.UserRepo.Create(&user); err != nil {
		return "", nil, err
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
	if err != nil {
		return "", nil, err
	}

	_ = s.OTPRepo.Delete(record.ID)
	_ = s.PendingRepo.Delete(pending.ID)

	return token, &user, nil
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

func (s *AuthService) VerifyLogin(req dto.LoginVerifyRequest) (string, *models.User, error) {

	record, err := s.OTPRepo.Get(req.Email, "login", req.OTP)
	if err != nil {
		return "", nil, errors.New("invalid otp")
	}

	if time.Now().After(record.ExpiresAt) {
		return "", nil, errors.New("otp expired")
	}

	user, err := s.UserRepo.GetByEmail(req.Email)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, errors.New("user not found")
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
	if err != nil {
		return "", nil, err
	}

	_ = s.OTPRepo.Delete(record.ID)

	return token, user, nil
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

func (s *AuthService) VerifyMPIN(req dto.VerifyMPINRequest) (string, *models.User, error) {
	user, err := s.UserRepo.GetByEmail(req.Email)
	if err != nil || user == nil {
		return "", nil, errors.New("user not found")
	}

	if user.MPIN == "" {
		return "", nil, errors.New("MPIN not set for this account")
	}

	if user.MPINLockedUntil != nil && time.Now().Before(*user.MPINLockedUntil) {
		remaining := int(time.Until(*user.MPINLockedUntil).Minutes()) + 1
		return "", nil, fmt.Errorf(
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
		return "", nil, errors.New("incorrect MPIN")
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
		return "", nil, err
	}

	return token, user, nil
}
