package controllers

import (
	"log"
	"net/http"
	"split-udhar-apis/dto"
	"split-udhar-apis/services"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TransactionController struct {
	Service *services.TransactionService
}

func NewTransactionController(db *gorm.DB) *TransactionController {
	return &TransactionController{
		Service: services.NewTransactionService(db),
	}
}

func (t *TransactionController) Update(c *gin.Context) {

	idStr := c.Param("id")
	log.Printf("[API UPDATE TRANSACTION] Received Param ID: '%s'", idStr)

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		log.Printf("[API UPDATE TRANSACTION ERROR] Failed to parse ID '%s': %v", idStr, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid transaction id",
		})
		return
	}

	var req dto.UpdateTransactionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[API UPDATE TRANSACTION ERROR] Invalid JSON body for ID %d: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	userMobile := c.GetString("mobile")
	log.Printf("[API UPDATE TRANSACTION] Updating ID %d by User '%s' -> Amount: %.2f, Note: '%s'", id, userMobile, req.Amount, req.Note)

	err = t.Service.UpdateTransaction(
		uint(id),
		userMobile,
		req,
	)

	if err != nil {
		log.Printf("[API UPDATE TRANSACTION ERROR] ID %d: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	log.Printf("[API UPDATE TRANSACTION SUCCESS] Successfully updated Transaction ID %d", id)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Transaction updated successfully",
	})
}

func (t *TransactionController) Create(c *gin.Context) {

	var req dto.CreateTransactionRequest

	// Validate request body
	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Get logged-in user's mobile from JWT middleware
	userMobile := c.GetString("mobile")

	if userMobile == "" {

		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User mobile not found",
		})
		return
	}

	// Create transaction
	err := t.Service.CreateTransaction(
		userMobile,
		req,
	)

	if err != nil {

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Transaction created successfully",
	})
}

func (t *TransactionController) Delete(c *gin.Context) {

	id, err := strconv.ParseUint(
		c.Param("id"),
		10,
		64,
	)

	if err != nil {

		c.JSON(400, gin.H{
			"success": false,
			"message": "invalid transaction id",
		})

		return
	}

	err = t.Service.DeleteTransaction(
		uint(id),
		c.GetString("mobile"),
	)

	if err != nil {

		c.JSON(400, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "Transaction deleted successfully",
	})
}

func (t *TransactionController) Dashboard(c *gin.Context) {

	mobile := c.GetString("mobile")

	data, err := t.Service.GetDashboard(mobile)

	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{

		"success": true,

		"data": data,
	})
}

func (t *TransactionController) GetAll(c *gin.Context) {

	mobile := c.GetString("mobile")

	data, err := t.Service.GetAllTransactions(mobile)

	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func (t *TransactionController) GetEditLogs(c *gin.Context) {
	idStr := c.Param("id")
	log.Printf("[API EDIT LOGS] Requesting edit logs for Transaction ID: '%s'", idStr)

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		log.Printf("[API EDIT LOGS ERROR] Failed to parse ID '%s': %v", idStr, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid transaction id",
		})
		return
	}

	logs, err := t.Service.GetTransactionEditHistory(uint(id))
	if err != nil {
		log.Printf("[API EDIT LOGS ERROR] Failed to fetch edit logs for ID %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	log.Printf("[API EDIT LOGS SUCCESS] Found %d logs for Transaction ID %d", len(logs), id)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
	})
}

func (t *TransactionController) GetHistory(c *gin.Context) {

	userMobile := c.GetString("mobile")

	contactMobile := c.Param("mobile")

	data, err := t.Service.GetTransactionHistory(
		userMobile,
		contactMobile,
	)

	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}
