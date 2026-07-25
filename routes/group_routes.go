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
		groups.DELETE("/:id", controller.DeleteGroup)
		groups.POST("/:id/expenses", controller.AddExpense)
		groups.POST("/:id/settle", controller.Settle)
		groups.POST("/:id/members", controller.AddMember)
		groups.DELETE("/:id/members/:mobile", controller.RemoveMember)
	}
}
