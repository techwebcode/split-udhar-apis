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
	router.GET("/profile/image", user.GetProfileImage)
	router.POST("/profile/image", user.UploadProfileImage)
	router.DELETE("/profile/image", user.RemoveProfileImage)

	router.GET("/payment-settings", user.GetPaymentSettings)
	router.PUT("/payment-settings", user.UpdatePaymentSettings)
	router.DELETE("/payment-settings", user.DeletePaymentSettings)
	router.GET("/recipient-upi/:mobile", user.GetRecipientUpi)

	router.DELETE("/delete-account", user.DeleteAccount)
}
