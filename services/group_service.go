package services

import (
	"errors"
	"fmt"
	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"
	"time"

	"gorm.io/gorm"
)

type GroupService struct {
	groupRepo       *repositories.GroupRepository
	transactionRepo *repositories.TransactionRepository
	userRepo        *repositories.UserRepository
	fcmService      *FCMService
}

func NewGroupService(db *gorm.DB) *GroupService {
	return &GroupService{
		groupRepo:       repositories.NewGroupRepository(db),
		transactionRepo: repositories.NewTransactionRepository(db),
		userRepo:        repositories.NewUserRepository(db),
		fcmService:      NewFCMService(db),
	}
}

func (s *GroupService) CreateGroup(creatorMobile string, req dto.CreateGroupRequest) (*models.Group, error) {
	creatorName := "Creator"
	creatorUser, err := s.userRepo.GetByMobile(creatorMobile)
	if err == nil && creatorUser != nil {
		creatorName = creatorUser.FullName
	}

	members := []models.GroupMember{
		{
			UserMobile: creatorMobile,
			UserName:   creatorName,
			Balance:    0,
		},
	}

	// Add additional members
	for _, m := range req.Members {
		if m.Mobile == creatorMobile {
			continue
		}
		name := m.Name
		if name == "" {
			u, err := s.userRepo.GetByMobile(m.Mobile)
			if err == nil && u != nil {
				name = u.FullName
			} else {
				name = m.Mobile
			}
		}

		members = append(members, models.GroupMember{
			UserMobile: m.Mobile,
			UserName:   name,
			Balance:    0,
		})
	}

	group := models.Group{
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   creatorMobile,
		Members:     members,
	}

	err = s.groupRepo.Create(&group)
	if err != nil {
		return nil, err
	}

	return &group, nil
}

func (s *GroupService) GetUserGroups(userMobile string) ([]models.Group, error) {
	return s.groupRepo.GetUserGroups(userMobile)
}

func (s *GroupService) GetGroupDetails(groupID uint, userMobile string) (*models.Group, error) {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return nil, err
	}

	isMember := false
	for _, m := range group.Members {
		if m.UserMobile == userMobile {
			isMember = true
			break
		}
	}

	if !isMember {
		return nil, errors.New("unauthorized access to group")
	}

	return group, nil
}

func (s *GroupService) AddMember(groupID uint, requesterMobile string, memberReq dto.GroupMemberReq) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	// Only the group owner can add members
	if group.CreatedBy != requesterMobile {
		return errors.New("only the group owner can add members to this group")
	}

	// Check if already a member
	for _, m := range group.Members {
		if m.UserMobile == memberReq.Mobile {
			return errors.New("user is already a member of this group")
		}
	}

	name := memberReq.Name
	if name == "" {
		u, err := s.userRepo.GetByMobile(memberReq.Mobile)
		if err == nil && u != nil {
			name = u.FullName
		} else {
			name = memberReq.Mobile
		}
	}

	member := models.GroupMember{
		GroupID:    groupID,
		UserMobile: memberReq.Mobile,
		UserName:   name,
		Balance:    0,
	}

	return s.groupRepo.AddMember(&member)
}

func (s *GroupService) RemoveMember(groupID uint, requesterMobile string, targetMobile string) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	// Only the group owner can remove members
	if group.CreatedBy != requesterMobile {
		return errors.New("only the group owner can remove members from this group")
	}

	// Group owner cannot remove themselves
	if group.CreatedBy == targetMobile {
		return errors.New("group owner cannot be removed from the group")
	}

	if len(group.Members) <= 1 {
		return errors.New("cannot remove the last member of a group")
	}

	return s.groupRepo.RemoveMember(groupID, targetMobile)
}

