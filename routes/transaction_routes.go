package routes

import (
	"split-udhar-apis/controllers"
	"split-udhar-apis/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TransactionRoutes(router *gin.RouterGroup, db *gorm.DB) {

	controller := controllers.NewTransactionController(db)

	transactions := router.Group("/transactions")

	transactions.Use(middleware.AuthMiddleware())

	{
		transactions.POST("", controller.Create)

		transactions.GET("", controller.GetAll)

		transactions.GET("/dashboard", controller.Dashboard)

		transactions.GET("/logs/:id", controller.GetEditLogs)

		transactions.GET("/:mobile", controller.GetHistory)

		transactions.PUT("/:id", controller.Update)

		transactions.DELETE("/:id", controller.Delete)
	}
}
