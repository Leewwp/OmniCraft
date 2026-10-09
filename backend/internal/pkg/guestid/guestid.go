// Package guestid implements the #854 anonymous device identity: a random
// 256-bit id carried in an HttpOnly cookie, authenticated by an HMAC-SHA256
// signature under a per-instance secret. Storage and Redis keys never carry
// the raw id — they use the lowercase SHA-256 hex of it, so a database or
// cache leak does not leak usable cookie values.
package guestid

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrInvalidDeviceCookie marks any cookie that fails format or signature
// validation. Callers must treat it as "no valid identity" — never as an
// opportunity to silently mint a new one.
var ErrInvalidDeviceCookie = errors.New("invalid guest device cookie")

// idBytes is the raw entropy of a device id (256-bit).
const idBytes = 32

// Issue mints a fresh device identity. The returned cookie value has the
// stable "<id>.<hmac>" shape; deviceKey is the storage/Redis key derived from
// the id.
func Issue(secret string) (cookieValue, deviceKey string, err error) {
	raw := make([]byte, idBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	id := hex.EncodeToString(raw)
	return id + "." + sign(secret, id), DeviceKey(id), nil
}

// Verify validates a cookie value and returns the derived device key.
func Verify(secret, cookieValue string) (string, error) {
	id, sig, ok := strings.Cut(cookieValue, ".")
	if !ok || strings.Contains(sig, ".") || id == "" || sig == "" {
		return "", ErrInvalidDeviceCookie
	}
	if !hmac.Equal([]byte(sign(secret, id)), []byte(sig)) {
		return "", ErrInvalidDeviceCookie
	}
	return DeviceKey(id), nil
}

// DeviceKey derives the storage key: lowercase SHA-256 hex of the raw id.
func DeviceKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

func sign(secret, id string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}
