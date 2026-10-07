package services_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"split-udhar-apis/models"
	"split-udhar-apis/services"
	"split-udhar-apis/services/storage"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite in memory db: %v", err)
	}
	err = db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.GroupMember{},
		&models.GroupExpense{},
		&models.GroupExpenseEditLog{},
		&models.Transaction{},
	)
	if err != nil {
		t.Fatalf("failed to auto migrate: %v", err)
	}
	return db
}

func createTestPNGBytes(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 100, G: 200, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestUserService_ProfileImageFlow(t *testing.T) {
	db := setupTestDB(t)
	memStorage := storage.NewMemoryStorageService("https://pub.r2.dev")
	userSvc := services.NewUserServiceWithStorage(db, memStorage)

	// Create user
	user := models.User{
		FullName: "Test User",
		Mobile:   "9876543210",
		Email:    "test@example.com",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 1. Upload initial profile picture
	pngBytes := createTestPNGBytes(64, 64)
	res, err := userSvc.UploadProfileImage(context.Background(), user.ID, pngBytes)
	if err != nil {
		t.Fatalf("failed to upload profile image: %v", err)
	}
	if res.ProfileImageKey == "" || res.ProfileImageURL == "" {
		t.Fatalf("expected key and URL in response, got %+v", res)
	}
	firstKey := res.ProfileImageKey
	if !memStorage.HasObject(firstKey) {
		t.Fatalf("storage does not have key: %s", firstKey)
	}

	// Verify profile reflects key and URL
	profile, err := userSvc.GetProfile(user.ID)
	if err != nil {
		t.Fatalf("failed to get profile: %v", err)
	}
	if profile.ProfileImageKey != firstKey {
		t.Errorf("expected ProfileImageKey %s, got %s", firstKey, profile.ProfileImageKey)
	}
	if profile.ProfileImageURL != res.ProfileImageURL {
		t.Errorf("expected ProfileImageURL %s, got %s", res.ProfileImageURL, profile.ProfileImageURL)
	}

	// 2. Replace profile picture (Upload second image)
	secondPNG := createTestPNGBytes(128, 128)
	res2, err := userSvc.UploadProfileImage(context.Background(), user.ID, secondPNG)
	if err != nil {
		t.Fatalf("failed to replace profile image: %v", err)
	}
	secondKey := res2.ProfileImageKey
	if secondKey == firstKey {
		t.Errorf("expected new key upon replace, got same key %s", secondKey)
	}
	if !memStorage.HasObject(secondKey) {
		t.Errorf("storage should have new key %s", secondKey)
	}
	// Old key should be deleted from storage
	if memStorage.HasObject(firstKey) {
		t.Errorf("storage should have deleted old key %s after replacement", firstKey)
	}

	// 3. Remove profile picture
	err = userSvc.RemoveProfileImage(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to remove profile image: %v", err)
	}
	if memStorage.HasObject(secondKey) {
		t.Errorf("storage should have deleted key %s upon removal", secondKey)
	}

	// Verify profile has cleared image
	profileAfterRemove, err := userSvc.GetProfile(user.ID)
	if err != nil {
		t.Fatalf("failed to get profile after remove: %v", err)
	}
	if profileAfterRemove.ProfileImageKey != "" {
		t.Errorf("expected empty ProfileImageKey, got %s", profileAfterRemove.ProfileImageKey)
	}
	if profileAfterRemove.ProfileImageURL != "" {
		t.Errorf("expected empty ProfileImageURL, got %s", profileAfterRemove.ProfileImageURL)
	}
}

func TestGroupService_GroupImageFlow(t *testing.T) {
	db := setupTestDB(t)
	memStorage := storage.NewMemoryStorageService("https://pub.r2.dev")
	groupSvc := services.NewGroupService(db)
	groupSvc.SetStorage(memStorage)

	creatorMobile := "9876543210"
	memberMobile := "9876543211"
	nonMemberMobile := "9999999999"

	// Create group with creator and member
	group := models.Group{
		Name:      "Trip Group",
		CreatedBy: creatorMobile,
		Members: []models.GroupMember{
			{UserName: "Creator", UserMobile: creatorMobile},
			{UserName: "Member", UserMobile: memberMobile},
		},
	}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	pngBytes := createTestPNGBytes(80, 80)

	// 1. Non-member attempts upload -> should be rejected
	_, err := groupSvc.UploadGroupImage(context.Background(), group.ID, nonMemberMobile, pngBytes)
	if err == nil {
		t.Fatalf("expected error when non-member uploads group image, got nil")
	}

	// 2. Member uploads group image -> should succeed
	res, err := groupSvc.UploadGroupImage(context.Background(), group.ID, memberMobile, pngBytes)
	if err != nil {
		t.Fatalf("failed to upload group image by member: %v", err)
	}
	if res.GroupImageKey == "" || res.GroupImageURL == "" {
		t.Fatalf("expected key and URL in response, got %+v", res)
	}
	firstKey := res.GroupImageKey
	if !memStorage.HasObject(firstKey) {
		t.Fatalf("storage does not have key: %s", firstKey)
	}

	// 3. Member replaces group image
	secondPNG := createTestPNGBytes(100, 100)
	res2, err := groupSvc.UploadGroupImage(context.Background(), group.ID, creatorMobile, secondPNG)
	if err != nil {
		t.Fatalf("failed to replace group image: %v", err)
	}
	secondKey := res2.GroupImageKey
	if secondKey == firstKey {
		t.Errorf("expected different key on replacement")
	}
	if !memStorage.HasObject(secondKey) {
		t.Errorf("storage should have new key %s", secondKey)
	}
	if memStorage.HasObject(firstKey) {
		t.Errorf("storage should have cleaned up old key %s", firstKey)
	}

	// 4. Non-member attempts delete -> should fail
	err = groupSvc.RemoveGroupImage(context.Background(), group.ID, nonMemberMobile)
	if err == nil {
		t.Fatalf("expected error when non-member removes group image, got nil")
	}

	// 5. Member deletes group image -> should succeed
	err = groupSvc.RemoveGroupImage(context.Background(), group.ID, memberMobile)
	if err != nil {
		t.Fatalf("failed to remove group image: %v", err)
	}
	if memStorage.HasObject(secondKey) {
		t.Errorf("storage should have deleted key %s", secondKey)
	}
}
