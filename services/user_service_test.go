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

func setupUserTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:memdb_user_svc_%d?mode=memory", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite test db: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.Group{}, &models.GroupMember{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	return db
}

func TestUserService_PreferredCurrency(t *testing.T) {
	db := setupUserTestDB(t)
	svc := NewUserService(db)
	userRepo := repositories.NewUserRepository(db)

	// 1. Verify new user defaults to INR
	user := &models.User{
		FullName: "Rohan Verma",
		Email:    "rohan@example.com",
		Mobile:   "9876543210",
	}
	if err := userRepo.Create(user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	profile, err := svc.GetProfile(user.ID)
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}
	if profile.PreferredCurrency != models.CurrencyINR {
		t.Errorf("expected default currency INR, got %s", profile.PreferredCurrency)
	}

	// 2. Test updating to valid international currencies
	supported := []string{"USD", "EUR", "GBP", "AED", "CAD", "AUD", "SGD", "JPY", "CHF", "INR"}
	for _, curr := range supported {
		err := svc.UpdateProfile(user.ID, dto.UpdateProfileRequest{
			PreferredCurrency: curr,
		})
		if err != nil {
			t.Errorf("expected success updating to %s, got error: %v", curr, err)
		}
		updated, _ := svc.GetProfile(user.ID)
		if updated.PreferredCurrency != curr {
			t.Errorf("expected currency %s, got %s", curr, updated.PreferredCurrency)
		}
	}

	// 3. Test updating to lowercase code gets normalized
	err = svc.UpdateProfile(user.ID, dto.UpdateProfileRequest{
		PreferredCurrency: "usd",
	})
	if err != nil {
		t.Fatalf("failed to update with lowercase usd: %v", err)
	}
	updated, _ := svc.GetProfile(user.ID)
	if updated.PreferredCurrency != "USD" {
		t.Errorf("expected USD, got %s", updated.PreferredCurrency)
	}

	// 4. Test unsupported currency rejection
	err = svc.UpdateProfile(user.ID, dto.UpdateProfileRequest{
		PreferredCurrency: "XYZ",
	})
	if err == nil {
		t.Errorf("expected error for unsupported currency XYZ, got nil")
	}

	// Ensure value remains USD
	updated, _ = svc.GetProfile(user.ID)
	if updated.PreferredCurrency != "USD" {
		t.Errorf("expected currency to remain USD, got %s", updated.PreferredCurrency)
	}

	// 5. Test empty currency does not overwrite existing preference
	err = svc.UpdateProfile(user.ID, dto.UpdateProfileRequest{
		FullName: "Rohan Updated",
	})
	if err != nil {
		t.Fatalf("update profile failed: %v", err)
	}
	updated, _ = svc.GetProfile(user.ID)
	if updated.PreferredCurrency != "USD" {
		t.Errorf("expected currency to remain USD after updating name, got %s", updated.PreferredCurrency)
	}
}
