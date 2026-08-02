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

func extractTenDigits(mobile string) string {
	digits := ""
	for _, ch := range mobile {
		if ch >= '0' && ch <= '9' {
			digits += string(ch)
		}
	}
	if len(digits) >= 10 {
		return digits[len(digits)-10:]
	}
	return digits
}

func (r *TransactionRepository) GetTransactionsBetween(userMobile, otherMobile string) ([]models.Transaction, error) {
	var transactions []models.Transaction
	u10 := extractTenDigits(userMobile)
	o10 := extractTenDigits(otherMobile)

	err := r.DB.
		Where(
			"((from_mobile = ? OR RIGHT(from_mobile, 10) = ?) AND (to_mobile = ? OR RIGHT(to_mobile, 10) = ?)) OR "+
				"((from_mobile = ? OR RIGHT(from_mobile, 10) = ?) AND (to_mobile = ? OR RIGHT(to_mobile, 10) = ?))",
			userMobile, u10, otherMobile, o10,
			otherMobile, o10, userMobile, u10,
		).
		Order("transaction_date DESC").
		Find(&transactions).Error

	return transactions, err
}

func (r *TransactionRepository) GetUserTransactions(userMobile string) ([]models.Transaction, error) {
	var transactions []models.Transaction
	u10 := extractTenDigits(userMobile)

	err := r.DB.
		Where(
			"from_mobile = ? OR to_mobile = ? OR RIGHT(from_mobile, 10) = ? OR RIGHT(to_mobile, 10) = ?",
			userMobile, userMobile, u10, u10,
		).
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
	u10 := extractTenDigits(mobile)

	err = r.DB.
		Where(
			"from_mobile = ? OR to_mobile = ? OR RIGHT(from_mobile, 10) = ? OR RIGHT(to_mobile, 10) = ?",
			mobile, mobile, u10, u10,
		).
		Find(&transactions).Error

	if err != nil {
		return
	}

	contactBalances := make(map[string]float64)

	for _, transaction := range transactions {
		if transaction.IsDeleted {
			continue
		}
		from10 := extractTenDigits(transaction.FromMobile)
		to10 := extractTenDigits(transaction.ToMobile)

		var otherContact string
		if transaction.FromMobile == mobile || from10 == u10 {
			otherContact = to10
			if otherContact == "" {
				otherContact = transaction.ToMobile
			}
			contactBalances[otherContact] += transaction.Amount
		} else if transaction.ToMobile == mobile || to10 == u10 {
			otherContact = from10
			if otherContact == "" {
				otherContact = transaction.FromMobile
			}
			contactBalances[otherContact] -= transaction.Amount
		}
	}

	for _, net := range contactBalances {
		if net > 0 {
			received += net
		} else if net < 0 {
			given += -net
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
	u10 := extractTenDigits(userMobile)
	c10 := extractTenDigits(contactMobile)

	err := r.DB.
		Where(
			"((from_mobile = ? OR RIGHT(from_mobile, 10) = ?) AND (to_mobile = ? OR RIGHT(to_mobile, 10) = ?)) OR "+
				"((from_mobile = ? OR RIGHT(from_mobile, 10) = ?) AND (to_mobile = ? OR RIGHT(to_mobile, 10) = ?))",
			userMobile, u10, contactMobile, c10,
			contactMobile, c10, userMobile, u10,
		).
		Order("transaction_date DESC").
		Find(&transactions).Error

	return transactions, err
}
