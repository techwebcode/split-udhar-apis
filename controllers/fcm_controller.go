package controllers

import (
	"net/http"
	"split-udhar-apis/dto"
	"split-udhar-apis/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FCMController struct {
	Service *services.FCMService
}

func NewFCMController(db *gorm.DB) *FCMController {
	return &FCMController{
		Service: services.NewFCMService(db),
	}
}

func (f *FCMController) SaveToken(c *gin.Context) {
	userMobile := c.GetString("mobile")
	if userMobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User mobile not found in token context",
		})
		return
	}

	var req dto.SaveFCMTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := f.Service.SaveFCMToken(userMobile, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "FCM device token saved successfully",
	})
}

func (f *FCMController) DeleteToken(c *gin.Context) {
	userMobile := c.GetString("mobile")
	if userMobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User mobile not found in token context",
		})
		return
	}

	var req dto.DeleteFCMTokenRequest
	_ = c.ShouldBindJSON(&req)

	if err := f.Service.DeleteFCMToken(userMobile, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "FCM device token deleted successfully",
	})
}
