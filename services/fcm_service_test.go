package services

import (
	"fmt"
	"testing"
	"time"

	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupFCMTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:memdb_fcm_svc_%d?mode=memory", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite test db: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.UserDevice{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	return db
}

func TestFCMService_SaveValidationAndOwnership(t *testing.T) {
	db := setupFCMTestDB(t)
	svc := NewFCMService(db)
	userRepo := repositories.NewUserRepository(db)

	user := &models.User{
		FullName: "Aarav Sharma",
		Email:    "aarav@example.com",
		Mobile:   "9876543210",
	}
	if err := userRepo.Create(user); err != nil {
		t.Fatalf("failed creating user: %v", err)
	}

	// 1. Invalid token (too short)
	err := svc.SaveFCMToken(user.Mobile, user.ID, dto.SaveFCMTokenRequest{
		FCMToken: "short",
		Platform: "android",
	})
	if err == nil {
		t.Fatal("expected error for short fcm_token, got nil")
	}

	// 2. Unsupported platform
	err = svc.SaveFCMToken(user.Mobile, user.ID, dto.SaveFCMTokenRequest{
		FCMToken: "valid_fcm_token_1234567890",
		Platform: "windows_phone",
	})
	if err == nil {
		t.Fatal("expected error for invalid platform, got nil")
	}

	// 3. Valid registration
	err = svc.SaveFCMToken(user.Mobile, user.ID, dto.SaveFCMTokenRequest{
		FCMToken: "valid_fcm_token_1234567890",
		Platform: "android",
	})
	if err != nil {
		t.Fatalf("expected successful save, got: %v", err)
	}

	// 4. Test notification with no tokens for non-existent user
	attempted, _, err := svc.SendTestNotification("9999999999", 999, dto.TestNotificationRequest{})
	if err != nil || attempted != 0 {
		t.Fatalf("expected error/0 tokens for non-existent user, got attempted=%d, err=%v", attempted, err)
	}

	// 5. Unregister token
	err = svc.DeleteFCMToken(user.ID, user.Mobile, "valid_fcm_token_1234567890")
	if err != nil {
		t.Fatalf("expected successful token unregister, got: %v", err)
	}

	// Attempting unregister with blank token
	err = svc.DeleteFCMToken(user.ID, user.Mobile, "")
	if err == nil {
		t.Fatal("expected error when deleting empty token, got nil")
	}
}
