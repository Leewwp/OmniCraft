package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestBrowseHistoryGetReturnsCompatibleItemsAndRetentionDays(t *testing.T) {
	router, db := setupBrowseHistoryHandlerRouter(t)
	author := seedBrowseHistoryHandlerUser(t, db, 10, "history-author")
	viewer := seedBrowseHistoryHandlerUser(t, db, 20, "history-viewer")
	content := seedBrowseHistoryHandlerContent(t, db, 101, author.ID, "Article", "article", "published", nil)
	seedBrowseHistoryHandlerRow(t, db, 1, viewer.ID, content.ID, time.Now().Add(-time.Hour))

	rec := requestBrowseHistory(t, router, viewer.ID, http.MethodGet, "/api/v1/users/me/history", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Items []struct {
			ID          int64           `json:"id"`
			Content     json.RawMessage `json:"content"`
			ContentItem json.RawMessage `json:"content_item"`
			ViewedAt    string          `json:"viewed_at"`
		} `json:"items"`
		History       []json.RawMessage `json:"history"`
		Total         int64             `json:"total"`
		Page          int               `json:"page"`
		PageSize      int               `json:"page_size"`
		RetentionDays int               `json:"retention_days"`
	}
	decodeJSON(t, rec, &payload)
	if payload.Total != 1 || payload.Page != 1 || payload.PageSize != 20 || payload.RetentionDays != 7 {
		t.Fatalf("metadata = total:%d page:%d page_size:%d retention:%d, want 1/1/20/7", payload.Total, payload.Page, payload.PageSize, payload.RetentionDays)
	}
	if len(payload.Items) != 1 || len(payload.History) != 1 {
		t.Fatalf("items/history lengths = %d/%d, want 1/1", len(payload.Items), len(payload.History))
	}
	if string(payload.Items[0].Content) == "null" || string(payload.Items[0].ContentItem) == "null" {
		t.Fatalf("content aliases should be populated for published item: %#v", payload.Items[0])
	}
	if string(payload.Items[0].Content) != string(payload.Items[0].ContentItem) {
		t.Fatalf("content aliases differ: content=%s content_item=%s", payload.Items[0].Content, payload.Items[0].ContentItem)
	}
	if !strings.Contains(string(payload.History[0]), `"content_item"`) {
		t.Fatalf("legacy history item lacks content_item alias: %s", payload.History[0])
	}
}

func TestBrowseHistoryGetReturnsNullContentForUnavailableContent(t *testing.T) {
	router, db := setupBrowseHistoryHandlerRouter(t)
	author := seedBrowseHistoryHandlerUser(t, db, 10, "history-author-unavailable")
	viewer := seedBrowseHistoryHandlerUser(t, db, 20, "history-viewer-unavailable")
	content := seedBrowseHistoryHandlerContent(t, db, 101, author.ID, "Unavailable", "article", "under_review", nil)
	seedBrowseHistoryHandlerRow(t, db, 1, viewer.ID, content.ID, time.Now().Add(-time.Hour))

	rec := requestBrowseHistory(t, router, viewer.ID, http.MethodGet, "/api/v1/users/me/history", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Items []struct {
			Content     *json.RawMessage `json:"content"`
			ContentItem *json.RawMessage `json:"content_item"`
		} `json:"items"`
	}
	decodeJSON(t, rec, &payload)
	if len(payload.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(payload.Items))
	}
	if payload.Items[0].Content != nil || payload.Items[0].ContentItem != nil {
		t.Fatalf("unavailable content aliases = %#v, want nil", payload.Items[0])
	}
}

