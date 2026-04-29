package util

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func GenerateTOTP(secret string, now time.Time) (string, int64, error) {
	cleaned := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	cleaned = strings.TrimPrefix(cleaned, "OTP://")
	if strings.HasPrefix(strings.ToLower(cleaned), "otpauth://") {
		return "", 0, fmt.Errorf("paste the raw TOTP secret, not the otpauth URL")
	}
	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleaned)
	if err != nil {
		secretBytes, err = base32.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return "", 0, fmt.Errorf("invalid TOTP secret")
		}
	}
	counter := uint64(now.Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, secretBytes)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)
	otp := code % int(math.Pow10(6))
	remaining := 30 - (now.Unix() % 30)
	return leftPadZeros(strconv.Itoa(otp), 6), remaining, nil
}

func leftPadZeros(value string, size int) string {
	if len(value) >= size {
		return value
	}
	return strings.Repeat("0", size-len(value)) + value
}
