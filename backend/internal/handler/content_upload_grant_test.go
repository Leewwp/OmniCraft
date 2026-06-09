package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
)

func TestGenerateOSSTokenReturnsUploadGrant(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cfg := testOSSUploadConfig()
	h := NewContentHandler(db, cfg, rdb)
	r := gin.New()
	r.POST("/contents/oss-token", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(42))
		h.GenerateOSSToken(c)
	})

	body := strings.NewReader(`{"file_name":"avatar.png","file_type":"image","mime_type":"image/png","file_size":123}`)
	req := httptest.NewRequest(http.MethodPost, "/contents/oss-token", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		GrantID string `json:"grant_id"`
		OSSKey  string `json:"oss_key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.GrantID == "" {
		t.Fatalf("grant_id is empty; body=%s", w.Body.String())
	}
	if !strings.HasPrefix(resp.OSSKey, "uploads/42/image/") {
		t.Fatalf("oss_key = %q, want uploads/42/image/ prefix", resp.OSSKey)
	}
	if !mr.Exists("upload:grant:" + resp.GrantID) {
		t.Fatalf("redis missing upload grant %q", resp.GrantID)
	}
}

func testOSSUploadConfig() *config.Config {
	return &config.Config{
		OSS: config.OSSConfig{
			Endpoint:        "https://oss-cn-hangzhou.aliyuncs.com",
			AccessKeyID:     "test-access-key",
			AccessKeySecret: "test-access-secret",
			BucketName:      "test-bucket",
			DownloadURLTTL:  300,
		},
		Limits: config.LimitsConfig{
			VideoMaxMB:      300,
			VideoMaxSec:     180,
			ImageMaxMB:      20,
			TextMaxMB:       10,
			ModMaxMB:        500,
			SheetMusicMaxMB: 50,
		},
		Upload: config.UploadConfig{
			SheetMusicExtensions: []string{".mid", ".midi", ".xml", ".mxl", ".mscz", ".mscx", ".pdf"},
		},
		Feedback: config.FeedbackConfig{
			UploadGrantTTLSec: 300,
		},
	}
}
