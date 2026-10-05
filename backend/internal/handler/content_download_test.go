package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
)

func TestContentDownload_ReturnsJSONNotRedirect(t *testing.T) {
	source := readHandlerSource(t, "content.go")
	if strings.Contains(source, "c.Redirect(") {
		t.Fatal("DownloadContent must return JSON {download_url, expires_in}, not c.Redirect")
	}
}

func TestContentDownload_SupportsAttachmentID(t *testing.T) {
	source := readHandlerSource(t, "content.go")
	if !strings.Contains(source, "attachment_id") {
		t.Fatal("DownloadContent must accept attachment_id query parameter")
	}
}

// #794：迁表守门改行为断言（原为按 content.go 字符串位置找
// AMBIGUOUS_ATTACHMENT 的守门）——多主附件或无主附件且未指定
// attachment_id 时，真实路由必须以 400 AMBIGUOUS_ATTACHMENT 的完整信封拒绝。
func TestContentDownload_RejectsAmbiguousWithoutAttachmentID(t *testing.T) {
	t.Run("multiple primary attachments", func(t *testing.T) {
		router, db := setupDownloadRouter(t, 999, true)
		author := createDownloadUser(t, db, "author-ambiguous@example.com", "author-ambiguous", false)
		content := createDownloadContent(t, db, author.ID, nil)
		second := model.ContentAttachment{
			ContentItemID: content.ID,
			FileType:      "sheet_music_pdf",
			OSSKey:        "uploads/1/sheet-2.pdf",
			MimeType:      "application/pdf",
			IsPrimary:     boolPtr(true),
		}
		if err := db.Create(&second).Error; err != nil {
			t.Fatalf("create second primary attachment: %v", err)
		}

		rec := requestDownload(t, router, content.ID)
		assertDownloadEnvelope(t, rec, http.StatusBadRequest, "AMBIGUOUS_ATTACHMENT", "specify attachment_id; cannot determine a unique primary attachment")
	})

	t.Run("no primary attachments", func(t *testing.T) {
		router, db := setupDownloadRouter(t, 999, true)
		author := createDownloadUser(t, db, "author-noprimary@example.com", "author-noprimary", false)
		content := createDownloadContent(t, db, author.ID, nil)
		if err := db.Model(&model.ContentAttachment{}).Where("content_item_id = ?", content.ID).Update("is_primary", false).Error; err != nil {
			t.Fatalf("clear primary flag: %v", err)
		}

		rec := requestDownload(t, router, content.ID)
		assertDownloadEnvelope(t, rec, http.StatusBadRequest, "AMBIGUOUS_ATTACHMENT", "specify attachment_id; cannot determine a unique primary attachment")
	})
}

func TestContentDownload_UsesConfigurableTTL(t *testing.T) {
	// SP-16 #451: the TTL policy moved into the service orchestration.
	source := readHandlerSource(t, "../service/content_download.go")
	if !strings.Contains(source, "DownloadURLTTL") && !strings.Contains(source, "download_url_ttl") {
		t.Fatal("DownloadContent must use configurable download URL TTL from config, not hardcoded value")
	}
}

func TestContentDownloadProtectedRouteRequiresAuth(t *testing.T) {
	router, contentID, _ := setupProtectedDownloadRoute(t, downloadRouteUserState{Verified: true, Reputation: 10})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+strconvFormatInt(contentID)+"/download", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "UNAUTHORIZED") {
		t.Fatalf("body = %s, want UNAUTHORIZED", rec.Body.String())
	}
}

func TestContentDownloadProtectedRouteRejectsUnverifiedUser(t *testing.T) {
	router, contentID, token := setupProtectedDownloadRoute(t, downloadRouteUserState{Verified: false, Reputation: 10})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+strconvFormatInt(contentID)+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "EMAIL_NOT_VERIFIED") {
		t.Fatalf("body = %s, want EMAIL_NOT_VERIFIED", rec.Body.String())
	}
}

