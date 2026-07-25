package services

import (
	"errors"
	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"

	"gorm.io/gorm"
)

type GroupService struct {
	groupRepo       *repositories.GroupRepository
	transactionRepo *repositories.TransactionRepository
	userRepo        *repositories.UserRepository
}

func NewGroupService(db *gorm.DB) *GroupService {
	return &GroupService{
		groupRepo:       repositories.NewGroupRepository(db),
		transactionRepo: repositories.NewTransactionRepository(db),
		userRepo:        repositories.NewUserRepository(db),
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

	return nil
}

func (s *GroupService) DeleteGroup(groupID uint, userMobile string) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	isMember := false
	for _, m := range group.Members {
		if m.UserMobile == userMobile {
			isMember = true
			break
		}
	}
	if !isMember {
		return errors.New("unauthorized to delete group")
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

	return nil
}
