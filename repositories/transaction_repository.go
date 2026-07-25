package repositories

import (
	"split-udhar-apis/models"

	"gorm.io/gorm"
)

type TransactionRepository struct {
	DB *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{
		DB: db,
	}
}

func (r *TransactionRepository) Create(transaction *models.Transaction) error {
	return r.DB.Create(transaction).Error
}

func (r *TransactionRepository) Delete(id uint) error {
	return r.DB.Delete(&models.Transaction{}, id).Error
}

func (r *TransactionRepository) GetTransactionsBetween(userMobile, otherMobile string) ([]models.Transaction, error) {

	var transactions []models.Transaction

	err := r.DB.
		Where(
			"(from_mobile=? AND to_mobile=?) OR (from_mobile=? AND to_mobile=?)",
			userMobile,
			otherMobile,
			otherMobile,
			userMobile,
		).
		Order("transaction_date DESC").
		Find(&transactions).Error

	return transactions, err
}

func (r *TransactionRepository) GetUserTransactions(userMobile string) ([]models.Transaction, error) {

	var transactions []models.Transaction

	err := r.DB.
		Where("from_mobile=? OR to_mobile=?", userMobile, userMobile).
		Order("transaction_date DESC").
		Find(&transactions).Error

	return transactions, err
}

func (r *TransactionRepository) GetByID(id uint) (*models.Transaction, error) {

	var transaction models.Transaction

	err := r.DB.First(&transaction, id).Error
	if err != nil {
		return nil, err
	}

	return &transaction, nil
}

func (r *TransactionRepository) Update(transaction *models.Transaction) error {
	return r.DB.Save(transaction).Error
}

func (r *TransactionRepository) CreateEditLog(log *models.TransactionEditLog) error {
	return r.DB.Create(log).Error
}

func (r *TransactionRepository) GetEditLogs(transactionID uint) ([]models.TransactionEditLog, error) {
	var logs []models.TransactionEditLog
	err := r.DB.Where("transaction_id = ?", transactionID).Order("created_at DESC").Find(&logs).Error
	return logs, err
}

func (r *TransactionRepository) GetDashboardData(mobile string) (
	given float64,
	received float64,
	count int64,
	err error,
) {

	var transactions []models.Transaction

	err = r.DB.
		Where(
			"from_mobile = ? OR to_mobile = ?",
			mobile,
			mobile,
		).
		Find(&transactions).Error

	if err != nil {
		return
	}

	for _, transaction := range transactions {

		if transaction.FromMobile == mobile {

			given += transaction.Amount

		} else if transaction.ToMobile == mobile {

			received += transaction.Amount

		}
	}

	count = int64(len(transactions))

	return
}

func (r *TransactionRepository) GetTransactionsByMobile(
	userMobile string,
	contactMobile string,
) ([]models.Transaction, error) {

	var transactions []models.Transaction

	err := r.DB.
		Where(
			"(from_mobile = ? AND to_mobile = ?) OR (from_mobile = ? AND to_mobile = ?)",
			userMobile,
			contactMobile,
			contactMobile,
			userMobile,
		).
		Order("transaction_date DESC").
		Find(&transactions).Error

	return transactions, err
}
