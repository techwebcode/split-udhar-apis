package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"split-udhar-apis/dto"
	"split-udhar-apis/services"
)

type AuthController struct {
	Service *services.AuthService
}

func NewAuthController(db *gorm.DB) *AuthController {
	return &AuthController{
		Service: services.NewAuthService(db),
	}
}

func (a *AuthController) CheckEmail(c *gin.Context) {
	var req dto.CheckEmailRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Please enter a valid email address",
		})
		return
	}

	exists, err := a.Service.CheckEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"exists":  exists,
		"message": "Email status retrieved",
	})
}

func (a *AuthController) GoogleAuth(c *gin.Context) {
	var req dto.GoogleAuthRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result, err := a.Service.GoogleAuth(req.IDToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"profile_complete": result.ProfileComplete,
		"token":            result.Token,
		"refresh_token":    result.RefreshToken,
		"email":            result.Email,
		"full_name":        result.FullName,
		"google_id":        result.GoogleID,
		"user":             result.User,
	})
}

func (a *AuthController) CompleteGoogleSignup(c *gin.Context) {
	var req dto.CompleteGoogleSignupRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	token, refreshToken, user, err := a.Service.CompleteGoogleSignup(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"token":         token,
		"refresh_token": refreshToken,
		"user":          user,
	})
}

func (a *AuthController) Signup(c *gin.Context) {

	var req dto.SignupRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	if err := a.Service.Signup(req); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OTP sent successfully",
	})
}

func (a *AuthController) VerifySignup(c *gin.Context) {

	var req dto.SignupVerifyRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	token, user, err := a.Service.VerifySignup(req)

	if err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user":    user,
	})
}

func (a *AuthController) Login(c *gin.Context) {

	var req dto.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	hasMPIN, err := a.Service.Login(req)

	if err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	if hasMPIN {
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"has_mpin": true,
			"message":  "Please enter MPIN",
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"has_mpin": false,
			"message":  "OTP sent successfully",
		})
	}
}

func (a *AuthController) SendForgotPasscodeOTP(c *gin.Context) {
	var req dto.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Valid email address is required",
		})
		return
	}

	if err := a.Service.SendForgotPasscodeOTP(req.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OTP code sent to your registered email",
	})
}

func (a *AuthController) VerifyLogin(c *gin.Context) {

	var req dto.LoginVerifyRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	token, user, err := a.Service.VerifyLogin(req)

	if err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user":    user,
	})
}

func (a *AuthController) SetMPIN(c *gin.Context) {

	var req dto.SetMPINRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	// Target account comes from the authenticated JWT, not the request body.
	userID := c.GetUint("user_id")

	if err := a.Service.SetMPIN(userID, req); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "MPIN set successfully",
	})
}

func (a *AuthController) VerifyMPIN(c *gin.Context) {

	var req dto.VerifyMPINRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	token, user, err := a.Service.VerifyMPIN(req)

	if err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user":    user,
	})
}
