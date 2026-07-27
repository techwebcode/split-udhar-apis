package routes

import (
	"split-udhar-apis/controllers"
	"split-udhar-apis/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func FCMRoutes(apiGroup *gin.RouterGroup, db *gorm.DB) {
	fcmController := controllers.NewFCMController(db)

	userGroup := apiGroup.Group("/user")
	userGroup.Use(middleware.AuthMiddleware())
	{
		userGroup.POST("/fcm-token", fcmController.SaveToken)
		userGroup.DELETE("/fcm-token", fcmController.DeleteToken)
	}
}
