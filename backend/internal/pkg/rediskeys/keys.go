package rediskeys

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// PublishFreezeKey returns the canonical Redis key marking a user's publishing
// as frozen. The middleware publish guard, the review service writer and the
// content service reader must all agree on this key.
func PublishFreezeKey(userID int64) string {
	return "publish:freeze:" + strconv.FormatInt(userID, 10)
}

// TokenBlacklistKey returns the canonical Redis blacklist key for an access
// token. The key stores only sha256(token) so a Redis snapshot never contains
// a still-valid bearer token in plaintext (mirrors the refresh-token key
// convention in auth_service). AuthService writes it on Logout; the auth
// middlewares read it on every authenticated request.
func TokenBlacklistKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "blacklist:token:" + hex.EncodeToString(sum[:])
}
