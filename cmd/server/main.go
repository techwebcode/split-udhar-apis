package main

import (
	"log"
	"os"

	"split-udhar-apis/config"
	"split-udhar-apis/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found")
	}

	// Connect Database
	db, err := config.ConnectDB()
	if err != nil {
		log.Fatal(err)
	}

	// Auto migrate
	if err := config.AutoMigrate(db); err != nil {
		log.Fatal(err)
	}

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Split Udhar API Running",
		})
	})

	// Register Routes
	routes.SetupRoutes(router, db)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server running on :%s", port)

	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
