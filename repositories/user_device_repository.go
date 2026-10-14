package repositories

import (
	"errors"
	"split-udhar-apis/models"
	"split-udhar-apis/utils"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserDeviceRepository struct {
	db *gorm.DB
}

func NewUserDeviceRepository(db *gorm.DB) *UserDeviceRepository {
	return &UserDeviceRepository{db: db}
}

// SaveToken atomically inserts or updates a device token registration.
// If the token already exists (even if previously soft-deleted on logout),
// its ownership is reassigned to the current authenticated user and deleted_at is cleared.
func (r *UserDeviceRepository) SaveToken(device *models.UserDevice) error {
	if device == nil || device.FCMToken == "" {
		return errors.New("fcm_token is required")
	}
	if device.UserID == 0 {
		return errors.New("authenticated user_id is required")
	}

	now := time.Now()
	device.UpdatedAt = now

	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "fcm_token"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"user_id":    device.UserID,
			"mobile":     device.Mobile,
			"platform":   device.Platform,
			"deleted_at": nil,
			"updated_at": now,
		}),
	}).Create(device).Error
}

// DeleteToken safely removes or soft-deletes a specific token owned by the authenticated user.
// It will not delete tokens belonging to another account, nor other devices for this account.
func (r *UserDeviceRepository) DeleteToken(userID uint, token string) error {
	if token == "" {
		return errors.New("fcm_token is required for deletion")
	}
	if userID == 0 {
		return errors.New("authenticated user_id is required for deletion")
	}

	return r.db.Where("fcm_token = ? AND user_id = ?", token, userID).
		Delete(&models.UserDevice{}).Error
}

// GetTokensByUserID returns all active (non-deleted) FCM tokens for a specific user ID.
func (r *UserDeviceRepository) GetTokensByUserID(userID uint) ([]string, error) {
	if userID == 0 {
		return []string{}, nil
	}

	var tokens []string
	err := r.db.Model(&models.UserDevice{}).
		Where("deleted_at IS NULL AND user_id = ? AND fcm_token IS NOT NULL AND fcm_token <> ''", userID).
		Distinct("fcm_token").
		Pluck("fcm_token", &tokens).Error

	return tokens, err
}

// GetTokensByUserIDs returns all active tokens for a list of user IDs.
func (r *UserDeviceRepository) GetTokensByUserIDs(userIDs []uint) ([]string, error) {
	if len(userIDs) == 0 {
		return []string{}, nil
	}

	var tokens []string
	err := r.db.Model(&models.UserDevice{}).
		Where("deleted_at IS NULL AND user_id IN ? AND fcm_token IS NOT NULL AND fcm_token <> ''", userIDs).
		Distinct("fcm_token").
		Pluck("fcm_token", &tokens).Error

	return tokens, err
}

// GetTokensByMobile provides recipient token resolution:
// If an authoritative userID is available, it queries by userID.
// Otherwise, it matches unassigned/legacy tokens matching the mobile number.
func (r *UserDeviceRepository) GetTokensByMobile(mobile string, userID uint) ([]string, error) {
	if userID > 0 {
		return r.GetTokensByUserID(userID)
	}

	if mobile == "" {
		return []string{}, nil
	}

	cleanMobile := utils.NormalizeMobile(mobile)
	var tokens []string

	query := r.db.Model(&models.UserDevice{}).
		Where("deleted_at IS NULL AND user_id = 0 AND fcm_token IS NOT NULL AND fcm_token <> ''")

	if cleanMobile != "" && cleanMobile != mobile {
		query = query.Where("(mobile = ? OR mobile = ?)", mobile, cleanMobile)
	} else {
		query = query.Where("mobile = ?", mobile)
	}

	err := query.Distinct("fcm_token").Pluck("fcm_token", &tokens).Error
	return tokens, err
}

// GetTokensByMobiles retrieves tokens for multiple mobile numbers.
func (r *UserDeviceRepository) GetTokensByMobiles(mobiles []string) ([]string, error) {
	if len(mobiles) == 0 {
		return []string{}, nil
	}

	mobileSet := make(map[string]bool)
	for _, m := range mobiles {
		if m != "" {
			mobileSet[m] = true
			if norm := utils.NormalizeMobile(m); norm != "" {
				mobileSet[norm] = true
			}
		}
	}

	searchMobiles := make([]string, 0, len(mobileSet))
	for m := range mobileSet {
		searchMobiles = append(searchMobiles, m)
	}

	var tokens []string
	err := r.db.Model(&models.UserDevice{}).
		Where("deleted_at IS NULL AND mobile IN ? AND fcm_token IS NOT NULL AND fcm_token <> ''", searchMobiles).
		Distinct("fcm_token").
		Pluck("fcm_token", &tokens).Error

	return tokens, err
}

// DeleteInvalidTokens removes permanently invalid or unregistered tokens from the database.
func (r *UserDeviceRepository) DeleteInvalidTokens(invalidTokens []string) error {
	if len(invalidTokens) == 0 {
		return nil
	}
	// Permanently purge invalid tokens with Unscoped
	return r.db.Unscoped().Where("fcm_token IN ?", invalidTokens).Delete(&models.UserDevice{}).Error
}
