package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
)

const (
	DenialReasonAuthStatusUnavailable  = "AUTH_STATUS_UNAVAILABLE"
	DenialReasonUserBanned             = "USER_BANNED"
	DenialReasonEmailNotVerified       = "EMAIL_NOT_VERIFIED"
	DenialReasonConfigError            = "CONFIG_ERROR"
	DenialReasonInsufficientReputation = "INSUFFICIENT_REPUTATION"
)

var (
	ErrUserStatusNotFound = errors.New("USER_NOT_FOUND")
	ErrUserStatusDeleted  = errors.New("USER_DELETED")
)

type RuntimeUserStatus struct {
	ID              int64
	Role            string
	IsBanned        bool
	EmailVerifiedAt *time.Time
	Reputation      int
}

type InteractionAccessDecision struct {
	Allowed      bool
	DenialReason string
}

func EvaluateInteractionAccess(status *RuntimeUserStatus, cfg *config.Config, requireVerifiedEmail, requireReputation bool) InteractionAccessDecision {
	if status == nil || cfg == nil {
		return InteractionAccessDecision{DenialReason: DenialReasonAuthStatusUnavailable}
	}
	if status.IsBanned {
		return InteractionAccessDecision{DenialReason: DenialReasonUserBanned}
	}
	if requireVerifiedEmail && status.EmailVerifiedAt == nil {
		return InteractionAccessDecision{DenialReason: DenialReasonEmailNotVerified}
	}
	if requireReputation {
		if cfg.Reputation.MinScoreForInteraction <= 0 {
			return InteractionAccessDecision{DenialReason: DenialReasonConfigError}
		}
		if status.Reputation < cfg.Reputation.MinScoreForInteraction {
			return InteractionAccessDecision{DenialReason: DenialReasonInsufficientReputation}
		}
	}
	return InteractionAccessDecision{Allowed: true}
}

func IsSoftDenialReason(reason string) bool {
	return reason == DenialReasonEmailNotVerified || reason == DenialReasonInsufficientReputation
}

type RuntimeStatusCache struct {
	rdb *redis.Client
	cfg *config.Config
}

func NewRuntimeStatusCache(rdb *redis.Client, cfg *config.Config) *RuntimeStatusCache {
	return &RuntimeStatusCache{rdb: rdb, cfg: cfg}
}

func (c *RuntimeStatusCache) Invalidate(userID int64) error {
	if c.rdb == nil {
		return nil
	}
	statusKey := fmt.Sprintf("user:status:%d", userID)
	return c.rdb.Del(context.Background(), statusKey).Err()
}

func (c *RuntimeStatusCache) Set(userID int64, status *RuntimeUserStatus) {
	if c.rdb == nil {
		return
	}
	ttl := time.Duration(c.cfg.Cache.UserStatusTTL) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	statusKey := fmt.Sprintf("user:status:%d", userID)
	data, err := json.Marshal(status)
	if err != nil {
		return
	}
	c.rdb.Set(context.Background(), statusKey, string(data), ttl)
}

func (c *RuntimeStatusCache) Get(userID int64) (*RuntimeUserStatus, bool) {
	if c.rdb == nil {
		return nil, false
	}
	statusKey := fmt.Sprintf("user:status:%d", userID)
	val, err := c.rdb.Get(context.Background(), statusKey).Result()
	if err != nil {
		return nil, false
	}
	var status RuntimeUserStatus
	if json.Unmarshal([]byte(val), &status) != nil {
		return nil, false
	}
	return &status, true
}

func ResolveRuntimeUserStatus(ctx context.Context, db *gorm.DB, cache *RuntimeStatusCache, userID int64) (*RuntimeUserStatus, error) {
	if cache != nil {
		if status, ok := cache.Get(userID); ok {
			return status, nil
		}
	}

	if db == nil {
		return nil, fmt.Errorf(DenialReasonAuthStatusUnavailable)
	}
	var result struct {
		ID              int64      `gorm:"column:id"`
		Role            string     `gorm:"column:role"`
		IsBanned        bool       `gorm:"column:is_banned"`
		EmailVerifiedAt *time.Time `gorm:"column:email_verified_at"`
		Reputation      int        `gorm:"column:reputation"`
		DeletedAt       *time.Time `gorm:"column:deleted_at"`
	}

	err := db.Table("users").
		Select("id, role, is_banned, email_verified_at, reputation, deleted_at").
		Where("id = ?", userID).
		Scan(&result).Error
	if err != nil {
		return nil, fmt.Errorf(DenialReasonAuthStatusUnavailable)
	}

	if result.ID == 0 {
		return nil, ErrUserStatusNotFound
	}

	if result.DeletedAt != nil {
		return nil, ErrUserStatusDeleted
	}

	status := &RuntimeUserStatus{
		ID:              result.ID,
		Role:            result.Role,
		IsBanned:        result.IsBanned,
		EmailVerifiedAt: result.EmailVerifiedAt,
		Reputation:      result.Reputation,
	}

	if cache != nil {
		cache.Set(userID, status)
	}

	return status, nil
}
