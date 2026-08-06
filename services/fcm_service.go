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

func (s *FCMService) SaveFCMToken(userMobile string, userID uint, req dto.SaveFCMTokenRequest) error {
	var user *models.User
	if userID > 0 {
		user, _ = s.userRepo.GetByID(userID)
	}
	if user == nil && userMobile != "" {
		user, _ = s.userRepo.GetByMobile(userMobile)
		if user == nil {
			user, _ = s.userRepo.GetByEmail(userMobile)
		}
	}

	mobileVal := userMobile
	if user != nil {
		userID = user.ID
		if user.Mobile != "" {
			mobileVal = user.Mobile
		}
	}

	platform := req.Platform
	if platform == "" {
		platform = "android"
	}

	device := models.UserDevice{
		UserID:   userID,
		Mobile:   mobileVal,
		FCMToken: req.FCMToken,
		Platform: platform,
		DeviceID: req.DeviceID,
	}

	log.Printf("[FCM SERVICE] Saving token for user '%s' (UserID: %d, DeviceID: '%s')", mobileVal, userID, req.DeviceID)
	return s.deviceRepo.SaveToken(&device)
}

func (s *FCMService) DeleteFCMToken(userMobile string, req dto.DeleteFCMTokenRequest) error {
	log.Printf("[FCM SERVICE] Deleting device token for user '%s'", userMobile)
	return s.deviceRepo.DeleteToken(userMobile, req.FCMToken, req.DeviceID)
}

func (s *FCMService) SendNotificationToUser(targetMobile string, title, body string, data map[string]string) error {
	var userID uint = 0
	u, _ := s.userRepo.GetByMobile(targetMobile)
	if u != nil {
		userID = u.ID
	}

	tokens, err := s.deviceRepo.GetTokensByMobile(targetMobile, userID)
	if err != nil || len(tokens) == 0 {
		log.Printf("[FCM SERVICE] No registered FCM device tokens found for mobile '%s' (UserID: %d)", targetMobile, userID)
		return nil
	}
	tokens = deduplicateTokens(tokens)

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
	tokens = deduplicateTokens(tokens)

	invalidTokens, err := utils.SendMulticastFCM(tokens, title, body, data)
	if len(invalidTokens) > 0 {
		_ = s.deviceRepo.DeleteInvalidTokens(invalidTokens)
	}

	return err
}

func (s *FCMService) SendTestNotification(userMobile string, userID uint, req dto.TestNotificationRequest) (int, error) {
	if userID == 0 && userMobile != "" {
		u, _ := s.userRepo.GetByMobile(userMobile)
		if u == nil {
			u, _ = s.userRepo.GetByEmail(userMobile)
		}
		if u != nil {
			userID = u.ID
			if userMobile == "" {
				userMobile = u.Mobile
			}
		}
	}

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

	tokens, err := s.deviceRepo.GetTokensByMobile(userMobile, userID)
	if err != nil || len(tokens) == 0 {
		log.Printf("[FCM SERVICE] No registered device tokens found for mobile '%s', UserID %d", userMobile, userID)
		return 0, nil
	}
	tokens = deduplicateTokens(tokens)
	log.Printf("[FCM SERVICE] Found %d token(s) for user '%s' (UserID %d): %v", len(tokens), userMobile, userID, tokens)

	invalidTokens, err := utils.SendMulticastFCM(tokens, title, body, data)
	if len(invalidTokens) > 0 {
		_ = s.deviceRepo.DeleteInvalidTokens(invalidTokens)
	}

	return len(tokens) - len(invalidTokens), err
}

func deduplicateTokens(tokens []string) []string {
	seen := make(map[string]bool)
	var deduped []string
	for _, token := range tokens {
		if token != "" && !seen[token] {
			seen[token] = true
			deduped = append(deduped, token)
		}
	}
	return deduped
}
