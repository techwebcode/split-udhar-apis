package repositories

import (
	"split-udhar-apis/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserDeviceRepository struct {
	db *gorm.DB
}

func NewUserDeviceRepository(db *gorm.DB) *UserDeviceRepository {
	return &UserDeviceRepository{db: db}
}

func (r *UserDeviceRepository) SaveToken(device *models.UserDevice) error {
	var existing models.UserDevice

	// Try to find existing record by FCMToken or (Mobile + DeviceID)
	query := r.db.Where("fcm_token = ?", device.FCMToken)
	if device.DeviceID != "" {
		query = r.db.Where("fcm_token = ? OR (mobile = ? AND device_id = ?)", device.FCMToken, device.Mobile, device.DeviceID)
	}

	err := query.First(&existing).Error
	if err == nil {
		// Update existing record
		existing.Mobile = device.Mobile
		existing.UserID = device.UserID
		existing.FCMToken = device.FCMToken
		if device.Platform != "" {
			existing.Platform = device.Platform
		}
		if device.DeviceID != "" {
			existing.DeviceID = device.DeviceID
		}
		return r.db.Save(&existing).Error
	}

	// Upsert clause on unique index
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fcm_token"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "mobile", "platform", "device_id", "updated_at"}),
	}).Create(device).Error
}

func (r *UserDeviceRepository) DeleteToken(mobile, token, deviceID string) error {
	query := r.db.Where("mobile = ?", mobile)
	if token != "" && deviceID != "" {
		query = query.Where("fcm_token = ? OR device_id = ?", token, deviceID)
	} else if token != "" {
		query = query.Where("fcm_token = ?", token)
	} else if deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	return query.Delete(&models.UserDevice{}).Error
}

func (r *UserDeviceRepository) GetTokensByMobile(mobile string) ([]string, error) {
	var tokens []string
	err := r.db.Model(&models.UserDevice{}).
		Where("mobile = ? AND fcm_token != ''", mobile).
		Pluck("fcm_token", &tokens).Error
	return tokens, err
}

func (r *UserDeviceRepository) GetTokensByMobiles(mobiles []string) ([]string, error) {
	if len(mobiles) == 0 {
		return []string{}, nil
	}
	var tokens []string
	err := r.db.Model(&models.UserDevice{}).
		Where("mobile IN ? AND fcm_token != ''", mobiles).
		Pluck("fcm_token", &tokens).Error
	return tokens, err
}

func (r *UserDeviceRepository) DeleteInvalidTokens(invalidTokens []string) error {
	if len(invalidTokens) == 0 {
		return nil
	}
	return r.db.Where("fcm_token IN ?", invalidTokens).Delete(&models.UserDevice{}).Error
}
