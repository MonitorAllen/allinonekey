package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

const sealedSessionTTL = 24 * time.Hour

type SealedSessionPayload struct {
	UserID    uint      `json:"user_id"`
	Role      string    `json:"role"`
	MasterKey string    `json:"mk"`
	ExpiresAt time.Time `json:"exp"`
}

func SessionSecret() []byte {
	secret := os.Getenv("ALLINONEKEY_SESSION_SECRET")
	if secret == "" {
		secret = os.Getenv("ALLINONEKEY_JWT_SECRET")
	}
	if secret == "" {
		panic("ALLINONEKEY_SESSION_SECRET or ALLINONEKEY_JWT_SECRET is required")
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func SealSession(userID uint, role string, masterKey string) (string, error) {
	payload := SealedSessionPayload{
		UserID:    userID,
		Role:      role,
		MasterKey: masterKey,
		ExpiresAt: time.Now().Add(sealedSessionTTL),
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(SessionSecret())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, plain, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func OpenSession(token string) (*SealedSessionPayload, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(SessionSecret())
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("sealed session too short")
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	var payload SealedSessionPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, err
	}
	if payload.UserID == 0 || payload.Role == "" || payload.MasterKey == "" {
		return nil, errors.New("invalid sealed session payload")
	}
	if time.Now().After(payload.ExpiresAt) {
		return nil, errors.New("sealed session expired")
	}
	return &payload, nil
}