func TestContentDownloadProtectedRouteRejectsLowReputationUser(t *testing.T) {
	router, contentID, token := setupProtectedDownloadRoute(t, downloadRouteUserState{Verified: true, Reputation: 2})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+strconvFormatInt(contentID)+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "INSUFFICIENT_REPUTATION") {
		t.Fatalf("body = %s, want INSUFFICIENT_REPUTATION", rec.Body.String())
	}
}

func TestOSSDownloadURLTTLConfigExists(t *testing.T) {
	source := readConfigSource(t)
	if !strings.Contains(source, "DownloadURLTTL") && !strings.Contains(source, "download_url_ttl") {
		t.Fatal("config must define DownloadURLTTL or download_url_ttl field for OSS download URL TTL")
	}
}

func readHandlerSource(t *testing.T, filename string) string {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	return string(data)
}

func readConfigSource(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"../config/config.go",
		"../../config/config.go",
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data)
		}
	}
	t.Fatal("cannot find config.go")
	return ""
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestContentDownload_RejectsAuthorBannedBeforeOSS(t *testing.T) {
	router, db := setupDownloadVisibilityRouter(t)
	author := createDownloadUser(t, db, "author-banned@example.com", "author-banned", true)
	content := createDownloadContent(t, db, author.ID, nil)

	rec := requestDownload(t, router, content.ID)

	assertContentUnavailable(t, rec)
}

func TestContentDownload_RejectsAuthorDeletedBeforeOSS(t *testing.T) {
	router, db := setupDownloadVisibilityRouter(t)
	author := createDownloadUser(t, db, "author-deleted@example.com", "author-deleted", false)
	if err := db.Exec("UPDATE users SET deleted_at = ? WHERE id = ?", time.Now(), author.ID).Error; err != nil {
		t.Fatalf("mark author deleted: %v", err)
	}
	content := createDownloadContent(t, db, author.ID, nil)

	rec := requestDownload(t, router, content.ID)

	assertContentUnavailable(t, rec)
}

func TestContentDownload_RejectsBannedIPBeforeOSS(t *testing.T) {
	router, db := setupDownloadVisibilityRouter(t)
	author := createDownloadUser(t, db, "author-ip@example.com", "author-ip", false)
	bannedIP := model.IP{Name: "Banned IP", Slug: "banned-ip", Status: "banned"}
	if err := db.Create(&bannedIP).Error; err != nil {
		t.Fatalf("create banned ip: %v", err)
	}
	content := createDownloadContent(t, db, author.ID, &bannedIP.ID)

	rec := requestDownload(t, router, content.ID)

	assertContentUnavailable(t, rec)
}

func TestContentDownload_DirectHandlerRequiresAuthenticatedUser(t *testing.T) {
	router, db := setupDownloadRouter(t, 0, true)
	author := createDownloadUser(t, db, "author-auth@example.com", "author-auth", false)
	content := createDownloadContent(t, db, author.ID, nil)

	rec := requestDownload(t, router, content.ID)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
	}
}

// #794：未知错误（无 sentinel 命中）必须落 DownloadContent 的历史兜底
// 500 DB_ERROR（与发布端点的 INTERNAL_ERROR 兜底不同）。真实路由驱动：
// 内容行与作者行就绪后 drop ips 表，可见性计数子查询以非 sentinel 的裸
// DB 错误失败——正好越过全部映射规则抵达兜底。
func TestContentDownload_UnknownErrorFallsBackToDBError(t *testing.T) {
	router, db := setupDownloadRouter(t, 999, true)
	author := createDownloadUser(t, db, "author-fallback@example.com", "author-fallback", false)
	content := createDownloadContent(t, db, author.ID, nil)
	if err := db.Migrator().DropTable(&model.IP{}); err != nil {
		t.Fatalf("drop ips table: %v", err)
	}

	rec := requestDownload(t, router, content.ID)
	assertDownloadEnvelope(t, rec, http.StatusInternalServerError, "DB_ERROR", "database operation failed, please try again later")
}

