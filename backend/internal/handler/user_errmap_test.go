package handler

// user 三端点错误面的契约表（#924 从手写链迁到 endpointErrMap 声明）：
// GetUser 的 NotFound（迁移前 `==` 直比变体规则化）、UpdateUser 头像审核
// switch、ChangePassword 哈希失败两分支。期望值全部为迁移前手写字面量；
// 规则数锁历史值防漂移。

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// TestGetUserErrMap_RuleContract pins GetUser：gorm.ErrRecordNotFound →
// 404 固定文案；未知错误 Safe 500 DB_ERROR。`==` 直比规则化后 errors.Is
// 同规则生效（wrapped 变体证明）。
func TestGetUserErrMap_RuleContract(t *testing.T) {
	if len(getUserErrMap.rules) != 1 {
		t.Fatalf("get user map must keep exactly 1 rule, got %d", len(getUserErrMap.rules))
	}
	assertErrMapResponse(t, getUserErrMap, gorm.ErrRecordNotFound, http.StatusNotFound,
		envelope("USER_NOT_FOUND", "user not found"))
	wrapped := fmt.Errorf("find user: %w", gorm.ErrRecordNotFound)
	assertErrMapResponse(t, getUserErrMap, wrapped, http.StatusNotFound,
		envelope("USER_NOT_FOUND", "user not found"))

	leaky := errors.New("user repo: dial tcp 10.0.0.1:5432: secret")
	assertErrMapResponse(t, getUserErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}

// TestAvatarReviewErrMap_RuleContract pins UpdateUser 头像审核面（order =
// pre-#924 switch case order）：Blocked → 400；ModerationUnavailable →
// 503；未知错误 Safe 500 INTERNAL_ERROR。
func TestAvatarReviewErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "avatar blocked", err: ErrAvatarBlocked, status: http.StatusBadRequest, code: "AVATAR_BLOCKED", message: "avatar image was blocked by content moderation"},
		{name: "avatar moderation unavailable", err: ErrAvatarModerationUnavailable, status: http.StatusServiceUnavailable, code: "MODERATION_UNAVAILABLE", message: "avatar moderation is temporarily unavailable, please try again later"},
	}
	if len(cases) != 2 {
		t.Fatalf("avatar review contract table must keep the historical 2 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, avatarReviewErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("avatar review gate: %w", tt.err)
			assertErrMapResponse(t, avatarReviewErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}

	unknown := errors.New("reviewer: green rpc timeout")
	assertErrMapResponse(t, avatarReviewErrMap, unknown, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "an unexpected error occurred, please try again later"))
}

// TestHashPasswordErrMap_RuleContract pins ChangePassword 哈希失败面：
// bcrypt 超长 → 400 PASSWORD_TOO_LONG；其余 → 500 固定兜底文案
// "failed to hash password"（迁移前固定 message 分支，非 Safe 语义）。
func TestHashPasswordErrMap_RuleContract(t *testing.T) {
	if len(hashPasswordErrMap.rules) != 1 {
		t.Fatalf("hash password map must keep exactly 1 rule, got %d", len(hashPasswordErrMap.rules))
	}
	assertErrMapResponse(t, hashPasswordErrMap, bcrypt.ErrPasswordTooLong, http.StatusBadRequest,
		envelope("PASSWORD_TOO_LONG", "password must be at most 72 bytes"))
	wrapped := fmt.Errorf("hash: %w", bcrypt.ErrPasswordTooLong)
	assertErrMapResponse(t, hashPasswordErrMap, wrapped, http.StatusBadRequest,
		envelope("PASSWORD_TOO_LONG", "password must be at most 72 bytes"))

	unknown := errors.New("bcrypt: entropy source unavailable")
	assertErrMapResponse(t, hashPasswordErrMap, unknown, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "failed to hash password"))
}
