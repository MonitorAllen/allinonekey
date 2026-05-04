package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

type exportPayload struct {
	APIKeys          []apiKey          `json:"api_keys"`
	Accounts         []account         `json:"accounts"`
	AccountPlatforms []accountPlatform `json:"account_platforms"`
	AccountItems     []accountItem     `json:"account_items"`
}

type apiKey struct {
	ID       uint   `json:"id"`
	KeyName  string `json:"key_name"`
	Provider string `json:"provider"`
	KeyValue string `json:"key_value"`
}

type account struct {
	ID         uint   `json:"id"`
	Platform   string `json:"platform"`
	Account    string `json:"account"`
	Password   string `json:"password"`
	TOTPSecret string `json:"totp_secret"`
}

type accountPlatform struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type accountItem struct {
	ID         uint   `json:"id"`
	PlatformID uint   `json:"platform_id"`
	Account    string `json:"account"`
	Password   string `json:"password"`
	TOTPSecret string `json:"totp_secret"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: go run scripts/decrypt.go <export.json|ciphertext_base64> <master_key>")
		os.Exit(1)
	}
	input := os.Args[1]
	masterKey := os.Args[2]
	if _, err := os.Stat(input); err == nil {
		if err := decryptExportFile(input, masterKey); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	plain, err := decryptString(input, masterKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(plain)
}

func decryptExportFile(path string, masterKey string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var payload exportPayload
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}
	for _, k := range payload.APIKeys {
		plain, err := decryptString(k.KeyValue, masterKey)
		if err != nil {
			plain = "[DECRYPT_FAILED]"
		}
		fmt.Printf("APIKey #%d %s/%s: %s\n", k.ID, k.Provider, k.KeyName, plain)
	}
	for _, a := range payload.Accounts {
		password, err := decryptString(a.Password, masterKey)
		if err != nil {
			password = "[DECRYPT_FAILED]"
		}
		fmt.Printf("Account #%d %s/%s password: %s\n", a.ID, a.Platform, a.Account, password)
		if a.TOTPSecret != "" {
			secret, err := decryptString(a.TOTPSecret, masterKey)
			if err != nil {
				secret = "[DECRYPT_FAILED]"
			}
			fmt.Printf("Account #%d %s/%s totp_secret: %s\n", a.ID, a.Platform, a.Account, secret)
		}
	}
	platforms := map[uint]string{}
	for _, p := range payload.AccountPlatforms {
		platforms[p.ID] = p.Name
	}
	for _, a := range payload.AccountItems {
		password, err := decryptString(a.Password, masterKey)
		if err != nil {
			password = "[DECRYPT_FAILED]"
		}
		platform := platforms[a.PlatformID]
		fmt.Printf("Account #%d %s/%s password: %s\n", a.ID, platform, a.Account, password)
		if a.TOTPSecret != "" {
			secret, err := decryptString(a.TOTPSecret, masterKey)
			if err != nil {
				secret = "[DECRYPT_FAILED]"
			}
			fmt.Printf("Account #%d %s/%s totp_secret: %s\n", a.ID, platform, a.Account, secret)
		}
	}
	return nil
}

func decryptString(ciphertextB64 string, masterKey string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}
	key := make([]byte, 32)
	copy(key, masterKey)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(decoded) < gcm.NonceSize() {
		return "", fmt.Errorf("short ciphertext")
	}
	nonce, actualCiphertext := decoded[:gcm.NonceSize()], decoded[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
