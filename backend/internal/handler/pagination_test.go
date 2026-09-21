package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/model"
)

// SP-25 FR-10（低-26）：平台统一分页钳制契约——page≥1、1≤page_size≤100、
// 越界（含不可解析→0）回落 20；page_size=100000 拉全量在全仓消失。

func TestClampPageContract(t *testing.T) {
	cases := []struct {
		name                 string
		page, size           int
		wantPage, wantSize   int
	}{
		{"in-range passes", 1, 20, 1, 20},
		{"upper bound passes", 3, 100, 3, 100},
		{"zero page resets to 1", 0, 20, 1, 20},
		{"negative page resets to 1", -5, 20, 1, 20},
		{"zero size falls back", 1, 0, 1, 20},
		{"oversize size falls back", 1, 100000, 1, 20},
		{"just over bound falls back", 1, 101, 1, 20},
		{"negative size falls back", 1, -1, 1, 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, size := clampPage(tc.page, tc.size)
			if page != tc.wantPage || size != tc.wantSize {
				t.Fatalf("clampPage(%d, %d) = (%d, %d), want (%d, %d)",
					tc.page, tc.size, page, size, tc.wantPage, tc.wantSize)
			}
		})
	}
}

func TestPageQueryUsesSiteDefaultOnlyWhenAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newCtx := func(url string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, url, nil)
		return c
	}
	// 站点默认 50 只在参数缺省时生效。
	if _, size := pageQuery(newCtx("/?page=1"), 50); size != 50 {
		t.Fatalf("absent page_size must keep the site default 50, got %d", size)
	}
	// 越界一律回落平台默认 20（不是站点默认）。
	if _, size := pageQuery(newCtx("/?page=1&page_size=100000"), 50); size != 20 {
		t.Fatalf("oversize must fall back to the platform default 20, got %d", size)
	}
}

// 契约级验证：notifications 列表（家族端点）在 page_size=100000 下只回一页 20 条。
func TestListNotificationsClampsOversizePageSize(t *testing.T) {
	router, db := setupNotificationDecorateRouter(t)

	const seeded = 150
	rows := make([]model.Notification, 0, seeded)
	for i := 0; i < seeded; i++ {
		rows = append(rows, model.Notification{
			UserID: 1, Channel: "system", Type: "test",
			Title: strPtr("n"), Body: strPtr("b"),
		})
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?page=1&page_size=100000", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Notifications []map[string]any `json:"notifications"`
		Total         int64            `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Total != seeded {
		t.Fatalf("total = %d, want %d", payload.Total, seeded)
	}
	if len(payload.Notifications) != 20 {
		t.Fatalf("oversize page_size=100000 returned %d rows, want the clamped page of 20 (no full-table dump)",
			len(payload.Notifications))
	}
}

func strPtr(s string) *string { return &s }
