package handler

// tag 建议面的契约表（#924 从手写链迁到 endpointErrMap 声明）。期望值全
// 部为迁移前手写字面量；两条分支都走 SafeErrorResponse（按 code 解析安
// 全文案），规则化后同为 Safe 路径（message 空）。

import (
	"fmt"
	"net/http"
	"testing"

	"omnicraft/backend/internal/service"
)

// TestSuggestTagErrMap_RuleContract pins SuggestTag：限流 sentinel → Safe
// 429 专用码（文案按 code 解析，历史行为）；其余错误 Safe 400 ERROR。
func TestSuggestTagErrMap_RuleContract(t *testing.T) {
	if len(suggestTagErrMap.rules) != 1 {
		t.Fatalf("suggest tag map must keep exactly 1 rule, got %d", len(suggestTagErrMap.rules))
	}
	// TAG_SUGGEST_RATE_LIMIT_EXCEEDED 不在安全文案表内，落通用安全文案
	//（迁移前后同一 SafeErrorResponse 调用，行为一致）。
	assertErrMapResponse(t, suggestTagErrMap, service.ErrTagSuggestRateLimited, http.StatusTooManyRequests,
		envelope("TAG_SUGGEST_RATE_LIMIT_EXCEEDED", "an unexpected error occurred, please try again later"))
	wrapped := fmt.Errorf("suggest tag: %w", service.ErrTagSuggestRateLimited)
	assertErrMapResponse(t, suggestTagErrMap, wrapped, http.StatusTooManyRequests,
		envelope("TAG_SUGGEST_RATE_LIMIT_EXCEEDED", "an unexpected error occurred, please try again later"))

	// 兜底 = 迁移前 SafeErrorResponse(c, 400, "ERROR", err)。
	leaky := fmt.Errorf("tag svc: sql: no such table %s", "tag_suggestions; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, suggestTagErrMap, leaky, http.StatusBadRequest,
		envelope("ERROR", "an error occurred, please try again later"))
}
