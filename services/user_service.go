package services

import (
	"context"
	"errors"
	"fmt"

	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"
	"split-udhar-apis/services/storage"
	"split-udhar-apis/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserService struct {
	UserRepo  *repositories.UserRepository
	GroupRepo *repositories.GroupRepository
	Storage   storage.ImageStorageService
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{
		UserRepo:  repositories.NewUserRepository(db),
		GroupRepo: repositories.NewGroupRepository(db),
		Storage:   storage.NewStorageServiceFromEnv(),
	}
}

func NewUserServiceWithStorage(db *gorm.DB, s storage.ImageStorageService) *UserService {
	return &UserService{
		UserRepo:  repositories.NewUserRepository(db),
		GroupRepo: repositories.NewGroupRepository(db),
		Storage:   s,
	}
}

func (s *UserService) GetProfile(userID uint) (*models.User, error) {
	user, err := s.UserRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user != nil && user.ProfileImageKey != "" && s.Storage != nil {
		user.ProfileImageURL = s.Storage.GetURL(user.ProfileImageKey)
	}
	return user, nil
}

func (s *UserService) UpdateProfile(userID uint, req dto.UpdateProfileRequest) error {
	user, err := s.UserRepo.GetByID(userID)
	if err != nil {
		return err
	}

	if req.FullName != "" {
		user.FullName = req.FullName
	}

	if req.Mobile != "" {
		existingUser, err := s.UserRepo.GetByMobile(req.Mobile)
		if err == nil && existingUser != nil && existingUser.ID != userID {
			return errors.New("mobile number is already registered with another account")
		}
		user.Mobile = req.Mobile
	}

	if err := s.UserRepo.Update(user); err != nil {
		return err
	}

	if s.GroupRepo != nil && (req.FullName != "" || req.Mobile != "") {
		_ = s.GroupRepo.LinkUserToGroupMembers(user)
	}

	return nil
}

func (s *UserService) GetProfileImage(userID uint) (*dto.ProfileImageResponse, error) {
	user, err := s.UserRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	var url string
	if user.ProfileImageKey != "" && s.Storage != nil {
		url = s.Storage.GetURL(user.ProfileImageKey)
	}

	return &dto.ProfileImageResponse{
		ProfileImageKey: user.ProfileImageKey,
		ProfileImageURL: url,
	}, nil
}

func (s *UserService) UploadProfileImage(ctx context.Context, userID uint, fileData []byte) (*dto.ProfileImageResponse, error) {
	validated, err := utils.ValidateImage(fileData)
	if err != nil {
		return nil, err
	}

	user, err := s.UserRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if s.Storage == nil {
		return nil, errors.New("storage service is not configured")
	}

	uniqueID := uuid.New().String()
	newKey := fmt.Sprintf("users/%d/profile/%s%s", userID, uniqueID, validated.Extension)

	// 1. Upload new image to storage
	if err := s.Storage.Upload(ctx, newKey, fileData, validated.ContentType); err != nil {
		return nil, fmt.Errorf("failed to upload image to storage: %w", err)
	}

	// 2. Persist new key in MySQL
	oldKey := user.ProfileImageKey
	if err := s.UserRepo.UpdateProfileImageKey(userID, newKey); err != nil {
		// Roll back newly uploaded object to prevent orphaned storage bloat
		_ = s.Storage.Delete(ctx, newKey)
		return nil, fmt.Errorf("failed to save image reference to database: %w", err)
	}

	// 3. Delete previous image ONLY after new reference is safely stored
	if oldKey != "" && oldKey != newKey {
		_ = s.Storage.Delete(ctx, oldKey)
	}

	return &dto.ProfileImageResponse{
		ProfileImageKey: newKey,
		ProfileImageURL: s.Storage.GetURL(newKey),
	}, nil
}

func (s *UserService) RemoveProfileImage(ctx context.Context, userID uint) error {
	user, err := s.UserRepo.GetByID(userID)
	if err != nil {
		return errors.New("user not found")
	}

	if user.ProfileImageKey == "" {
		return nil
	}

	oldKey := user.ProfileImageKey

	// 1. Remove reference from MySQL first
	if err := s.UserRepo.UpdateProfileImageKey(userID, ""); err != nil {
		return fmt.Errorf("failed to remove image reference from database: %w", err)
	}

	// 2. Delete object from storage
	if s.Storage != nil && oldKey != "" {
		_ = s.Storage.Delete(ctx, oldKey)
	}

	return nil
}

func (s *UserService) DeleteAccount(userID uint) error {
	user, err := s.UserRepo.GetByID(userID)
	if err == nil && user != nil && user.ProfileImageKey != "" && s.Storage != nil {
		_ = s.Storage.Delete(context.Background(), user.ProfileImageKey)
	}
	return s.UserRepo.DeleteAccount(userID)
}