func TestContentDownload_BlocksAllowCopyFalse(t *testing.T) {
	router, db := setupDownloadRouter(t, 999, true)
	author := createDownloadUser(t, db, "author-copy@example.com", "author-copy", false)
	content := createDownloadContent(t, db, author.ID, nil)
	if err := db.Model(&model.ContentItem{}).Where("id = ?", content.ID).Update("allow_copy", false).Error; err != nil {
		t.Fatalf("disable allow_copy: %v", err)
	}

	rec := requestDownload(t, router, content.ID)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "download not allowed") {
		t.Fatalf("body = %s, want allow_copy denial", rec.Body.String())
	}
}

func TestContentDownload_RejectsAttachmentIDFromOtherContent(t *testing.T) {
	router, db := setupDownloadRouter(t, 999, true)
	author := createDownloadUser(t, db, "author-attachment@example.com", "author-attachment", false)
	content := createDownloadContent(t, db, author.ID, nil)
	other := createDownloadContent(t, db, author.ID, nil)
	var otherAttachment model.ContentAttachment
	if err := db.Where("content_item_id = ?", other.ID).First(&otherAttachment).Error; err != nil {
		t.Fatalf("load other attachment: %v", err)
	}

	rec := requestDownloadPath(t, router, content.ID, "?attachment_id="+strconvFormatInt(otherAttachment.ID))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ATTACHMENT_MISMATCH") {
		t.Fatalf("body = %s, want ATTACHMENT_MISMATCH", rec.Body.String())
	}
}

func TestContentDownload_PrivateContentOnlyDownloadableByAuthor(t *testing.T) {
	router, db := setupDownloadRouter(t, 999, true)
	author := createDownloadUser(t, db, "author-private@example.com", "author-private", false)
	content := createDownloadContent(t, db, author.ID, nil)
	if err := db.Model(&model.ContentItem{}).Where("id = ?", content.ID).Update("is_public", false).Error; err != nil {
		t.Fatalf("mark private: %v", err)
	}

	nonAuthorRec := requestDownload(t, router, content.ID)
	assertContentUnavailable(t, nonAuthorRec)

	authorRouter, _ := setupDownloadRouterWithDB(t, db, author.ID, true)
	authorRec := requestDownload(t, authorRouter, content.ID)
	if authorRec.Code != http.StatusOK {
		t.Fatalf("author status = %d, want 200; body = %s", authorRec.Code, authorRec.Body.String())
	}
}

