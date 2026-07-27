package services

import (
	"log"
	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"
	"split-udhar-apis/utils"

	"gorm.io/gorm"
)

type FCMService struct {
	deviceRepo *repositories.UserDeviceRepository
	userRepo   *repositories.UserRepository
}

func NewFCMService(db *gorm.DB) *FCMService {
	return &FCMService{
		deviceRepo: repositories.NewUserDeviceRepository(db),
		userRepo:   repositories.NewUserRepository(db),
	}
}

func (s *FCMService) SaveFCMToken(userMobile string, req dto.SaveFCMTokenRequest) error {
	user, err := s.userRepo.GetByMobile(userMobile)
	var userID uint = 0
	if err == nil && user != nil {
		userID = user.ID
	}

	platform := req.Platform
	if platform == "" {
		platform = "android"
	}

	device := models.UserDevice{
		UserID:   userID,
		Mobile:   userMobile,
		FCMToken: req.FCMToken,
		Platform: platform,
		DeviceID: req.DeviceID,
	}

	log.Printf("[FCM SERVICE] Saving token for user '%s' (DeviceID: '%s')", userMobile, req.DeviceID)
	return s.deviceRepo.SaveToken(&device)
}

func (s *FCMService) DeleteFCMToken(userMobile string, req dto.DeleteFCMTokenRequest) error {
	log.Printf("[FCM SERVICE] Deleting device token for user '%s'", userMobile)
	return s.deviceRepo.DeleteToken(userMobile, req.FCMToken, req.DeviceID)
}

func (s *FCMService) SendNotificationToUser(targetMobile string, title, body string, data map[string]string) error {
	tokens, err := s.deviceRepo.GetTokensByMobile(targetMobile)
	if err != nil || len(tokens) == 0 {
		log.Printf("[FCM SERVICE] No registered FCM device tokens found for mobile '%s'", targetMobile)
		return nil
	}

	invalidTokens, err := utils.SendMulticastFCM(tokens, title, body, data)
	if len(invalidTokens) > 0 {
		_ = s.deviceRepo.DeleteInvalidTokens(invalidTokens)
	}

	return err
}

func (s *FCMService) SendNotificationToUsers(targetMobiles []string, title, body string, data map[string]string) error {
	if len(targetMobiles) == 0 {
		return nil
	}

	tokens, err := s.deviceRepo.GetTokensByMobiles(targetMobiles)
	if err != nil || len(tokens) == 0 {
		log.Printf("[FCM SERVICE] No registered FCM device tokens found for %d users", len(targetMobiles))
		return nil
	}

	invalidTokens, err := utils.SendMulticastFCM(tokens, title, body, data)
	if len(invalidTokens) > 0 {
		_ = s.deviceRepo.DeleteInvalidTokens(invalidTokens)
	}

	return err
}

func (s *FCMService) SendTestNotification(userMobile string, req dto.TestNotificationRequest) (int, error) {
	title := req.Title
	if title == "" {
		title = "SplitUdhar Test Notification 🔔"
	}
	body := req.Body
	if body == "" {
		body = "Push notification service is working successfully!"
	}
	msgType := req.Type
	if msgType == "" {
		msgType = "transaction"
	}

	data := map[string]string{
		"type":           msgType,
		"test":           "true",
		"transaction_id": "1",
	}

	tokens, err := s.deviceRepo.GetTokensByMobile(userMobile)
	if err != nil || len(tokens) == 0 {
		return 0, nil
	}

	invalidTokens, err := utils.SendMulticastFCM(tokens, title, body, data)
	if len(invalidTokens) > 0 {
		_ = s.deviceRepo.DeleteInvalidTokens(invalidTokens)
	}

	return len(tokens) - len(invalidTokens), err
}
