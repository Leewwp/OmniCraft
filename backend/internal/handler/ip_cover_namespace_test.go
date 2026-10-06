package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
)

/* #813（run-1 审计 #2）：IP 封面创建闸主体命名空间绑定——平台域内他人
 * uploads 命名空间与隔离区对象在进入 service 之前被拒；发起人本人命名空
 * 空间不受影响（放行后进入 service 正常流）。 */

func newIPCoverNamespaceTestHandler(t *testing.T) *IPHandler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	h := NewIPHandler(db)
	h.cfg = &config.Config{OSS: config.OSSConfig{Domain: "https://cdn.example.test"}}
	return h
}

func postCreateIPCoverAsUser(t *testing.T, h *IPHandler, callerID int64, coverURL string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.POST("/api/v1/ips", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, callerID)
		h.CreateIP(c)
	})
	rec := httptest.NewRecorder()
	body := fmt.Sprintf(`{"name":"测试IP","cover_url":%q}`, coverURL)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ips", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateIPCoverRejectsCrossNamespaceAndQuarantineURL(t *testing.T) {
	h := newIPCoverNamespaceTestHandler(t)
	const callerID = int64(1)

	cases := map[string]string{
		"other user's uploads namespace": "https://cdn.example.test/uploads/2/image/cover.png",
		"quarantine object":              "https://cdn.example.test/quarantine/archive-scan/9/2/job11",
	}
	for name, coverURL := range cases {
		t.Run(name, func(t *testing.T) {
			rec := postCreateIPCoverAsUser(t, h, callerID, coverURL)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte("COVER_NOT_PLATFORM_OSS_OBJECT")) {
				t.Fatalf("body = %s, want COVER_NOT_PLATFORM_OSS_OBJECT", rec.Body.String())
			}
		})
	}

	// 本人命名空间通过封面闸（进入 service 后因测试库缺表而非 400 封面错）。
	rec := postCreateIPCoverAsUser(t, h, callerID, "https://cdn.example.test/uploads/1/image/cover.png")
	if bytes.Contains(rec.Body.Bytes(), []byte("COVER_NOT_PLATFORM_OSS_OBJECT")) {
		t.Fatalf("own-namespace cover must pass the cover gate; body = %s", rec.Body.String())
	}
}
