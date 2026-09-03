package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
)

func setupPublicConfigTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Server: config.ServerConfig{Port: "8080", Mode: "debug"},
		Features: config.FeaturesConfig{
			PaymentEnabled:        false,
			CreatorSupportEnabled: false,
			DesktopDeployEnabled:  false,
		},
		Agent: config.AgentConfig{
			WebAgentEnabled: false,
		},
		Captcha: config.CaptchaConfig{
			Provider: "aliyun_v2",
			Prefix:   "",
			SceneID:  "",
			Region:   "cn",
		},
		Client: config.ClientConfig{
			DownloadEnabled: false,
			DownloadURL:     "",
			LatestVersion:   "",
		},
		Upload: config.UploadConfig{
			ImageGalleryMinItems: 2,
			ImageGalleryMaxItems: 9,
			VideoGalleryMinItems: 1,
			VideoGalleryMaxItems: 3,
		},
		Limits: config.LimitsConfig{
			VideoMaxMB:      300,
			VideoMaxSec:     180,
			ImageMaxMB:      20,
			TextMaxMB:       10,
			ModMaxMB:        500,
			SheetMusicMaxMB: 50,
		},
		Publish: config.PublishConfig{
			TypeOrderOriginal: []string{"image", "article", "video"},
			TypeOrderFanwork:  []string{"mod", "prompt", "image"},
		},
		Social: config.SocialConfig{
			CommentFoldThreshold: 0.30,
		},
		Collaboration: config.CollaborationConfig{
			InviteDailyLimit:       20,
			InviteExpireDays:       7,
			MaxInviteesPerPublish:  5,
			MaxContributorsPerItem: 10,
		},
	}

	r := gin.New()
	handler := NewPublicConfigHandler(cfg)
	v1 := r.Group("/api/v1")
	{
		v1.GET("/config/public", handler.GetPublicConfig)
	}

	return r
}

func TestPublicConfigAllowlist(t *testing.T) {
	r := setupPublicConfigTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/public", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	features, ok := resp["features"].(map[string]interface{})
	if !ok {
		t.Fatal("response must contain 'features' object")
	}
	for _, key := range []string{"web_agent_enabled", "payment_enabled", "creator_support_enabled", "desktop_deploy_enabled"} {
		if _, has := features[key]; !has {
			t.Errorf("features must contain '%s'", key)
		}
	}

	captcha, ok := resp["captcha"].(map[string]interface{})
	if !ok {
		t.Fatal("response must contain 'captcha' object")
	}
	for _, key := range []string{"provider", "prefix", "scene_id", "region"} {
		if _, has := captcha[key]; !has {
			t.Errorf("captcha must contain '%s'", key)
		}
	}

	client, ok := resp["client"].(map[string]interface{})
	if !ok {
		t.Fatal("response must contain 'client' object")
	}
	for _, key := range []string{"download_enabled", "download_url", "latest_version"} {
		if _, has := client[key]; !has {
			t.Errorf("client must contain '%s'", key)
		}
	}

	body := w.Body.String()
	// T47（FIX-29c）按票面裁决暴露 comment_fold_threshold，故 'threshold'
	// 不再是禁词；其余敏感词照旧全量禁止。
	forbidden := []string{"secret", "access_key", "api_key", "dsn", "password", "hmac", "private", "cdn", "ttl", "rate_limit", "min_score", "auto_hide"}
	for _, word := range forbidden {
		if strings.Contains(strings.ToLower(body), strings.ToLower(word)) {
			t.Errorf("public config must not contain '%s'", word)
		}
	}

	if _, has := resp["reputation"]; has {
		t.Error("public config must not expose 'reputation'")
	}
	if _, has := resp["judge"]; has {
		t.Error("public config must not expose 'judge'")
	}
	// T47：social 仅允许暴露 comment_fold_threshold（折叠展示阈值），
	// report_auto_hide_rate 等审核旋钮保持 server-only。
	social, ok := resp["social"].(map[string]interface{})
	if !ok {
		t.Fatal("response must contain 'social' object (comment_fold_threshold)")
	}
	for key := range social {
		if key != "comment_fold_threshold" {
			t.Errorf("social must not expose server-only key '%s'", key)
		}
	}
	if _, has := resp["cache"]; has {
		t.Error("public config must not expose 'cache'")
	}
	if _, has := resp["rate_limit"]; has {
		t.Error("public config must not expose 'rate_limit'")
	}
	if _, has := resp["recommendation"]; has {
		t.Error("public config must not expose 'recommendation'")
	}

	collab, ok := resp["collaboration"].(map[string]interface{})
	if !ok {
		t.Fatal("response must contain 'collaboration' object")
	}
	for key := range collab {
		if key != "max_invitees_per_publish" {
			t.Errorf("collaboration must not expose server-only key '%s'", key)
		}
	}
	for _, word := range []string{"invite_daily_limit", "invite_expire_days", "max_contributors_per_item", "inviter"} {
		if strings.Contains(body, word) {
			t.Errorf("public config must not contain '%s'", word)
		}
	}
}

