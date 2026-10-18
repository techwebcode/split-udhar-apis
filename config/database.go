package config

import (
	"fmt"
	"log"
	"os"
	"split-udhar-apis/models"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
		&models.PendingUser{},
		&models.OTPVerification{},
		&models.Transaction{},
		&models.TransactionEditLog{},
		&models.Group{},
		&models.GroupMember{},
		&models.GroupExpense{},
		&models.GroupExpenseEditLog{},
		&models.UserDevice{},
		&models.NotificationOutbox{},
	); err != nil {
		return err
	}

	// Ensure existing users default to INR if preferred_currency is blank
	_ = db.Model(&models.User{}).
		Where("preferred_currency = '' OR preferred_currency IS NULL").
		Update("preferred_currency", models.CurrencyINR).Error

	return nil
}

var DB *gorm.DB

func ConnectDB() (*gorm.DB, error) {

	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found")
	}

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	DB = db

	log.Println("✅ Database connected successfully")

	return db, nil
}
