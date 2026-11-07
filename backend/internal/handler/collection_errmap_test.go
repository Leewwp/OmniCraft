package handler

// collection 全路由共享合同的契约表（#924 从共享 switch 函数
// writeCollectionError 迁到 collectionErrMap 声明）。期望值全部为迁移前
// 手写字面量；规则数锁历史值 6 防漂移，规则序 = 迁移前 case 序。

import (
	"fmt"
	"net/http"
	"testing"

	"omnicraft/backend/internal/repository"
)

// TestCollectionErrMap_RuleContract pins every collection rule (order =
// pre-#924 switch case order); the wrapped variant proves errors.Is reaches
// the same rule through fmt.Errorf %w chains.
func TestCollectionErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "collection not found", err: repository.ErrCollectionNotFound, status: http.StatusNotFound, code: "COLLECTION_NOT_FOUND", message: "collection not found"},
		{name: "zone mismatch", err: repository.ErrZoneMismatch, status: http.StatusBadRequest, code: "ZONE_MISMATCH", message: "collection and content zones do not match"},
		{name: "duplicate collection item", err: repository.ErrDuplicateCollectionItem, status: http.StatusConflict, code: "DUPLICATE_COLLECTION_ITEM", message: "content already exists in collection"},
		{name: "default collection protected", err: repository.ErrDefaultCollectionProtected, status: http.StatusBadRequest, code: "DEFAULT_COLLECTION_PROTECTED", message: "default collection cannot be deleted"},
		{name: "zone immutable", err: repository.ErrZoneImmutable, status: http.StatusBadRequest, code: "ZONE_IMMUTABLE", message: "collection zone cannot be changed"},
		{name: "invalid content", err: repository.ErrInvalidContent, status: http.StatusBadRequest, code: "INVALID_CONTENT", message: "content is unavailable"},
	}
	if len(cases) != 6 {
		t.Fatalf("collection contract table must keep the historical 6 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, collectionErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("collection mutation failed: %w", tt.err)
			assertErrMapResponse(t, collectionErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestCollectionErrMap_UnknownErrorFallback pins the pre-#924 default
// branch: unknown errors answer Safe 500 DB_ERROR.
func TestCollectionErrMap_UnknownErrorFallback(t *testing.T) {
	leaky := fmt.Errorf("collection repo: sql: no such table %s", "collections; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, collectionErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}
