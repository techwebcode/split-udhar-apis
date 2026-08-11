package repositories

import (
	"split-udhar-apis/models"
	"split-udhar-apis/utils"

	"gorm.io/gorm"
)

type UserRepository struct {
	DB *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		DB: db,
	}
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	if email == "" {
		return nil, nil
	}
	var users []models.User
	err := r.DB.Where("email = ?", email).Limit(1).Find(&users).Error
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

func (r *UserRepository) GetByGoogleID(googleID string) (*models.User, error) {
	if googleID == "" {
		return nil, nil
	}
	var users []models.User
	err := r.DB.Where("google_id = ?", googleID).Limit(1).Find(&users).Error
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

func (r *UserRepository) GetByMobile(mobile string) (*models.User, error) {
	if mobile == "" {
		return nil, nil
	}

	var users []models.User
	cleanMobile := mobile
	if normalized := utils.NormalizeMobile(mobile); len(normalized) == 10 {
		cleanMobile = normalized
	}

	err := r.DB.Where("mobile = ? OR RIGHT(mobile, 10) = ?", mobile, cleanMobile).Limit(1).Find(&users).Error
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, nil
	}

	return &users[0], nil
}

func (r *UserRepository) Create(user *models.User) error {
	return r.DB.Create(user).Error
}

func (r *UserRepository) GetByID(id uint) (*models.User, error) {

	var user models.User

	err := r.DB.First(&user, id).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) Update(user *models.User) error {
	return r.DB.Save(user).Error
}

func (r *UserRepository) DeleteAccount(userID uint) error {
	var user models.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return err
	}

	// 1. Delete user device tokens
	r.DB.Where("user_id = ? OR mobile = ?", user.ID, user.Mobile).Delete(&models.UserDevice{})

	// 2. Delete OTP verifications associated with email/mobile
	if user.Email != "" || user.Mobile != "" {
		r.DB.Where("email = ? OR mobile = ?", user.Email, user.Mobile).Delete(&models.OTPVerification{})
	}

	// 3. Delete pending user registration records if any
	if user.Email != "" || user.Mobile != "" {
		r.DB.Where("email = ? OR mobile = ?", user.Email, user.Mobile).Delete(&models.PendingUser{})
	}

	// 4. Delete user profile & credentials completely (unscoped hard delete)
	return r.DB.Unscoped().Delete(&user).Error
}
