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
	forbidden := []string{"secret", "access_key", "api_key", "dsn", "password", "hmac", "private", "cdn", "ttl", "rate_limit", "threshold", "min_score"}
	for _, word := range forbidden {
		if strings.Contains(strings.ToLower(body), strings.ToLower(word)) {
			t.Errorf("public config must not contain '%s'", word)
		}
	}

	if _, has := resp["limits"]; has {
		t.Error("public config must not expose 'limits'")
	}
	if _, has := resp["reputation"]; has {
		t.Error("public config must not expose 'reputation'")
	}
	if _, has := resp["judge"]; has {
		t.Error("public config must not expose 'judge'")
	}
	if _, has := resp["social"]; has {
		t.Error("public config must not expose 'social'")
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
