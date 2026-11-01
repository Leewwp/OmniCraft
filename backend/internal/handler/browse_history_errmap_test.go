package handler

// 浏览历史清除确认面的契约表（#924 从手写链迁到 endpointErrMap 声明）。
// ClearHistory 的空 body（io.EOF）要求显式确认；其余绑定错误走迁移前
// ValidationError 形态的固定兜底（逐字节保留，非 Safe 语义）。

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// TestClearHistoryBindErrMap_RuleContract pins ClearHistory 绑定失败面：
// io.EOF → 400 CLEAR_CONFIRMATION_REQUIRED 固定文案；其余绑定错误 →
// 400 VALIDATION_ERROR 固定文案 "invalid request body"。
func TestClearHistoryBindErrMap_RuleContract(t *testing.T) {
	if len(clearHistoryBindErrMap.rules) != 1 {
		t.Fatalf("clear history bind map must keep exactly 1 rule, got %d", len(clearHistoryBindErrMap.rules))
	}
	assertErrMapResponse(t, clearHistoryBindErrMap, io.EOF, http.StatusBadRequest,
		envelope("CLEAR_CONFIRMATION_REQUIRED", "explicit clear confirmation is required"))
	wrapped := fmt.Errorf("bind clear body: %w", io.EOF)
	assertErrMapResponse(t, clearHistoryBindErrMap, wrapped, http.StatusBadRequest,
		envelope("CLEAR_CONFIRMATION_REQUIRED", "explicit clear confirmation is required"))

	// 兜底 = 迁移前 ValidationError（固定 message，非 Safe 按 code 解析）。
	leaky := errors.New("json: cannot unmarshal string into clear body (dsn=postgres://u:secret@10.0.0.1/db)")
	assertErrMapResponse(t, clearHistoryBindErrMap, leaky, http.StatusBadRequest,
		envelope("VALIDATION_ERROR", "invalid request body"))
}
