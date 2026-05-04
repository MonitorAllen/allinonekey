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

type accountPlatformDTO struct {
	ID         uint             `json:"id"`
	UserID     uint             `json:"user_id"`
	Name       string           `json:"name"`
	Platform   string           `json:"platform"`
	URL        string           `json:"url"`
	FaviconURL string           `json:"favicon_url"`
	Note       string           `json:"note"`
	Items      []accountItemDTO `json:"items"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

type accountItemDTO struct {
	ID          uint                `json:"id"`
	UserID      uint                `json:"user_id"`
	PlatformID  uint                `json:"platform_id"`
	Platform    string              `json:"platform"`
	Account     string              `json:"account"`
	Password    string              `json:"password"`
	TOTPSecret  string              `json:"totp_secret"`
	HasTOTP     bool                `json:"has_totp"`
	Note        string              `json:"note"`
	Credentials []credentialListDTO `json:"credentials"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type credentialListDTO struct {
	ID        uint       `json:"id"`
	UserID    uint       `json:"user_id"`
	AccountID uint       `json:"account_id"`
	Name      string     `json:"name"`
	Note      string     `json:"note"`
	ExpiresAt *time.Time `json:"expires_at"`
	IsExpired bool       `json:"is_expired"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (h *AccountHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	ensureLegacyAccountsMigrated(h.DB, userID)

	var platforms []model.AccountPlatform
	query := h.DB.Where("user_id = ?", userID).Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at desc")
	}).Preload("Items.Credentials", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at desc")
	}).Order("created_at desc")

	q := strings.TrimSpace(c.Query("q"))
	if q != "" {
		like := "%" + q + "%"
		query = query.Where(`name LIKE ? OR url LIKE ? OR note LIKE ? OR id IN (
			SELECT platform_id FROM account_items WHERE user_id = ? AND (account LIKE ? OR note LIKE ?)
		) OR id IN (
			SELECT ai.platform_id FROM account_items ai JOIN account_credentials ac ON ac.account_id = ai.id
			WHERE ai.user_id = ? AND ac.user_id = ? AND (ac.name LIKE ? OR ac.note LIKE ?)
		)`, like, like, like, userID, like, like, userID, userID, like, like)
	}
	query.Find(&platforms)

	out := make([]accountPlatformDTO, 0, len(platforms))
	for _, p := range platforms {
		out = append(out, platformDTO(p))
	}
	c.JSON(http.StatusOK, out)
}

func (h *AccountHandler) CreatePlatform(c *gin.Context) {
	var in struct {
		Name        string `json:"name" binding:"required"`
		URL         string `json:"url"`
		FaviconURL  string `json:"favicon_url"`
		Note        string `json:"note"`
		Account     string `json:"account"`
		Password    string `json:"password"`
		TOTPSecret  string `json:"totp_secret"`
		AccountNote string `json:"account_note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetUint("user_id")
	withAccount := strings.TrimSpace(in.Account) != "" || strings.TrimSpace(in.Password) != "" || strings.TrimSpace(in.TOTPSecret) != "" || strings.TrimSpace(in.AccountNote) != ""
	var password string
	var encryptedTOTP string
	if withAccount {
		if strings.TrimSpace(in.Account) == "" || strings.TrimSpace(in.Password) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "account and password are required when creating an account with platform"})
			return
		}
		var err error
		password, encryptedTOTP, err = encryptedAccountSecrets(in.Password, in.TOTPSecret, c.GetString("master_key"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	platform := model.AccountPlatform{UserID: userID, Name: strings.TrimSpace(in.Name), URL: strings.TrimSpace(in.URL), FaviconURL: faviconURL(in.URL, in.FaviconURL), Note: strings.TrimSpace(in.Note)}
	if err := h.DB.Create(&platform).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account platform"})
		return
	}

	if withAccount {
		account := model.AccountItem{UserID: userID, PlatformID: platform.ID, Account: strings.TrimSpace(in.Account), Password: password, TOTPSecret: encryptedTOTP, HasTOTP: encryptedTOTP != "", Note: strings.TrimSpace(in.AccountNote)}
		if err := h.DB.Create(&account).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
			return
		}
	}

	h.DB.Preload("Items").Preload("Items.Credentials").First(&platform, platform.ID)
	h.DB.Create(&model.AuditLog{UserID: platform.UserID, Action: "CREATE_ACCOUNT_PLATFORM", Detail: "Platform: " + platform.Name, IP: c.ClientIP()})
	c.JSON(http.StatusOK, platformDTO(platform))
}

