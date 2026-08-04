package services

import (
	"errors"
	"fmt"
	"math"
	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"
	"split-udhar-apis/utils"
	"strings"
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

// resolveMemberMobile maps a caller-supplied number onto the exact string stored
// for that member. Members are often added from the phonebook as "+9198...",
// while the JWT carries a bare 10-digit number; balance updates match
// user_mobile exactly, so an unresolved number would silently update no rows.
func resolveMemberMobile(group *models.Group, mobile string) (string, bool) {
	for _, m := range group.Members {
		if utils.SameMobile(m.UserMobile, mobile) {
			return m.UserMobile, true
		}
	}
	return "", false
}

// isGroupMember reports whether mobile belongs to the group, comparing numbers
// in normalized form.
func isGroupMember(group *models.Group, mobile string) bool {
	_, ok := resolveMemberMobile(group, mobile)
	return ok
}

// splitMembersFor returns the member mobiles an expense was divided across.
// Expenses recorded before the split set was persisted fall back to every
// current member, which is the behaviour those rows were created under.
func splitMembersFor(group *models.Group, expense *models.GroupExpense) []string {
	if expense.SplitWith != "" {
		parts := strings.Split(expense.SplitWith, ",")
		mobiles := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				mobiles = append(mobiles, p)
			}
		}
		if len(mobiles) > 0 {
			return mobiles
		}
	}

	mobiles := make([]string, 0, len(group.Members))
	for _, m := range group.Members {
		mobiles = append(mobiles, m.UserMobile)
	}
	return mobiles
}

// expenseBalanceDeltas returns the per-member balance change for an expense: the
// payer is credited the full amount, and everyone it was split across is debited
// an equal share. Pass sign -1 to produce the reverting deltas. The returned
// deltas always sum to zero, so applying and then reverting is a no-op.
func expenseBalanceDeltas(
	payerMobile string,
	splitMobiles []string,
	amount float64,
	sign float64,
) map[string]float64 {
	deltas := make(map[string]float64)
	if amount <= 0 || len(splitMobiles) == 0 {
		return deltas
	}

	share := amount / float64(len(splitMobiles))

	deltas[payerMobile] += sign * amount
	for _, memberMobile := range splitMobiles {
		deltas[memberMobile] -= sign * share
	}

	return deltas
}

// createSplitTransactions records one personal-ledger entry per member the
// expense was split across, each tagged with the originating expense so it can
// be cleaned up if that expense is later edited or removed.
func (s *GroupService) createSplitTransactions(
	group *models.Group,
	expense *models.GroupExpense,
	splitMobiles []string,
	createdBy string,
) {
	if expense.Amount <= 0 || len(splitMobiles) == 0 {
		return
	}

	perPersonShare := expense.Amount / float64(len(splitMobiles))

	// Without an explicit date these rows sort to the epoch, pushing group
	// expenses to the bottom of every transaction_date DESC listing.
	txnDate := time.Now()
	if expense.ExpenseDate != nil {
		txnDate = *expense.ExpenseDate
	}

	for _, memberMobile := range splitMobiles {
		if utils.SameMobile(memberMobile, expense.PayerMobile) {
			continue
		}

		txn := models.Transaction{
			FromMobile:      expense.PayerMobile,
			ToMobile:        memberMobile,
			Type:            models.TransactionGive,
			Amount:          perPersonShare,
			Note:            "Group: " + group.Name + " - " + expense.Description,
			ExpenseType:     models.ExpenseGroup,
			GroupID:         &group.ID,
			GroupExpenseID:  &expense.ID,
			Status:          models.StatusPending,
			TransactionDate: txnDate,
			CreatedBy:       createdBy,
		}
		_ = s.transactionRepo.Create(&txn)
	}
}

// settlementBalanceDeltas returns the balance change for a settlement: the payer
// hands money directly to the receiver, with no split involved.
func settlementBalanceDeltas(payerMobile, receiverMobile string, amount, sign float64) map[string]float64 {
	deltas := make(map[string]float64)
	if amount <= 0 || payerMobile == "" || receiverMobile == "" {
		return deltas
	}

	deltas[payerMobile] += sign * amount
	deltas[receiverMobile] -= sign * amount

	return deltas
}

