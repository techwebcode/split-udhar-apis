package repositories

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
	dsn := fmt.Sprintf("file:memdb_user_device_%d?mode=memory&cache=shared", time.Now().UnixNano())
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

func TestUserDeviceRepository_LifecycleAndOwnership(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserDeviceRepository(db)

	tokenA := "fcm_token_device_A_1234567890"
	tokenB := "fcm_token_device_B_0987654321"

	// 1. Initial registration for User 1
	dev1 := &models.UserDevice{
		UserID:   1,
		Mobile:   "9000000001",
		FCMToken: tokenA,
		Platform: "android",
	}
	if err := repo.SaveToken(dev1); err != nil {
		t.Fatalf("failed to save tokenA: %v", err)
	}

	tokens, err := repo.GetTokensByUserID(1)
	if err != nil || len(tokens) != 1 || tokens[0] != tokenA {
		t.Fatalf("expected [tokenA], got %v, err: %v", tokens, err)
	}

	// 2. Registering a second device for User 1 (Multi-device support)
	dev2 := &models.UserDevice{
		UserID:   1,
		Mobile:   "9000000001",
		FCMToken: tokenB,
		Platform: "ios",
	}
	if err := repo.SaveToken(dev2); err != nil {
		t.Fatalf("failed to save tokenB: %v", err)
	}

	tokens, err = repo.GetTokensByUserID(1)
	if err != nil || len(tokens) != 2 {
		t.Fatalf("expected 2 tokens for multi-device User 1, got %v", tokens)
	}

	// 3. Repeated registration of tokenA does not create duplicates
	dev1Repeat := &models.UserDevice{
		UserID:   1,
		Mobile:   "9000000001",
		FCMToken: tokenA,
		Platform: "android",
	}
	if err := repo.SaveToken(dev1Repeat); err != nil {
		t.Fatalf("repeated save of tokenA failed: %v", err)
	}

	tokens, err = repo.GetTokensByUserID(1)
	if err != nil || len(tokens) != 2 {
		t.Fatalf("expected still 2 tokens after repeated registration, got %v", tokens)
	}

	// 4. Token ownership transfer (e.g. Account switch on same device)
	// tokenA is now logged into by User 2
	devTransfer := &models.UserDevice{
		UserID:   2,
		Mobile:   "9000000002",
		FCMToken: tokenA,
		Platform: "android",
	}
	if err := repo.SaveToken(devTransfer); err != nil {
		t.Fatalf("failed to transfer tokenA to User 2: %v", err)
	}

	// User 1 should now only have tokenB
	tokensUser1, err := repo.GetTokensByUserID(1)
	if err != nil || len(tokensUser1) != 1 || tokensUser1[0] != tokenB {
		t.Fatalf("expected User 1 to have only tokenB, got %v", tokensUser1)
	}

	// User 2 should now have tokenA
	tokensUser2, err := repo.GetTokensByUserID(2)
	if err != nil || len(tokensUser2) != 1 || tokensUser2[0] != tokenA {
		t.Fatalf("expected User 2 to have tokenA, got %v", tokensUser2)
	}

	// 5. Deletion of one token does not affect other tokens
	if err := repo.DeleteToken(1, tokenB); err != nil {
		t.Fatalf("failed to delete tokenB: %v", err)
	}

	tokensUser1AfterDelete, _ := repo.GetTokensByUserID(1)
	if len(tokensUser1AfterDelete) != 0 {
		t.Fatalf("expected User 1 tokens to be empty after deletion, got %v", tokensUser1AfterDelete)
	}

	// User 2 tokenA must still be preserved
	tokensUser2After, _ := repo.GetTokensByUserID(2)
	if len(tokensUser2After) != 1 || tokensUser2After[0] != tokenA {
		t.Fatalf("User 2 tokenA was improperly deleted: %v", tokensUser2After)
	}

	// 6. Resurrecting soft-deleted token: User 1 logs back in on device B
	dev2LoginAgain := &models.UserDevice{
		UserID:   1,
		Mobile:   "9000000001",
		FCMToken: tokenB,
		Platform: "ios",
	}
	if err := repo.SaveToken(dev2LoginAgain); err != nil {
		t.Fatalf("re-registering soft-deleted tokenB failed: %v", err)
	}

	tokensUser1Revived, _ := repo.GetTokensByUserID(1)
	if len(tokensUser1Revived) != 1 || tokensUser1Revived[0] != tokenB {
		t.Fatalf("expected revived tokenB for User 1, got %v", tokensUser1Revived)
	}

	// 7. Invalid token cleanup
	if err := repo.DeleteInvalidTokens([]string{tokenA}); err != nil {
		t.Fatalf("DeleteInvalidTokens failed: %v", err)
	}
	tokensUser2Purged, _ := repo.GetTokensByUserID(2)
	if len(tokensUser2Purged) != 0 {
		t.Fatalf("expected User 2 token to be purged, got %v", tokensUser2Purged)
	}
}
