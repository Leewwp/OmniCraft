package handler

// agent 面（认证态）的 sentinel→HTTP 契约表（#923 从手写 if 链迁到
// endpointErrMap 声明）。期望值全部为手写字面量（=迁移前分支的逐字节
// wire）；规则数锁历史值防漂移。reserveGenerationQuota 的 quota 合同、
// 会话 NotFound 规则化（含 gorm.ErrRecordNotFound D 型两处）与两表面共
// 享的输入审核合同都在本表锁定。

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TestAgentGenerationQuotaErrMap_RuleContract pins the quota reservation
// contract（reserveGenerationQuota 迁移前的三个失败出口：超限 429 规则 +
// 503 兜底×2——Redis 不可用与 reserver 未接线，后者经内部占位错误恒落兜
// 底）。
func TestAgentGenerationQuotaErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "quota exceeded", err: middleware.ErrAgentQuotaExceeded, status: http.StatusTooManyRequests, code: "AGENT_RATE_LIMIT_EXCEEDED", message: "agent request limit exceeded"},
		{name: "reserver unwired falls to fallback", err: errAgentQuotaReserverUnwired, status: http.StatusServiceUnavailable, code: "AGENT_QUOTA_UNAVAILABLE", message: "agent quota service is temporarily unavailable"},
		{name: "redis unavailable falls to fallback", err: errors.New("redis: connection pool timeout"), status: http.StatusServiceUnavailable, code: "AGENT_QUOTA_UNAVAILABLE", message: "agent quota service is temporarily unavailable"},
	}
	if len(cases) != 3 {
		t.Fatalf("agent generation quota contract table must keep the historical 3 outcomes, got %d", len(cases))
	}
	// 规则集本身锁 1 条（429）；其余两行是兜底路径的行为锁定。
	if len(agentGenerationQuotaErrMap.rules) != 1 {
		t.Fatalf("agent generation quota map must keep exactly 1 rule (429), got %d", len(agentGenerationQuotaErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, agentGenerationQuotaErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
		})
	}
	wrapped := fmt.Errorf("reserve: limiter pipeline: %w", middleware.ErrAgentQuotaExceeded)
	assertErrMapResponse(t, agentGenerationQuotaErrMap, wrapped, http.StatusTooManyRequests,
		envelope("AGENT_RATE_LIMIT_EXCEEDED", "agent request limit exceeded"))
}

// TestAgentConversationErrMap_RuleContract pins the owner-scoped
// conversation contract：服务层 sentinel（ChatStream 所有权门）与
// gorm.ErrRecordNotFound（UpdateConversation 回读 / GetConversationMessages
// 首查，D 型规则化）双规则同一 404，未知错误 Safe 500 AGENT_ERROR。
func TestAgentConversationErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "service conversation not found", err: service.ErrAgentConversationNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "conversation not found"},
		{name: "gorm record not found (read-back D-form)", err: gorm.ErrRecordNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "conversation not found"},
	}
	if len(cases) != 2 {
		t.Fatalf("agent conversation contract table must keep the historical 2 rules, got %d", len(cases))
	}
	if len(agentConversationErrMap.rules) != 2 {
		t.Fatalf("agent conversation map must keep exactly 2 rules, got %d", len(agentConversationErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, agentConversationErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("conversation gate: owner scope: %w", tt.err)
			assertErrMapResponse(t, agentConversationErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
	// 兜底：Safe 500 AGENT_ERROR（安全文案来自 response.safeMessages）。
	boom := errors.New("conversation repo: dial tcp 10.9.9.9:5432: boom")
	assertErrMapResponse(t, agentConversationErrMap, boom, http.StatusInternalServerError,
		envelope("AGENT_ERROR", "agent service unavailable"))
	if strings.Contains(lastErrMapBody(t, agentConversationErrMap, boom), "dial tcp") {
		t.Fatalf("agent conversation fallback leaked internal error")
	}
}

// TestAgentModerationErrMap_RuleContract pins the A-05 input admission
// contract（认证态 ChatStream 与游客态 ChatStream 共享同一合同：拦截
// 422 / release 态审核不可用 503 / 未知 Safe 500 AGENT_ERROR）。
func TestAgentModerationErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "input blocked", err: service.ErrAgentInputBlocked, status: http.StatusUnprocessableEntity, code: "CONTENT_BLOCKED", message: "content was rejected by content moderation"},
		{name: "moderation unavailable", err: service.ErrAgentModerationUnavailable, status: http.StatusServiceUnavailable, code: "MODERATION_UNAVAILABLE", message: "content moderation unavailable"},
	}
	if len(cases) != 2 {
		t.Fatalf("agent moderation contract table must keep the historical 2 rules, got %d", len(cases))
	}
	if len(agentModerationErrMap.rules) != 2 {
		t.Fatalf("agent moderation map must keep exactly 2 rules, got %d", len(agentModerationErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, agentModerationErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("moderate chat input: green pipeline: %w", tt.err)
			assertErrMapResponse(t, agentModerationErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
	boom := errors.New("green client: signature expired")
	assertErrMapResponse(t, agentModerationErrMap, boom, http.StatusInternalServerError,
		envelope("AGENT_ERROR", "agent service unavailable"))
}

// lastErrMapBody drives one write and returns the response body (供泄漏面
// 检查复用；断言主体仍走 assertErrMapResponse)。
func lastErrMapBody(t *testing.T, m endpointErrMap, err error) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	m.write(c, err)
	return rec.Body.String()
}
