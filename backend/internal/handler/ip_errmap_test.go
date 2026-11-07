package handler

// ip 两端点错误面的契约表（#924 从手写链迁到 endpointErrMap 声明）：
// CreateIP 的 slug 冲突（errors.Is 链）与详情读侧双路由的 NotFound
//（迁移前 `==` 直比变体，GetIP / GetIPContents 共用一份声明）。期望值
// 全部为迁移前手写字面量；规则数锁历史值防漂移。

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"omnicraft/backend/internal/service"
)

// TestCreateIPErrMap_RuleContract pins CreateIP：slug 冲突 → 409 固定文
// 案；其余 Safe 500 INTERNAL_ERROR。
func TestCreateIPErrMap_RuleContract(t *testing.T) {
	if len(createIPErrMap.rules) != 1 {
		t.Fatalf("create ip map must keep exactly 1 rule, got %d", len(createIPErrMap.rules))
	}
	assertErrMapResponse(t, createIPErrMap, service.ErrIPSlugTaken, http.StatusConflict,
		envelope("IP_SLUG_TAKEN", "could not derive a unique slug for this name, try a different name"))
	wrapped := fmt.Errorf("create ip: %w", service.ErrIPSlugTaken)
	assertErrMapResponse(t, createIPErrMap, wrapped, http.StatusConflict,
		envelope("IP_SLUG_TAKEN", "could not derive a unique slug for this name, try a different name"))

	leaky := errors.New("ip service: dial tcp 10.0.0.1:5432: secret")
	assertErrMapResponse(t, createIPErrMap, leaky, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "an unexpected error occurred, please try again later"))
}

// TestIPDetailErrMap_RuleContract pins GetIP / GetIPContents：NotFound →
// 404 固定文案（`==` 直比规则化后 wrapped 变体同规则生效）；其余 Safe
// 500 DB_ERROR。
func TestIPDetailErrMap_RuleContract(t *testing.T) {
	if len(ipDetailErrMap.rules) != 1 {
		t.Fatalf("ip detail map must keep exactly 1 rule, got %d", len(ipDetailErrMap.rules))
	}
	assertErrMapResponse(t, ipDetailErrMap, service.ErrIPNotFound, http.StatusNotFound,
		envelope("IP_NOT_FOUND", "ip not found"))
	wrapped := fmt.Errorf("get ip: %w", service.ErrIPNotFound)
	assertErrMapResponse(t, ipDetailErrMap, wrapped, http.StatusNotFound,
		envelope("IP_NOT_FOUND", "ip not found"))

	leaky := fmt.Errorf("ip repo: sql: no such table %s", "ips; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, ipDetailErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}