func TestBrowseHistoryGetPaginationAndValidation(t *testing.T) {
	router, db := setupBrowseHistoryHandlerRouter(t)
	author := seedBrowseHistoryHandlerUser(t, db, 10, "history-author-pagination")
	viewer := seedBrowseHistoryHandlerUser(t, db, 20, "history-viewer-pagination")
	for i := int64(1); i <= 3; i++ {
		content := seedBrowseHistoryHandlerContent(t, db, 100+i, author.ID, "Article", "article", "published", nil)
		seedBrowseHistoryHandlerRow(t, db, i, viewer.ID, content.ID, time.Now().Add(-time.Duration(i)*time.Hour))
	}

	rec := requestBrowseHistory(t, router, viewer.ID, http.MethodGet, "/api/v1/users/me/history?limit=1&page_size=2&page=1&content_type=article", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var okPayload struct {
		Items    []json.RawMessage `json:"items"`
		Total    int64             `json:"total"`
		PageSize int               `json:"page_size"`
	}
	decodeJSON(t, rec, &okPayload)
	if okPayload.Total != 3 || okPayload.PageSize != 2 || len(okPayload.Items) != 2 {
		t.Fatalf("payload = %#v, want total 3 page_size 2 items 2", okPayload)
	}

	invalidType := requestBrowseHistory(t, router, viewer.ID, http.MethodGet, "/api/v1/users/me/history?content_type=movie", "")
	assertBrowseHistoryError(t, invalidType, http.StatusBadRequest, "INVALID_CONTENT_TYPE")

	invalidDate := requestBrowseHistory(t, router, viewer.ID, http.MethodGet, "/api/v1/users/me/history?start_date=2026-07-xx", "")
	assertBrowseHistoryError(t, invalidDate, http.StatusBadRequest, "INVALID_DATE")

	reversedDate := requestBrowseHistory(t, router, viewer.ID, http.MethodGet, "/api/v1/users/me/history?start_date=2026-07-03&end_date=2026-07-01", "")
	assertBrowseHistoryError(t, reversedDate, http.StatusBadRequest, "INVALID_DATE")
}

func TestBrowseHistoryDeleteSelectedAndExplicitClearAll(t *testing.T) {
	router, db := setupBrowseHistoryHandlerRouter(t)
	author := seedBrowseHistoryHandlerUser(t, db, 10, "history-author-delete")
	viewer := seedBrowseHistoryHandlerUser(t, db, 20, "history-viewer-delete")
	for i := int64(1); i <= 3; i++ {
		content := seedBrowseHistoryHandlerContent(t, db, 100+i, author.ID, "Article", "article", "published", nil)
		seedBrowseHistoryHandlerRow(t, db, i, viewer.ID, content.ID, time.Now().Add(-time.Duration(i)*time.Hour))
	}

	rec := requestBrowseHistory(t, router, viewer.ID, http.MethodDelete, "/api/v1/users/me/history", `{"ids":[1,3]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete selected status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	assertBrowseHistoryRowCount(t, db, viewer.ID, 1)

	clearAll := requestBrowseHistory(t, router, viewer.ID, http.MethodDelete, "/api/v1/users/me/history", `{"clear_all":true}`)
	if clearAll.Code != http.StatusOK {
		t.Fatalf("explicit clear-all status = %d, want 200; body = %s", clearAll.Code, clearAll.Body.String())
	}
	assertBrowseHistoryRowCount(t, db, viewer.ID, 0)
}

func TestBrowseHistoryDeleteRequiresExplicitMode(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing body", body: ""},
		{name: "empty object", body: `{}`},
		{name: "empty ids", body: `{"ids":[]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, db := setupBrowseHistoryHandlerRouter(t)
			author := seedBrowseHistoryHandlerUser(t, db, 10, "history-author-explicit-"+strings.ReplaceAll(tt.name, " ", "-"))
			viewer := seedBrowseHistoryHandlerUser(t, db, 20, "history-viewer-explicit-"+strings.ReplaceAll(tt.name, " ", "-"))
			content := seedBrowseHistoryHandlerContent(t, db, 200, author.ID, "Protected", "article", "published", nil)
			seedBrowseHistoryHandlerRow(t, db, 10, viewer.ID, content.ID, time.Now().Add(-time.Hour))

			rec := requestBrowseHistory(t, router, viewer.ID, http.MethodDelete, "/api/v1/users/me/history", tt.body)

			assertBrowseHistoryError(t, rec, http.StatusBadRequest, "CLEAR_CONFIRMATION_REQUIRED")
			assertBrowseHistoryRowCount(t, db, viewer.ID, 1)
		})
	}
}

func TestBrowseHistoryDeleteTreatsStrippedUnknownLengthBodyAsMissing(t *testing.T) {
	router, db := setupBrowseHistoryHandlerRouter(t)
	author := seedBrowseHistoryHandlerUser(t, db, 10, "history-author-stripped")
	viewer := seedBrowseHistoryHandlerUser(t, db, 20, "history-viewer-stripped")
	content := seedBrowseHistoryHandlerContent(t, db, 200, author.ID, "Protected", "article", "published", nil)
	seedBrowseHistoryHandlerRow(t, db, 10, viewer.ID, content.ID, time.Now().Add(-time.Hour))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/me/history", strings.NewReader(""))
	req.ContentLength = -1
	req.TransferEncoding = []string{"chunked"}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", strconv.FormatInt(viewer.ID, 10))
	router.ServeHTTP(rec, req)

	assertBrowseHistoryError(t, rec, http.StatusBadRequest, "CLEAR_CONFIRMATION_REQUIRED")
	assertBrowseHistoryRowCount(t, db, viewer.ID, 1)
}

