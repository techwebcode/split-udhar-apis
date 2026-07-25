package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"split-udhar-apis/controllers"
)

func AuthRoutes(router *gin.RouterGroup, db *gorm.DB) {

	auth := controllers.NewAuthController(db)

	router.POST("/signup", auth.Signup)

	router.POST("/signup/verify", auth.VerifySignup)

	router.POST("/login", auth.Login)

	router.POST("/login/verify", auth.VerifyLogin)

	router.POST("/mpin/set", auth.SetMPIN)

	router.POST("/mpin/verify", auth.VerifyMPIN)
}
