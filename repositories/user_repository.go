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

func (r *UserRepository) GetByMobile(mobile string) (*models.User, error) {
	if mobile == "" {
		return nil, nil
	}

	var users []models.User
	cleanMobile := mobile
	digits := ""
	for _, ch := range mobile {
		if ch >= '0' && ch <= '9' {
			digits += string(ch)
		}
	}
	if len(digits) >= 10 {
		cleanMobile = digits[len(digits)-10:]
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
