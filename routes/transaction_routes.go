package routes

import (
	"split-udhar-apis/controllers"
	"split-udhar-apis/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TransactionRoutes(router *gin.RouterGroup, db *gorm.DB) {

	controller := controllers.NewTransactionController(db)
	payReportController := controllers.NewPaymentReportController(db)

	transactions := router.Group("/transactions")

	transactions.Use(middleware.AuthMiddleware())

	{
		transactions.POST("", controller.Create)

		transactions.GET("", controller.GetAll)

		transactions.GET("/dashboard", controller.Dashboard)

		transactions.GET("/logs/:id", controller.GetEditLogs)

		// 1-to-1 Payment Reporting and Confirmation endpoints
		// Registered before :mobile wildcard so path resolution doesn't conflict
		transactions.POST("/payment-reports", payReportController.Create)
		transactions.POST("/payment-reports/:id/confirm", payReportController.Confirm)
		transactions.POST("/payment-reports/:id/reject", payReportController.Reject)
		transactions.GET("/payment-reports/:mobile", payReportController.GetBetween)

		transactions.GET("/:mobile", controller.GetHistory)

		transactions.PUT("/:id", controller.Update)

		transactions.PUT("/:id/archive", controller.Archive)

		transactions.PUT("/:id/unarchive", controller.Unarchive)

		transactions.DELETE("/:id", controller.Delete)
	}
}
