package api

import (
	"allinonekey/internal/model"
	"allinonekey/internal/service"
	"allinonekey/internal/util"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type KeyHandler struct {
	DB           *gorm.DB
	QuotaService *service.QuotaService
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
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "CHECK_KEY_QUOTA", Detail: fmt.Sprintf("Checked API key quota status: %s", result.Status), IP: c.ClientIP()})
	if result.Error != "" {
		c.JSON(200, gin.H{"status": result.Status, "error": result.Error})
		return
	}
	c.JSON(200, gin.H{"status": result.Status})
}

func (h *KeyHandler) CreateBulk(c *gin.Context) {
	var in struct {
		Provider string `json:"provider" binding:"required"`
		Group    string `json:"pool_group"`
		BaseURL  string `json:"base_url"`
		ProxyURL string `json:"proxy_url"`
		RawKeys  string `json:"raw_keys" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if in.Group == "" {
		in.Group = "default"
	}

	masterKey := c.GetString("master_key")
	lines := strings.Split(in.RawKeys, "\n")
	count := 0
	for _, line := range lines {
		val := strings.TrimSpace(line)
		if val == "" {
			continue
		}
		enc, err := util.EncryptToString(val, masterKey)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to encrypt key"})
			return
		}
		h.DB.Create(&model.APIKey{
			UserID:    c.GetUint("user_id"),
			Provider:  in.Provider,
			PoolGroup: in.Group,
			KeyName:   in.Provider + "-" + util.ShortID(6),
			KeyValue:  enc,
			BaseURL:   in.BaseURL,
			ProxyURL:  in.ProxyURL,
			Status:    "active",
		})
		count++
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "BULK_ADD_KEY", Detail: fmt.Sprintf("Added %d keys", count), IP: c.ClientIP()})
	c.JSON(200, gin.H{"message": "success", "count": count})
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