// balanceDeltasFor computes the balance impact of an already-recorded row,
// dispatching on its kind. ok is false for a legacy settlement with no recorded
// receiver, which cannot be replayed or reverted safely.
func balanceDeltasFor(
	group *models.Group,
	expense *models.GroupExpense,
	sign float64,
) (map[string]float64, bool) {
	if expense.Kind == models.GroupExpenseKindSettlement {
		if expense.ReceiverMobile == "" {
			return nil, false
		}
		return settlementBalanceDeltas(
			expense.PayerMobile, expense.ReceiverMobile, expense.Amount, sign,
		), true
	}

	return expenseBalanceDeltas(
		expense.PayerMobile, splitMembersFor(group, expense), expense.Amount, sign,
	), true
}

// persistDeltas writes a set of balance changes to the group's members.
func (s *GroupService) persistDeltas(groupID uint, deltas map[string]float64) {
	for memberMobile, delta := range deltas {
		if delta == 0 {
			continue
		}
		_ = s.groupRepo.UpdateMemberBalance(groupID, memberMobile, delta)
	}
}

// applyExpenseBalances persists the deltas from expenseBalanceDeltas.
func (s *GroupService) applyExpenseBalances(
	groupID uint,
	payerMobile string,
	splitMobiles []string,
	amount float64,
	sign float64,
) {
	s.persistDeltas(groupID, expenseBalanceDeltas(payerMobile, splitMobiles, amount, sign))
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
		if utils.SameMobile(m.Mobile, creatorMobile) {
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

	if !isGroupMember(group, userMobile) {
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
	if !utils.SameMobile(group.CreatedBy, requesterMobile) {
		return errors.New("only the group owner can add members to this group")
	}

	// Check if already a member
	for _, m := range group.Members {
		if utils.SameMobile(m.UserMobile, memberReq.Mobile) {
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
	if !utils.SameMobile(group.CreatedBy, requesterMobile) {
		return errors.New("only the group owner can remove members from this group")
	}

	// Group owner cannot remove themselves
	if utils.SameMobile(group.CreatedBy, targetMobile) {
		return errors.New("group owner cannot be removed from the group")
	}

	if len(group.Members) <= 1 {
		return errors.New("cannot remove the last member of a group")
	}

	// Delete matches user_mobile exactly, so use the stored form of the number.
	storedMobile, ok := resolveMemberMobile(group, targetMobile)
	if !ok {
		return errors.New("user is not a member of this group")
	}

	return s.groupRepo.RemoveMember(groupID, storedMobile)
}

func (s *GroupService) AddGroupExpense(groupID uint, userMobile string, req dto.AddGroupExpenseRequest) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	if !isGroupMember(group, userMobile) {
		return errors.New("only group members can add expenses to this group")
	}

	if req.Amount <= 0 {
		return errors.New("expense amount must be greater than 0")
	}

	// Canonicalise to the stored member string so balance updates match a row.
	payerMobile, ok := resolveMemberMobile(group, req.PayerMobile)
	if !ok {
		return errors.New("the payer must be a member of this group")
	}

	payerName := payerMobile
	payerUser, err := s.userRepo.GetByMobile(payerMobile)
	if err == nil && payerUser != nil {
		payerName = payerUser.FullName
	}

	expDate := time.Now()
	if req.ExpenseDate != "" {
		if parsedDate, err := time.Parse("2006-01-02", req.ExpenseDate); err == nil {
			expDate = parsedDate
		} else if parsedDate, err := time.Parse(time.RFC3339, req.ExpenseDate); err == nil {
			expDate = parsedDate
		}
	}

	// Resolve who the expense is split across, rejecting anyone outside the group
	// and normalising each entry to its stored form.
	var splitMobiles []string
	if len(req.SplitWith) == 0 {
		// Split among all group members by default
		for _, m := range group.Members {
			splitMobiles = append(splitMobiles, m.UserMobile)
		}
	} else {
		for _, memberMobile := range req.SplitWith {
			resolved, ok := resolveMemberMobile(group, memberMobile)
			if !ok {
				return errors.New("cannot split an expense with a non-member: " + memberMobile)
			}
			splitMobiles = append(splitMobiles, resolved)
		}
	}

	// Save GroupExpense record, retaining the split set so edits and deletes can
	// revert exactly what was applied.
	expense := models.GroupExpense{
		GroupID:     groupID,
		Description: req.Description,
		Amount:      req.Amount,
		PayerMobile: payerMobile,
		PayerName:   payerName,
		CreatedBy:   userMobile,
		ExpenseDate: &expDate,
		SplitWith:   strings.Join(splitMobiles, ","),
		Kind:        models.GroupExpenseKindExpense,
	}
	if err := s.groupRepo.CreateExpense(&expense); err != nil {
		return err
	}

	s.applyExpenseBalances(groupID, payerMobile, splitMobiles, req.Amount, 1)
	s.createSplitTransactions(group, &expense, splitMobiles, userMobile)

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
	if !utils.SameMobile(group.CreatedBy, userMobile) {
		return errors.New("only the group owner can delete this group")
	}

	return s.groupRepo.Delete(groupID)
}

func (s *GroupService) SettleGroup(groupID uint, userMobile string, req dto.SettleGroupRequest) error {
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	if !isGroupMember(group, userMobile) {
		return errors.New("only group members can settle up in this group")
	}

	if req.Amount <= 0 {
		return errors.New("settlement amount must be greater than 0")
	}

	payerMobile, payerOK := resolveMemberMobile(group, req.PayerMobile)
	receiverMobile, receiverOK := resolveMemberMobile(group, req.ReceiverMobile)
	if !payerOK || !receiverOK {
		return errors.New("both parties to a settlement must be members of this group")
	}

	if utils.SameMobile(payerMobile, receiverMobile) {
		return errors.New("cannot settle a payment with yourself")
	}

	payerName := payerMobile
	receiverName := receiverMobile

	for _, m := range group.Members {
		if m.UserMobile == payerMobile {
			payerName = m.UserName
		}
		if m.UserMobile == receiverMobile {
			receiverName = m.UserName
		}
	}

	// Payer pays receiver: payer balance increases (+amount), receiver balance decreases (-amount)
	_ = s.groupRepo.UpdateMemberBalance(groupID, payerMobile, req.Amount)
	_ = s.groupRepo.UpdateMemberBalance(groupID, receiverMobile, -req.Amount)

	// Record settlement in group expenses log
	settledAt := time.Now()
	expense := models.GroupExpense{
		GroupID:        groupID,
		Description:    "Settlement: " + payerName + " paid " + receiverName,
		Amount:         req.Amount,
		PayerMobile:    payerMobile,
		PayerName:      payerName,
		CreatedBy:      userMobile,
		ExpenseDate:    &settledAt,
		Kind:           models.GroupExpenseKindSettlement,
		ReceiverMobile: receiverMobile,
	}
	_ = s.groupRepo.CreateExpense(&expense)

	// Record individual settlement transaction
	txn := models.Transaction{
		FromMobile:  payerMobile,
		ToMobile:    receiverMobile,
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

		targetMobile := receiverMobile
		if targetMobile == userMobile {
			targetMobile = payerMobile
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
	if !utils.SameMobile(expense.CreatedBy, userMobile) && !utils.SameMobile(expense.PayerMobile, userMobile) && !utils.SameMobile(group.CreatedBy, userMobile) {
		return errors.New("only the owner who added this transaction can delete it")
	}

	// Revert using the row's own semantics: an expense unwinds across its
	// original split set, a settlement unwinds between its two parties.
	deltas, ok := balanceDeltasFor(group, expense, -1)
	if !ok {
		return errors.New("this settlement predates receiver tracking and cannot be reverted automatically")
	}
	s.persistDeltas(groupID, deltas)

	// Drop the personal-ledger rows this expense generated.
	_ = s.transactionRepo.DeleteByGroupExpense(expenseID)

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

	if !utils.SameMobile(expense.CreatedBy, userMobile) && !utils.SameMobile(expense.PayerMobile, userMobile) && !utils.SameMobile(group.CreatedBy, userMobile) {
		return errors.New("only the owner who added this transaction can edit it")
	}

	if req.Amount <= 0 {
		return errors.New("expense amount must be greater than 0")
	}

	// A settlement is a payment between two people, not a split, so the expense
	// edit path would apply the wrong balance maths to it.
	if expense.Kind == models.GroupExpenseKindSettlement {
		return errors.New("settlements cannot be edited; delete it and record a new one instead")
	}

	// Resolve and validate the new payer *before* touching any balances, so a
	// rejected edit cannot leave the group half-reverted.
	newPayer := req.PayerMobile
	if newPayer == "" {
		newPayer = expense.PayerMobile
	}
	if !isGroupMember(group, newPayer) {
		return errors.New("the payer must be a member of this group")
	}

	// 1. Revert old expense balances across the original split set
	splitMobiles := splitMembersFor(group, expense)
	s.applyExpenseBalances(groupID, expense.PayerMobile, splitMobiles, expense.Amount, -1)

	// 2. Apply new expense info
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

	// 3. Re-apply across the same split set the expense was created with
	s.applyExpenseBalances(groupID, newPayer, splitMobiles, req.Amount, 1)

	if err := s.groupRepo.UpdateExpense(expense); err != nil {
		return err
	}

	// Rebuild the personal-ledger rows so they reflect the new amount and payer.
	_ = s.transactionRepo.DeleteByGroupExpense(expenseID)
	s.createSplitTransactions(group, expense, splitMobiles, userMobile)

	return nil
}

func (s *GroupService) GetGroupExpenseEditHistory(expenseID uint) ([]models.GroupExpenseEditLog, error) {
	return s.groupRepo.GetExpenseEditLogs(expenseID)
}

// -------------------- MAINTENANCE --------------------

// GroupBalanceDrift reports the gap between a member's stored balance and the
// balance implied by replaying the group's recorded expenses and settlements.
type GroupBalanceDrift struct {
	GroupID    uint
	GroupName  string
	UserMobile string
	Stored     float64
	Expected   float64
}

func (d GroupBalanceDrift) Delta() float64 { return d.Expected - d.Stored }

// GroupRecomputeReport summarises a balance recompute run.
type GroupRecomputeReport struct {
	GroupsScanned int
	Drifts        []GroupBalanceDrift
	// Skipped lists groups left untouched because at least one row could not be
	// replayed; correcting part of a group would make its balances worse.
	Skipped []string
	Applied bool
}

// RecomputeGroupBalances rebuilds every member balance by replaying the group's
// expenses and settlements from scratch, and reports where the stored value has
// drifted. Balances are only written when apply is true.
//
// Groups containing a legacy settlement with no recorded receiver are skipped
// rather than partially corrected.
func (s *GroupService) RecomputeGroupBalances(apply bool) (*GroupRecomputeReport, error) {
	groups, err := s.groupRepo.GetAllWithRelations()
	if err != nil {
		return nil, err
	}

	report := &GroupRecomputeReport{GroupsScanned: len(groups), Applied: apply}

	for i := range groups {
		group := &groups[i]

		expected := make(map[string]float64, len(group.Members))
		for _, m := range group.Members {
			expected[m.UserMobile] = 0
		}

		unreplayable := false
		for j := range group.Expenses {
			expense := &group.Expenses[j]

			deltas, ok := balanceDeltasFor(group, expense, 1)
			if !ok {
				unreplayable = true
				break
			}
			for mobile, delta := range deltas {
				// Fold onto the stored member string so a differently formatted
				// payer does not create a phantom entry.
				key := mobile
				if resolved, found := resolveMemberMobile(group, mobile); found {
					key = resolved
				}
				expected[key] += delta
			}
		}

		if unreplayable {
			report.Skipped = append(report.Skipped, fmt.Sprintf(
				"group %d (%s): settlement without a recorded receiver", group.ID, group.Name,
			))
			continue
		}

		for _, m := range group.Members {
			want := expected[m.UserMobile]
			if math.Abs(want-m.Balance) < 0.005 {
				continue
			}

			report.Drifts = append(report.Drifts, GroupBalanceDrift{
				GroupID:    group.ID,
				GroupName:  group.Name,
				UserMobile: m.UserMobile,
				Stored:     m.Balance,
				Expected:   want,
			})

			if apply {
				if err := s.groupRepo.SetMemberBalance(group.ID, m.UserMobile, want); err != nil {
					return report, fmt.Errorf(
						"group %d member %s: %w", group.ID, m.UserMobile, err,
					)
				}
			}
		}
	}

	return report, nil
}
