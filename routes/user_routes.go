package routes

import (
	"split-udhar-apis/controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func UserRoutes(router *gin.RouterGroup, db *gorm.DB) {

	user := controllers.NewUserController(db)

	router.GET("/profile", user.GetProfile)

	router.PUT("/profile", user.UpdateProfile)
}
