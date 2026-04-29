package api

import (
	"allinonekey/internal/model"
	"allinonekey/internal/util"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AccountHandler struct {
	DB *gorm.DB
}

func (h *AccountHandler) List(c *gin.Context) {
	var accs []model.Account
	h.DB.Where("user_id = ?", c.GetUint("user_id")).Order("created_at desc").Find(&accs)
	c.JSON(http.StatusOK, accs)
}

func (h *AccountHandler) Create(c *gin.Context) {
	var in struct {
		Platform   string `json:"platform" binding:"required"`
		URL        string `json:"url"`
		Account    string `json:"account"`
		Password   string `json:"password" binding:"required"`
		TOTPSecret string `json:"totp_secret"`
		FaviconURL string `json:"favicon_url"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	masterKey := c.GetString("master_key")
	password, err := util.EncryptToString(in.Password, masterKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt password"})
		return
	}
	var encryptedTOTP string
	if strings.TrimSpace(in.TOTPSecret) != "" {
		if _, _, err := util.GenerateTOTP(in.TOTPSecret, time.Now()); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		encryptedTOTP, err = util.EncryptToString(strings.TrimSpace(in.TOTPSecret), masterKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt TOTP secret"})
			return
		}
	}
	account := model.Account{
		UserID:     c.GetUint("user_id"),
		Platform:   in.Platform,
		URL:        in.URL,
		Account:    in.Account,
		Password:   password,
		TOTPSecret: encryptedTOTP,
		HasTOTP:    encryptedTOTP != "",
		FaviconURL: faviconURL(in.URL, in.FaviconURL),
	}
	if err := h.DB.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: account.UserID, Action: "CREATE_ACCOUNT", Detail: "Platform: " + account.Platform, IP: c.ClientIP()})
	c.JSON(http.StatusOK, account)
}

func (h *AccountHandler) Update(c *gin.Context) {
	var in struct {
		Platform   string `json:"platform"`
		URL        string `json:"url"`
		Account    string `json:"account"`
		Password   string `json:"password"`
		TOTPSecret string `json:"totp_secret"`
		FaviconURL string `json:"favicon_url"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{}
	if in.Platform != "" {
		updates["platform"] = in.Platform
	}
	if in.URL != "" {
		updates["url"] = in.URL
		updates["favicon_url"] = faviconURL(in.URL, in.FaviconURL)
	}
	if in.FaviconURL != "" {
		updates["favicon_url"] = in.FaviconURL
	}
	if in.Account != "" {
		updates["account"] = in.Account
	}
	if in.Password != "" {
		enc, err := util.EncryptToString(in.Password, c.GetString("master_key"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt password"})
			return
		}
		updates["password"] = enc
	}
	if strings.TrimSpace(in.TOTPSecret) != "" {
		if _, _, err := util.GenerateTOTP(in.TOTPSecret, time.Now()); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		enc, err := util.EncryptToString(strings.TrimSpace(in.TOTPSecret), c.GetString("master_key"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt TOTP secret"})
			return
		}
		updates["totp_secret"] = enc
		updates["has_totp"] = true
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}
	res := h.DB.Model(&model.Account{}).Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).Updates(updates)
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "UPDATE_ACCOUNT", Detail: "Updated account", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *AccountHandler) Delete(c *gin.Context) {
	res := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).Delete(&model.Account{})
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "DELETE_ACCOUNT", Detail: "Deleted account", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AccountHandler) Decrypt(c *gin.Context) {
	var a model.Account
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).First(&a).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	val, err := util.DecryptToString(a.Password, c.GetString("master_key"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "decrypt failed"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "DECRYPT_ACCOUNT", Detail: "Decrypted account password", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"password": val})
}

func (h *AccountHandler) TOTP(c *gin.Context) {
	var a model.Account
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).First(&a).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if a.TOTPSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOTP is not configured"})
		return
	}
	secret, err := util.DecryptToString(a.TOTPSecret, c.GetString("master_key"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "decrypt failed"})
		return
	}
	code, remaining, err := util.GenerateTOTP(secret, time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "GENERATE_TOTP", Detail: "Generated account TOTP", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"code": code, "remaining": remaining})
}

func faviconURL(rawURL string, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	if strings.TrimSpace(rawURL) == "" {
		return ""
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return ""
	}
	return "https://www.google.com/s2/favicons?domain=" + url.QueryEscape(u.Host) + "&sz=64"
}
