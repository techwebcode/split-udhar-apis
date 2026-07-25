package services

import (
	"errors"
	"time"

	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"
	"split-udhar-apis/utils"

	"gorm.io/gorm"
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

	if _, err := s.UserRepo.GetByEmail(req.Email); err == nil {
		return errors.New("email already registered")
	}

	if _, err := s.UserRepo.GetByMobile(req.Mobile); err == nil {
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
		ExpiresAt: time.Now().Add(5 * time.Minute),
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
	if err != nil {
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
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	if err := s.OTPRepo.CreateOrUpdate(&verification); err != nil {
		return false, err
	}

	if err := s.EmailService.SendOTP(user.Email, otp); err != nil {
		return false, err
	}

	return false, nil
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

	token, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
	if err != nil {
		return "", nil, err
	}

	_ = s.OTPRepo.Delete(record.ID)

	return token, user, nil
}

// -------------------- SET MPIN --------------------

func (s *AuthService) SetMPIN(req dto.SetMPINRequest) error {
	user, err := s.UserRepo.GetByEmail(req.Email)
	if err != nil {
		return errors.New("user not found")
	}

	user.MPIN = req.MPIN
	return s.UserRepo.Update(user)
}

// -------------------- VERIFY MPIN --------------------

func (s *AuthService) VerifyMPIN(req dto.VerifyMPINRequest) (string, *models.User, error) {
	user, err := s.UserRepo.GetByEmail(req.Email)
	if err != nil {
		return "", nil, errors.New("user not found")
	}

	if user.MPIN == "" {
		return "", nil, errors.New("MPIN not set for this account")
	}

	if user.MPIN != req.MPIN {
		return "", nil, errors.New("Incorrect MPIN")
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Mobile)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}
