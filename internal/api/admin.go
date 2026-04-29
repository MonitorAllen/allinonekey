package api

import (
	"allinonekey/internal/model"
	"allinonekey/internal/util"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const defaultInviteTTL = 7 * 24 * time.Hour

type AdminHandler struct {
	DB *gorm.DB
}

func (h *AdminHandler) ListInvites(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := h.DB.Model(&model.InvitationCode{})
	status := c.Query("status")
	now := time.Now()
	if status == "used" {
		query = query.Where("is_used = ?", true)
	} else if status == "available" {
		query = query.Where("is_used = ? AND (expires_at IS NULL OR expires_at > ?)", false, now)
	} else if status == "expired" {
		query = query.Where("is_used = ? AND expires_at IS NOT NULL AND expires_at <= ?", false, now)
	}

	var total int64
	query.Count(&total)

	var invites []model.InvitationCode
	query.Order("created_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&invites)

	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       invites,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	})
}

func (h *AdminHandler) CreateInvite(c *gin.Context) {
	var in struct {
		ExpiresInHours int `json:"expires_in_hours"`
	}
	_ = c.ShouldBindJSON(&in)
	userID := c.GetUint("user_id")
	code := "INV-" + util.ShortID(12)
	expiresIn := defaultInviteTTL
	if in.ExpiresInHours > 0 {
		expiresIn = time.Duration(in.ExpiresInHours) * time.Hour
	}
	expiresAt := time.Now().Add(expiresIn)

	invite := model.InvitationCode{
		Code:      code,
		CreatedBy: userID,
		ExpiresAt: &expiresAt,
	}

	if err := h.DB.Create(&invite).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate code"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "CREATE_INVITE", Detail: "Created invite code", IP: c.ClientIP()})
	c.JSON(http.StatusOK, invite)
}

func (h *AdminHandler) DeleteInvite(c *gin.Context) {
	var invite model.InvitationCode
	if err := h.DB.Where("id = ?", c.Param("id")).First(&invite).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if invite.IsUsed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Used invite cannot be deleted"})
		return
	}
	if err := h.DB.Delete(&invite).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete invite"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "DELETE_INVITE", Detail: "Deleted unused invite code", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	var users []model.User
	h.DB.Find(&users)
	c.JSON(http.StatusOK, users)
}

func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
	var input struct {
		UserID uint   `json:"user_id" binding:"required"`
		Role   string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.DB.Model(&model.User{}).Where("id = ?", input.UserID).Update("role", input.Role)
	c.JSON(http.StatusOK, gin.H{"message": "Role updated"})
}
