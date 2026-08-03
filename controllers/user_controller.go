package controllers

import (
	"net/http"

	"split-udhar-apis/dto"
	"split-udhar-apis/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserController struct {
	Service *services.UserService
}

func NewUserController(db *gorm.DB) *UserController {
	return &UserController{
		Service: services.NewUserService(db),
	}
}

func (u *UserController) GetProfile(c *gin.Context) {

	userID := c.GetUint("user_id")

	user, err := u.Service.GetProfile(userID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

func (u *UserController) UpdateProfile(c *gin.Context) {

	userID := c.GetUint("user_id")

	var req dto.UpdateProfileRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	err := u.Service.UpdateProfile(userID, req)

	if err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Profile updated successfully",
	})
}

func (u *UserController) DeleteAccount(c *gin.Context) {
	userID := c.GetUint("user_id")

	err := u.Service.DeleteAccount(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete account: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Account deleted successfully",
	})
}

