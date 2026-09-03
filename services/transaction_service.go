package services

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"
	"split-udhar-apis/utils"

	"gorm.io/gorm"
)

type TransactionService struct {
	transactionRepo *repositories.TransactionRepository
	userRepo        *repositories.UserRepository
	fcmService      *FCMService
}

func NewTransactionService(db *gorm.DB) *TransactionService {
	return &TransactionService{
		transactionRepo: repositories.NewTransactionRepository(db),
		userRepo:        repositories.NewUserRepository(db),
		fcmService:      NewFCMService(db),
	}
}

func (s *TransactionService) CreateTransaction(userMobile string, req dto.CreateTransactionRequest) error {

	// Prevent self transaction
	if userMobile == req.Mobile {
		return errors.New("you cannot create a transaction with yourself")
	}

	// Verify logged-in user exists
	currentUser, err := s.userRepo.GetByMobile(userMobile)
	if err != nil {
		return err
	}
	if currentUser == nil {
		return errors.New("user not found")
	}

	transaction := models.Transaction{
		ReferenceID:     fmt.Sprintf("TXN-%d-%04d", time.Now().UnixNano(), rand.Intn(10000)),
		Amount:          req.Amount,
		Note:            req.Note,
		ContactName:     req.ContactName,
		Type:            models.TransactionType(req.Type),
		ExpenseType:     models.ExpensePersonal,
		Status:          models.StatusPending,
		TransactionDate: time.Now(),
		CreatedBy:       currentUser.Email,
	}

	switch req.Type {

	case string(models.TransactionGive):
		transaction.FromMobile = userMobile
		transaction.ToMobile = req.Mobile

	case string(models.TransactionReceive):
		transaction.FromMobile = req.Mobile
		transaction.ToMobile = userMobile

	default:
		return errors.New("invalid transaction type")
	}

	if err := s.transactionRepo.Create(&transaction); err != nil {
		return err
	}

	// Dispatch FCM notification to target recipient asynchronously
	go func() {
		senderName := userMobile
		if currentUser != nil && currentUser.FullName != "" {
			senderName = currentUser.FullName
		}

		title := "New Transaction"
		body := fmt.Sprintf("%s added ₹%.0f.", senderName, req.Amount)

		data := map[string]string{
			"type":           "transaction",
			"transaction_id": fmt.Sprintf("%d", transaction.ID),
			"mobile":         userMobile,
			"amount":         fmt.Sprintf("%.2f", req.Amount),
		}

		_ = s.fcmService.SendNotificationToUser(req.Mobile, title, body, data)
	}()

	return nil
}

func (s *TransactionService) UpdateTransaction(
	transactionID uint,
	userMobile string,
	req dto.UpdateTransactionRequest,
) error {

	transaction, err := s.transactionRepo.GetByID(transactionID)
	if err != nil {
		return err
	}

	// Only transaction creator/owner can update
	currentUser, _ := s.userRepo.GetByMobile(userMobile)
	userEmail := ""
	if currentUser != nil {
		userEmail = currentUser.Email
	}

	isOwner := false
	if transaction.CreatedBy != "" {
		isOwner = (transaction.CreatedBy == userMobile || (userEmail != "" && transaction.CreatedBy == userEmail))
	} else {
		// Fallback for legacy transactions where CreatedBy was empty
		isOwner = (transaction.Type == models.TransactionGive && transaction.FromMobile == userMobile) ||
			(transaction.Type == models.TransactionReceive && transaction.ToMobile == userMobile)
	}

	if !isOwner {
		return errors.New("only the transaction creator can edit this transaction")
	}

	oldAmount := transaction.Amount
	oldNote := transaction.Note
	oldType := string(transaction.Type)
	newType := req.Type

	typeChanged := newType != "" && (newType == "give" || newType == "receive") && newType != oldType

	if typeChanged {
		transaction.Type = models.TransactionType(newType)
		if newType == string(models.TransactionReceive) && transaction.FromMobile == userMobile {
			contactMobile := transaction.ToMobile
			transaction.FromMobile = contactMobile
			transaction.ToMobile = userMobile
		} else if newType == string(models.TransactionGive) && transaction.ToMobile == userMobile {
			contactMobile := transaction.FromMobile
			transaction.FromMobile = userMobile
			transaction.ToMobile = contactMobile
		}
	}

	if oldAmount != req.Amount || oldNote != req.Note || typeChanged {
		now := time.Now()
		transaction.IsEdited = true
		transaction.PreviousAmount = oldAmount
		transaction.PreviousNote = oldNote
		transaction.EditedAt = &now
		transaction.UpdatedBy = userMobile

		logNote := req.Note
		if typeChanged {
			logNote = fmt.Sprintf("[%s ➔ %s] %s", oldType, newType, req.Note)
		}

		editLog := models.TransactionEditLog{
			TransactionID: transaction.ID,
			OldAmount:     oldAmount,
			NewAmount:     req.Amount,
			OldNote:       oldNote,
			NewNote:       logNote,
			EditedBy:      userMobile,
			EditedAt:      now,
		}
		if err := s.transactionRepo.CreateEditLog(&editLog); err != nil {
			log.Printf("[EDIT LOG CREATION ERROR] Transaction ID %d: %v", transaction.ID, err)
		} else {
			log.Printf("[EDIT LOG CREATED] Log ID %d created for Transaction ID %d", editLog.ID, transaction.ID)
		}
	}

	transaction.Amount = req.Amount
	transaction.Note = req.Note

	return s.transactionRepo.Update(transaction)
}

