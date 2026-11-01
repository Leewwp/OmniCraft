package handler

// series 九路由共享合同的契约表（#924 从共享 switch 函数 writeSeriesError
// 迁到 seriesErrMap 声明）。期望值全部为迁移前手写字面量（= wire 的逐字
// 节锁定）；规则数锁历史值 10 防漂移，规则序 = 迁移前 case 序。

import (
	"fmt"
	"net/http"
	"testing"

	"omnicraft/backend/internal/repository"
)

// TestSeriesErrMap_RuleContract pins every series rule (order = pre-#924
// switch case order); the wrapped variant proves errors.Is reaches the same
// rule through fmt.Errorf %w chains.
func TestSeriesErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "series not found", err: repository.ErrSeriesNotFound, status: http.StatusNotFound, code: "SERIES_NOT_FOUND", message: "series not found"},
		{name: "not series owner", err: repository.ErrNotSeriesOwner, status: http.StatusForbidden, code: "NOT_SERIES_OWNER", message: "only the series owner can manage it"},
		{name: "content not owned or contributed", err: repository.ErrContentNotOwnedOrContributed, status: http.StatusBadRequest, code: "CONTENT_NOT_OWNED_OR_CONTRIBUTED", message: "content is not owned or contributed by the series owner"},
		{name: "zone mismatch", err: repository.ErrSeriesZoneMismatch, status: http.StatusBadRequest, code: "ZONE_MISMATCH", message: "series and content zones do not match"},
		{name: "duplicate series item", err: repository.ErrDuplicateSeriesItem, status: http.StatusConflict, code: "DUPLICATE_SERIES_ITEM", message: "content already exists in series"},
		{name: "cover not in series", err: repository.ErrCoverNotInSeries, status: http.StatusBadRequest, code: "COVER_NOT_IN_SERIES", message: "cover content must belong to the series"},
		{name: "series content unavailable", err: repository.ErrSeriesContentUnavailable, status: http.StatusBadRequest, code: "CONTENT_UNAVAILABLE", message: "content is unavailable for series management"},
		{name: "series item set mismatch", err: repository.ErrSeriesItemSetMismatch, status: http.StatusBadRequest, code: "SERIES_ITEM_SET_MISMATCH", message: "item_ids must match the complete series item set"},
		{name: "series item not found", err: repository.ErrSeriesItemNotFound, status: http.StatusNotFound, code: "SERIES_ITEM_NOT_FOUND", message: "series item not found"},
		{name: "invalid series", err: repository.ErrInvalidSeries, status: http.StatusBadRequest, code: "INVALID_SERIES", message: "invalid series"},
	}
	if len(cases) != 10 {
		t.Fatalf("series contract table must keep the historical 10 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, seriesErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("series mutation failed: %w", tt.err)
			assertErrMapResponse(t, seriesErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestSeriesErrMap_MultiMatchKeepsCaseOrder：组合错误同时命中多条规则时
// 按表序（=迁移前 case 序）首命中。
func TestSeriesErrMap_MultiMatchKeepsCaseOrder(t *testing.T) {
	combined := fmt.Errorf("combined failure: %w; %w", repository.ErrInvalidSeries, repository.ErrSeriesNotFound)
	assertErrMapResponse(t, seriesErrMap, combined, http.StatusNotFound,
		envelope("SERIES_NOT_FOUND", "series not found"))
}

// TestSeriesErrMap_UnknownErrorFallback pins the pre-#924 default branch:
// unknown errors answer Safe 500 DB_ERROR and never leak the raw cause.
func TestSeriesErrMap_UnknownErrorFallback(t *testing.T) {
	leaky := fmt.Errorf("series repo: sql: no such table %s", "content_series; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, seriesErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}
