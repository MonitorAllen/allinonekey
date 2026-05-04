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
	ProviderURL  string    `json:"provider_url"`
	ProviderIcon string    `json:"provider_icon"`
	Note         string    `json:"note"`
	QuotaTotal   float64   `json:"quota_total"`
	QuotaUsed    float64   `json:"quota_used"`
	QuotaBalance float64   `json:"quota_balance"`
	LastCheck    time.Time `json:"last_check"`
	Status       string    `gorm:"default:'active'" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AccountPlatform struct {
	ID         uint          `gorm:"primaryKey" json:"id"`
	UserID     uint          `gorm:"index;not null" json:"user_id"`
	Name       string        `gorm:"index;not null" json:"name"`
	URL        string        `json:"url"`
	FaviconURL string        `json:"favicon_url"`
	Note       string        `json:"note"`
	Items      []AccountItem `gorm:"foreignKey:PlatformID" json:"items,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

type AccountItem struct {
	ID          uint                `gorm:"primaryKey" json:"id"`
	UserID      uint                `gorm:"index;not null" json:"user_id"`
	PlatformID  uint                `gorm:"index;not null" json:"platform_id"`
	Account     string              `gorm:"index" json:"account"`
	Password    string              `gorm:"not null" json:"password"`
	TOTPSecret  string              `json:"totp_secret"`
	HasTOTP     bool                `gorm:"default:false" json:"has_totp"`
	Note        string              `json:"note"`
	Credentials []AccountCredential `gorm:"foreignKey:AccountID" json:"credentials,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type AccountCredential struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"index;not null" json:"user_id"`
	AccountID uint       `gorm:"index;not null" json:"account_id"`
	Name      string     `gorm:"index;not null" json:"name"`
	Value     string     `gorm:"not null" json:"value"`
	Note      string     `json:"note"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Account is kept only for importing/migrating pre-Platform legacy backups and DB rows.
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
