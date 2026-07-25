package repositories

import (
	"split-udhar-apis/models"

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

func (r *GroupRepository) Create(group *models.Group) error {
	return r.DB.Create(group).Error
}

func (r *GroupRepository) GetUserGroups(userMobile string) ([]models.Group, error) {
	var groupIDs []uint

	err := r.DB.Model(&models.GroupMember{}).
		Where("user_mobile = ?", userMobile).
		Pluck("group_id", &groupIDs).Error

	if err != nil {
		return nil, err
	}

	if len(groupIDs) == 0 {
		return []models.Group{}, nil
	}

	var groups []models.Group
	err = r.DB.Preload("Members").Preload("Expenses", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	}).
		Where("id IN ?", groupIDs).
		Order("created_at DESC").
		Find(&groups).Error

	return groups, err
}

func (r *GroupRepository) GetByID(id uint) (*models.Group, error) {
	var group models.Group
	err := r.DB.Preload("Members").Preload("Expenses", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	}).First(&group, id).Error

	if err != nil {
		return nil, err
	}
	return &group, nil
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

func (r *GroupRepository) Delete(id uint) error {
	return r.DB.Select("Members", "Expenses").Delete(&models.Group{Model: gorm.Model{ID: id}}).Error
}