func (s *TransactionService) GetAllTransactions(userMobile string) ([]models.Transaction, error) {
	return s.transactionRepo.GetUserTransactions(userMobile)
}

func (s *TransactionService) GetTransactionEditHistory(transactionID uint) ([]models.TransactionEditLog, error) {
	return s.transactionRepo.GetEditLogs(transactionID)
}

func (s *TransactionService) GetTransactionsByMobile(
	userMobile string,
	contactMobile string,
) ([]models.Transaction, error) {

	return s.transactionRepo.GetTransactionsByMobile(
		userMobile,
		contactMobile,
	)
}

func (s *TransactionService) DeleteTransaction(
	id uint,
	userMobile string,
) error {

	transaction, err := s.transactionRepo.GetByID(id)

	if err != nil {
		return err
	}

	// Only transaction creator/owner can delete
	currentUser, _ := s.userRepo.GetByMobile(userMobile)
	userEmail := ""
	if currentUser != nil {
		userEmail = currentUser.Email
	}

	isOwner := false
	if transaction.CreatedBy != "" {
		isOwner = (transaction.CreatedBy == userMobile || (userEmail != "" && transaction.CreatedBy == userEmail))
	} else {
		// Fallback for legacy transactions where CreatedBy was empty
		isOwner = (transaction.Type == models.TransactionGive && transaction.FromMobile == userMobile) ||
			(transaction.Type == models.TransactionReceive && transaction.ToMobile == userMobile)
	}

	if !isOwner {
		return errors.New("only the transaction creator can delete this transaction")
	}

	now := time.Now()
	// Record deletion audit entry in TransactionEditLog
	editLog := models.TransactionEditLog{
		TransactionID: transaction.ID,
		OldAmount:     transaction.Amount,
		NewAmount:     0,
		OldNote:       transaction.Note,
		NewNote:       "[DELETED] Transaction deleted",
		EditedBy:      userMobile,
		EditedAt:      now,
	}
	if err := s.transactionRepo.CreateEditLog(&editLog); err != nil {
		log.Printf("[DELETE LOG ERROR] Transaction ID %d: %v", id, err)
	}

	// Mark as deleted on transaction record to maintain full history
	transaction.IsDeleted = true
	transaction.IsEdited = true
	transaction.UpdatedBy = userMobile
	transaction.EditedAt = &now

	return s.transactionRepo.Update(transaction)
}

type DashboardResponse struct {
	TotalGiven        float64                          `json:"total_given"`
	TotalReceived     float64                          `json:"total_received"`
	Balance           float64                          `json:"balance"`
	TotalTransactions int64                            `json:"total_transactions"`
	Summary           []dto.TransactionSummaryResponse `json:"summary"`
}

func (s *TransactionService) GetDashboard(
	mobile string,
) (*DashboardResponse, error) {

	given, received, count, err :=
		s.transactionRepo.GetDashboardData(mobile)

	if err != nil {
		return nil, err
	}

	summary, err := s.GetTransactionSummary(mobile)
	if err != nil {
		summary = []dto.TransactionSummaryResponse{}
	}

	return &DashboardResponse{
		TotalGiven:    given,
		TotalReceived: received,
		// Positive means the user is owed money overall, matching the sign
		// convention of GetTransactionHistory and of summary[].balance.
		Balance:           received - given,
		TotalTransactions: count,
		Summary:           summary,
	}, nil
}

