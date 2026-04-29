package api

import (
	"allinonekey/internal/model"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ExportHandler struct {
	DB *gorm.DB
}

type exportPayload struct {
	ExportedAt time.Time       `json:"exported_at"`
	Version    string          `json:"version"`
	APIKeys    []model.APIKey  `json:"api_keys"`
	Accounts   []model.Account `json:"accounts"`
}

const maxImportItems = 5000

func (h *ExportHandler) ExportJSON(c *gin.Context) {
	userID := c.GetUint("user_id")
	payload := h.exportPayload(userID)
	c.Header("Content-Disposition", "attachment; filename=allinonekey-export.json")
	c.JSON(http.StatusOK, payload)
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_DATA_JSON", Detail: "Exported encrypted JSON data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportCSV(c *gin.Context) {
	userID := c.GetUint("user_id")
	var keys []model.APIKey
	var accounts []model.Account
	h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&keys)
	h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&accounts)

	c.Header("Content-Disposition", "attachment; filename=allinonekey-export.csv")
	c.Header("Content-Type", "text/csv")
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"type", "id", "name", "provider_or_platform", "account", "pool_group", "base_url", "proxy_url", "url", "ciphertext", "totp_ciphertext", "created_at"})
	for _, k := range keys {
		_ = writer.Write([]string{"api_key", strconv.FormatUint(uint64(k.ID), 10), k.KeyName, k.Provider, "", k.PoolGroup, k.BaseURL, k.ProxyURL, "", k.KeyValue, "", k.CreatedAt.Format(time.RFC3339)})
	}
	for _, a := range accounts {
		_ = writer.Write([]string{"account", strconv.FormatUint(uint64(a.ID), 10), a.Platform, a.Platform, a.Account, "", "", "", a.URL, a.Password, a.TOTPSecret, a.CreatedAt.Format(time.RFC3339)})
	}
	writer.Flush()
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_DATA_CSV", Detail: "Exported encrypted CSV data", IP: c.ClientIP()})
}

func (h *ExportHandler) ImportJSON(c *gin.Context) {
	var payload exportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(payload.APIKeys)+len(payload.Accounts) > maxImportItems {
		c.JSON(http.StatusBadRequest, gin.H{"error": "import file too large"})
		return
	}

	userID := c.GetUint("user_id")
	importedKeys := 0
	importedAccounts := 0
	for _, k := range payload.APIKeys {
		if k.KeyValue == "" || k.Provider == "" {
			continue
		}
		if err := validateCiphertext(k.KeyValue); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key ciphertext"})
			return
		}
		k.ID = 0
		k.UserID = userID
		if k.Status == "" {
			k.Status = "active"
		}
		if err := h.DB.Create(&k).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import api key"})
			return
		}
		importedKeys++
	}
	for _, a := range payload.Accounts {
		if a.Password == "" || a.Platform == "" {
			continue
		}
		if err := validateCiphertext(a.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account password ciphertext"})
			return
		}
		if strings.TrimSpace(a.TOTPSecret) != "" {
			if err := validateCiphertext(a.TOTPSecret); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid totp ciphertext"})
				return
			}
		}
		a.ID = 0
		a.UserID = userID
		a.HasTOTP = a.TOTPSecret != ""
		if a.FaviconURL == "" {
			a.FaviconURL = faviconURL(a.URL, "")
		}
		if err := h.DB.Create(&a).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import account"})
			return
		}
		importedAccounts++
	}
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "IMPORT_DATA_JSON", Detail: "Imported encrypted data", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"keys": importedKeys, "accounts": importedAccounts})
}

func (h *ExportHandler) exportPayload(userID uint) exportPayload {
	var keys []model.APIKey
	var accounts []model.Account
	h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&keys)
	h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&accounts)
	return exportPayload{ExportedAt: time.Now(), Version: "0.0.0", APIKeys: keys, Accounts: accounts}
}

func validateCiphertext(value string) error {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	if len(decoded) <= 12 {
		return errors.New("ciphertext too short")
	}
	return nil
}
