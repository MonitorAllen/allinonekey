package api

import (
	"allinonekey/internal/config"
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
	ExportedAt         time.Time                 `json:"exported_at"`
	Version            string                    `json:"version"`
	APIKeys            []model.APIKey            `json:"api_keys"`
	Accounts           []model.Account           `json:"accounts,omitempty"`
	AccountPlatforms   []model.AccountPlatform   `json:"account_platforms,omitempty"`
	AccountItems       []model.AccountItem       `json:"account_items,omitempty"`
	AccountCredentials []model.AccountCredential `json:"account_credentials,omitempty"`
}

const maxImportItems = 5000

func (h *ExportHandler) ExportJSON(c *gin.Context) {
	userID := c.GetUint("user_id")
	payload := h.exportPayload(userID)
	c.Header("Content-Disposition", "attachment; filename=allinonekey-export.json")
	c.JSON(http.StatusOK, payload)
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_DATA_JSON", Detail: "Exported encrypted JSON data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportKeysJSON(c *gin.Context) {
	userID := c.GetUint("user_id")
	payload := exportPayload{ExportedAt: time.Now(), Version: config.AppVersion(), APIKeys: h.userKeys(userID)}
	c.Header("Content-Disposition", "attachment; filename=allinonekey-keys.json")
	c.JSON(http.StatusOK, payload)
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_KEYS_JSON", Detail: "Exported encrypted API Keys JSON data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportAccountsJSON(c *gin.Context) {
	userID := c.GetUint("user_id")
	payload := h.accountExportPayload(userID)
	c.Header("Content-Disposition", "attachment; filename=allinonekey-accounts.json")
	c.JSON(http.StatusOK, payload)
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_ACCOUNTS_JSON", Detail: "Exported encrypted Accounts JSON data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportCSV(c *gin.Context) {
	userID := c.GetUint("user_id")
	c.Header("Content-Disposition", "attachment; filename=allinonekey-export.csv")
	c.Header("Content-Type", "text/csv")
	h.writeCSV(c.Writer, h.userKeys(userID), h.userAccountPlatforms(userID), h.userAccountItems(userID), h.userAccountCredentials(userID))
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_DATA_CSV", Detail: "Exported encrypted CSV data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportKeysCSV(c *gin.Context) {
	userID := c.GetUint("user_id")
	c.Header("Content-Disposition", "attachment; filename=allinonekey-keys.csv")
	c.Header("Content-Type", "text/csv")
	h.writeCSV(c.Writer, h.userKeys(userID), nil, nil, nil)
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "EXPORT_KEYS_CSV", Detail: "Exported encrypted API Keys CSV data", IP: c.ClientIP()})
}

func (h *ExportHandler) ExportAccountsCSV(c *gin.Context) {
	userID := c.GetUint("user_id")
	c.Header("Content-Disposition", "attachment; filename=allinonekey-accounts.csv")
	c.Header("Content-Type", "text/csv")
	h.writeCSV(c.Writer, nil, h.userAccountPlatforms(userID), h.userAccountItems(userID), h.userAccountCredentials(userID))
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
	_, accounts, err := h.importPayload(c, exportPayload{Accounts: payload.Accounts, AccountPlatforms: payload.AccountPlatforms, AccountItems: payload.AccountItems, AccountCredentials: payload.AccountCredentials})
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
	_, accounts, err := h.importPayload(c, exportPayload{Accounts: payload.Accounts, AccountPlatforms: payload.AccountPlatforms, AccountItems: payload.AccountItems, AccountCredentials: payload.AccountCredentials})
	if err != nil {
		c.JSON(importStatus(err), gin.H{"error": err.Error()})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "IMPORT_ACCOUNTS_CSV", Detail: "Imported encrypted Accounts CSV data", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"accounts": accounts})
}

func (h *ExportHandler) importPayload(c *gin.Context, payload exportPayload) (int, int, error) {
	itemCount := len(payload.APIKeys) + len(payload.Accounts) + len(payload.AccountPlatforms) + len(payload.AccountItems) + len(payload.AccountCredentials)
	if itemCount > maxImportItems {
		return 0, 0, clientImportError("import file too large")
	}
	userID := c.GetUint("user_id")
	importedKeys := 0
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
	importedAccounts, err := h.importAccounts(userID, payload)
	if err != nil {
		return 0, 0, err
	}
	return importedKeys, importedAccounts, nil
}

func (h *ExportHandler) importAccounts(userID uint, payload exportPayload) (int, error) {
	oldPlatformIDs := map[uint]uint{}
	oldAccountIDs := map[uint]uint{}
	imported := 0

	for _, p := range payload.AccountPlatforms {
		if strings.TrimSpace(p.Name) == "" {
			return 0, clientImportError("account platform name is required")
		}
		oldID := p.ID
		p.ID = 0
		p.UserID = userID
		if p.FaviconURL == "" {
			p.FaviconURL = faviconURL(p.URL, "")
		}
		if err := h.DB.Create(&p).Error; err != nil {
			return 0, errors.New("failed to import account platform")
		}
		oldPlatformIDs[oldID] = p.ID
	}

	for _, a := range payload.Accounts {
		if a.Password == "" || a.Platform == "" {
			return 0, clientImportError("account platform and password ciphertext are required")
		}
		if err := validateAccountCiphertexts(a.Password, a.TOTPSecret); err != nil {
			return 0, err
		}
		platformID, err := h.ensureImportedPlatform(userID, a.Platform, a.URL, a.FaviconURL)
		if err != nil {
			return 0, err
		}
		item := model.AccountItem{UserID: userID, PlatformID: platformID, Account: a.Account, Password: a.Password, TOTPSecret: a.TOTPSecret, HasTOTP: a.TOTPSecret != ""}
		if err := h.DB.Create(&item).Error; err != nil {
			return 0, errors.New("failed to import account")
		}
		imported++
	}

	for _, item := range payload.AccountItems {
		if item.Password == "" || item.PlatformID == 0 {
			return 0, clientImportError("account platform_id and password ciphertext are required")
		}
		if err := validateAccountCiphertexts(item.Password, item.TOTPSecret); err != nil {
			return 0, err
		}
		oldID := item.ID
		mappedPlatformID, ok := oldPlatformIDs[item.PlatformID]
		if !ok {
			return 0, clientImportError("account item references missing platform")
		}
		item.ID = 0
		item.UserID = userID
		item.PlatformID = mappedPlatformID
		item.HasTOTP = item.TOTPSecret != ""
		if err := h.DB.Create(&item).Error; err != nil {
			return 0, errors.New("failed to import account")
		}
		oldAccountIDs[oldID] = item.ID
		imported++
	}

	for _, credential := range payload.AccountCredentials {
		if credential.Value == "" || credential.Name == "" || credential.AccountID == 0 {
			return 0, clientImportError("credential account_id, name and ciphertext are required")
		}
		if err := validateCiphertext(credential.Value); err != nil {
			return 0, clientImportError("invalid credential ciphertext")
		}
		mappedAccountID, ok := oldAccountIDs[credential.AccountID]
		if !ok {
			return 0, clientImportError("credential references missing account")
		}
		credential.ID = 0
		credential.UserID = userID
		credential.AccountID = mappedAccountID
		if err := h.DB.Create(&credential).Error; err != nil {
			return 0, errors.New("failed to import account credential")
		}
	}
	return imported, nil
}

func (h *ExportHandler) ensureImportedPlatform(userID uint, name string, rawURL string, favicon string) (uint, error) {
	var platform model.AccountPlatform
	if err := h.DB.Where("user_id = ? AND name = ?", userID, name).First(&platform).Error; err == nil {
		return platform.ID, nil
	}
	platform = model.AccountPlatform{UserID: userID, Name: name, URL: rawURL, FaviconURL: faviconURL(rawURL, favicon)}
	if err := h.DB.Create(&platform).Error; err != nil {
		return 0, errors.New("failed to import account platform")
	}
	return platform.ID, nil
}

func (h *ExportHandler) exportPayload(userID uint) exportPayload {
	payload := h.accountExportPayload(userID)
	payload.APIKeys = h.userKeys(userID)
	return payload
}

func (h *ExportHandler) accountExportPayload(userID uint) exportPayload {
	return exportPayload{ExportedAt: time.Now(), Version: config.AppVersion(), AccountPlatforms: h.userAccountPlatforms(userID), AccountItems: h.userAccountItems(userID), AccountCredentials: h.userAccountCredentials(userID)}
}

func (h *ExportHandler) userKeys(userID uint) []model.APIKey {
	var keys []model.APIKey
	h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&keys)
	return keys
}