func TestContentDownload_ResponseIncludesSignedURLAndExpiresIn(t *testing.T) {
	router, db := setupDownloadRouter(t, 999, true)
	author := createDownloadUser(t, db, "author-response@example.com", "author-response", false)
	content := createDownloadContent(t, db, author.ID, nil)

	rec := requestDownload(t, router, content.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "" {
		t.Fatalf("download response must be JSON, got redirect Location %q", rec.Header().Get("Location"))
	}
	var body struct {
		DownloadURL string `json:"download_url"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}
	if body.DownloadURL == "" || body.ExpiresIn <= 0 {
		t.Fatalf("response = %#v, want download_url and positive expires_in", body)
	}
}

func TestContentDownload_ArchiveGateOnlySignsCleanAttachments(t *testing.T) {
	for _, status := range []string{model.ScanStatusPending, model.ScanStatusBlocked} {
		router, db := setupArchiveGateDownloadRouter(t, status)
		author := createDownloadUser(t, db, "archive-gate-"+status+"@example.com", "archive-gate-"+status, false)
		content := createDownloadContent(t, db, author.ID, nil)
		if err := db.Model(&model.ContentAttachment{}).Where("content_item_id = ?", content.ID).Updates(map[string]interface{}{
			"file_type":     "mod",
			"scan_status":   status,
			"scan_required": true,
		}).Error; err != nil {
			t.Fatalf("set %s scan state: %v", status, err)
		}

		rec := requestDownload(t, router, content.ID)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status %s response code = %d, want 403; body = %s", status, rec.Code, rec.Body.String())
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s response: %v", status, err)
		}
		if body.Code != "ARCHIVE_NOT_CLEAN" {
			t.Fatalf("status %s response code = %q, want ARCHIVE_NOT_CLEAN", status, body.Code)
		}
	}

	router, db := setupArchiveGateDownloadRouter(t, model.ScanStatusClean)
	author := createDownloadUser(t, db, "archive-gate-clean@example.com", "archive-gate-clean", false)
	content := createDownloadContent(t, db, author.ID, nil)
	if err := db.Model(&model.ContentAttachment{}).Where("content_item_id = ?", content.ID).Updates(map[string]interface{}{
		"file_type":     "mod",
		"scan_status":   model.ScanStatusClean,
		"scan_required": true,
	}).Error; err != nil {
		t.Fatalf("set clean scan state: %v", err)
	}
	if rec := requestDownload(t, router, content.ID); rec.Code != http.StatusOK {
		t.Fatalf("clean response code = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	if err := db.Model(&model.ContentAttachment{}).Where("content_item_id = ?", content.ID).Updates(map[string]interface{}{
		"file_type":     "image",
		"scan_status":   model.ScanStatusNotRequired,
		"scan_required": false,
		"oss_key":       "quarantine/archive-scan/1/1/1",
	}).Error; err != nil {
		t.Fatalf("set quarantined non-archive attachment: %v", err)
	}
	quarantineRec := requestDownload(t, router, content.ID)
	if quarantineRec.Code != http.StatusForbidden || !strings.Contains(quarantineRec.Body.String(), "ARCHIVE_NOT_CLEAN") {
		t.Fatalf("quarantine response = %d %s, want 403 ARCHIVE_NOT_CLEAN", quarantineRec.Code, quarantineRec.Body.String())
	}
}

func setupDownloadVisibilityRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	return setupDownloadRouter(t, 999, false)
}

func setupArchiveGateDownloadRouter(t *testing.T, _ string) (*gin.Engine, *gorm.DB) {
	t.Helper()
	router, db := setupDownloadRouter(t, 999, true)
	cfg := &config.Config{
		Features: config.FeaturesConfig{ArchiveMalwareScanEnabled: true},
		OSS: config.OSSConfig{
			Endpoint:        "https://oss-cn-hangzhou.aliyuncs.com",
			AccessKeyID:     "test-access-key-id",
			AccessKeySecret: "test-access-key-secret",
			BucketName:      "test-bucket",
			DownloadURLTTL:  300,
		},
	}
	h, _ := newContentHandlerForTest(db, cfg, nil)
	router = gin.New()
	router.GET("/contents/:id/download", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(999))
		h.DownloadContent(c)
	})
	return router, db
}

func setupDownloadRouter(t *testing.T, userID int64, withOSS bool) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentAttachment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, check := range []struct {
		table  any
		column string
	}{
		{table: &model.User{}, column: "deleted_at"},
		{table: &model.ContentItem{}, column: "deleted_at"},
	} {
		if !db.Migrator().HasColumn(check.table, check.column) {
			t.Fatalf("expected AutoMigrate to own %s", check.column)
		}
	}

	return setupDownloadRouterWithDB(t, db, userID, withOSS)
}

func setupDownloadRouterWithDB(t *testing.T, db *gorm.DB, userID int64, withOSS bool) (*gin.Engine, *gorm.DB) {
	t.Helper()
	cfg := &config.Config{}
	if withOSS {
		cfg.OSS.Endpoint = "https://oss-cn-hangzhou.aliyuncs.com"
		cfg.OSS.AccessKeyID = "test-access-key-id"
		cfg.OSS.AccessKeySecret = "test-access-key-secret"
		cfg.OSS.BucketName = "test-bucket"
		cfg.OSS.DownloadURLTTL = 300
	}
	handler, _ := newContentHandlerForTest(db, cfg, nil)
	router := gin.New()
	router.GET("/contents/:id/download", func(c *gin.Context) {
		if userID > 0 {
			c.Set(middleware.UserIDKey, userID)
		}
		handler.DownloadContent(c)
	})
	return router, db
}

type downloadRouteUserState struct {
	Verified   bool
	Reputation int
}

func setupProtectedDownloadRoute(t *testing.T, state downloadRouteUserState) (*gin.Engine, int64, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentAttachment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{}
	cfg.JWT.Secret = "download-test-secret"
	cfg.Reputation.MinScoreForInteraction = 3
	cfg.OSS.Endpoint = "https://oss-cn-hangzhou.aliyuncs.com"
	cfg.OSS.AccessKeyID = "test-access-key-id"
	cfg.OSS.AccessKeySecret = "test-access-key-secret"
	cfg.OSS.BucketName = "test-bucket"
	cfg.OSS.DownloadURLTTL = 300

	handler, _ := newContentHandlerForTest(db, cfg, nil)
	authReq := middleware.AuthRequired(cfg, nil, db)
	downloadsGuard := middleware.InteractionRequired(cfg, db, nil, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})

	router := gin.New()
	router.GET("/api/v1/contents/:id/download", authReq, downloadsGuard, handler.DownloadContent)

	author := createDownloadUser(t, db, "route-author@example.com", "route-author", false)
	content := createDownloadContent(t, db, author.ID, nil)

	now := time.Now()
	viewer := model.User{
		Email:        "route-viewer@example.com",
		Username:     "route-viewer",
		PasswordHash: "hash",
		Reputation:   state.Reputation,
		Role:         "user",
	}
	if state.Verified {
		viewer.EmailVerifiedAt = &now
	}
	if err := db.Create(&viewer).Error; err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	pair, err := jwtutil.GenerateTokenPair(viewer.ID, viewer.Role, cfg.JWT.Secret, 120, 7)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	return router, content.ID, pair.AccessToken
}

func createDownloadUser(t *testing.T, db *gorm.DB, email, username string, banned bool) model.User {
	t.Helper()
	user := model.User{
		Email:        email,
		Username:     username,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
		IsBanned:     banned,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func createDownloadContent(t *testing.T, db *gorm.DB, authorID int64, ipID *int64) model.ContentItem {
	t.Helper()
	content := model.ContentItem{
		Title:       "downloadable",
		AuthorID:    authorID,
		IPID:        ipID,
		Zone:        "original",
		Category:    "game",
		ContentType: "sheet_music",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	attachment := model.ContentAttachment{
		ContentItemID: content.ID,
		FileType:      "sheet_music_pdf",
		OSSKey:        "uploads/1/sheet.pdf",
		MimeType:      "application/pdf",
		IsPrimary:     boolPtr(true),
	}
	if err := db.Create(&attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	return content
}

func requestDownload(t *testing.T, router *gin.Engine, contentID int64) *httptest.ResponseRecorder {
	t.Helper()
	return requestDownloadPath(t, router, contentID, "")
}

func requestDownloadPath(t *testing.T, router *gin.Engine, contentID int64, query string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/contents/"+strconvFormatInt(contentID)+"/download"+query, nil)
	router.ServeHTTP(rec, req)
	return rec
}

func assertContentUnavailable(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "CONTENT_UNAVAILABLE") {
		t.Fatalf("expected CONTENT_UNAVAILABLE response, got %s", rec.Body.String())
	}
}

// assertDownloadEnvelope pins the full download error contract: status plus
// the complete {"code","message"} envelope (#794 迁表守门的行为断言形态).
func assertDownloadEnvelope(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode, wantMessage string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}
	if body.Code != wantCode || body.Message != wantMessage {
		t.Fatalf("envelope = {code:%q message:%q}, want {code:%q message:%q}", body.Code, body.Message, wantCode, wantMessage)
	}
}

func strconvFormatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}

func boolPtr(v bool) *bool {
	return &v
}