func (h *AccountHandler) UpdatePlatform(c *gin.Context) {
	var in struct {
		Name       string `json:"name"`
		URL        string `json:"url"`
		FaviconURL string `json:"favicon_url"`
		Note       string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{"name": strings.TrimSpace(in.Name), "url": strings.TrimSpace(in.URL), "favicon_url": faviconURL(in.URL, in.FaviconURL), "note": in.Note}
	if updates["name"] == "" {
		delete(updates, "name")
	}
	res := h.DB.Model(&model.AccountPlatform{}).Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).Updates(updates)
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "UPDATE_ACCOUNT_PLATFORM", Detail: "Updated account platform", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *AccountHandler) DeletePlatform(c *gin.Context) {
	userID := c.GetUint("user_id")
	platformID := c.Param("id")
	var platform model.AccountPlatform
	if err := h.DB.Where("id = ? AND user_id = ?", platformID, userID).First(&platform).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		var itemIDs []uint
		if err := tx.Model(&model.AccountItem{}).Where("platform_id = ? AND user_id = ?", platform.ID, userID).Pluck("id", &itemIDs).Error; err != nil {
			return err
		}
		if len(itemIDs) > 0 {
			if err := tx.Where("account_id IN ? AND user_id = ?", itemIDs, userID).Delete(&model.AccountCredential{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("platform_id = ? AND user_id = ?", platform.ID, userID).Delete(&model.AccountItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ? AND user_id = ?", platform.ID, userID).Delete(&model.AccountPlatform{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.AuditLog{UserID: userID, Action: "DELETE_ACCOUNT_PLATFORM", Detail: "Deleted account platform", IP: c.ClientIP()}).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account platform"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AccountHandler) Create(c *gin.Context) {
	var in struct {
		PlatformID uint   `json:"platform_id"`
		Platform   string `json:"platform"`
		URL        string `json:"url"`
		Account    string `json:"account"`
		Password   string `json:"password" binding:"required"`
		TOTPSecret string `json:"totp_secret"`
		FaviconURL string `json:"favicon_url"`
		Note       string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetUint("user_id")
	platform, ok := h.resolvePlatform(userID, in.PlatformID, in.Platform, in.URL, in.FaviconURL)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform_id or platform is required"})
		return
	}
	password, encryptedTOTP, err := encryptedAccountSecrets(in.Password, in.TOTPSecret, c.GetString("master_key"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	account := model.AccountItem{UserID: userID, PlatformID: platform.ID, Account: in.Account, Password: password, TOTPSecret: encryptedTOTP, HasTOTP: encryptedTOTP != "", Note: in.Note}
	if err := h.DB.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}
	h.DB.Preload("Credentials").First(&account, account.ID)
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "CREATE_ACCOUNT", Detail: "Platform: " + platform.Name, IP: c.ClientIP()})
	c.JSON(http.StatusOK, itemDTO(account, platform.Name))
}

func (h *AccountHandler) Update(c *gin.Context) {
	var in struct {
		PlatformID uint   `json:"platform_id"`
		Platform   string `json:"platform"`
		URL        string `json:"url"`
		Account    string `json:"account"`
		Password   string `json:"password"`
		TOTPSecret string `json:"totp_secret"`
		FaviconURL string `json:"favicon_url"`
		Note       string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetUint("user_id")
	updates := map[string]any{"account": in.Account, "note": in.Note}
	if in.PlatformID != 0 {
		if !h.platformExists(userID, in.PlatformID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "platform not found"})
			return
		}
		updates["platform_id"] = in.PlatformID
	} else if strings.TrimSpace(in.Platform) != "" {
		p, _ := h.resolvePlatform(userID, 0, in.Platform, in.URL, in.FaviconURL)
		updates["platform_id"] = p.ID
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
	res := h.DB.Model(&model.AccountItem{}).Where("id = ? AND user_id = ?", c.Param("id"), userID).Updates(updates)
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "UPDATE_ACCOUNT", Detail: "Updated account", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *AccountHandler) Delete(c *gin.Context) {
	userID := c.GetUint("user_id")
	accountID := c.Param("id")
	var account model.AccountItem
	if err := h.DB.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ? AND user_id = ?", account.ID, userID).Delete(&model.AccountCredential{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ? AND user_id = ?", account.ID, userID).Delete(&model.AccountItem{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.AuditLog{UserID: userID, Action: "DELETE_ACCOUNT", Detail: "Deleted account", IP: c.ClientIP()}).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AccountHandler) Decrypt(c *gin.Context) {
	var a model.AccountItem
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
	var a model.AccountItem
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

func (h *AccountHandler) CreateCredential(c *gin.Context) {
	var in struct {
		Name      string     `json:"name" binding:"required"`
		Value     string     `json:"value" binding:"required"`
		Note      string     `json:"note"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetUint("user_id")
	var account model.AccountItem
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), userID).First(&account).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	value, err := util.EncryptToString(in.Value, c.GetString("master_key"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt credential"})
		return
	}
	credential := model.AccountCredential{UserID: userID, AccountID: account.ID, Name: strings.TrimSpace(in.Name), Value: value, Note: in.Note, ExpiresAt: in.ExpiresAt}
	if err := h.DB.Create(&credential).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create credential"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: userID, Action: "CREATE_ACCOUNT_CREDENTIAL", Detail: "Credential: " + credential.Name, IP: c.ClientIP()})
	c.JSON(http.StatusOK, credentialDTO(credential))
}

func (h *AccountHandler) UpdateCredential(c *gin.Context) {
	var in struct {
		Name      string     `json:"name"`
		Value     string     `json:"value"`
		Note      string     `json:"note"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{"name": strings.TrimSpace(in.Name), "note": in.Note, "expires_at": in.ExpiresAt}
	if updates["name"] == "" {
		delete(updates, "name")
	}
	if in.Value != "" {
		enc, err := util.EncryptToString(in.Value, c.GetString("master_key"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt credential"})
			return
		}
		updates["value"] = enc
	}
	res := h.DB.Model(&model.AccountCredential{}).Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).Updates(updates)
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "UPDATE_ACCOUNT_CREDENTIAL", Detail: "Updated account credential", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *AccountHandler) DeleteCredential(c *gin.Context) {
	res := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).Delete(&model.AccountCredential{})
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "DELETE_ACCOUNT_CREDENTIAL", Detail: "Deleted account credential", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AccountHandler) DecryptCredential(c *gin.Context) {
	var credential model.AccountCredential
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), c.GetUint("user_id")).First(&credential).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	val, err := util.DecryptToString(credential.Value, c.GetString("master_key"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "decrypt failed"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: c.GetUint("user_id"), Action: "DECRYPT_ACCOUNT_CREDENTIAL", Detail: "Decrypted account credential", IP: c.ClientIP()})
	c.JSON(http.StatusOK, gin.H{"value": val})
}

func (h *AccountHandler) resolvePlatform(userID uint, platformID uint, name string, rawURL string, faviconOverride string) (model.AccountPlatform, bool) {
	if platformID != 0 {
		var platform model.AccountPlatform
		if err := h.DB.Where("id = ? AND user_id = ?", platformID, userID).First(&platform).Error; err == nil {
			return platform, true
		}
		return model.AccountPlatform{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return model.AccountPlatform{}, false
	}
	var platform model.AccountPlatform
	if err := h.DB.Where("user_id = ? AND name = ?", userID, name).First(&platform).Error; err == nil {
		return platform, true
	}
	platform = model.AccountPlatform{UserID: userID, Name: name, URL: strings.TrimSpace(rawURL), FaviconURL: faviconURL(rawURL, faviconOverride)}
	h.DB.Create(&platform)
	return platform, true
}

func (h *AccountHandler) platformExists(userID uint, platformID uint) bool {
	var count int64
	h.DB.Model(&model.AccountPlatform{}).Where("id = ? AND user_id = ?", platformID, userID).Count(&count)
	return count > 0
}

func encryptedAccountSecrets(password string, totpSecret string, masterKey string) (string, string, error) {
	passwordCipher, err := util.EncryptToString(password, masterKey)
	if err != nil {
		return "", "", clientImportError("Failed to encrypt password")
	}
	if strings.TrimSpace(totpSecret) == "" {
		return passwordCipher, "", nil
	}
	if _, _, err := util.GenerateTOTP(totpSecret, time.Now()); err != nil {
		return "", "", err
	}
	totpCipher, err := util.EncryptToString(strings.TrimSpace(totpSecret), masterKey)
	if err != nil {
		return "", "", clientImportError("Failed to encrypt TOTP secret")
	}
	return passwordCipher, totpCipher, nil
}

func platformDTO(p model.AccountPlatform) accountPlatformDTO {
	items := make([]accountItemDTO, 0, len(p.Items))
	for _, item := range p.Items {
		items = append(items, itemDTO(item, p.Name))
	}
	return accountPlatformDTO{ID: p.ID, UserID: p.UserID, Name: p.Name, Platform: p.Name, URL: p.URL, FaviconURL: p.FaviconURL, Note: p.Note, Items: items, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

func itemDTO(item model.AccountItem, platformName string) accountItemDTO {
	credentials := make([]credentialListDTO, 0, len(item.Credentials))
	for _, credential := range item.Credentials {
		credentials = append(credentials, credentialDTO(credential))
	}
	return accountItemDTO{ID: item.ID, UserID: item.UserID, PlatformID: item.PlatformID, Platform: platformName, Account: item.Account, Password: item.Password, TOTPSecret: item.TOTPSecret, HasTOTP: item.HasTOTP, Note: item.Note, Credentials: credentials, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func credentialDTO(credential model.AccountCredential) credentialListDTO {
	isExpired := credential.ExpiresAt != nil && credential.ExpiresAt.Before(time.Now())
	return credentialListDTO{ID: credential.ID, UserID: credential.UserID, AccountID: credential.AccountID, Name: credential.Name, Note: credential.Note, ExpiresAt: credential.ExpiresAt, IsExpired: isExpired, CreatedAt: credential.CreatedAt, UpdatedAt: credential.UpdatedAt}
}

func ensureLegacyAccountsMigrated(db *gorm.DB, userID uint) {
	var legacy []model.Account
	if err := db.Where("user_id = ?", userID).Find(&legacy).Error; err != nil || len(legacy) == 0 {
		return
	}

	_ = db.Transaction(func(tx *gorm.DB) error {
		migratedIDs := make([]uint, 0, len(legacy))
		for _, old := range legacy {
			var platform model.AccountPlatform
			if err := tx.Where("user_id = ? AND name = ?", userID, old.Platform).First(&platform).Error; err != nil {
				platform = model.AccountPlatform{UserID: userID, Name: old.Platform, URL: old.URL, FaviconURL: old.FaviconURL}
				if err := tx.Create(&platform).Error; err != nil {
					return err
				}
			}

			var exists int64
			if err := tx.Model(&model.AccountItem{}).Where("user_id = ? AND platform_id = ? AND account = ? AND password = ?", userID, platform.ID, old.Account, old.Password).Count(&exists).Error; err != nil {
				return err
			}
			if exists == 0 {
				item := model.AccountItem{UserID: userID, PlatformID: platform.ID, Account: old.Account, Password: old.Password, TOTPSecret: old.TOTPSecret, HasTOTP: old.HasTOTP, CreatedAt: old.CreatedAt, UpdatedAt: old.UpdatedAt}
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
			}
			migratedIDs = append(migratedIDs, old.ID)
		}
		if len(migratedIDs) == 0 {
			return nil
		}
		return tx.Where("id IN ? AND user_id = ?", migratedIDs, userID).Delete(&model.Account{}).Error
	})
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
