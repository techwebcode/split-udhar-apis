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

func TestUserDeviceRepository_RecipientLookupByCanonicalMobile(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserDeviceRepository(db)

	token1 := "fcm_token_device_canonical_1"
	token2 := "fcm_token_device_canonical_2"

	// Register device with 10-digit mobile
	dev1 := &models.UserDevice{
		UserID:   10,
		Mobile:   "9876543210",
		FCMToken: token1,
		Platform: "android",
	}
	if err := repo.SaveToken(dev1); err != nil {
		t.Fatalf("failed to save dev1: %v", err)
	}

	// Register device with +91 prefixed mobile
	dev2 := &models.UserDevice{
		UserID:   20,
		Mobile:   "+919123456789",
		FCMToken: token2,
		Platform: "ios",
	}
	if err := repo.SaveToken(dev2); err != nil {
		t.Fatalf("failed to save dev2: %v", err)
	}

	// Test 1: Query by +91 format when stored as 10-digit, userID = 0
	tokens, err := repo.GetTokensByMobile("+919876543210", 0)
	if err != nil || len(tokens) != 1 || tokens[0] != token1 {
		t.Fatalf("expected [token1] when querying +919876543210 with userID=0, got %v, err: %v", tokens, err)
	}

	// Test 2: Query by 10-digit format when stored as +91, userID = 0
	tokens, err = repo.GetTokensByMobile("9123456789", 0)
	if err != nil || len(tokens) != 1 || tokens[0] != token2 {
		t.Fatalf("expected [token2] when querying 9123456789 with userID=0, got %v, err: %v", tokens, err)
	}

	// Test 3: Query with resolved userID > 0 prefers user ID lookup
	tokens, err = repo.GetTokensByMobile("9876543210", 10)
	if err != nil || len(tokens) != 1 || tokens[0] != token1 {
		t.Fatalf("expected [token1] by userID, got %v, err: %v", tokens, err)
	}

	// Test 4: Query bulk mobiles with and without prefix
	bulkTokens, err := repo.GetTokensByMobiles([]string{"+919876543210", "9123456789"})
	if err != nil || len(bulkTokens) != 2 {
		t.Fatalf("expected 2 tokens from GetTokensByMobiles, got %v, err: %v", bulkTokens, err)
	}

	// Test 5: Soft-deleted token should not be returned
	if err := repo.DeleteToken(10, token1); err != nil {
		t.Fatalf("failed to delete token1: %v", err)
	}
	tokensAfterDelete, err := repo.GetTokensByMobile("+919876543210", 0)
	if err != nil || len(tokensAfterDelete) != 0 {
		t.Fatalf("expected 0 tokens after delete, got %v, err: %v", tokensAfterDelete, err)
	}

	// Test 6: Ambiguous, short, or invalid mobile format safely returns empty tokens
	ambiguousTokens, err := repo.GetTokensByMobile("123", 0)
	if err != nil || len(ambiguousTokens) != 0 {
		t.Fatalf("expected 0 tokens for short/ambiguous mobile, got %v, err: %v", ambiguousTokens, err)
	}

	emptyTokens, err := repo.GetTokensByMobile("", 0)
	if err != nil || len(emptyTokens) != 0 {
		t.Fatalf("expected 0 tokens for empty mobile, got %v, err: %v", emptyTokens, err)
	}

	bulkInvalidTokens, err := repo.GetTokensByMobiles([]string{"123", ""})
	if err != nil || len(bulkInvalidTokens) != 0 {
		t.Fatalf("expected 0 tokens for bulk invalid mobiles, got %v, err: %v", bulkInvalidTokens, err)
	}
}