func (s *GroupService) AddGroupExpense(groupID uint, userMobile string, req dto.AddGroupExpenseRequest) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	if req.Amount <= 0 {
		return errors.New("expense amount must be greater than 0")
	}

	payerName := req.PayerMobile
	payerUser, err := s.userRepo.GetByMobile(req.PayerMobile)
	if err == nil && payerUser != nil {
		payerName = payerUser.FullName
	}

	// Save GroupExpense record
	expense := models.GroupExpense{
		GroupID:     groupID,
		Description: req.Description,
		Amount:      req.Amount,
		PayerMobile: req.PayerMobile,
		PayerName:   payerName,
		CreatedBy:   userMobile,
	}
	_ = s.groupRepo.CreateExpense(&expense)

	splitMobiles := req.SplitWith
	if len(splitMobiles) == 0 {
		// Split among all group members by default
		for _, m := range group.Members {
			splitMobiles = append(splitMobiles, m.UserMobile)
		}
	}

	splitCount := float64(len(splitMobiles))
	perPersonShare := req.Amount / splitCount

	for _, memberMobile := range splitMobiles {
		if memberMobile == req.PayerMobile {
			// Payer paid total amount, gets credit for other members' shares
			_ = s.groupRepo.UpdateMemberBalance(groupID, memberMobile, req.Amount-perPersonShare)
		} else {
			// Member owes perPersonShare
			_ = s.groupRepo.UpdateMemberBalance(groupID, memberMobile, -perPersonShare)

			// Record individual transaction entry
			txn := models.Transaction{
				FromMobile:  req.PayerMobile,
				ToMobile:    memberMobile,
				Type:        models.TransactionGive,
				Amount:      perPersonShare,
				Note:        "Group: " + group.Name + " - " + req.Description,
				ExpenseType: models.ExpenseGroup,
				GroupID:     &groupID,
				CreatedBy:   userMobile,
			}
			_ = s.transactionRepo.Create(&txn)
		}
	}

	// Dispatch FCM push notifications to all group members except creator
	go func() {
		creatorName := userMobile
		creatorUser, err := s.userRepo.GetByMobile(userMobile)
		if err == nil && creatorUser != nil && creatorUser.FullName != "" {
			creatorName = creatorUser.FullName
		}

		var recipientMobiles []string
		for _, m := range group.Members {
			if m.UserMobile != userMobile {
				recipientMobiles = append(recipientMobiles, m.UserMobile)
			}
		}

		if len(recipientMobiles) > 0 {
			title := "New Group Expense"
			body := fmt.Sprintf("%s added ₹%.0f in %s", creatorName, req.Amount, group.Name)

			data := map[string]string{
				"type":        "group",
				"group_id":    fmt.Sprintf("%d", groupID),
				"group_name":  group.Name,
				"description": req.Description,
				"amount":      fmt.Sprintf("%.2f", req.Amount),
			}

			_ = s.fcmService.SendNotificationToUsers(recipientMobiles, title, body, data)
		}
	}()

	return nil
}

func (s *GroupService) DeleteGroup(groupID uint, userMobile string) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	// Only the group owner can delete group
	if group.CreatedBy != userMobile {
		return errors.New("only the group owner can delete this group")
	}

	return s.groupRepo.Delete(groupID)
}

func (s *GroupService) SettleGroup(groupID uint, userMobile string, req dto.SettleGroupRequest) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	if req.Amount <= 0 {
		return errors.New("settlement amount must be greater than 0")
	}

	payerName := req.PayerMobile
	receiverName := req.ReceiverMobile

	for _, m := range group.Members {
		if m.UserMobile == req.PayerMobile {
			payerName = m.UserName
		}
		if m.UserMobile == req.ReceiverMobile {
			receiverName = m.UserName
		}
	}

	// Payer pays receiver: payer balance increases (+amount), receiver balance decreases (-amount)
	_ = s.groupRepo.UpdateMemberBalance(groupID, req.PayerMobile, req.Amount)
	_ = s.groupRepo.UpdateMemberBalance(groupID, req.ReceiverMobile, -req.Amount)

	// Record settlement in group expenses log
	expense := models.GroupExpense{
		GroupID:     groupID,
		Description: "Settlement: " + payerName + " paid " + receiverName,
		Amount:      req.Amount,
		PayerMobile: req.PayerMobile,
		PayerName:   payerName,
		CreatedBy:   userMobile,
	}
	_ = s.groupRepo.CreateExpense(&expense)

	// Record individual settlement transaction
	txn := models.Transaction{
		FromMobile:  req.PayerMobile,
		ToMobile:    req.ReceiverMobile,
		Type:        models.TransactionReceive,
		Amount:      req.Amount,
		Note:        "Group Settlement: " + group.Name,
		ExpenseType: models.ExpenseGroup,
		GroupID:     &groupID,
		Status:      models.StatusSettled,
		CreatedBy:   userMobile,
	}
	_ = s.transactionRepo.Create(&txn)

	// Dispatch FCM notification for Settlement asynchronously
	go func() {
		settlerName := userMobile
		settlerUser, err := s.userRepo.GetByMobile(userMobile)
		if err == nil && settlerUser != nil && settlerUser.FullName != "" {
			settlerName = settlerUser.FullName
		}

		targetMobile := req.ReceiverMobile
		if targetMobile == userMobile {
			targetMobile = req.PayerMobile
		}

		if targetMobile != "" && targetMobile != userMobile {
			title := "Settlement"
			body := fmt.Sprintf("%s settled ₹%.0f", settlerName, req.Amount)

			data := map[string]string{
				"type":       "settlement",
				"group_id":   fmt.Sprintf("%d", groupID),
				"group_name": group.Name,
				"amount":     fmt.Sprintf("%.2f", req.Amount),
			}

			_ = s.fcmService.SendNotificationToUser(targetMobile, title, body, data)
		}
	}()

	return nil
}

