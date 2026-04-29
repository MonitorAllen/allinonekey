package service

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"allinonekey/internal/model"
	"allinonekey/internal/util"

	"gorm.io/gorm"
)

const quotaCheckTimeout = 10 * time.Second

type QuotaService struct {
	DB *gorm.DB
}

type QuotaCheckResult struct {
	Status string
	Error  string
}

func (s *QuotaService) StartCron() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			s.CheckActiveUsersKeys()
		}
	}()
}

func (s *QuotaService) CheckActiveUsersKeys() {
	log.Println("[Quota] Starting active users quota sync...")
	for userID, mk := range util.ActiveKeySnapshot() {
		var keys []model.APIKey
		s.DB.Where("user_id = ?", userID).Find(&keys)
		for _, k := range keys {
			go s.UpdateQuotaForKey(k, mk)
		}
	}
}

func (s *QuotaService) UpdateQuotaForKey(k model.APIKey, mk string) QuotaCheckResult {
	plainKey, err := util.DecryptToString(k.KeyValue, mk)
	if err != nil {
		s.updateKeyQuotaState(&k, "decrypt_error", 0, 0, 0)
		return QuotaCheckResult{Status: "decrypt_error", Error: "decrypt failed"}
	}

	result := probeProvider(k.Provider, k.BaseURL, k.ProxyURL, plainKey)
	if result.Status == "active" {
		s.updateKeyQuotaState(&k, "active", 0, 0, 0)
		return result
	}

	s.updateKeyQuotaState(&k, result.Status, 0, 0, 0)
	return result
}

func (s *QuotaService) updateKeyQuotaState(k *model.APIKey, status string, total float64, used float64, balance float64) {
	s.DB.Model(k).Updates(map[string]any{
		"status":        status,
		"quota_total":   total,
		"quota_used":    used,
		"quota_balance": balance,
		"last_check":    time.Now(),
	})
}

func probeProvider(provider string, baseURL string, proxyURL string, key string) QuotaCheckResult {
	providerName := strings.ToLower(strings.TrimSpace(provider))
	customBaseURL := strings.TrimSpace(baseURL)
	switch providerName {
	case "openai", "openai-compatible", "custom", "relay", "proxy":
		return probeOpenAICompatible(defaultBaseURL(customBaseURL, "https://api.openai.com"), proxyURL, key)
	case "deepseek":
		return probeOpenAICompatible(defaultBaseURL(customBaseURL, "https://api.deepseek.com"), proxyURL, key)
	case "anthropic", "claude":
		return probeAnthropic(defaultBaseURL(customBaseURL, "https://api.anthropic.com"), proxyURL, key)
	case "gemini", "google":
		return probeGemini(defaultBaseURL(customBaseURL, "https://generativelanguage.googleapis.com"), proxyURL, key)
	default:
		if customBaseURL != "" {
			return probeOpenAICompatible(customBaseURL, proxyURL, key)
		}
		return QuotaCheckResult{Status: "quota_unsupported", Error: "provider quota check unsupported"}
	}
}

func probeOpenAICompatible(baseURL string, proxyURL string, key string) QuotaCheckResult {
	endpoint, err := joinURL(baseURL, "/v1/models")
	if err != nil {
		return QuotaCheckResult{Status: "quota_error", Error: "invalid base_url"}
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return QuotaCheckResult{Status: "quota_error", Error: "failed to create request"}
	}
	req.Header.Set("Authorization", "Bearer "+key)
	return executeProbe(req, proxyURL)
}

func probeAnthropic(baseURL string, proxyURL string, key string) QuotaCheckResult {
	endpoint, err := joinURL(baseURL, "/v1/models")
	if err != nil {
		return QuotaCheckResult{Status: "quota_error", Error: "invalid base_url"}
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return QuotaCheckResult{Status: "quota_error", Error: "failed to create request"}
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	return executeProbe(req, proxyURL)
}

func probeGemini(baseURL string, proxyURL string, key string) QuotaCheckResult {
	endpoint, err := joinURL(baseURL, "/v1beta/models")
	if err != nil {
		return QuotaCheckResult{Status: "quota_error", Error: "invalid base_url"}
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return QuotaCheckResult{Status: "quota_error", Error: "invalid base_url"}
	}
	q := u.Query()
	q.Set("key", key)
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return QuotaCheckResult{Status: "quota_error", Error: "failed to create request"}
	}
	return executeProbe(req, proxyURL)
}

func executeProbe(req *http.Request, proxyURL string) QuotaCheckResult {
	client, err := probeHTTPClient(proxyURL)
	if err != nil {
		return QuotaCheckResult{Status: "quota_error", Error: "invalid proxy_url"}
	}
	resp, err := client.Do(req)
	if err != nil {
		return QuotaCheckResult{Status: "quota_error", Error: "provider request failed"}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return QuotaCheckResult{Status: "active"}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return QuotaCheckResult{Status: "auth_error", Error: fmt.Sprintf("provider returned %d", resp.StatusCode)}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return QuotaCheckResult{Status: "rate_limited", Error: "provider rate limited"}
	}
	return QuotaCheckResult{Status: "quota_error", Error: fmt.Sprintf("provider returned %d", resp.StatusCode)}
}

func probeHTTPClient(proxyURL string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	trimmed := strings.TrimSpace(proxyURL)
	if trimmed != "" {
		u, err := url.Parse(trimmed)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("invalid proxy url")
		}
		transport.Proxy = http.ProxyURL(u)
	}
	return &http.Client{Timeout: quotaCheckTimeout, Transport: transport}, nil
}

func defaultBaseURL(baseURL string, fallback string) string {
	if strings.TrimSpace(baseURL) == "" {
		return fallback
	}
	return strings.TrimSpace(baseURL)
}

func joinURL(baseURL string, path string) (string, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid base url")
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = ""
	return u.String(), nil
}
