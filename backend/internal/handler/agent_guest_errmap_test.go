package handler

// agent 游客面的 sentinel→HTTP 契约表（#923 从手写 switch/反转谓词迁到
// endpointErrMap 声明）。期望值全部为手写字面量（=迁移前分支的逐字节
// wire）；规则数锁历史值防漂移。两处会话门（GetConversationMessages 与
// ChatStream 所有权门）迁移前是同一 switch 合同，共享一份 map；quota 状
// 态查询的反转谓词（StateLost 除外才 503）由无规则兜底 map 承接响应字
// 节，谓词本身留在 handler。

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/service"
)

// TestGuestConversationErrMap_RuleContract pins the guest conversation
// contract（两个调用点迁移前同一 switch：过期 410 区别于所有者 404，未
// 知 Safe 500 AGENT_ERROR）。
func TestGuestConversationErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "conversation expired", err: service.ErrAgentConversationExpired, status: http.StatusGone, code: "AGENT_CONVERSATION_EXPIRED", message: "conversation has expired"},
		{name: "conversation not found", err: service.ErrAgentConversationNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "conversation not found"},
	}
	if len(cases) != 2 {
		t.Fatalf("guest conversation contract table must keep the historical 2 rules, got %d", len(cases))
	}
	if len(guestConversationErrMap.rules) != 2 {
		t.Fatalf("guest conversation map must keep exactly 2 rules, got %d", len(guestConversationErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, guestConversationErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("guest conversation gate: ttl sweep: %w", tt.err)
			assertErrMapResponse(t, guestConversationErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
	boom := errors.New("guest conversation repo: connection refused")
	assertErrMapResponse(t, guestConversationErrMap, boom, http.StatusInternalServerError,
		envelope("AGENT_ERROR", "agent service unavailable"))
}

// TestGuestQuotaReserveErrMap_RuleContract pins the guest turn reservation
// contract（超限 429 / 并发上限 429 / 状态丢失 403 / 其余 503 fail-closed
// 固定文案兜底）。
func TestGuestQuotaReserveErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "quota exhausted", err: middleware.ErrGuestQuotaExceeded, status: http.StatusTooManyRequests, code: "GUEST_QUOTA_EXHAUSTED", message: "guest turn budget is used up"},
		{name: "concurrency limit", err: middleware.ErrGuestConcurrencyLimit, status: http.StatusTooManyRequests, code: "GUEST_CONCURRENCY_LIMIT", message: "too many concurrent guest requests"},
		{name: "quota state lost", err: middleware.ErrGuestQuotaStateLost, status: http.StatusForbidden, code: "GUEST_QUOTA_STATE_LOST", message: "guest quota state is unavailable"},
		{name: "unknown falls to fail-closed 503", err: errors.New("redis: pool exhausted"), status: http.StatusServiceUnavailable, code: "GUEST_QUOTA_UNAVAILABLE", message: "guest quota service is temporarily unavailable"},
	}
	if len(cases) != 4 {
		t.Fatalf("guest quota reserve contract table must keep the historical 3 rules + fallback, got %d", len(cases))
	}
	if len(guestQuotaReserveErrMap.rules) != 3 {
		t.Fatalf("guest quota reserve map must keep exactly 3 rules, got %d", len(guestQuotaReserveErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, guestQuotaReserveErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
		})
	}
	wrapped := fmt.Errorf("guest reserve: bucket drain: %w", middleware.ErrGuestQuotaExceeded)
	assertErrMapResponse(t, guestQuotaReserveErrMap, wrapped, http.StatusTooManyRequests,
		envelope("GUEST_QUOTA_EXHAUSTED", "guest turn budget is used up"))
}

// TestGuestQuotaStatusErrMap_FallbackContract pins the quota status query
// contract：map 无规则，任何送入的错误都落 503 fail-closed 固定文案（迁
// 移前「err 非 nil 且非 StateLost」反转谓词分支的响应字节）；StateLost
// 的放行判定是调用点的控制流（fail closed 读作余量 0），不在 map 面。
func TestGuestQuotaStatusErrMap_FallbackContract(t *testing.T) {
	if len(guestQuotaStatusErrMap.rules) != 0 {
		t.Fatalf("guest quota status map must stay rule-less (any error -> fail-closed fallback), got %d rules", len(guestQuotaStatusErrMap.rules))
	}
	for _, err := range []error{
		errors.New("redis: i/o timeout"),
		middleware.ErrGuestQuotaStateLost, // 谓词层面不会送入；若送入同样 503，行为面只由调用点谓词决定
	} {
		assertErrMapResponse(t, guestQuotaStatusErrMap, err, http.StatusServiceUnavailable,
			envelope("GUEST_QUOTA_UNAVAILABLE", "guest quota service is temporarily unavailable"))
	}
}
