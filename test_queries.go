package main

import (
	"fmt"
	"log"
	"split-udhar-apis/config"
	"split-udhar-apis/models"
)

func main() {
	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	mobile := "919999999999"

	var transactions []models.Transaction
	u10 := "9999999999"

	fmt.Println("Running GetDashboardData query...")
	err = db.
		Select("transactions.*").
		Joins("LEFT JOIN `groups` ON transactions.group_id = `groups`.id").
		Where("`groups`.deleted_at IS NULL OR transactions.group_id IS NULL").
		Where(
			"transactions.from_mobile = ? OR transactions.to_mobile = ? OR RIGHT(transactions.from_mobile, 10) = ? OR RIGHT(transactions.to_mobile, 10) = ?",
			mobile, mobile, u10, u10,
		).
		Find(&transactions).Error

	if err != nil {
		fmt.Printf("GetDashboardData error: %v\n", err)
	} else {
		fmt.Println("GetDashboardData succeeded")
	}

	fmt.Println("Running GetUserTransactions query...")
	err = db.
		Select("transactions.*, CASE WHEN `groups`.deleted_at IS NOT NULL THEN true ELSE false END as is_archived").
		Joins("LEFT JOIN `groups` ON transactions.group_id = `groups`.id").
		Where(
			"transactions.from_mobile = ? OR transactions.to_mobile = ? OR RIGHT(transactions.from_mobile, 10) = ? OR RIGHT(transactions.to_mobile, 10) = ?",
			mobile, mobile, u10, u10,
		).
		Order("transactions.transaction_date DESC").
		Find(&transactions).Error

	if err != nil {
		fmt.Printf("GetUserTransactions error: %v\n", err)
	} else {
		fmt.Println("GetUserTransactions succeeded")
	}
}
