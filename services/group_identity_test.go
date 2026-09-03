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

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:memdb_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}

	err = db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.GroupMember{},
		&models.GroupExpense{},
		&models.Transaction{},
		&models.UserDevice{},
	)
	if err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	return db
}

func TestGroupMemberIdentityAndRegistrationLinking(t *testing.T) {
	db := setupTestDB(t)

	groupService := NewGroupService(db)
	userRepo := repositories.NewUserRepository(db)
	groupRepo := repositories.NewGroupRepository(db)

	// 1. Create a registered creator and a registered member (Rahul Kumar, 9876543210)
	creator := models.User{
		FullName:   "Creator Boss",
		Email:      "creator@test.com",
		Mobile:     "9000000001",
		IsVerified: true,
	}
	if err := userRepo.Create(&creator); err != nil {
		t.Fatalf("failed to create creator: %v", err)
	}

	registeredUser := models.User{
		FullName:   "Rahul Kumar",
		Email:      "rahul@test.com",
		Mobile:     "9876543210",
		IsVerified: true,
	}
	if err := userRepo.Create(&registeredUser); err != nil {
		t.Fatalf("failed to create registered user: %v", err)
	}

	// 2. Creator creates a group with:
	// - Registered user (Rahul): client sends contact name "Rahul Office"
	// - Unregistered person (Priya): client sends contact name "Priya Gym"
	createReq := dto.CreateGroupRequest{
		Name:        "Trip Group",
		Description: "Goa trip",
		Members: []dto.GroupMemberReq{
			{
				Mobile: "9876543210",
				Name:   "Rahul Office", // Should be ignored in favor of registered profile name
			},
			{
				Mobile: "9111111111",
				Name:   "Priya Gym", // Should be kept as manual/contact name since unregistered
			},
		},
	}

	group, err := groupService.CreateGroup("9000000001", createReq)
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	// 3. Verify member count and details
	if len(group.Members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(group.Members))
	}

	var rahulMember *models.GroupMember
	var priyaMember *models.GroupMember
	var creatorMember *models.GroupMember

	for i := range group.Members {
		m := &group.Members[i]
		if m.UserMobile == "9000000001" {
			creatorMember = m
		} else if m.UserMobile == "9876543210" {
			rahulMember = m
		} else if m.UserMobile == "9111111111" {
			priyaMember = m
		}
	}

	if creatorMember == nil || creatorMember.UserID == nil || *creatorMember.UserID != creator.ID || creatorMember.UserName != "Creator Boss" {
		t.Errorf("creator member mismatch: %+v", creatorMember)
	}

	// RULE 1 VERIFICATION: Registered user must use database profile name ("Rahul Kumar"), NOT local contact ("Rahul Office")
	if rahulMember == nil {
		t.Fatalf("rahulMember not found")
	}
	if rahulMember.UserID == nil || *rahulMember.UserID != registeredUser.ID {
		t.Errorf("expected rahulMember.UserID to be %d, got %v", registeredUser.ID, rahulMember.UserID)
	}
	if rahulMember.UserName != "Rahul Kumar" {
		t.Errorf("expected rahulMember.UserName to be 'Rahul Kumar', got '%s'", rahulMember.UserName)
	}

	// RULE 2 VERIFICATION: Unregistered member must have UserID = nil and keep stored contact name
	if priyaMember == nil {
		t.Fatalf("priyaMember not found")
	}
	if priyaMember.UserID != nil {
		t.Errorf("expected priyaMember.UserID to be nil, got %v", *priyaMember.UserID)
	}
	if priyaMember.UserName != "Priya Gym" {
		t.Errorf("expected priyaMember.UserName to be 'Priya Gym', got '%s'", priyaMember.UserName)
	}

	// Set an existing balance on Priya to ensure linking preserves financial state
	_ = groupRepo.SetMemberBalance(group.ID, "9111111111", -250.50)

	// 4. RULE 3 VERIFICATION: AFTER REGISTRATION
	// Priya later registers in SplitUdhar with profile name "Priya Sharma"
	priyaUser := models.User{
		FullName:   "Priya Sharma",
		Email:      "priya@test.com",
		Mobile:     "9111111111",
		IsVerified: true,
	}
	if err := userRepo.Create(&priyaUser); err != nil {
		t.Fatalf("failed to register priya: %v", err)
	}

	// Link user to existing group members
	err = groupRepo.LinkUserToGroupMembers(&priyaUser)
	if err != nil {
		t.Fatalf("LinkUserToGroupMembers failed: %v", err)
	}

	// Reload group details
	reloadedGroup, err := groupService.GetGroupDetails(group.ID, "9000000001")
	if err != nil {
		t.Fatalf("GetGroupDetails failed: %v", err)
	}

	var reloadedPriya *models.GroupMember
	for i := range reloadedGroup.Members {
		if reloadedGroup.Members[i].UserMobile == "9111111111" {
			reloadedPriya = &reloadedGroup.Members[i]
		}
	}

	if reloadedPriya == nil {
		t.Fatalf("reloadedPriya not found")
	}
	// Verify Priya is now linked to real user_id
	if reloadedPriya.UserID == nil || *reloadedPriya.UserID != priyaUser.ID {
		t.Errorf("expected reloadedPriya.UserID to be %d, got %v", priyaUser.ID, reloadedPriya.UserID)
	}
	// Verify display name is updated to SplitUdhar profile name
	if reloadedPriya.UserName != "Priya Sharma" {
		t.Errorf("expected reloadedPriya.UserName to be 'Priya Sharma', got '%s'", reloadedPriya.UserName)
	}
	// Verify financial balance is completely preserved
	if reloadedPriya.Balance != -250.50 {
		t.Errorf("expected reloadedPriya.Balance to remain -250.50, got %f", reloadedPriya.Balance)
	}

	// Verify no duplicate members created
	if len(reloadedGroup.Members) != 3 {
		t.Errorf("expected member count to stay 3, got %d", len(reloadedGroup.Members))
	}
}