func TestPublicConfigExposesOnlyGalleryLimits(t *testing.T) {
	r := setupPublicConfigTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/public", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	upload, ok := resp["upload"].(map[string]interface{})
	if !ok {
		t.Fatal("response must expose the non-sensitive 'upload' limits object")
	}
	want := map[string]int{
		"image_gallery_min_items": 2,
		"image_gallery_max_items": 9,
		"video_gallery_min_items": 1,
		"video_gallery_max_items": 3,
	}
	if len(upload) != len(want) {
		t.Fatalf("upload object has %d keys, want exactly %d (only the gallery limits)", len(upload), len(want))
	}
	for key, wantValue := range want {
		got, has := upload[key]
		if !has {
			t.Errorf("upload must contain '%s'", key)
			continue
		}
		number, ok := got.(float64)
		if !ok || int(number) != wantValue {
			t.Errorf("upload.%s = %v, want %d", key, got, wantValue)
		}
	}
	for key := range upload {
		if _, known := want[key]; !known {
			t.Errorf("upload must not expose unexpected key '%s'", key)
		}
	}
}

func TestPublicConfigExposesOnlyCollaborationMaxInviteesPerPublish(t *testing.T) {
	r := setupPublicConfigTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/public", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	collab, ok := resp["collaboration"].(map[string]interface{})
	if !ok {
		t.Fatal("response must expose the non-sensitive 'collaboration' object")
	}
	want := map[string]int{"max_invitees_per_publish": 5}
	if len(collab) != len(want) {
		t.Fatalf("collaboration object has %d keys, want exactly %d (only max_invitees_per_publish)", len(collab), len(want))
	}
	for key, wantValue := range want {
		got, has := collab[key]
		if !has {
			t.Errorf("collaboration must contain '%s'", key)
			continue
		}
		number, ok := got.(float64)
		if !ok || int(number) != wantValue {
			t.Errorf("collaboration.%s = %v, want %d", key, got, wantValue)
		}
	}
	for key := range collab {
		if _, known := want[key]; !known {
			t.Errorf("collaboration must not expose unexpected key '%s'", key)
		}
	}
}

func TestPublicConfigNormalizesOmittedGalleryLimits(t *testing.T) {
	cfg := &config.Config{}
	r := gin.New()
	r.GET("/api/v1/config/public", NewPublicConfigHandler(cfg).GetPublicConfig)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/config/public", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp PublicConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Upload != (PublicUploadDTO{ImageGalleryMinItems: 2, ImageGalleryMaxItems: 9, VideoGalleryMinItems: 1, VideoGalleryMaxItems: 3}) {
		t.Fatalf("upload limits = %#v, want specification defaults", resp.Upload)
	}
}

// T25（FIX-41）：发布类型顺序与上传大小上限随公开配置下发（脱敏数值，
// 前端动态消费，admin 改配置无需发版）。
func TestPublicConfigExposesPublishTypeOrderAndUploadCaps(t *testing.T) {
	r := setupPublicConfigTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/public", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Publish struct {
			TypeOrderOriginal []string `json:"type_order_original"`
			TypeOrderFanwork  []string `json:"type_order_fanwork"`
		} `json:"publish"`
		Limits map[string]int `json:"limits"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Publish.TypeOrderOriginal) != 3 || resp.Publish.TypeOrderOriginal[0] != "image" {
		t.Errorf("publish.type_order_original = %v, want config order", resp.Publish.TypeOrderOriginal)
	}
	if len(resp.Publish.TypeOrderFanwork) != 3 || resp.Publish.TypeOrderFanwork[0] != "mod" {
		t.Errorf("publish.type_order_fanwork = %v, want config order", resp.Publish.TypeOrderFanwork)
	}

	wantCaps := map[string]int{
		"video_max_mb":      300,
		"image_max_mb":      20,
		"text_max_mb":       10,
		"mod_max_mb":        500,
		"sheet_music_max_mb": 50,
	}
	if len(resp.Limits) != len(wantCaps) {
		t.Fatalf("limits object has %d keys %v, want exactly the %d upload caps", len(resp.Limits), resp.Limits, len(wantCaps))
	}
	for key, wantValue := range wantCaps {
		got, has := resp.Limits[key]
		if !has {
			t.Errorf("limits must contain '%s'", key)
			continue
		}
		if got != wantValue {
			t.Errorf("limits.%s = %d, want %d", key, got, wantValue)
		}
	}
}

// T47（FIX-29c）：评论折叠阈值随公开配置下发（前端禁止硬编码 0.30）。
func TestPublicConfigExposesCommentFoldThreshold(t *testing.T) {
	r := setupPublicConfigTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/config/public", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Social struct {
			CommentFoldThreshold float64 `json:"comment_fold_threshold"`
		} `json:"social"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Social.CommentFoldThreshold <= 0 || resp.Social.CommentFoldThreshold >= 1 {
		t.Fatalf("social.comment_fold_threshold = %v, want a configured ratio in (0,1)", resp.Social.CommentFoldThreshold)
	}
}
