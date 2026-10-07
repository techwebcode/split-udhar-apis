package controllers

import (
	"io"
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

func (g *GroupController) UpdateGroup(c *gin.Context) {
	idParam := c.Param("id")
	groupID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid group ID",
		})
		return
	}

	var req dto.UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	userMobile := c.GetString("mobile")
	err = g.Service.UpdateGroup(uint(groupID), userMobile, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group details updated successfully",
	})
}


func (g *GroupController) GetGroupImage(c *gin.Context) {
	idParam := c.Param("id")
	groupID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid group ID"})
		return
	}

	userMobile := c.GetString("mobile")
	res, err := g.Service.GetGroupImage(uint(groupID), userMobile)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "unauthorized to view group" {
			status = http.StatusForbidden
		} else if err.Error() == "group not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": res})
}

func (g *GroupController) UploadGroupImage(c *gin.Context) {
	idParam := c.Param("id")
	groupID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid group ID"})
		return
	}

	userMobile := c.GetString("mobile")
	fileHeader, err := c.FormFile("image")
	if err != nil {
		fileHeader, err = c.FormFile("file")
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "image file is required (field 'image' or 'file')"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "failed to open uploaded file"})
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "failed to read uploaded file"})
		return
	}

	res, err := g.Service.UploadGroupImage(c.Request.Context(), uint(groupID), userMobile, fileBytes)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "unauthorized to update group image" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group picture updated successfully",
		"data":    res,
	})
}

func (g *GroupController) RemoveGroupImage(c *gin.Context) {
	idParam := c.Param("id")
	groupID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid group ID"})
		return
	}

	userMobile := c.GetString("mobile")
	err = g.Service.RemoveGroupImage(c.Request.Context(), uint(groupID), userMobile)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "unauthorized to update group image" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group picture removed successfully",
	})
}
