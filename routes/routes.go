package routes

import (
	"split-udhar-apis/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupRoutes(router *gin.Engine, db *gorm.DB) {

	api := router.Group("/api")

	// Public routes
	AuthRoutes(api.Group("/auth"), db)

	// Protected routes
	protected := api.Group("/")
	protected.Use(middleware.AuthMiddleware())

	protected.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"success": true,
			"user": gin.H{
				"user_id": c.GetUint("user_id"),
				"email":   c.GetString("email"),
				"mobile":  c.GetString("mobile"),
			},
		})
	})

	UserRoutes(protected.Group("/users"), db)
	TransactionRoutes(api, db)
	GroupRoutes(api, db)
}
