package services

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"
	"split-udhar-apis/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaymentReportService struct {
	db          *gorm.DB
	reportRepo  *repositories.PaymentReportRepository
	userRepo    *repositories.UserRepository
	txnRepo     *repositories.TransactionRepository
	dispatcher  *NotificationDispatcher
}

func NewPaymentReportService(db *gorm.DB) *PaymentReportService {
	return &PaymentReportService{
		db:         db,
		reportRepo: repositories.NewPaymentReportRepository(db),
		userRepo:   repositories.NewUserRepository(db),
		txnRepo:    repositories.NewTransactionRepository(db),
		dispatcher: NewNotificationDispatcher(db, NewFCMService(db)),
	}
}

func (s *PaymentReportService) CreateReport(
	payerMobile string,
	req dto.CreatePaymentReportRequest,
) (*models.PaymentReport, error) {
	if utils.SameMobile(payerMobile, req.ReceiverMobile) {
		return nil, errors.New("you cannot report a payment to yourself")
	}

	if req.Amount <= 0 {
		return nil, errors.New("payment amount must be greater than 0")
	}

	payerUser, _ := s.userRepo.GetByMobile(payerMobile)
	payerName := payerMobile
	if payerUser != nil && payerUser.FullName != "" {
		payerName = payerUser.FullName
	}

	receiverUser, _ := s.userRepo.GetByMobile(req.ReceiverMobile)
	receiverName := req.ReceiverName
	if receiverName == "" && receiverUser != nil && receiverUser.FullName != "" {
		receiverName = receiverUser.FullName
	}

	report := models.PaymentReport{
		PayerMobile:    payerMobile,
		ReceiverMobile: req.ReceiverMobile,
		PayerName:      payerName,
		ReceiverName:   receiverName,
		Amount:         req.Amount,
		Currency:       models.CurrencyINR,
		UTR:            req.UTR,
		Note:           req.Note,
		Status:         models.PaymentReportStatusAwaiting,
		ExpenseID:      req.ExpenseID,
	}

	if err := s.reportRepo.Create(&report); err != nil {
		return nil, err
	}

	// Dispatch notification to receiver asynchronously
	go func() {
		title := "Payment Reported"
		utrInfo := ""
		if req.UTR != "" {
			utrInfo = fmt.Sprintf(" (UTR: %s)", req.UTR)
		}
		body := fmt.Sprintf("%s reported paying ₹%.0f%s. Tap to review & confirm.", payerName, req.Amount, utrInfo)

		data := map[string]string{
			"type":              "payment_report",
			"payment_report_id": fmt.Sprintf("%d", report.ID),
			"mobile":            payerMobile,
			"amount":            fmt.Sprintf("%.2f", req.Amount),
		}

		idempotencyKey := fmt.Sprintf("pay_report_%d_create", report.ID)
		_, _ = s.dispatcher.EnqueueNotification(idempotencyKey, "user", []string{req.ReceiverMobile}, title, body, data)
	}()

	return &report, nil
}

