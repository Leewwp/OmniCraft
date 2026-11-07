package handler

// agent PAT（设置页令牌管理）端点的 sentinel→HTTP 契约表（#923 从手写
// switch/if 链迁到 endpointErrMap 声明）。期望值全部为手写字面量（=迁
// 移前分支的逐字节 wire）；两份 map 各一张表，规则数锁历史值防漂移。
// 迁移前 Create 的双 sentinel 单 case（Name/Scopes 同 400）拆为同响应的
// 两条规则，语义等价（同 status/code/message，命中先后无差别）。

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"omnicraft/backend/internal/service"
)

// TestCreateAgentTokenErrMap_RuleContract pins every Create rule (order =
// pre-#923 case order).
func TestCreateAgentTokenErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "token name invalid", err: service.ErrAgentTokenNameInvalid, status: http.StatusBadRequest, code: "AGENT_TOKEN_INVALID", message: "invalid token name or scopes"},
		{name: "token scopes invalid", err: service.ErrAgentTokenScopesInvalid, status: http.StatusBadRequest, code: "AGENT_TOKEN_INVALID", message: "invalid token name or scopes"},
		{name: "token limit reached", err: service.ErrAgentTokenLimitReached, status: http.StatusConflict, code: "AGENT_TOKEN_LIMIT_REACHED", message: "too many active tokens, revoke one first"},
	}
	if len(cases) != 3 {
		t.Fatalf("create agent token contract table must keep the historical 3 rules, got %d", len(cases))
	}
	if len(createAgentTokenErrMap.rules) != 3 {
		t.Fatalf("create agent token map must keep exactly 3 rules, got %d", len(createAgentTokenErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, createAgentTokenErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("issue token: validation pipeline: %w", tt.err)
			assertErrMapResponse(t, createAgentTokenErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
	boom := errors.New("token repo: dial tcp 10.9.9.9:5432: boom")
	assertErrMapResponse(t, createAgentTokenErrMap, boom, http.StatusInternalServerError,
		envelope("DB_ERROR", "failed to create token"))
}

// TestRevokeAgentTokenErrMap_RuleContract pins the Revoke rule.
func TestRevokeAgentTokenErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "token not found", err: service.ErrAgentTokenNotFound, status: http.StatusNotFound, code: "AGENT_TOKEN_NOT_FOUND", message: "token not found"},
	}
	if len(cases) != 1 {
		t.Fatalf("revoke agent token contract table must keep the historical 1 rule, got %d", len(cases))
	}
	if len(revokeAgentTokenErrMap.rules) != 1 {
		t.Fatalf("revoke agent token map must keep exactly 1 rule, got %d", len(revokeAgentTokenErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, revokeAgentTokenErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("revoke token: owner scope: %w", tt.err)
			assertErrMapResponse(t, revokeAgentTokenErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
	boom := errors.New("token repo: context deadline exceeded")
	assertErrMapResponse(t, revokeAgentTokenErrMap, boom, http.StatusInternalServerError,
		envelope("DB_ERROR", "failed to revoke token"))
}
