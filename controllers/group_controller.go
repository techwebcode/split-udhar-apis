package controllers

import (
	"net/http"
	"split-udhar-apis/dto"
	"split-udhar-apis/services"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GroupController struct {
	Service *services.GroupService
}

func NewGroupController(db *gorm.DB) *GroupController {
	return &GroupController{
		Service: services.NewGroupService(db),
	}
}

func (g *GroupController) Create(c *gin.Context) {
	var req dto.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	userMobile := c.GetString("mobile")
	group, err := g.Service.CreateGroup(userMobile, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Group created successfully",
		"data":    group,
	})
}

func (g *GroupController) GetUserGroups(c *gin.Context) {
	userMobile := c.GetString("mobile")
	groups, err := g.Service.GetUserGroups(userMobile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    groups,
	})
}

func (g *GroupController) GetGroupDetails(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid group id",
		})
		return
	}

	userMobile := c.GetString("mobile")
	group, err := g.Service.GetGroupDetails(uint(id), userMobile)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    group,
	})
}

func (g *GroupController) AddMember(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid group id",
		})
		return
	}

	var req dto.GroupMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	userMobile := c.GetString("mobile")
	err = g.Service.AddMember(uint(id), userMobile, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Member added to group successfully",
	})
}

func (g *GroupController) RemoveMember(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid group id",
		})
		return
	}

	targetMobile := c.Param("mobile")
	userMobile := c.GetString("mobile")

	err = g.Service.RemoveMember(uint(id), userMobile, targetMobile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Member removed from group successfully",
	})
}

func (g *GroupController) AddExpense(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid group id",
		})
		return
	}

	var req dto.AddGroupExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	userMobile := c.GetString("mobile")
	err = g.Service.AddGroupExpense(uint(id), userMobile, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group expense added successfully",
	})
}

func (g *GroupController) DeleteGroup(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid group id",
		})
		return
	}

	userMobile := c.GetString("mobile")
	err = g.Service.DeleteGroup(uint(id), userMobile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group deleted successfully",
	})
}

func (g *GroupController) Settle(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid group id",
		})
		return
	}

	var req dto.SettleGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	userMobile := c.GetString("mobile")
	err = g.Service.SettleGroup(uint(id), userMobile, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group settled up successfully",
	})
}

func (g *GroupController) DeleteExpense(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := strconv.ParseUint(groupIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid group id"})
		return
	}

	expenseIDStr := c.Param("expense_id")
	expenseID, err := strconv.ParseUint(expenseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid expense id"})
		return
	}

	userMobile := c.GetString("mobile")
	err = g.Service.DeleteGroupExpense(uint(groupID), uint(expenseID), userMobile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Group expense deleted successfully"})
}

func (g *GroupController) UpdateExpense(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := strconv.ParseUint(groupIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid group id"})
		return
	}

	expenseIDStr := c.Param("expense_id")
	expenseID, err := strconv.ParseUint(expenseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid expense id"})
		return
	}

	var req dto.UpdateGroupExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	userMobile := c.GetString("mobile")
	err = g.Service.UpdateGroupExpense(uint(groupID), uint(expenseID), userMobile, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Group expense updated successfully"})
}

func (g *GroupController) GetExpenseEditLogs(c *gin.Context) {
	expenseIDStr := c.Param("expense_id")
	expenseID, err := strconv.ParseUint(expenseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid expense id"})
		return
	}

	logs, err := g.Service.GetGroupExpenseEditHistory(uint(expenseID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": logs})
}
