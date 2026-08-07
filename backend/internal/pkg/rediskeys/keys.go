package rediskeys

import "strconv"

// PublishFreezeKey returns the canonical Redis key marking a user's publishing
// as frozen. The middleware publish guard, the review service writer and the
// content service reader must all agree on this key.
func PublishFreezeKey(userID int64) string {
	return "publish:freeze:" + strconv.FormatInt(userID, 10)
}
