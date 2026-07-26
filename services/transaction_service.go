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

	"gorm.io/gorm"
)

type TransactionService struct {
	transactionRepo *repositories.TransactionRepository
	userRepo        *repositories.UserRepository
}

func NewTransactionService(db *gorm.DB) *TransactionService {
	return &TransactionService{
		transactionRepo: repositories.NewTransactionRepository(db),
		userRepo:        repositories.NewUserRepository(db),
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

	return s.transactionRepo.Create(&transaction)
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

	isOwner := (transaction.CreatedBy == userMobile || (userEmail != "" && transaction.CreatedBy == userEmail)) ||
		(transaction.Type == models.TransactionGive && transaction.FromMobile == userMobile) ||
		(transaction.Type == models.TransactionReceive && transaction.ToMobile == userMobile)

	if !isOwner {
		return errors.New("only the transaction creator can edit this transaction")
	}

	oldAmount := transaction.Amount
	oldNote := transaction.Note

	if oldAmount != req.Amount || oldNote != req.Note {
		now := time.Now()
		transaction.IsEdited = true
		transaction.PreviousAmount = oldAmount
		transaction.PreviousNote = oldNote
		transaction.EditedAt = &now
		transaction.UpdatedBy = userMobile

		editLog := models.TransactionEditLog{
			TransactionID: transaction.ID,
			OldAmount:     oldAmount,
			NewAmount:     req.Amount,
			OldNote:       oldNote,
			NewNote:       req.Note,
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

	isOwner := (transaction.CreatedBy == userMobile || (userEmail != "" && transaction.CreatedBy == userEmail)) ||
		(transaction.Type == models.TransactionGive && transaction.FromMobile == userMobile) ||
		(transaction.Type == models.TransactionReceive && transaction.ToMobile == userMobile)

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
		TotalGiven:        given,
		TotalReceived:     received,
		Balance:           given - received,
		TotalTransactions: count,
		Summary:           summary,
	}, nil
}

func (s *TransactionService) GetTransactionSummary(
	userMobile string,
) ([]dto.TransactionSummaryResponse, error) {

	transactions, err :=
		s.transactionRepo.GetTransactionSummary(userMobile)

	if err != nil {
		return nil, err
	}

	summaryMap := make(map[string]*dto.TransactionSummaryResponse)

	for _, transaction := range transactions {

		var contactMobile string

		if transaction.FromMobile == userMobile {

			contactMobile = transaction.ToMobile

		} else {

			contactMobile = transaction.FromMobile
		}

		if _, exists := summaryMap[contactMobile]; !exists {
			displayName := transaction.ContactName
			regUser, err := s.userRepo.GetByMobile(contactMobile)
			if err == nil && regUser != nil && regUser.FullName != "" {
				displayName = regUser.FullName
			} else if displayName == "" {
				displayName = contactMobile
			}

			summaryMap[contactMobile] = &dto.TransactionSummaryResponse{
				Mobile:              contactMobile,
				ContactName:         displayName,
				LastTransactionDate: transaction.TransactionDate,
				Transactions:        make([]models.Transaction, 0),
			}
		}

		item := summaryMap[contactMobile]

		item.TotalTransactions++
		item.Transactions = append(item.Transactions, transaction)

		if !transaction.IsDeleted {
			if transaction.FromMobile == userMobile {
				// User gave money
				item.Balance += transaction.Amount
			} else {
				// User received money
				item.Balance -= transaction.Amount
			}
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
		if transaction.IsDeleted {
			continue
		}
		if transaction.FromMobile == userMobile {
			response.TotalGiven += transaction.Amount
		} else {
			response.TotalReceived += transaction.Amount
		}
	}

	response.Balance =
		response.TotalGiven - response.TotalReceived

	return response, nil
}
