package services

import (
	"errors"

	"split-udhar-apis/dto"
	"split-udhar-apis/models"
	"split-udhar-apis/repositories"

	"gorm.io/gorm"
)

type UserService struct {
	UserRepo *repositories.UserRepository
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{
		UserRepo: repositories.NewUserRepository(db),
	}
}

func (s *UserService) GetProfile(userID uint) (*models.User, error) {
	return s.UserRepo.GetByID(userID)
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

	return s.UserRepo.Update(user)
}

func (s *UserService) DeleteAccount(userID uint) error {
	return s.UserRepo.DeleteAccount(userID)
}