func (s *PaymentReportService) ConfirmReport(
	reportID uint,
	userMobile string,
) (*models.PaymentReport, error) {
	var confirmedReport *models.PaymentReport

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var report models.PaymentReport
		q := tx
		if tx.Dialector.Name() != "sqlite" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := q.First(&report, reportID).Error; err != nil {
			return errors.New("payment report not found")
		}

		if !utils.SameMobile(report.ReceiverMobile, userMobile) {
			return errors.New("only the intended recipient can confirm this payment")
		}

		if report.Status == models.PaymentReportStatusConfirmed {
			return errors.New("payment report has already been confirmed")
		}
		if report.Status == models.PaymentReportStatusRejected {
			return errors.New("cannot confirm a rejected payment report")
		}
		if report.Status != models.PaymentReportStatusAwaiting {
			return errors.New("payment report is not awaiting confirmation")
		}

		now := time.Now()

		note := report.Note
		if report.UTR != "" {
			if note != "" {
				note = fmt.Sprintf("%s (UTR: %s)", note, report.UTR)
			} else {
				note = fmt.Sprintf("UPI Settlement (UTR: %s)", report.UTR)
			}
		} else if note == "" {
			note = "UPI Settlement Payment"
		}

		// Apply the settlement by creating a 1-to-1 transaction using existing backend settlement logic
		settlementTx := models.Transaction{
			ReferenceID:     fmt.Sprintf("TXN-%d-%04d", now.UnixNano(), rand.Intn(10000)),
			FromMobile:      report.PayerMobile,
			ToMobile:        report.ReceiverMobile,
			Type:            models.TransactionGive,
			Amount:          report.Amount,
			Note:            note,
			ContactName:     report.ReceiverName,
			ExpenseType:     models.ExpensePersonal,
			Status:          models.StatusSettled,
			TransactionDate: now,
			CreatedBy:       userMobile,
		}

		if err := tx.Create(&settlementTx).Error; err != nil {
			return err
		}

		report.Status = models.PaymentReportStatusConfirmed
		report.ConfirmedAt = &now
		report.TransactionID = &settlementTx.ID

		if err := tx.Save(&report).Error; err != nil {
			return err
		}

		confirmedReport = &report
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Dispatch notification to payer
	go func() {
		receiverUser, _ := s.userRepo.GetByMobile(confirmedReport.ReceiverMobile)
		receiverName := confirmedReport.ReceiverMobile
		if receiverUser != nil && receiverUser.FullName != "" {
			receiverName = receiverUser.FullName
		}

		title := "Payment Confirmed"
		body := fmt.Sprintf("%s confirmed receipt of ₹%.0f.", receiverName, confirmedReport.Amount)

		data := map[string]string{
			"type":              "payment_confirmed",
			"payment_report_id": fmt.Sprintf("%d", confirmedReport.ID),
			"mobile":            confirmedReport.ReceiverMobile,
			"amount":            fmt.Sprintf("%.2f", confirmedReport.Amount),
		}

		idempotencyKey := fmt.Sprintf("pay_report_%d_confirmed", confirmedReport.ID)
		_, _ = s.dispatcher.EnqueueNotification(idempotencyKey, "user", []string{confirmedReport.PayerMobile}, title, body, data)
	}()

	return confirmedReport, nil
}

func (s *PaymentReportService) RejectReport(
	reportID uint,
	userMobile string,
) (*models.PaymentReport, error) {
	var rejectedReport *models.PaymentReport

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var report models.PaymentReport
		q := tx
		if tx.Dialector.Name() != "sqlite" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := q.First(&report, reportID).Error; err != nil {
			return errors.New("payment report not found")
		}

		if !utils.SameMobile(report.ReceiverMobile, userMobile) {
			return errors.New("only the intended recipient can reject this payment")
		}

		if report.Status == models.PaymentReportStatusConfirmed {
			return errors.New("cannot reject an already confirmed payment")
		}
		if report.Status == models.PaymentReportStatusRejected {
			return errors.New("payment report has already been rejected")
		}

		now := time.Now()
		report.Status = models.PaymentReportStatusRejected
		report.RejectedAt = &now

		if err := tx.Save(&report).Error; err != nil {
			return err
		}

		rejectedReport = &report
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Dispatch notification to payer
	go func() {
		receiverUser, _ := s.userRepo.GetByMobile(rejectedReport.ReceiverMobile)
		receiverName := rejectedReport.ReceiverMobile
		if receiverUser != nil && receiverUser.FullName != "" {
			receiverName = receiverUser.FullName
		}

		title := "Payment Report Rejected"
		body := fmt.Sprintf("%s rejected the reported payment of ₹%.0f.", receiverName, rejectedReport.Amount)

		data := map[string]string{
			"type":              "payment_rejected",
			"payment_report_id": fmt.Sprintf("%d", rejectedReport.ID),
			"mobile":            rejectedReport.ReceiverMobile,
			"amount":            fmt.Sprintf("%.2f", rejectedReport.Amount),
		}

		idempotencyKey := fmt.Sprintf("pay_report_%d_rejected", rejectedReport.ID)
		_, _ = s.dispatcher.EnqueueNotification(idempotencyKey, "user", []string{rejectedReport.PayerMobile}, title, body, data)
	}()

	return rejectedReport, nil
}

func (s *PaymentReportService) GetReportsBetween(
	userMobile string,
	otherMobile string,
) ([]models.PaymentReport, error) {
	return s.reportRepo.GetReportsBetween(userMobile, otherMobile)
}
