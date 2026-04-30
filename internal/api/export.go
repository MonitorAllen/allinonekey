package api

import (
	"allinonekey/internal/model"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"io"
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

const (
	maxImportItems = 5000
	appVersion     = "0.1.0"
)

func (h *ExportHandler) ExportJSON(c *gin.Context) {
	userID := c.GetUint("user_id")
	payload := h.exportPayload(userID)
	c.Header("Content-Disposition", "attachment; filename=allinonekey-export.json")
	c.JSON(http.StatusOK, payload)
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_DATA_JSON", Detail: "Exported encrypted JSON data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportKeysJSON(c *gin.Context) {
	userID := c.GetUint("user_id")
	payload := exportPayload{ExportedAt: time.Now(), Version: appVersion, APIKeys: h.userKeys(userID)}
	c.Header("Content-Disposition", "attachment; filename=allinonekey-keys.json")
	c.JSON(http.StatusOK, payload)
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_KEYS_JSON", Detail: "Exported encrypted API Keys JSON data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportAccountsJSON(c *gin.Context) {
	userID := c.GetUint("user_id")
	payload := exportPayload{ExportedAt: time.Now(), Version: appVersion, Accounts: h.userAccounts(userID)}
	c.Header("Content-Disposition", "attachment; filename=allinonekey-accounts.json")
	c.JSON(http.StatusOK, payload)
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_ACCOUNTS_JSON", Detail: "Exported encrypted Accounts JSON data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportCSV(c *gin.Context) {
	userID := c.GetUint("user_id")
	c.Header("Content-Disposition", "attachment; filename=allinonekey-export.csv")
	c.Header("Content-Type", "text/csv")
	h.writeCSV(c.Writer, h.userKeys(userID), h.userAccounts(userID))
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_DATA_CSV", Detail: "Exported encrypted CSV data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportKeysCSV(c *gin.Context) {
	userID := c.GetUint("user_id")
	c.Header("Content-Disposition", "attachment; filename=allinonekey-keys.csv")
	c.Header("Content-Type", "text/csv")
	h.writeCSV(c.Writer, h.userKeys(userID), nil)
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_KEYS_CSV", Detail: "Exported encrypted API Keys CSV data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportAccountsCSV(c *gin.Context) {
	userID := c.GetUint("user_id")
	c.Header("Content-Disposition", "attachment; filename=allinonekey-accounts.csv")
	c.Header("Content-Type", "text/csv")
	h.writeCSV(c.Writer, nil, h.userAccounts(userID))
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_ACCOUNTS_CSV", Detail: "Exported encrypted Accounts CSV data", IP: c.ClientIP()})
}

func (h *ExportHandler) ImportJSON(c *gin.Context) {
	var payload exportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	keys, accounts, err := h.importPayload(c, payload)
	if err != nil {
		c.JSON(importStatus(err), gin.H{"error": err.Error()})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "IMPORT_DATA_JSON", Detail: "Imported encrypted JSON data", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"keys": keys, "accounts": accounts})
}

func (h *ExportHandler) ImportKeysJSON(c *gin.Context) {
	var payload exportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	keys, _, err := h.importPayload(c, exportPayload{APIKeys: payload.APIKeys})
	if err != nil {
		c.JSON(importStatus(err), gin.H{"error": err.Error()})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "IMPORT_KEYS_JSON", Detail: "Imported encrypted API Keys JSON data", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

func (h *ExportHandler) ImportAccountsJSON(c *gin.Context) {
	var payload exportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, accounts, err := h.importPayload(c, exportPayload{Accounts: payload.Accounts})
	if err != nil {
		c.JSON(importStatus(err), gin.H{"error": err.Error()})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "IMPORT_ACCOUNTS_JSON", Detail: "Imported encrypted Accounts JSON data", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"accounts": accounts})
}

func (h *ExportHandler) ImportCSV(c *gin.Context) {
	payload, err := parseImportCSV(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	keys, accounts, err := h.importPayload(c, payload)
	if err != nil {
		c.JSON(importStatus(err), gin.H{"error": err.Error()})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "IMPORT_DATA_CSV", Detail: "Imported encrypted CSV data", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"keys": keys, "accounts": accounts})
}

func (h *ExportHandler) ImportKeysCSV(c *gin.Context) {
	payload, err := parseImportCSV(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	keys, _, err := h.importPayload(c, exportPayload{APIKeys: payload.APIKeys})
	if err != nil {
		c.JSON(importStatus(err), gin.H{"error": err.Error()})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "IMPORT_KEYS_CSV", Detail: "Imported encrypted API Keys CSV data", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

func (h *ExportHandler) ImportAccountsCSV(c *gin.Context) {
	payload, err := parseImportCSV(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, accounts, err := h.importPayload(c, exportPayload{Accounts: payload.Accounts})
	if err != nil {
		c.JSON(importStatus(err), gin.H{"error": err.Error()})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "IMPORT_ACCOUNTS_CSV", Detail: "Imported encrypted Accounts CSV data", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"accounts": accounts})
}

func (h *ExportHandler) importPayload(c *gin.Context, payload exportPayload) (int, int, error) {
	if len(payload.APIKeys)+len(payload.Accounts) > maxImportItems {
		return 0, 0, clientImportError("import file too large")
	}
	userID := c.GetUint("user_id")
	importedKeys := 0
	importedAccounts := 0
	for _, k := range payload.APIKeys {
		if k.KeyValue == "" || k.Provider == "" || k.KeyName == "" {
			return 0, 0, clientImportError("api key provider, key_name and ciphertext are required")
		}
		if err := validateCiphertext(k.KeyValue); err != nil {
			return 0, 0, clientImportError("invalid api key ciphertext")
		}
		k.ID = 0
		k.UserID = userID
		if k.Status == "" {
			k.Status = "active"
		}
		if err := h.DB.Create(&k).Error; err != nil {
			return 0, 0, errors.New("failed to import api key")
		}
		importedKeys++
	}
	for _, a := range payload.Accounts {
		if a.Password == "" || a.Platform == "" {
			return 0, 0, clientImportError("account platform and password ciphertext are required")
		}
		if err := validateCiphertext(a.Password); err != nil {
			return 0, 0, clientImportError("invalid account password ciphertext")
		}
		if strings.TrimSpace(a.TOTPSecret) != "" {
			if err := validateCiphertext(a.TOTPSecret); err != nil {
				return 0, 0, clientImportError("invalid totp ciphertext")
			}
		}
		a.ID = 0
		a.UserID = userID
		a.HasTOTP = a.TOTPSecret != ""
		if a.FaviconURL == "" {
			a.FaviconURL = faviconURL(a.URL, "")
		}
		if err := h.DB.Create(&a).Error; err != nil {
			return 0, 0, errors.New("failed to import account")
		}
		importedAccounts++
	}
	return importedKeys, importedAccounts, nil
}

func (h *ExportHandler) exportPayload(userID uint) exportPayload {
	return exportPayload{ExportedAt: time.Now(), Version: appVersion, APIKeys: h.userKeys(userID), Accounts: h.userAccounts(userID)}
}

func (h *ExportHandler) userKeys(userID uint) []model.APIKey {
	var keys []model.APIKey
	h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&keys)
	return keys
}

func (h *ExportHandler) userAccounts(userID uint) []model.Account {
	var accounts []model.Account
	h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&accounts)
	return accounts
}

func (h *ExportHandler) writeCSV(w io.Writer, keys []model.APIKey, accounts []model.Account) {
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"type", "id", "name", "provider_or_platform", "account", "pool_group", "base_url", "proxy_url", "url", "ciphertext", "totp_ciphertext", "created_at"})
	for _, k := range keys {
		_ = writer.Write([]string{"api_key", strconv.FormatUint(uint64(k.ID), 10), k.KeyName, k.Provider, "", k.PoolGroup, k.BaseURL, k.ProxyURL, "", k.KeyValue, "", k.CreatedAt.Format(time.RFC3339)})
	}
	for _, a := range accounts {
		_ = writer.Write([]string{"account", strconv.FormatUint(uint64(a.ID), 10), a.Platform, a.Platform, a.Account, "", "", "", a.URL, a.Password, a.TOTPSecret, a.CreatedAt.Format(time.RFC3339)})
	}
	writer.Flush()
}

func parseImportCSV(r io.Reader) (exportPayload, error) {
	reader := csv.NewReader(r)
	rows, err := reader.ReadAll()
	if err != nil {
		return exportPayload{}, err
	}
	if len(rows) == 0 {
		return exportPayload{}, clientImportError("empty csv")
	}
	var payload exportPayload
	for _, row := range rows[1:] {
		if len(row) < 12 {
			return exportPayload{}, clientImportError("invalid csv row")
		}
		switch row[0] {
		case "api_key":
			payload.APIKeys = append(payload.APIKeys, model.APIKey{
				KeyName:   row[2],
				Provider:  row[3],
				PoolGroup: row[5],
				BaseURL:   row[6],
				ProxyURL:  row[7],
				KeyValue:  row[9],
				Status:    "active",
			})
		case "account":
			payload.Accounts = append(payload.Accounts, model.Account{
				Platform:   row[3],
				Account:    row[4],
				URL:        row[8],
				Password:   row[9],
				TOTPSecret: row[10],
			})
		default:
			return exportPayload{}, clientImportError("unknown csv row type")
		}
	}
	return payload, nil
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

type clientImportError string

func (e clientImportError) Error() string { return string(e) }

func importStatus(err error) int {
	var clientErr clientImportError
	if errors.As(err, &clientErr) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
