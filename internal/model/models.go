package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Username    string         `gorm:"uniqueIndex;not null" json:"username"`
	Role        string         `gorm:"default:'user'" json:"role"`
	KeyVerifier string         `gorm:"not null" json:"-"`
	Salt        string         `gorm:"not null" json:"-"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type APIKey struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	Provider     string    `gorm:"index;not null" json:"provider"`
	PoolGroup    string    `gorm:"index;default:'default'" json:"pool_group"`
	KeyName      string    `json:"key_name"`
	KeyValue     string    `gorm:"not null" json:"key_value"`
	BaseURL      string    `json:"base_url"`
	ProxyURL     string    `json:"proxy_url"`
	QuotaTotal   float64   `json:"quota_total"`
	QuotaUsed    float64   `json:"quota_used"`
	QuotaBalance float64   `json:"quota_balance"`
	LastCheck    time.Time `json:"last_check"`
	Status       string    `gorm:"default:'active'" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Account struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Platform   string    `gorm:"index;not null" json:"platform"`
	URL        string    `json:"url"`
	Account    string    `json:"account"`
	Password   string    `gorm:"not null" json:"password"`
	TOTPSecret string    `json:"totp_secret"`
	HasTOTP    bool      `gorm:"default:false" json:"has_totp"`
	FaviconURL string    `json:"favicon_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type InvitationCode struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Code      string     `gorm:"uniqueIndex;not null" json:"code"`
	CreatedBy uint       `json:"created_by"`
	UsedBy    uint       `json:"used_by"`
	IsUsed    bool       `gorm:"default:false" json:"is_used"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}
