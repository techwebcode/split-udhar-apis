package services

import (
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

	user.FullName = req.FullName

	return s.UserRepo.Update(user)
}
