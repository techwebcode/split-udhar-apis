package repositories

import (
	"split-udhar-apis/models"

	"gorm.io/gorm"
)

type PendingUserRepository struct {
	DB *gorm.DB
}

func NewPendingUserRepository(db *gorm.DB) *PendingUserRepository {
	return &PendingUserRepository{
		DB: db,
	}
}

func (r *PendingUserRepository) CreateOrUpdate(user *models.PendingUser) error {

	var existing models.PendingUser

	err := r.DB.Where("email = ? OR mobile = ?", user.Email, user.Mobile).First(&existing).Error

	if err == nil {

		existing.FullName = user.FullName
		existing.Email = user.Email
		existing.Mobile = user.Mobile

		return r.DB.Save(&existing).Error
	}

	if err == gorm.ErrRecordNotFound {
		return r.DB.Create(user).Error
	}

	return err
}

func (r *PendingUserRepository) GetByEmail(email string) (*models.PendingUser, error) {

	var user models.PendingUser

	err := r.DB.Where("email = ?", email).First(&user).Error

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *PendingUserRepository) Delete(id uint) error {
	return r.DB.Delete(&models.PendingUser{}, id).Error
}
