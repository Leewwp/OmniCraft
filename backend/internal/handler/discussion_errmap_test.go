package handler

// /ips/:id 讨论写侧两路由共享合同的契约表（#924 从共享 switch 函数
// respondSocialServiceError 迁到 discussionWriteErrMap 声明）。期望值全
// 部为迁移前手写字面量；规则数锁历史值 4 防漂移，规则序 = 迁移前 case
// 序（与 /social 路径同一错误面）。

import (
	"fmt"
	"net/http"
	"testing"

	"omnicraft/backend/internal/service"
)

// TestDiscussionWriteErrMap_RuleContract pins every discussion write rule
// (order = pre-#924 switch case order); the wrapped variant proves errors.Is
// reaches the same rule through fmt.Errorf %w chains.
func TestDiscussionWriteErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "low reputation", err: service.ErrLowReputation, status: http.StatusForbidden, code: "FORBIDDEN", message: "reputation score too low to perform this action"},
		{name: "text blocked", err: service.ErrTextBlocked, status: http.StatusUnprocessableEntity, code: "CONTENT_BLOCKED", message: "content was rejected by content moderation"},
		{name: "moderation unavailable", err: service.ErrModerationUnavailable, status: http.StatusServiceUnavailable, code: "MODERATION_UNAVAILABLE", message: "content moderation is temporarily unavailable, please try again later"},
		{name: "discussion target invalid", err: service.ErrDiscussionTargetInvalid, status: http.StatusNotFound, code: "DISCUSSION_TARGET_INVALID", message: "discussion target not found or not visible"},
	}
	if len(cases) != 4 {
		t.Fatalf("discussion write contract table must keep the historical 4 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, discussionWriteErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("post discussion failed: %w", tt.err)
			assertErrMapResponse(t, discussionWriteErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestDiscussionWriteErrMap_UnknownErrorFallback pins the pre-#924 default
// branch: unknown errors answer Safe 500 DB_ERROR.
func TestDiscussionWriteErrMap_UnknownErrorFallback(t *testing.T) {
	leaky := fmt.Errorf("social service: sql: no such table %s", "discussions; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, discussionWriteErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}