func (s *TransactionService) GetTransactionSummary(
	userMobile string,
) ([]dto.TransactionSummaryResponse, error) {

	transactions, err :=
		s.transactionRepo.GetUserTransactions(userMobile)

	if err != nil {
		return nil, err
	}

	summaryMap := make(map[string]*dto.TransactionSummaryResponse)

	for _, transaction := range transactions {

		if transaction.IsDeleted || transaction.IsArchived {
			continue
		}

		// Exclude group split transactions so they are not merged into peer friend balances
		if transaction.GroupID != nil || transaction.ExpenseType == models.ExpenseGroup {
			continue
		}

		// Match on the normalized number: the same contact can be stored as
		// "+919876543210" on one row and "9876543210" on another.
		userIsPayer := utils.SameMobile(transaction.FromMobile, userMobile)

		var contactMobile string
		if userIsPayer {
			contactMobile = transaction.ToMobile
		} else {
			contactMobile = transaction.FromMobile
		}

		// Key by the normalized number so one contact yields one summary row.
		key := utils.NormalizeMobile(contactMobile)
		if key == "" {
			key = contactMobile
		}

		regUser, err := s.userRepo.GetByMobile(contactMobile)
		isRegistered := false
		registeredName := ""
		displayName := transaction.ContactName
		if err == nil && regUser != nil && regUser.FullName != "" {
			isRegistered = true
			registeredName = regUser.FullName
			displayName = regUser.FullName
		}

		if _, exists := summaryMap[key]; !exists {
			summaryMap[key] = &dto.TransactionSummaryResponse{
				Mobile:              contactMobile,
				ContactName:         displayName,
				RegisteredName:      registeredName,
				IsRegistered:        isRegistered,
				LastTransactionDate: transaction.TransactionDate,
				Transactions:        make([]models.Transaction, 0),
			}
		} else {
			if isRegistered {
				summaryMap[key].IsRegistered = true
				summaryMap[key].RegisteredName = registeredName
				summaryMap[key].ContactName = registeredName
			} else if summaryMap[key].ContactName == "" || summaryMap[key].ContactName == contactMobile {
				if transaction.ContactName != "" {
					summaryMap[key].ContactName = transaction.ContactName
				}
			}
		}

		item := summaryMap[key]

		item.TotalTransactions++
		item.Transactions = append(item.Transactions, transaction)

		if transaction.TransactionDate.After(item.LastTransactionDate) {
			item.LastTransactionDate = transaction.TransactionDate
		}

		if userIsPayer {
			// User gave money, so the contact owes it back
			item.Balance += transaction.Amount
		} else {
			// User received money and owes it back
			item.Balance -= transaction.Amount
		}
	}

	response := make([]dto.TransactionSummaryResponse, 0)

	for _, item := range summaryMap {

		response = append(response, *item)

	}

	return response, nil
}

func (s *TransactionService) GetTransactionHistory(
	userMobile string,
	contactMobile string,
) (*dto.TransactionHistoryResponse, error) {

	transactions, err :=
		s.transactionRepo.GetTransactionsByMobile(
			userMobile,
			contactMobile,
		)

	if err != nil {
		return nil, err
	}

	response := &dto.TransactionHistoryResponse{
		Mobile:       contactMobile,
		Transactions: transactions,
	}

	for _, transaction := range transactions {
		if transaction.IsDeleted || transaction.IsArchived {
			continue
		}
		if utils.SameMobile(transaction.FromMobile, userMobile) {
			response.TotalGiven += transaction.Amount
		} else {
			response.TotalReceived += transaction.Amount
		}
	}

	response.Balance =
		response.TotalGiven - response.TotalReceived

	return response, nil
}

func (s *TransactionService) ArchiveTransaction(id uint, userMobile string) error {
	transaction, err := s.transactionRepo.GetByID(id)
	if err != nil {
		return err
	}
	if transaction.IsDeleted {
		return errors.New("cannot archive a deleted transaction")
	}

	currentUser, _ := s.userRepo.GetByMobile(userMobile)
	userEmail := ""
	if currentUser != nil {
		userEmail = currentUser.Email
	}

	isAuthorized := false
	if transaction.CreatedBy != "" && (transaction.CreatedBy == userMobile || (userEmail != "" && transaction.CreatedBy == userEmail)) {
		isAuthorized = true
	} else if transaction.FromMobile == userMobile || transaction.ToMobile == userMobile {
		isAuthorized = true
	}

	if !isAuthorized {
		return errors.New("only transaction participants can archive this transaction")
	}

	return s.transactionRepo.Archive(id)
}

func (s *TransactionService) UnarchiveTransaction(id uint, userMobile string) error {
	transaction, err := s.transactionRepo.GetByID(id)
	if err != nil {
		return err
	}
	if transaction.IsDeleted {
		return errors.New("cannot unarchive a deleted transaction")
	}

	// If transaction belongs to a soft-deleted group, cannot unarchive directly
	if transaction.GroupID != nil {
		var group models.Group
		if err := s.transactionRepo.DB.Unscoped().First(&group, *transaction.GroupID).Error; err == nil {
			if group.DeletedAt.Valid {
				return errors.New("cannot unarchive transaction of an archived group")
			}
		}
	}

	currentUser, _ := s.userRepo.GetByMobile(userMobile)
	userEmail := ""
	if currentUser != nil {
		userEmail = currentUser.Email
	}

	isAuthorized := false
	if transaction.CreatedBy != "" && (transaction.CreatedBy == userMobile || (userEmail != "" && transaction.CreatedBy == userEmail)) {
		isAuthorized = true
	} else if transaction.FromMobile == userMobile || transaction.ToMobile == userMobile {
		isAuthorized = true
	}

	if !isAuthorized {
		return errors.New("only transaction participants can unarchive this transaction")
	}

	return s.transactionRepo.Unarchive(id)
}
