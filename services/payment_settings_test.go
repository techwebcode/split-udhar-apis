package services

import (
	"fmt"
	"testing"
	"time"

	"split-udhar-apis/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupPaymentSettingsTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:memdb_payment_settings_%d?mode=memory&cache=shared&_busy_timeout=5000", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
	); err != nil {
		t.Fatalf("failed to automigrate models: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)

	return db
}

func TestPaymentSettingsService(t *testing.T) {
	db := setupPaymentSettingsTestDB(t)
	svc := NewUserService(db)

	// Create test users
	user1 := models.User{
		FullName: "Rajat Singh",
		Email:    "rajat@example.com",
		Mobile:   "9876543210",
	}
	db.Create(&user1)

	user2 := models.User{
		FullName: "Priya Sharma",
		Email:    "priya@example.com",
		Mobile:   "9123456789",
	}
	db.Create(&user2)

	// 1. Initial state: no UPI ID configured
	settings, err := svc.GetPaymentSettings(user1.ID)
	if err != nil {
		t.Fatalf("failed to get payment settings: %v", err)
	}
	if settings.IsSet || settings.UpiID != "" {
		t.Fatalf("expected unset UPI ID initially, got %s", settings.UpiID)
	}

	// 2. Reject invalid UPI ID formats
	invalidFormats := []string{
		"invalid-upi",
		"@okhdfcbank",
		"rajat@",
		"rajat@@okaxis",
		"rajat name@ybl",
		"r@b", // too short before @
	}
	for _, inv := range invalidFormats {
		_, err := svc.UpdatePaymentSettings(user1.ID, inv)
		if err == nil {
			t.Fatalf("expected error for invalid UPI ID '%s', got nil", inv)
		}
	}

	// 3. Accept valid UPI ID formats
	validFormats := []string{
		"rajat@okaxis",
		"priya.sharma@ybl",
		"user_name-123@icici",
		"9876543210@upi",
	}
	for _, val := range validFormats {
		res, err := svc.UpdatePaymentSettings(user1.ID, val)
		if err != nil {
			t.Fatalf("expected valid UPI ID '%s' to succeed, got error: %v", val, err)
		}
		if !res.IsSet || res.UpiID != val {
			t.Fatalf("expected UpiID to be '%s', got '%s'", val, res.UpiID)
		}
	}

	// 4. Verify persistence in DB
	var dbUser models.User
	db.First(&dbUser, user1.ID)
	if dbUser.UpiID != "9876543210@upi" {
		t.Fatalf("expected DB UpiID to be '9876543210@upi', got '%s'", dbUser.UpiID)
	}

	// 5. Lookup recipient UPI by mobile
	recipRes, err := svc.GetRecipientUpiByMobile("9876543210")
	if err != nil {
		t.Fatalf("failed to get recipient UPI: %v", err)
	}
	if !recipRes.HasUpi || recipRes.UpiID != "9876543210@upi" {
		t.Fatalf("expected recipient UPI '9876543210@upi', got '%s'", recipRes.UpiID)
	}

	// 6. User2 has no UPI configured
	recip2, err := svc.GetRecipientUpiByMobile("9123456789")
	if err != nil {
		t.Fatalf("failed to get recipient2 UPI: %v", err)
	}
	if recip2.HasUpi || recip2.UpiID != "" {
		t.Fatalf("expected recipient2 to have no UPI, got '%s'", recip2.UpiID)
	}

	// 7. Clear / Delete UPI ID
	err = svc.DeletePaymentSettings(user1.ID)
	if err != nil {
		t.Fatalf("failed to delete payment settings: %v", err)
	}

	clearedSettings, err := svc.GetPaymentSettings(user1.ID)
	if err != nil {
		t.Fatalf("failed to get cleared settings: %v", err)
	}
	if clearedSettings.IsSet || clearedSettings.UpiID != "" {
		t.Fatalf("expected cleared UPI ID, got '%s'", clearedSettings.UpiID)
	}
}
