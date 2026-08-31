package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"split-udhar-apis/controllers"
	"split-udhar-apis/middleware"
)

func AuthRoutes(router *gin.RouterGroup, db *gorm.DB) {

	auth := controllers.NewAuthController(db)

	router.POST("/check-email", auth.CheckEmail)
	router.POST("/google", auth.GoogleAuth)
	router.POST("/google/complete", auth.CompleteGoogleSignup)

	router.POST("/signup", auth.Signup)

	router.POST("/signup/verify", auth.VerifySignup)

	router.POST("/login", auth.Login)

	router.POST("/login/verify", auth.VerifyLogin)

	router.POST("/forgot-password/send-otp", auth.SendForgotPasscodeOTP)

	router.POST("/send-otp", auth.SendForgotPasscodeOTP)

	// MPIN verification is itself a login mechanism, so it stays public.
	router.POST("/mpin/verify", auth.VerifyMPIN)
	router.POST("/refresh", auth.RefreshToken)

	// Setting an MPIN requires an existing session. Without this an attacker
	// could set the MPIN of any account by email and then log in with it.
	authenticated := router.Group("")
	authenticated.Use(middleware.AuthMiddleware())
	{
		authenticated.POST("/mpin/set", auth.SetMPIN)
	}
}
