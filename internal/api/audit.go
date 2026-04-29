package api

import (
	"allinonekey/internal/model"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuditHandler struct {
	DB *gorm.DB
}

func (h *AuditHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("user_role")

	page := parsePositiveInt(c.DefaultQuery("page", "1"), 1)
	pageSize := parsePositiveInt(c.DefaultQuery("page_size", "20"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	query := h.DB.Model(&model.AuditLog{})
	if role != "admin" {
		query = query.Where("user_id = ?", userID)
	}
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		query = query.Where("action = ?", action)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("action LIKE ? OR detail LIKE ? OR ip LIKE ?", like, like, like)
	}
	if start := strings.TrimSpace(c.Query("start_time")); start != "" {
		startTime, err := time.Parse(time.RFC3339, start)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "start_time must be RFC3339"})
			return
		}
		query = query.Where("created_at >= ?", startTime)
	}
	if end := strings.TrimSpace(c.Query("end_time")); end != "" {
		endTime, err := time.Parse(time.RFC3339, end)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "end_time must be RFC3339"})
			return
		}
		query = query.Where("created_at <= ?", endTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count audit logs"})
		return
	}

	var logs []model.AuditLog
	offset := (page - 1) * pageSize
	if err := query.Order("created_at desc").Limit(pageSize).Offset(offset).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       logs,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": int(math.Ceil(float64(total) / float64(pageSize))),
	})
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
