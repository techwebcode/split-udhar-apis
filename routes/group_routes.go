package routes

import (
	"split-udhar-apis/controllers"
	"split-udhar-apis/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GroupRoutes(router *gin.RouterGroup, db *gorm.DB) {
	controller := controllers.NewGroupController(db)

	groups := router.Group("/groups")
	groups.Use(middleware.AuthMiddleware())
	{
		groups.POST("", controller.Create)
		groups.GET("", controller.GetUserGroups)
		groups.GET("/:id", controller.GetGroupDetails)
		groups.PUT("/:id", controller.UpdateGroup)
		groups.DELETE("/:id", controller.DeleteGroup)
		groups.POST("/:id/expenses", controller.AddExpense)
		groups.PUT("/:id/expenses/:expense_id", controller.UpdateExpense)
		groups.DELETE("/:id/expenses/:expense_id", controller.DeleteExpense)
		groups.GET("/expenses/:expense_id/logs", controller.GetExpenseEditLogs)
		groups.POST("/:id/settle", controller.Settle)
		groups.POST("/:id/members", controller.AddMember)
		groups.DELETE("/:id/members/:mobile", controller.RemoveMember)
	}
}
