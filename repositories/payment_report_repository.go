package repositories

import (
	"split-udhar-apis/models"
	"split-udhar-apis/utils"

	"gorm.io/gorm"
)

type PaymentReportRepository struct {
	DB *gorm.DB
}

func NewPaymentReportRepository(db *gorm.DB) *PaymentReportRepository {
	return &PaymentReportRepository{DB: db}
}

func (r *PaymentReportRepository) WithTx(tx *gorm.DB) *PaymentReportRepository {
	return &PaymentReportRepository{DB: tx}
}

func (r *PaymentReportRepository) Create(report *models.PaymentReport) error {
	return r.DB.Create(report).Error
}

func (r *PaymentReportRepository) GetByID(id uint) (*models.PaymentReport, error) {
	var report models.PaymentReport
	err := r.DB.First(&report, id).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *PaymentReportRepository) Update(report *models.PaymentReport) error {
	return r.DB.Save(report).Error
}

func (r *PaymentReportRepository) GetReportsBetween(userMobile, otherMobile string) ([]models.PaymentReport, error) {
	var reports []models.PaymentReport
	u10 := utils.NormalizeMobile(userMobile)
	o10 := utils.NormalizeMobile(otherMobile)

	err := r.DB.Where(
		"((payer_mobile = ? OR RIGHT(payer_mobile, 10) = ?) AND (receiver_mobile = ? OR RIGHT(receiver_mobile, 10) = ?)) OR "+
			"((payer_mobile = ? OR RIGHT(payer_mobile, 10) = ?) AND (receiver_mobile = ? OR RIGHT(receiver_mobile, 10) = ?))",
		userMobile, u10, otherMobile, o10,
		otherMobile, o10, userMobile, u10,
	).Order("created_at DESC").Find(&reports).Error

	return reports, err
}

func (r *PaymentReportRepository) GetPendingReportsForUser(userMobile string) ([]models.PaymentReport, error) {
	var reports []models.PaymentReport
	u10 := utils.NormalizeMobile(userMobile)

	err := r.DB.Where(
		"(receiver_mobile = ? OR RIGHT(receiver_mobile, 10) = ? OR payer_mobile = ? OR RIGHT(payer_mobile, 10) = ?) AND status = ?",
		userMobile, u10, userMobile, u10, models.PaymentReportStatusAwaiting,
	).Order("created_at DESC").Find(&reports).Error

	return reports, err
}
