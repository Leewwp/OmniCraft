package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/middleware"
)

/* SP-19 G1-3（#517）：IP 创建分类校验——config ip_categories allowlist 之外的
 * 值在进入 service 之前被拒（400 VALIDATION_ERROR）；allowlist 未配置时放行
 * （旧构造器/测试路径不阻塞）。 */

func newIPCategoryTestHandler(t *testing.T, allowlist []string) *IPHandler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	h := NewIPHandler(db)
	h.ipCategories = allowlist
	return h
}

func postCreateIP(t *testing.T, h *IPHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.POST("/api/v1/ips", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		h.CreateIP(c)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ips", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateIPRejectsCategoryOutsideAllowlist(t *testing.T) {
	h := newIPCategoryTestHandler(t, []string{"game", "anime", "other"})

	rec := postCreateIP(t, h, `{"name":"测试IP","category":"gaming"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("VALIDATION_ERROR")) {
		t.Fatalf("body = %s, want VALIDATION_ERROR", rec.Body.String())
	}

	// 合法值放行（进入 service 后因测试库缺表而 5xx 也证明已过校验层，
	// 但这里用合法值 + 完整迁移不可行——直接断言不再返回 400 校验错）。
	rec = postCreateIP(t, h, `{"name":"测试IP","category":"game"}`)
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("valid category must pass the allowlist gate; body = %s", rec.Body.String())
	}
}

func TestCreateIPCategoryGateSkippedWithoutAllowlist(t *testing.T) {
	h := newIPCategoryTestHandler(t, nil)

	rec := postCreateIP(t, h, `{"name":"测试IP","category":"gaming"}`)
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("empty allowlist must not block creation; body = %s", rec.Body.String())
	}
}

func TestCreateIPAllowsEmptyCategory(t *testing.T) {
	h := newIPCategoryTestHandler(t, []string{"game"})
	_ = h // 分类为空 = 不分类，始终允许；空 body 分类字段走 required 校验之外。
	rec := postCreateIP(t, h, `{"name":"测试IP"}`)
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("empty category must pass; body = %s", rec.Body.String())
	}
}
