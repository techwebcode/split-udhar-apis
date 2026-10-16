package migrations

import (
	"fmt"
	"testing"
	"time"

	"split-udhar-apis/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:memdb_migration_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite test db: %v", err)
	}

	if err := db.AutoMigrate(&models.UserDevice{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	return db
}

func TestPreflightCheckUserDevices(t *testing.T) {
	db := setupTestDB(t)

	// Seed records
	dev1 := &models.UserDevice{
		UserID:   1,
		Mobile:   "9876543210",
		FCMToken: "fcm_token_1",
		Platform: "android",
	}
	dev2 := &models.UserDevice{
		UserID:   2,
		Mobile:   "9123456789",
		FCMToken: "fcm_token_2",
		Platform: "ios",
	}
	db.Create(dev1)
	db.Create(dev2)

	report, err := PreflightCheckUserDevices(db)
	if err != nil {
		t.Fatalf("PreflightCheckUserDevices failed: %v", err)
	}

	if report.TotalDevices != 2 {
		t.Errorf("expected 2 devices, got %d", report.TotalDevices)
	}
	if report.OversizedCount != 0 {
		t.Errorf("expected 0 oversized tokens, got %d", report.OversizedCount)
	}
	if report.DuplicateCount != 0 {
		t.Errorf("expected 0 duplicate tokens, got %d", report.DuplicateCount)
	}
}

func TestPreflightCheckUserDevices_DetectsOversized(t *testing.T) {
	db := setupTestDB(t)

	// Generate a 260-character token
	longToken := ""
	for len(longToken) < 260 {
		longToken += "abcdef123456"
	}

	dev := &models.UserDevice{
		UserID:   1,
		Mobile:   "9876543210",
		FCMToken: longToken,
		Platform: "android",
	}
	db.Create(dev)

	report, err := PreflightCheckUserDevices(db)
	if err != nil {
		t.Fatalf("PreflightCheckUserDevices failed: %v", err)
	}

	if report.OversizedCount != 1 {
		t.Errorf("expected 1 oversized token, got %d", report.OversizedCount)
	}
	if report.MaxTokenLength < 260 {
		t.Errorf("expected max token length >= 260, got %d", report.MaxTokenLength)
	}
}