func (h *ExportHandler) userAccountPlatforms(userID uint) []model.AccountPlatform {
	var platforms []model.AccountPlatform
	h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&platforms)
	return platforms
}

func (h *ExportHandler) userAccountItems(userID uint) []model.AccountItem {
	var items []model.AccountItem
	h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&items)
	return items
}

func (h *ExportHandler) userAccountCredentials(userID uint) []model.AccountCredential {
	var credentials []model.AccountCredential
	h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&credentials)
	return credentials
}

func (h *ExportHandler) writeCSV(w io.Writer, keys []model.APIKey, platforms []model.AccountPlatform, items []model.AccountItem, credentials []model.AccountCredential) {
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"type", "id", "name", "provider_or_platform", "account", "pool_group", "base_url", "proxy_url", "url", "ciphertext", "totp_ciphertext", "created_at", "provider_url", "provider_icon", "note", "platform_id", "account_id", "expires_at"})
	for _, k := range keys {
		_ = writer.Write([]string{"api_key", strconv.FormatUint(uint64(k.ID), 10), k.KeyName, k.Provider, "", k.PoolGroup, k.BaseURL, k.ProxyURL, "", k.KeyValue, "", k.CreatedAt.Format(time.RFC3339), k.ProviderURL, k.ProviderIcon, k.Note, "", "", ""})
	}
	for _, p := range platforms {
		_ = writer.Write([]string{"account_platform", strconv.FormatUint(uint64(p.ID), 10), p.Name, p.Name, "", "", "", "", p.URL, "", "", p.CreatedAt.Format(time.RFC3339), "", p.FaviconURL, p.Note, "", "", ""})
	}
	for _, a := range items {
		_ = writer.Write([]string{"account", strconv.FormatUint(uint64(a.ID), 10), "", "", a.Account, "", "", "", "", a.Password, a.TOTPSecret, a.CreatedAt.Format(time.RFC3339), "", "", a.Note, strconv.FormatUint(uint64(a.PlatformID), 10), "", ""})
	}
	for _, credential := range credentials {
		expiresAt := ""
		if credential.ExpiresAt != nil {
			expiresAt = credential.ExpiresAt.Format(time.RFC3339)
		}
		_ = writer.Write([]string{"account_credential", strconv.FormatUint(uint64(credential.ID), 10), credential.Name, "", "", "", "", "", "", credential.Value, "", credential.CreatedAt.Format(time.RFC3339), "", "", credential.Note, "", strconv.FormatUint(uint64(credential.AccountID), 10), expiresAt})
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
		row = paddedRow(row, 18)
		switch row[0] {
		case "api_key":
			payload.APIKeys = append(payload.APIKeys, model.APIKey{KeyName: row[2], Provider: row[3], PoolGroup: row[5], BaseURL: row[6], ProxyURL: row[7], KeyValue: row[9], Status: "active", ProviderURL: row[12], ProviderIcon: row[13], Note: row[14]})
		case "account_platform":
			id, _ := strconv.ParseUint(row[1], 10, 64)
			payload.AccountPlatforms = append(payload.AccountPlatforms, model.AccountPlatform{ID: uint(id), Name: row[2], URL: row[8], FaviconURL: row[13], Note: row[14]})
		case "account":
			platformID, _ := strconv.ParseUint(row[15], 10, 64)
			if platformID == 0 && row[3] != "" {
				payload.Accounts = append(payload.Accounts, model.Account{Platform: row[3], Account: row[4], URL: row[8], Password: row[9], TOTPSecret: row[10]})
				continue
			}
			id, _ := strconv.ParseUint(row[1], 10, 64)
			payload.AccountItems = append(payload.AccountItems, model.AccountItem{ID: uint(id), PlatformID: uint(platformID), Account: row[4], Password: row[9], TOTPSecret: row[10], Note: row[14]})
		case "account_credential":
			id, _ := strconv.ParseUint(row[1], 10, 64)
			accountID, _ := strconv.ParseUint(row[16], 10, 64)
			var expiresAt *time.Time
			if row[17] != "" {
				parsed, err := time.Parse(time.RFC3339, row[17])
				if err != nil {
					return exportPayload{}, clientImportError("invalid credential expires_at")
				}
				expiresAt = &parsed
			}
			payload.AccountCredentials = append(payload.AccountCredentials, model.AccountCredential{ID: uint(id), AccountID: uint(accountID), Name: row[2], Value: row[9], Note: row[14], ExpiresAt: expiresAt})
		default:
			return exportPayload{}, clientImportError("unknown csv row type")
		}
	}
	return payload, nil
}

func validateAccountCiphertexts(password string, totp string) error {
	if err := validateCiphertext(password); err != nil {
		return clientImportError("invalid account password ciphertext")
	}
	if strings.TrimSpace(totp) != "" {
		if err := validateCiphertext(totp); err != nil {
			return clientImportError("invalid totp ciphertext")
		}
	}
	return nil
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

func paddedRow(row []string, size int) []string {
	for len(row) < size {
		row = append(row, "")
	}
	return row
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
