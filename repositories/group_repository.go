package repositories

import (
	"errors"
	"time"

	"split-udhar-apis/models"
	"split-udhar-apis/utils"

	"gorm.io/gorm"
)

type GroupRepository struct {
	DB *gorm.DB
}

func NewGroupRepository(db *gorm.DB) *GroupRepository {
	return &GroupRepository{
		DB: db,
	}
}

// WithTx returns a repository bound to tx, so a caller can run several writes
// inside one database transaction and have them roll back together.
func (r *GroupRepository) WithTx(tx *gorm.DB) *GroupRepository {
	return &GroupRepository{DB: tx}
}

func (r *GroupRepository) Create(group *models.Group) error {
	return r.DB.Create(group).Error
}

func (r *GroupRepository) GetUserGroups(userMobile string) ([]models.Group, error) {
	var groupIDs []uint

	// Members may be stored with a country code while the JWT carries a bare
	// 10-digit number, so match on both forms or the user sees no groups.
	err := r.DB.Model(&models.GroupMember{}).
		Where("user_mobile = ? OR RIGHT(user_mobile, 10) = ?",
			userMobile, utils.NormalizeMobile(userMobile)).
		Pluck("group_id", &groupIDs).Error

	if err != nil {
		return nil, err
	}

	if len(groupIDs) == 0 {
		return []models.Group{}, nil
	}

	var groups []models.Group
	err = r.DB.
		Preload("Members", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL")
		}).
		Preload("Expenses", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL AND is_deleted = false").Order("created_at DESC")
		}).
		Where("id IN ?", groupIDs).
		Order("created_at DESC").
		Find(&groups).Error

	return groups, err
}

func (r *GroupRepository) GetByID(id uint) (*models.Group, error) {
	var group models.Group
	err := r.DB.
		Preload("Members", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL")
		}).
		Preload("Expenses", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL AND is_deleted = false").Order("created_at DESC")
		}).First(&group, id).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		err = r.DB.Unscoped().
			Preload("Members", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL")
			}).
			Preload("Expenses", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL AND is_deleted = false").Order("created_at DESC")
			}).First(&group, id).Error
	}

	if err != nil {
		return nil, err
	}
	return &group, nil
}

// GetAllWithRelations loads every group with its members and expenses, for
// maintenance tasks that replay the whole ledger.
func (r *GroupRepository) GetAllWithRelations() ([]models.Group, error) {
	var groups []models.Group
	err := r.DB.Preload("Members").
		Preload("Expenses", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Order("id ASC").
		Find(&groups).Error
	return groups, err
}

// SetMemberBalance overwrites a member's balance with an absolute value, as
// opposed to UpdateMemberBalance which applies a relative delta.
func (r *GroupRepository) SetMemberBalance(groupID uint, userMobile string, balance float64) error {
	return r.DB.Model(&models.GroupMember{}).
		Where("group_id = ? AND user_mobile = ?", groupID, userMobile).
		Update("balance", balance).Error
}

func (r *GroupRepository) UpdateMemberBalance(groupID uint, userMobile string, delta float64) error {
	return r.DB.Model(&models.GroupMember{}).
		Where("group_id = ? AND user_mobile = ?", groupID, userMobile).
		Update("balance", gorm.Expr("balance + ?", delta)).Error
}

func (r *GroupRepository) AddMember(member *models.GroupMember) error {
	return r.DB.Create(member).Error
}

func (r *GroupRepository) RemoveMember(groupID uint, userMobile string) error {
	return r.DB.Where("group_id = ? AND user_mobile = ?", groupID, userMobile).
		Delete(&models.GroupMember{}).Error
}

func (r *GroupRepository) CreateExpense(expense *models.GroupExpense) error {
	return r.DB.Create(expense).Error
}

func (r *GroupRepository) GetExpenseByID(expenseID uint) (*models.GroupExpense, error) {
	var expense models.GroupExpense
	err := r.DB.First(&expense, expenseID).Error
	if err != nil {
		return nil, err
	}
	return &expense, nil
}

func (r *GroupRepository) UpdateExpense(expense *models.GroupExpense) error {
	return r.DB.Save(expense).Error
}

func (r *GroupRepository) DeleteExpense(expenseID uint, deletedBy string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"is_deleted":      true,
		"deleted_at_time": &now,
	}
	if deletedBy != "" {
		updates["deleted_by"] = deletedBy
	}
	if err := r.DB.Model(&models.GroupExpense{}).Where("id = ?", expenseID).Updates(updates).Error; err != nil {
		return err
	}
	return r.DB.Delete(&models.GroupExpense{Model: gorm.Model{ID: expenseID}}).Error
}

func (r *GroupRepository) CreateExpenseEditLog(log *models.GroupExpenseEditLog) error {
	return r.DB.Create(log).Error
}

func (r *GroupRepository) GetExpenseEditLogs(expenseID uint) ([]models.GroupExpenseEditLog, error) {
	var logs []models.GroupExpenseEditLog
	err := r.DB.Where("expense_id = ?", expenseID).Order("edited_at desc").Find(&logs).Error
	return logs, err
}

func (r *GroupRepository) UpdateGroupDetails(groupID uint, name, description string) error {
	return r.DB.Model(&models.Group{}).
		Where("id = ?", groupID).
		Updates(map[string]interface{}{
			"name":        name,
			"description": description,
		}).Error
}

func (r *GroupRepository) Delete(id uint) error {
	return r.DB.Select("Members", "Expenses").Delete(&models.Group{Model: gorm.Model{ID: id}}).Error
}
