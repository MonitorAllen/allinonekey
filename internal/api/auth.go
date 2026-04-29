package api

import (
	"allinonekey/internal/model"
	"allinonekey/internal/util"
	"crypto/rand"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB *gorm.DB
}

func (h *AuthHandler) Register(c *gin.Context) {
	var in struct {
		Username   string `json:"username" binding:"required"`
		MasterKey  string `json:"master_key" binding:"required"`
		InviteCode string `json:"invite_code"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := util.ValidateMasterKey(in.MasterKey); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var count int64
	h.DB.Model(&model.User{}).Count(&count)
	role := "user"
	var invite model.InvitationCode
	if count == 0 {
		role = "admin"
	} else {
		if err := h.DB.Where("code = ? AND is_used = ?", in.InviteCode, false).First(&invite).Error; err != nil {
			c.JSON(403, gin.H{"error": "Invalid invite code"})
			return
		}
		if invite.ExpiresAt != nil && !invite.ExpiresAt.After(time.Now()) {
			c.JSON(403, gin.H{"error": "Invite code expired"})
			return
		}
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate salt"})
		return
	}
	hash, err := util.HashMasterKey(in.MasterKey, salt)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to hash master key"})
		return
	}

	user := model.User{Username: in.Username, KeyVerifier: hash, Salt: string(salt), Role: role}
	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(400, gin.H{"error": "User exists"})
		return
	}
	if count > 0 {
		h.DB.Model(&invite).Updates(map[string]any{"is_used": true, "used_by": user.ID})
	}
	h.DB.Create(&model.AuditLog{UserID: user.ID, Action: "REGISTER", Detail: "User registered", IP: c.ClientIP()})
	c.JSON(200, gin.H{"message": "success"})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var in struct {
		Username  string `json:"username" binding:"required"`
		MasterKey string `json:"master_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	ip := c.ClientIP()
	if locked, until := isLoginLocked(in.Username, ip); locked {
		c.JSON(429, gin.H{"error": "Too many failed login attempts", "retry_after_seconds": int(time.Until(until).Seconds())})
		return
	}

	var user model.User
	if err := h.DB.Where("username = ?", in.Username).First(&user).Error; err != nil {
		recordLoginFailure(in.Username, ip)
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	match, err := util.VerifyMasterKey(in.MasterKey, user.KeyVerifier)
	if err != nil || !match {
		locked, until := recordLoginFailure(in.Username, ip)
		h.DB.Create(&model.AuditLog{UserID: user.ID, Action: "LOGIN_FAILED", Detail: "Invalid master key", IP: ip})
		if locked {
			c.JSON(429, gin.H{"error": "Too many failed login attempts", "retry_after_seconds": int(time.Until(until).Seconds())})
			return
		}
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	resetLoginFailures(in.Username, ip)
	util.SetActiveKey(user.ID, in.MasterKey)
	token, err := util.SealSession(user.ID, user.Role, in.MasterKey)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to seal session"})
		return
	}
	h.DB.Create(&model.AuditLog{UserID: user.ID, Action: "LOGIN", Detail: "User logged in", IP: ip})
	c.JSON(200, gin.H{"token": token, "role": user.Role})
}
