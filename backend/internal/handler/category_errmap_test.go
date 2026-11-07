package handler

// category admin 四端点的契约表（#924 从 respondCategoryAuditError 前置
// if + 兜底分支迁到各端点 map 声明；审计写失败 500 AUDIT_WRITE_FAILED
// 是各 map 首条规则，与 admin.go 收编模式一致）。期望值全部为迁移前手
// 写字面量；规则数锁历史值防漂移。

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"omnicraft/backend/internal/service"
)

// wrappedAuditTxError 模拟 withAuditTx 包装链上的审计失败（迁移前
// respondCategoryAuditError 用 errors.Is 识别同一 sentinel）。
func wrappedCategoryAuditTxError() error {
	return fmt.Errorf("category audit tx: %w", errAdminAuditWriteFailed)
}

// TestCategoryCreateErrMap_RuleContract pins AdminCreateCategory：审计写
// 失败 → 500；其余 Safe 500 DB_ERROR。
func TestCategoryCreateErrMap_RuleContract(t *testing.T) {
	if len(categoryCreateErrMap.rules) != 1 {
		t.Fatalf("category create map must keep exactly 1 rule, got %d", len(categoryCreateErrMap.rules))
	}
	assertErrMapResponse(t, categoryCreateErrMap, wrappedCategoryAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))

	leaky := fmt.Errorf("category tx: sql: no such table %s", "categories; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, categoryCreateErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}

// TestCategoryUpdateErrMap_RuleContract pins AdminUpdateCategory：审计写
// 失败 → 500；其余（含事务内翻出的 ErrCategoryNotFound）Safe 400 ERROR
// ——迁移前非审计失败一律 SafeErrorResponse(c, 400, "ERROR", err)。
func TestCategoryUpdateErrMap_RuleContract(t *testing.T) {
	if len(categoryUpdateErrMap.rules) != 1 {
		t.Fatalf("category update map must keep exactly 1 rule, got %d", len(categoryUpdateErrMap.rules))
	}
	assertErrMapResponse(t, categoryUpdateErrMap, errAdminAuditWriteFailed, http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))

	leaky := errors.New("category update: dial tcp 10.0.0.1:5432: secret")
	assertErrMapResponse(t, categoryUpdateErrMap, leaky, http.StatusBadRequest,
		envelope("ERROR", "an error occurred, please try again later"))
}

// TestCategoryDeleteErrMap_RuleContract pins AdminDeleteCategory：审计写
// 失败 → 500；有子类/挂内容 → Safe 409 CONFLICT（迁移前 `==` 直比两哨
// 兵，规则化后 errors.Is 同规则生效）；其余 Safe 500 DB_ERROR。
func TestCategoryDeleteErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "audit write failed", err: wrappedCategoryAuditTxError(), status: http.StatusInternalServerError, code: "AUDIT_WRITE_FAILED", message: "audit write failed"},
		{name: "category has children", err: service.ErrCategoryHasChildren, status: http.StatusConflict, code: "CONFLICT", message: "resource conflict"},
		{name: "category has content", err: service.ErrCategoryHasContent, status: http.StatusConflict, code: "CONFLICT", message: "resource conflict"},
	}
	if len(cases) != 3 {
		t.Fatalf("category delete contract table must keep the historical 3 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, categoryDeleteErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
		})
	}
	wrapped := fmt.Errorf("delete category: %w", service.ErrCategoryHasChildren)
	assertErrMapResponse(t, categoryDeleteErrMap, wrapped, http.StatusConflict,
		envelope("CONFLICT", "resource conflict"))

	leaky := fmt.Errorf("category tx: sql: no such table %s", "categories; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, categoryDeleteErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}

// TestCategoryReorderErrMap_RuleContract pins AdminReorderCategories：审
// 计写失败 → 500；其余 Safe 500 DB_ERROR。
func TestCategoryReorderErrMap_RuleContract(t *testing.T) {
	if len(categoryReorderErrMap.rules) != 1 {
		t.Fatalf("category reorder map must keep exactly 1 rule, got %d", len(categoryReorderErrMap.rules))
	}
	assertErrMapResponse(t, categoryReorderErrMap, errAdminAuditWriteFailed, http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))

	leaky := errors.New("category reorder: dial tcp 10.0.0.1:5432: secret")
	assertErrMapResponse(t, categoryReorderErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}
