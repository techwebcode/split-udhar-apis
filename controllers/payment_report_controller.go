package controllers

import (
	"net/http"
	"strconv"

	"split-udhar-apis/dto"
	"split-udhar-apis/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PaymentReportController struct {
	Service *services.PaymentReportService
}

func NewPaymentReportController(db *gorm.DB) *PaymentReportController {
	return &PaymentReportController{
		Service: services.NewPaymentReportService(db),
	}
}

func (ctrl *PaymentReportController) Create(c *gin.Context) {
	var req dto.CreatePaymentReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	userMobile := c.GetString("mobile")
	if userMobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User unauthorized",
		})
		return
	}

	report, err := ctrl.Service.CreateReport(userMobile, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Payment reported successfully. Awaiting recipient confirmation.",
		"data":    report,
	})
}

func (ctrl *PaymentReportController) Confirm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid payment report id",
		})
		return
	}

	userMobile := c.GetString("mobile")
	if userMobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User unauthorized",
		})
		return
	}

	report, err := ctrl.Service.ConfirmReport(uint(id), userMobile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Payment confirmed and balance settled successfully",
		"data":    report,
	})
}

func (ctrl *PaymentReportController) Reject(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid payment report id",
		})
		return
	}

	userMobile := c.GetString("mobile")
	if userMobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User unauthorized",
		})
		return
	}

	report, err := ctrl.Service.RejectReport(uint(id), userMobile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Payment report rejected successfully",
		"data":    report,
	})
}

func (ctrl *PaymentReportController) GetBetween(c *gin.Context) {
	userMobile := c.GetString("mobile")
	contactMobile := c.Param("mobile")

	reports, err := ctrl.Service.GetReportsBetween(userMobile, contactMobile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    reports,
	})
}