func TestBrowseHistoryDeleteRejectsConflictingModes(t *testing.T) {
	router, db := setupBrowseHistoryHandlerRouter(t)
	author := seedBrowseHistoryHandlerUser(t, db, 10, "history-author-conflict")
	viewer := seedBrowseHistoryHandlerUser(t, db, 20, "history-viewer-conflict")
	content := seedBrowseHistoryHandlerContent(t, db, 200, author.ID, "Protected", "article", "published", nil)
	seedBrowseHistoryHandlerRow(t, db, 10, viewer.ID, content.ID, time.Now().Add(-time.Hour))

	rec := requestBrowseHistory(t, router, viewer.ID, http.MethodDelete, "/api/v1/users/me/history", `{"ids":[10],"clear_all":true}`)

	assertBrowseHistoryError(t, rec, http.StatusBadRequest, "DELETE_MODE_CONFLICT")
	assertBrowseHistoryRowCount(t, db, viewer.ID, 1)
}

func TestBrowseHistoryDeleteRejectsTooManyIDs(t *testing.T) {
	router, _ := setupBrowseHistoryHandlerRouter(t)
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = strconv.Itoa(i + 1)
	}
	body := `{"ids":[` + strings.Join(ids, ",") + `]}`

	rec := requestBrowseHistory(t, router, 20, http.MethodDelete, "/api/v1/users/me/history", body)

	assertBrowseHistoryError(t, rec, http.StatusBadRequest, "TOO_MANY_IDS")
}

func setupBrowseHistoryHandlerRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.BrowseHistory{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{}
	cfg.BrowseHistory.RetentionDays = 7
	handler := NewBrowseHistoryHandler(db, cfg)
	router := gin.New()
	router.GET("/api/v1/users/me/history", func(c *gin.Context) {
		setBrowseHistoryTestUserID(t, c)
		handler.GetHistory(c)
	})
	router.DELETE("/api/v1/users/me/history", func(c *gin.Context) {
		setBrowseHistoryTestUserID(t, c)
		handler.ClearHistory(c)
	})
	return router, db
}

func requestBrowseHistory(t *testing.T, router *gin.Engine, userID int64, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-Test-User-ID", strconv.FormatInt(userID, 10))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(rec, req)
	return rec
}

func setBrowseHistoryTestUserID(t *testing.T, c *gin.Context) {
	t.Helper()
	raw := c.GetHeader("X-Test-User-ID")
	if raw == "" {
		return
	}
	userID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		t.Fatalf("parse user id: %v", err)
	}
	c.Set(middleware.UserIDKey, userID)
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("decode JSON: %v; body=%s", err, rec.Body.String())
	}
}

func assertBrowseHistoryError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, status, rec.Body.String())
	}
	var payload struct {
		Code string `json:"code"`
	}
	decodeJSON(t, rec, &payload)
	if payload.Code != code {
		t.Fatalf("code = %q, want %q; body = %s", payload.Code, code, rec.Body.String())
	}
}

func assertBrowseHistoryRowCount(t *testing.T, db *gorm.DB, userID int64, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.BrowseHistory{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("count history: %v", err)
	}
	if count != want {
		t.Fatalf("history count = %d, want %d", count, want)
	}
}

func seedBrowseHistoryHandlerUser(t *testing.T, db *gorm.DB, id int64, username string) model.User {
	t.Helper()
	user := model.User{
		ID:           id,
		Email:        username + "@example.test",
		Username:     username,
		PasswordHash: "hash",
		Role:         "user",
		Reputation:   10,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func seedBrowseHistoryHandlerContent(t *testing.T, db *gorm.DB, id, authorID int64, title, contentType, status string, deletedAt *time.Time) model.ContentItem {
	t.Helper()
	content := model.ContentItem{
		ID:          id,
		Title:       title,
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: contentType,
		Status:      status,
		IsPublic:    true,
		AllowCopy:   true,
		DeletedAt:   deletedAt,
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content %s: %v", title, err)
	}
	return content
}

func seedBrowseHistoryHandlerRow(t *testing.T, db *gorm.DB, id, userID, contentID int64, viewedAt time.Time) {
	t.Helper()
	row := model.BrowseHistory{ID: id, UserID: userID, ContentItemID: contentID, ViewedAt: viewedAt}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create history row %d: %v", id, err)
	}
}
