package repositories

import (
	"split-udhar-apis/models"

	"gorm.io/gorm"
)

type OTPRepository struct {
	DB *gorm.DB
}

func NewOTPRepository(db *gorm.DB) *OTPRepository {
	return &OTPRepository{
		DB: db,
	}
}

func (r *OTPRepository) CreateOrUpdate(otp *models.OTPVerification) error {

	var existing models.OTPVerification

	err := r.DB.Where(
		"email = ? AND purpose = ?",
		otp.Email,
		otp.Purpose,
	).First(&existing).Error

	if err == nil {

		existing.OTP = otp.OTP
		existing.ExpiresAt = otp.ExpiresAt
		existing.Verified = false

		return r.DB.Save(&existing).Error
	}

	if err == gorm.ErrRecordNotFound {
		return r.DB.Create(otp).Error
	}

	return err
}

func (r *OTPRepository) Get(email, purpose, otp string) (*models.OTPVerification, error) {

	var verification models.OTPVerification

	err := r.DB.Where(
		"email = ? AND purpose = ? AND otp = ?",
		email,
		purpose,
		otp,
	).First(&verification).Error

	if err != nil {
		return nil, err
	}

	return &verification, nil
}

func (r *OTPRepository) Delete(id uint) error {
	return r.DB.Delete(&models.OTPVerification{}, id).Error
}
