package controllers

import (
	"net/http"
	"strings"

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
	userID := c.GetUint("user_id")

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authenticated user session required",
		})
		return
	}

	var req dto.SaveFCMTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request payload: " + err.Error(),
		})
		return
	}

	req.FCMToken = strings.TrimSpace(req.FCMToken)
	if req.FCMToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "fcm_token is required and cannot be empty",
		})
		return
	}

	if err := f.Service.SaveFCMToken(userMobile, userID, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
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
	userID := c.GetUint("user_id")

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authenticated user session required",
		})
		return
	}

	var req dto.DeleteFCMTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request payload: " + err.Error(),
		})
		return
	}

	req.FCMToken = strings.TrimSpace(req.FCMToken)
	if req.FCMToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "fcm_token is required for unregistration",
		})
		return
	}

	if err := f.Service.DeleteFCMToken(userID, userMobile, req.FCMToken); err != nil {
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

func (f *FCMController) TestNotification(c *gin.Context) {
	userMobile := c.GetString("mobile")
	userID := c.GetUint("user_id")

	if userID == 0 && userMobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authenticated session required",
		})
		return
	}

	var req dto.TestNotificationRequest
	_ = c.ShouldBindJSON(&req)

	attemptedCount, acceptedCount, err := f.Service.SendTestNotification(userMobile, userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to send test notification: " + err.Error(),
		})
		return
	}

	if attemptedCount == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success":         false,
			"message":         "No registered active FCM device token found for your account. Please log in on the app to register your device.",
			"tokens_count":    0,
			"attempted_count": 0,
		})
		return
	}

	if acceptedCount == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success":         false,
			"message":         "FCM did not accept notifications for the registered device tokens (tokens may be expired or invalid)",
			"tokens_count":    0,
			"attempted_count": attemptedCount,
		})
		return
	}

	message := "Test push notification dispatched successfully"
	if acceptedCount < attemptedCount {
		message = "Test push notification partially dispatched (some tokens failed or were unregistered)"
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         message,
		"tokens_count":    acceptedCount,
		"attempted_count": attemptedCount,
	})
}
