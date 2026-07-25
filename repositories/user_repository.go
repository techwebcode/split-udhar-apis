package repositories

import (
	"split-udhar-apis/models"

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
	var user models.User

	err := r.DB.Where("email = ?", email).First(&user).Error

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetByMobile(mobile string) (*models.User, error) {
	var user models.User

	err := r.DB.Where("mobile = ?", mobile).First(&user).Error

	if err != nil {
		return nil, err
	}

	return &user, nil
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

func (r *TransactionRepository) GetTransactionSummary(
	mobile string,
) ([]models.Transaction, error) {

	var transactions []models.Transaction

	err := r.DB.
		Where(
			"from_mobile = ? OR to_mobile = ?",
			mobile,
			mobile,
		).
		Order("transaction_date DESC").
		Find(&transactions).Error

	return transactions, err
}
