package handler

// 私信面两份合同的契约表（#924 从手写链迁到 endpointErrMap 声明）：
// SendMessage 文本审核失败面（dmModerateErrMap）与冷启动守卫失败面
// （dmSendErrMap）。期望值全部为迁移前手写字面量（中文固定文案逐字节
// 保留）；规则数锁历史值防漂移。

import (
	"fmt"
	"net/http"
	"testing"

	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// TestDMModerateErrMap_RuleContract pins the DM moderation face (order =
// pre-#924 branch order)；文案为迁移前中文固定文案。
func TestDMModerateErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "text blocked", err: service.ErrTextBlocked, status: http.StatusUnprocessableEntity, code: "CONTENT_BLOCKED", message: "内容包含违规内容，无法发送"},
		{name: "moderation unavailable", err: service.ErrModerationUnavailable, status: http.StatusServiceUnavailable, code: "MODERATION_UNAVAILABLE", message: "内容审核服务暂时不可用，请稍后重试"},
	}
	if len(cases) != 2 {
		t.Fatalf("dm moderation contract table must keep the historical 2 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, dmModerateErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("dm moderation gate: %w", tt.err)
			assertErrMapResponse(t, dmModerateErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}

	leaky := fmt.Errorf("dm moderation: dial tcp 10.0.0.1:443: secret")
	assertErrMapResponse(t, dmModerateErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}

// TestDMSendErrMap_RuleContract pins the cold-start guard face：
// ErrDMReplyRequired → 403 固定中文文案；其余 Safe 500 DB_ERROR。
func TestDMSendErrMap_RuleContract(t *testing.T) {
	if len(dmSendErrMap.rules) != 1 {
		t.Fatalf("dm send map must keep exactly 1 rule, got %d", len(dmSendErrMap.rules))
	}
	assertErrMapResponse(t, dmSendErrMap, repository.ErrDMReplyRequired, http.StatusForbidden,
		envelope("DM_REPLY_REQUIRED", "对方尚未回复，请等待回复后再发送新消息"))
	wrapped := fmt.Errorf("send with cold start guard: %w", repository.ErrDMReplyRequired)
	assertErrMapResponse(t, dmSendErrMap, wrapped, http.StatusForbidden,
		envelope("DM_REPLY_REQUIRED", "对方尚未回复，请等待回复后再发送新消息"))

	leaky := fmt.Errorf("dm repo: sql: no such table %s", "messages; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, dmSendErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}
