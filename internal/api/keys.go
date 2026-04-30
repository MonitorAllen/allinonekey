package api

import (
	"allinonekey/internal/model"
	"allinonekey/internal/service"
	"allinonekey/internal/util"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type KeyHandler struct {
	DB           *gorm.DB
	QuotaService *service.QuotaService
}

type keyInput struct {
	KeyName  string `json:"key_name"`
	KeyValue string `json:"key_value"`
}

func (h *KeyHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	q := c.Query("q")
	var keys []model.APIKey
	db := h.DB.Where("user_id = ?", userID)
	if q != "" {
		db = db.Where("provider LIKE ? OR key_name LIKE ? OR pool_group LIKE ?", "%"+q+"%", "%"+q+"%", "%"+q+"%")
	}
	db.Order("created_at desc").Find(&keys)
	c.JSON(200, keys)
}

func (h *KeyHandler) GetStats(c *gin.Context) {
	userID := c.GetUint("user_id")
	var stats struct {
		Total   int64   `json:"total"`
		Active  int64   `json:"active"`
		Error   int64   `json:"error"`
		Balance float64 `json:"balance"`
	}
	h.DB.Model(&model.APIKey{}).Where("user_id = ?", userID).Count(&stats.Total)
	h.DB.Model(&model.APIKey{}).Where("user_id = ? AND status = 'active'", userID).Count(&stats.Active)
	h.DB.Model(&model.APIKey{}).Where("user_id = ? AND status NOT IN ?", userID, []string{"active", "quota_unsupported"}).Count(&stats.Error)
	h.DB.Model(&model.APIKey{}).Where("user_id = ?", userID).Select("COALESCE(SUM(quota_balance), 0)").Scan(&stats.Balance)
	c.JSON(200, stats)
}

func (h *KeyHandler) CheckQuota(c *gin.Context) {
	var k model.APIKey
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).First(&k).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	if h.QuotaService == nil {
		h.QuotaService = &service.QuotaService{DB: h.DB}
	}
	result := h.QuotaService.UpdateQuotaForKey(k, c.GetString("master_key"))
	h.DB.Where("id = ? AND user_id = ?", k.ID, c.GetUint("user_id")).First(&k)
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "CHECK_KEY_HEALTH", Detail: fmt.Sprintf("Checked API key health status: %s", result.Status), IP: c.ClientIP()})
	response := gin.H{
		"status":        result.Status,
		"quota_total":   k.QuotaTotal,
		"quota_used":    k.QuotaUsed,
		"quota_balance": k.QuotaBalance,
		"last_check":    k.LastCheck,
	}
	if result.Error != "" {
		response["error"] = result.Error
	}
	c.JSON(200, response)
}

func (h *KeyHandler) ListModels(c *gin.Context) {
	var k model.APIKey
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).First(&k).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	if h.QuotaService == nil {
		h.QuotaService = &service.QuotaService{DB: h.DB}
	}
	result := h.QuotaService.ListModelsForKey(k, c.GetString("master_key"))
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "LIST_KEY_MODELS", Detail: fmt.Sprintf("Listed API key models: %s", result.Status), IP: c.ClientIP()})
	response := gin.H{"status": result.Status, "models": result.Models}
	if result.Error != "" {
		response["error"] = result.Error
	}
	c.JSON(200, response)
}

func (h *KeyHandler) CreateBulk(c *gin.Context) {
	var in struct {
		Provider string     `json:"provider" binding:"required"`
		Group    string     `json:"pool_group"`
		BaseURL  string     `json:"base_url"`
		ProxyURL string     `json:"proxy_url"`
		RawKeys  string     `json:"raw_keys"`
		Keys     []keyInput `json:"keys"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if in.Group == "" {
		in.Group = "default"
	}
	items := normalizeKeyInputs(in.Keys, in.RawKeys)
	if len(items) == 0 {
		c.JSON(400, gin.H{"error": "at least one key is required"})
		return
	}

	masterKey := c.GetString("master_key")
	count := 0
	for _, item := range items {
		name := strings.TrimSpace(item.KeyName)
		val := strings.TrimSpace(item.KeyValue)
		if name == "" || val == "" {
			c.JSON(400, gin.H{"error": "key_name and key_value are required"})
			return
		}
		enc, err := util.EncryptToString(val, masterKey)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to encrypt key"})
			return
		}
		if err := h.DB.Create(&model.APIKey{
			UserID:    c.GetUint("user_id"),
			Provider:  strings.TrimSpace(in.Provider),
			PoolGroup: in.Group,
			KeyName:   name,
			KeyValue:  enc,
			BaseURL:   strings.TrimSpace(in.BaseURL),
			ProxyURL:  strings.TrimSpace(in.ProxyURL),
			Status:    "active",
		}).Error; err != nil {
			c.JSON(500, gin.H{"error": "Failed to create key"})
			return
		}
		count++
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "ADD_KEY", Detail: fmt.Sprintf("Added %d keys", count), IP: c.ClientIP()})
	c.JSON(200, gin.H{"message": "success", "count": count})
}

func normalizeKeyInputs(keys []keyInput, rawKeys string) []keyInput {
	if len(keys) > 0 {
		return keys
	}
	var items []keyInput
	for _, line := range strings.Split(rawKeys, "\n") {
		val := strings.TrimSpace(line)
		if val == "" {
			continue
		}
		items = append(items, keyInput{KeyValue: val})
	}
	return items
}

func (h *KeyHandler) Update(c *gin.Context) {
	var in struct {
		Provider  string `json:"provider"`
		PoolGroup string `json:"pool_group"`
		KeyName   string `json:"key_name"`
		BaseURL   string `json:"base_url"`
		ProxyURL  string `json:"proxy_url"`
		Status    string `json:"status"`
		KeyValue  string `json:"key_value"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{}
	if in.Provider != "" {
		updates["provider"] = in.Provider
	}
	if in.PoolGroup != "" {
		updates["pool_group"] = in.PoolGroup
	}
	if in.KeyName != "" {
		updates["key_name"] = in.KeyName
	}
	if in.BaseURL != "" {
		updates["base_url"] = in.BaseURL
	}
	if in.ProxyURL != "" {
		updates["proxy_url"] = in.ProxyURL
	}
	if in.Status != "" {
		updates["status"] = in.Status
	}
	if in.KeyValue != "" {
		enc, err := util.EncryptToString(in.KeyValue, c.GetString("master_key"))
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to encrypt key"})
			return
		}
		updates["key_value"] = enc
		updates["last_check"] = time.Time{}
	}
	if len(updates) == 0 {
		c.JSON(400, gin.H{"error": "No fields to update"})
		return
	}
	res := h.DB.Model(&model.APIKey{}).Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).Updates(updates)
	if res.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "UPDATE_KEY", Detail: "Updated API key metadata", IP: c.ClientIP()})
	c.JSON(200, gin.H{"message": "updated"})
}

func (h *KeyHandler) Delete(c *gin.Context) {
	res := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).Delete(&model.APIKey{})
	if res.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "DELETE_KEY", Detail: "Deleted API key", IP: c.ClientIP()})
	c.JSON(200, gin.H{"message": "deleted"})
}

func (h *KeyHandler) Decrypt(c *gin.Context) {
	var k model.APIKey
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).First(&k).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	val, err := util.DecryptToString(k.KeyValue, c.GetString("master_key"))
	if err != nil {
		c.JSON(400, gin.H{"error": "decrypt failed"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "DECRYPT_KEY", Detail: "Decrypted API key", IP: c.ClientIP()})
	c.JSON(200, gin.H{"key": val})
}
