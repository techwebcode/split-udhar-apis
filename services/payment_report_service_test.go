package services

import (
	"fmt"
	"testing"
	"time"

	"split-udhar-apis/dto"
	"split-udhar-apis/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupPaymentReportTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:memdb_payment_report_%d?mode=memory&cache=shared&_busy_timeout=5000", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(
		&models.User{},
		&models.Transaction{},
		&models.TransactionEditLog{},
		&models.PaymentReport{},
		&models.NotificationOutbox{},
		&models.UserDevice{},
	); err != nil {
		t.Fatalf("failed to automigrate models: %v", err)
	}

	return db
}

func TestPaymentReportFlow(t *testing.T) {
	db := setupPaymentReportTestDB(t)
	svc := NewPaymentReportService(db)

	payerMobile := "9876543210"
	receiverMobile := "9123456789"
	otherMobile := "9999999999"

	// 1. Prevent self-reporting
	_, err := svc.CreateReport(payerMobile, dto.CreatePaymentReportRequest{
		ReceiverMobile: payerMobile,
		Amount:         500,
	})
	if err == nil {
		t.Fatal("expected error when reporting payment to self, got nil")
	}

	// 2. Create valid payment report
	report, err := svc.CreateReport(payerMobile, dto.CreatePaymentReportRequest{
		ReceiverMobile: receiverMobile,
		ReceiverName:   "Priya",
		Amount:         500.0,
		UTR:            "UTR1234567890",
		Note:           "Settled via GPay",
	})
	if err != nil {
		t.Fatalf("failed to create payment report: %v", err)
	}

	if report.ID == 0 {
		t.Fatal("expected non-zero report ID")
	}
	if report.Status != models.PaymentReportStatusAwaiting {
		t.Fatalf("expected status %s, got %s", models.PaymentReportStatusAwaiting, report.Status)
	}

	// 3. Unauthorized confirmation (user other than receiver cannot confirm)
	_, err = svc.ConfirmReport(report.ID, otherMobile)
	if err == nil {
		t.Fatal("expected error on unauthorized confirm, got nil")
	}

	// 4. Unauthorized rejection (user other than receiver cannot reject)
	_, err = svc.RejectReport(report.ID, otherMobile)
	if err == nil {
		t.Fatal("expected error on unauthorized reject, got nil")
	}

	// 5. Successful confirmation by receiver
	confirmed, err := svc.ConfirmReport(report.ID, receiverMobile)
	if err != nil {
		t.Fatalf("failed to confirm payment report: %v", err)
	}
	if confirmed.Status != models.PaymentReportStatusConfirmed {
		t.Fatalf("expected status %s, got %s", models.PaymentReportStatusConfirmed, confirmed.Status)
	}
	if confirmed.TransactionID == nil || *confirmed.TransactionID == 0 {
		t.Fatal("expected valid transaction ID on confirmed report")
	}

	// 6. Verify Transaction was recorded in database
	var tx models.Transaction
	if err := db.First(&tx, *confirmed.TransactionID).Error; err != nil {
		t.Fatalf("failed to find created settlement transaction: %v", err)
	}
	if tx.Amount != 500.0 {
		t.Fatalf("expected transaction amount 500, got %f", tx.Amount)
	}
	if tx.FromMobile != payerMobile || tx.ToMobile != receiverMobile {
		t.Fatalf("expected payer %s and receiver %s, got fromMobile: %s, toMobile: %s", payerMobile, receiverMobile, tx.FromMobile, tx.ToMobile)
	}
	if tx.Type != models.TransactionGive {
		t.Fatalf("expected transaction type '%s', got %s", models.TransactionGive, tx.Type)
	}

	// 7. Duplicate confirmation prevention (cannot re-confirm)
	_, err = svc.ConfirmReport(report.ID, receiverMobile)
	if err == nil {
		t.Fatal("expected error on re-confirming already confirmed report, got nil")
	}

	// 8. Test Rejection flow on a second report
	report2, err := svc.CreateReport(payerMobile, dto.CreatePaymentReportRequest{
		ReceiverMobile: receiverMobile,
		Amount:         200.0,
		UTR:            "UTR9876543210",
	})
	if err != nil {
		t.Fatalf("failed to create second payment report: %v", err)
	}

	rejected, err := svc.RejectReport(report2.ID, receiverMobile)
	if err != nil {
		t.Fatalf("failed to reject payment report: %v", err)
	}
	if rejected.Status != models.PaymentReportStatusRejected {
		t.Fatalf("expected status %s, got %s", models.PaymentReportStatusRejected, rejected.Status)
	}
	if rejected.TransactionID != nil {
		t.Fatal("expected nil TransactionID on rejected report")
	}

	// 9. Verify no new transaction was created for rejected report
	var count int64
	db.Model(&models.Transaction{}).Where("amount = ?", 200.0).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 transactions for rejected report, got %d", count)
	}

	// 10. Cannot confirm an already rejected report
	_, err = svc.ConfirmReport(report2.ID, receiverMobile)
	if err == nil {
		t.Fatal("expected error when trying to confirm a rejected report, got nil")
	}
}
