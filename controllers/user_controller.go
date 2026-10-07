package controllers

import (
	"io"
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



func (u *UserController) GetProfileImage(c *gin.Context) {
	userID := c.GetUint("user_id")
	res, err := u.Service.GetProfileImage(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    res,
	})
}

func (u *UserController) UploadProfileImage(c *gin.Context) {
	userID := c.GetUint("user_id")

	fileHeader, err := c.FormFile("image")
	if err != nil {
		fileHeader, err = c.FormFile("file")
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "image file is required (field 'image' or 'file')",
		})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "failed to open uploaded file",
		})
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "failed to read uploaded file",
		})
		return
	}

	res, err := u.Service.UploadProfileImage(c.Request.Context(), userID, fileBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Profile picture updated successfully",
		"data":    res,
	})
}

func (u *UserController) RemoveProfileImage(c *gin.Context) {
	userID := c.GetUint("user_id")
	err := u.Service.RemoveProfileImage(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Profile picture removed successfully",
	})
}