func (s *GroupService) DeleteGroupExpense(groupID uint, expenseID uint, userMobile string) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	expense, err := s.groupRepo.GetExpenseByID(expenseID)
	if err != nil {
		return errors.New("expense not found")
	}

	// Only owner/creator of the expense or group creator can delete it
	if expense.CreatedBy != userMobile && expense.PayerMobile != userMobile && group.CreatedBy != userMobile {
		return errors.New("only the owner who added this transaction can delete it")
	}

	// Revert member balances
	if len(group.Members) > 0 && expense.Amount > 0 {
		splitCount := float64(len(group.Members))
		perPersonShare := expense.Amount / splitCount
		for _, m := range group.Members {
			if m.UserMobile == expense.PayerMobile {
				_ = s.groupRepo.UpdateMemberBalance(groupID, m.UserMobile, -(expense.Amount - perPersonShare))
			} else {
				_ = s.groupRepo.UpdateMemberBalance(groupID, m.UserMobile, perPersonShare)
			}
		}
	}

	return s.groupRepo.DeleteExpense(expenseID)
}

func (s *GroupService) UpdateGroupExpense(groupID uint, expenseID uint, userMobile string, req dto.UpdateGroupExpenseRequest) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	expense, err := s.groupRepo.GetExpenseByID(expenseID)
	if err != nil {
		return errors.New("expense not found")
	}

	if expense.CreatedBy != userMobile && expense.PayerMobile != userMobile && group.CreatedBy != userMobile {
		return errors.New("only the owner who added this transaction can edit it")
	}

	if req.Amount <= 0 {
		return errors.New("expense amount must be greater than 0")
	}

	// 1. Revert old expense balances
	if len(group.Members) > 0 && expense.Amount > 0 {
		oldSplitCount := float64(len(group.Members))
		oldShare := expense.Amount / oldSplitCount
		for _, m := range group.Members {
			if m.UserMobile == expense.PayerMobile {
				_ = s.groupRepo.UpdateMemberBalance(groupID, m.UserMobile, -(expense.Amount - oldShare))
			} else {
				_ = s.groupRepo.UpdateMemberBalance(groupID, m.UserMobile, oldShare)
			}
		}
	}

	// 2. Apply new expense info
	newPayer := req.PayerMobile
	if newPayer == "" {
		newPayer = expense.PayerMobile
	}
	payerName := newPayer
	payerUser, err := s.userRepo.GetByMobile(newPayer)
	if err == nil && payerUser != nil {
		payerName = payerUser.FullName
	}

	// Record edit log if description or amount changed
	now := time.Now()
	if expense.Amount != req.Amount || expense.Description != req.Description {
		editLog := models.GroupExpenseEditLog{
			ExpenseID:      expense.ID,
			GroupID:        groupID,
			OldAmount:      expense.Amount,
			NewAmount:      req.Amount,
			OldDescription: expense.Description,
			NewDescription: req.Description,
			EditedBy:       userMobile,
			EditedAt:       now,
		}
		_ = s.groupRepo.CreateExpenseEditLog(&editLog)

		expense.IsEdited = true
		expense.PreviousAmount = expense.Amount
		expense.PreviousDesc = expense.Description
		expense.EditedAt = &now
	}

	expense.Description = req.Description
	expense.Amount = req.Amount
	expense.PayerMobile = newPayer
	expense.PayerName = payerName

	// 3. Apply new member balances
	if len(group.Members) > 0 {
		newSplitCount := float64(len(group.Members))
		newShare := req.Amount / newSplitCount
		for _, m := range group.Members {
			if m.UserMobile == newPayer {
				_ = s.groupRepo.UpdateMemberBalance(groupID, m.UserMobile, req.Amount-newShare)
			} else {
				_ = s.groupRepo.UpdateMemberBalance(groupID, m.UserMobile, -newShare)
			}
		}
	}

	return s.groupRepo.UpdateExpense(expense)
}

func (s *GroupService) GetGroupExpenseEditHistory(expenseID uint) ([]models.GroupExpenseEditLog, error) {
	return s.groupRepo.GetExpenseEditLogs(expenseID)
}
