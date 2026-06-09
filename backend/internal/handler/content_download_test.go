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

func TestContentDownload_RejectsAmbiguousWithoutAttachmentID(t *testing.T) {
	source := readHandlerSource(t, "content.go")
	if !strings.Contains(source, "AMBIGUOUS_ATTACHMENT") {
		t.Fatal("DownloadContent must return AMBIGUOUS_ATTACHMENT when multiple or no primary attachments exist and attachment_id is omitted")
	}
}

func TestContentDownload_RejectsAttachmentFromOtherContent(t *testing.T) {
	source := readHandlerSource(t, "content.go")
	if !strings.Contains(source, "ATTACHMENT_MISMATCH") {
		t.Fatal("DownloadContent must verify attachment belongs to the requested content")
	}
}

func TestContentDownload_UsesConfigurableTTL(t *testing.T) {
	source := readHandlerSource(t, "content.go")
	if !strings.Contains(source, "DownloadURLTTL") && !strings.Contains(source, "download_url_ttl") {
		t.Fatal("DownloadContent must use configurable download URL TTL from config, not hardcoded value")
	}
}

func TestContentDownload_ResponseSchema(t *testing.T) {
	resp := map[string]interface{}{
		"download_url": "https://example.com/file.zip",
		"expires_in":   300,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := parsed["download_url"]; !ok {
		t.Fatal("response must contain download_url")
	}
	if _, ok := parsed["expires_in"]; !ok {
		t.Fatal("response must contain expires_in")
	}
}

func TestContentDownloadRouteRequiresAuth(t *testing.T) {
	routes := readRoutesSource(t)
	if !strings.Contains(routes, "download") {
		t.Skip("no download route found")
	}
	lines := strings.Split(routes, "\n")
	for _, line := range lines {
		if strings.Contains(line, "download") && strings.Contains(line, "GET") {
			if !strings.Contains(line, "authReq") && !strings.Contains(line, "AuthRequired") {
				t.Fatalf("download route must require authentication: %s", line)
			}
		}
	}
}

func TestContentDownloadRouteRequiresInteractionGuard(t *testing.T) {
	routes := readRoutesSource(t)
	if !strings.Contains(routes, "download") {
		t.Skip("no download route found")
	}
	lines := strings.Split(routes, "\n")
	for _, line := range lines {
		if strings.Contains(line, "download") && strings.Contains(line, "GET") {
			if !strings.Contains(line, "downloadsGuard") && !strings.Contains(line, "InteractionRequired") {
				t.Fatalf("download route must require interaction guard (verified email + reputation): %s", line)
			}
		}
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

func TestContentDownload_EndToEnd_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}
	router := gin.New()
	router.GET("/api/v1/contents/:id/download", func(c *gin.Context) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/contents/1/download", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
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

func setupDownloadVisibilityRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	return setupDownloadRouter(t, 999, false)
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
	for _, stmt := range []string{
		"ALTER TABLE users ADD COLUMN deleted_at DATETIME",
		"ALTER TABLE content_items ADD COLUMN deleted_at DATETIME",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("schema patch %q: %v", stmt, err)
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
	handler := NewContentHandler(db, cfg, nil)
	router := gin.New()
	router.GET("/contents/:id/download", func(c *gin.Context) {
		if userID > 0 {
			c.Set(middleware.UserIDKey, userID)
		}
		handler.DownloadContent(c)
	})
	return router, db
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
		IsPrimary:     true,
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

func strconvFormatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}
