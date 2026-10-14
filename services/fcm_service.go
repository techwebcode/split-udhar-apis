package services

import (
	"errors"
	"fmt"
	"log"
	"strings"

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

// SaveFCMToken registers or updates a device token under the authenticated user.
func (s *FCMService) SaveFCMToken(userMobile string, userID uint, req dto.SaveFCMTokenRequest) error {
	token := strings.TrimSpace(req.FCMToken)
	if len(token) < 10 || len(token) > 500 {
		return errors.New("invalid fcm_token length (must be between 10 and 500 characters)")
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform == "" {
		platform = "android"
	}
	if platform != "android" && platform != "ios" && platform != "web" {
		return fmt.Errorf("unsupported platform '%s': must be android, ios, or web", platform)
	}

	var user *models.User
	if userID > 0 {
		user, _ = s.userRepo.GetByID(userID)
	}
	if user == nil && userMobile != "" {
		user, _ = s.userRepo.GetByMobile(userMobile)
	}

	if user == nil {
		return errors.New("authenticated user account not found")
	}

	authoritativeUserID := user.ID
	authoritativeMobile := user.Mobile
	if authoritativeMobile == "" {
		authoritativeMobile = userMobile
	}

	device := models.UserDevice{
		UserID:   authoritativeUserID,
		Mobile:   authoritativeMobile,
		FCMToken: token,
		Platform: platform,
	}

	log.Printf("[FCM SERVICE] Registering token '%s' for UserID %d (mobile: %s, platform: %s)",
		utils.MaskToken(token), authoritativeUserID, authoritativeMobile, platform)

	return s.deviceRepo.SaveToken(&device)
}

// DeleteFCMToken unregisters a device token for the authenticated user.
func (s *FCMService) DeleteFCMToken(userID uint, userMobile, fcmToken string) error {
	token := strings.TrimSpace(fcmToken)
	if token == "" {
		return errors.New("fcm_token is required for deletion")
	}

	if userID == 0 && userMobile != "" {
		u, _ := s.userRepo.GetByMobile(userMobile)
		if u != nil {
			userID = u.ID
		}
	}

	if userID == 0 {
		return errors.New("authenticated user context is required for token unregistration")
	}

	log.Printf("[FCM SERVICE] Unregistering token '%s' for UserID %d", utils.MaskToken(token), userID)
	return s.deviceRepo.DeleteToken(userID, token)
}

// SendNotificationToUser resolves the recipient to an authoritative user and delivers push alerts.
func (s *FCMService) SendNotificationToUser(targetMobile string, title, body string, data map[string]string) error {
	var userID uint = 0
	u, err := s.userRepo.GetByMobile(targetMobile)
	if err != nil {
		log.Printf("[FCM SERVICE] Database error resolving user for mobile '%s': %v", targetMobile, err)
		return fmt.Errorf("failed resolving recipient user: %w", err)
	}
	if u != nil {
		userID = u.ID
	}

	tokens, err := s.deviceRepo.GetTokensByMobile(targetMobile, userID)
	if err != nil {
		log.Printf("[FCM SERVICE] Database error fetching tokens for UserID %d: %v", userID, err)
		return fmt.Errorf("failed fetching device tokens: %w", err)
	}

	if len(tokens) == 0 {
		log.Printf("[FCM SERVICE] No registered FCM device tokens found for mobile '%s' (UserID: %d)", targetMobile, userID)
		return nil
	}

	tokens = utils.DeduplicateTokens(tokens)
	successCount, invalidTokens, err := utils.SendMulticastFCM(tokens, title, body, data)
	if len(invalidTokens) > 0 {
		if delErr := s.deviceRepo.DeleteInvalidTokens(invalidTokens); delErr != nil {
			log.Printf("[FCM SERVICE] Warning: Failed to clean up %d invalid tokens: %v", len(invalidTokens), delErr)
		}
	}

	log.Printf("[FCM SERVICE] User notification delivery: %d of %d accepted by FCM (UserID %d)", successCount, len(tokens), userID)
	return err
}

// SendNotificationToUsers sends multicast push notifications to multiple recipients safely.
func (s *FCMService) SendNotificationToUsers(targetMobiles []string, title, body string, data map[string]string) error {
	if len(targetMobiles) == 0 {
		return nil
	}

	var userIDs []uint
	var unresolvedMobiles []string

	for _, m := range targetMobiles {
		if m == "" {
			continue
		}
		u, err := s.userRepo.GetByMobile(m)
		if err == nil && u != nil {
			userIDs = append(userIDs, u.ID)
		} else {
			unresolvedMobiles = append(unresolvedMobiles, m)
		}
	}

	var allTokens []string

	if len(userIDs) > 0 {
		userTokens, err := s.deviceRepo.GetTokensByUserIDs(userIDs)
		if err != nil {
			log.Printf("[FCM SERVICE] Error fetching tokens for user IDs: %v", err)
			return fmt.Errorf("failed fetching user tokens: %w", err)
		}
		allTokens = append(allTokens, userTokens...)
	}

	if len(unresolvedMobiles) > 0 {
		legacyTokens, err := s.deviceRepo.GetTokensByMobiles(unresolvedMobiles)
		if err != nil {
			log.Printf("[FCM SERVICE] Error fetching legacy tokens: %v", err)
			return fmt.Errorf("failed fetching legacy tokens: %w", err)
		}
		allTokens = append(allTokens, legacyTokens...)
	}

	allTokens = utils.DeduplicateTokens(allTokens)
	if len(allTokens) == 0 {
		log.Printf("[FCM SERVICE] No active device tokens found for %d recipients", len(targetMobiles))
		return nil
	}

	successCount, invalidTokens, err := utils.SendMulticastFCM(allTokens, title, body, data)
	if len(invalidTokens) > 0 {
		if delErr := s.deviceRepo.DeleteInvalidTokens(invalidTokens); delErr != nil {
			log.Printf("[FCM SERVICE] Warning: Failed to clean up %d invalid tokens: %v", len(invalidTokens), delErr)
		}
	}

	log.Printf("[FCM SERVICE] Bulk notification delivery: %d of %d accepted by FCM (%d recipients)",
		successCount, len(allTokens), len(targetMobiles))
	return err
}

// SendTestNotification sends a test push alert and returns (attemptedCount, acceptedCount, error).
func (s *FCMService) SendTestNotification(userMobile string, userID uint, req dto.TestNotificationRequest) (int, int, error) {
	if userID == 0 && userMobile != "" {
		u, err := s.userRepo.GetByMobile(userMobile)
		if err != nil {
			return 0, 0, fmt.Errorf("failed resolving user: %w", err)
		}
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

	if userID == 0 {
		return 0, 0, errors.New("authenticated user context not found")
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

	tokens, err := s.deviceRepo.GetTokensByUserID(userID)
	if err != nil {
		log.Printf("[FCM SERVICE] Database error querying tokens for UserID %d: %v", userID, err)
		return 0, 0, fmt.Errorf("database query failed: %w", err)
	}

	if len(tokens) == 0 {
		log.Printf("[FCM SERVICE] No registered device tokens found for UserID %d", userID)
		return 0, 0, nil
	}

	tokens = utils.DeduplicateTokens(tokens)
	log.Printf("[FCM SERVICE] Found %d active token(s) for UserID %d", len(tokens), userID)

	successCount, invalidTokens, err := utils.SendMulticastFCM(tokens, title, body, data)
	if len(invalidTokens) > 0 {
		_ = s.deviceRepo.DeleteInvalidTokens(invalidTokens)
	}

	return len(tokens), successCount, err
}
