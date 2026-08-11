package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
)

// Source-linkage contract: GET /api/v1/contents/:id/related-fanworks must
// return children by source column per source zone (original ->
// source_original_id, fanwork -> source_fanwork_id), exclude non-visible
// children through the centralized content visibility scope, allowlist
// content_type multi-values, default to hot ordering, and support the
// page_size/limit alias pair.

func TestListRelatedFanworksOriginalReturnsChildrenBySourceOriginalID(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	srcA := seedRelatedSource(t, db, "orig-a", "original")
	srcB := seedRelatedSource(t, db, "orig-b", "original")
	seedRelatedFanwork(t, db, "fw-a1", "article", srcA.ID, 0, 10, true, time.Now().Add(-3*time.Minute), "published")
	seedRelatedFanwork(t, db, "fw-a2", "article", srcA.ID, 0, 5, true, time.Now().Add(-2*time.Minute), "published")
	seedRelatedFanwork(t, db, "fw-b1", "article", srcB.ID, 0, 50, true, time.Now().Add(-1*time.Minute), "published")

	rec := getRelatedFanworks(t, router, srcA.ID, "sort=new")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var body relatedFanworksBody
	decodeRelatedFanworksBody(t, rec, &body)
	if body.Total != 2 {
		t.Fatalf("total = %d, want 2; body = %s", body.Total, rec.Body.String())
	}
	if body.SourceContentID != srcA.ID {
		t.Fatalf("source_content_id = %d, want %d", body.SourceContentID, srcA.ID)
	}
	if body.SourceZone != "original" {
		t.Fatalf("source_zone = %q, want original", body.SourceZone)
	}
	for _, item := range body.Contents {
		if item.Zone != "fanwork" {
			t.Fatalf("child zone = %q, want fanwork; body = %s", item.Zone, rec.Body.String())
		}
		if item.Title != "fw-a1" && item.Title != "fw-a2" {
			t.Fatalf("child %q leaked from another source; body = %s", item.Title, rec.Body.String())
		}
	}
}

func TestListRelatedFanworksFanworkReturnsDerivativesBySourceFanworkID(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	srcOriginal := seedRelatedSource(t, db, "orig-c", "original")
	baseFanwork := seedRelatedFanwork(t, db, "base-fw", "article", srcOriginal.ID, 0, 20, true, time.Now().Add(-5*time.Minute), "published")
	seedRelatedFanwork(t, db, "deriv-1", "prompt", 0, baseFanwork.ID, 10, true, time.Now().Add(-4*time.Minute), "published")
	seedRelatedFanwork(t, db, "deriv-2", "article", 0, baseFanwork.ID, 30, true, time.Now().Add(-3*time.Minute), "published")
	seedRelatedFanwork(t, db, "other-orig-child", "article", srcOriginal.ID, 0, 99, true, time.Now().Add(-2*time.Minute), "published")
	// A row keyed by source_original_id pointing at a fanwork must not leak in.
	seedRelatedFanwork(t, db, "bad-src-orig-row", "article", baseFanwork.ID, 0, 99, true, time.Now().Add(-1*time.Minute), "published")

	rec := getRelatedFanworks(t, router, baseFanwork.ID, "sort=new")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fanwork sources serve derivatives, not 400 NOT_ORIGINAL); body = %s", rec.Code, rec.Body.String())
	}

	var body relatedFanworksBody
	decodeRelatedFanworksBody(t, rec, &body)
	if body.Total != 2 {
		t.Fatalf("total = %d, want 2 (derivatives by source_fanwork_id only); body = %s", body.Total, rec.Body.String())
	}
	if body.SourceContentID != baseFanwork.ID {
		t.Fatalf("source_content_id = %d, want %d", body.SourceContentID, baseFanwork.ID)
	}
	if body.SourceZone != "fanwork" {
		t.Fatalf("source_zone = %q, want fanwork", body.SourceZone)
	}
	for _, item := range body.Contents {
		if item.Title != "deriv-1" && item.Title != "deriv-2" {
			t.Fatalf("child %q leaked; body = %s", item.Title, rec.Body.String())
		}
	}
}

func TestListRelatedFanworksExcludesNonVisibleChildren(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	src := seedRelatedSource(t, db, "orig-d", "original")
	seedRelatedFanwork(t, db, "visible", "article", src.ID, 0, 10, true, time.Now(), "published")
	seedRelatedFanwork(t, db, "pending", "article", src.ID, 0, 10, true, time.Now(), "pending")
	seedRelatedFanwork(t, db, "under-review", "article", src.ID, 0, 10, true, time.Now(), "under_review")
	deleted := seedRelatedFanwork(t, db, "deleted", "article", src.ID, 0, 10, true, time.Now(), "published")
	softDeleteRelatedContent(t, db, deleted.ID)
	seedRelatedFanwork(t, db, "private", "article", src.ID, 0, 10, false, time.Now(), "published")
	bannedAuthor := seedBannedRelatedAuthor(t, db, "banned-author@example.com")
	seedRelatedFanworkAs(t, db, "banned-author-child", bannedAuthor.ID, "article", src.ID, 0, 10, true, time.Now(), "published")

	rec := getRelatedFanworks(t, router, src.ID, "sort=new")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var body relatedFanworksBody
	decodeRelatedFanworksBody(t, rec, &body)
	if body.Total != 1 {
		t.Fatalf("total = %d, want 1 (only the published public child); body = %s", body.Total, rec.Body.String())
	}
	if len(body.Contents) != 1 || body.Contents[0].Title != "visible" {
		t.Fatalf("contents = %+v, want only 'visible'; body = %s", body.Contents, rec.Body.String())
	}
}

func TestListRelatedFanworksNonVisibleSourceReturnsNotFound(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	src := seedRelatedSource(t, db, "orig-e", "original")
	softDeleteRelatedContent(t, db, src.ID)
	seedRelatedFanwork(t, db, "orphan-child", "article", src.ID, 0, 10, true, time.Now(), "published")

	rec := getRelatedFanworks(t, router, src.ID, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for soft-deleted source; body = %s", rec.Code, rec.Body.String())
	}
}

func TestListRelatedFanworksContentTypeFilter(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	src := seedRelatedSource(t, db, "orig-f", "original")
	seedRelatedFanwork(t, db, "art", "article", src.ID, 0, 10, true, time.Now().Add(-2*time.Minute), "published")
	seedRelatedFanwork(t, db, "prompt", "prompt", src.ID, 0, 10, true, time.Now().Add(-1*time.Minute), "published")
	seedRelatedFanwork(t, db, "audio", "audio", src.ID, 0, 10, true, time.Now(), "published")

	rec := getRelatedFanworks(t, router, src.ID, "content_type=article&sort=new")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var body relatedFanworksBody
	decodeRelatedFanworksBody(t, rec, &body)
	if body.Total != 1 || len(body.Contents) != 1 || body.Contents[0].Title != "art" {
		t.Fatalf("total = %d, contents = %+v; want only 'art'; body = %s", body.Total, body.Contents, rec.Body.String())
	}
}

func TestListRelatedFanworksContentTypeMultiValues(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	src := seedRelatedSource(t, db, "orig-g", "original")
	seedRelatedFanwork(t, db, "art", "article", src.ID, 0, 10, true, time.Now().Add(-3*time.Minute), "published")
	seedRelatedFanwork(t, db, "prompt", "prompt", src.ID, 0, 10, true, time.Now().Add(-2*time.Minute), "published")
	seedRelatedFanwork(t, db, "audio", "audio", src.ID, 0, 10, true, time.Now().Add(-1*time.Minute), "published")
	seedRelatedFanwork(t, db, "image", "image", src.ID, 0, 10, true, time.Now(), "published")

	rec := getRelatedFanworks(t, router, src.ID, "content_type=article,prompt&sort=new")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var body relatedFanworksBody
	decodeRelatedFanworksBody(t, rec, &body)
	if body.Total != 2 {
		t.Fatalf("total = %d, want 2 (allowlisted IN query); body = %s", body.Total, rec.Body.String())
	}
	titles := map[string]bool{}
	for _, item := range body.Contents {
		titles[item.Title] = true
	}
	if !titles["art"] || !titles["prompt"] {
		t.Fatalf("contents = %+v, want article+prompt rows; body = %s", body.Contents, rec.Body.String())
	}
}

func TestListRelatedFanworksRejectsInvalidContentType(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	src := seedRelatedSource(t, db, "orig-h", "original")

	cases := []string{
		"content_type=bogus",
		"content_type=article,",        // empty trailing entry
		"content_type=,",               // only empty entries
		"content_type=article,,prompt", // empty middle entry
		"content_type=text",            // not part of the standard allowlist
	}
	for _, query := range cases {
		rec := getRelatedFanworks(t, router, src.ID, query)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400; body = %s", query, rec.Code, rec.Body.String())
		}
		var errBody struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
			t.Fatalf("%s: decode error body: %v", query, err)
		}
		if errBody.Code != "INVALID_CONTENT_TYPE" {
			t.Fatalf("%s: code = %q, want INVALID_CONTENT_TYPE", query, errBody.Code)
		}
	}
}

func TestListRelatedFanworksSortHotIsDefault(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	src := seedRelatedSource(t, db, "orig-i", "original")
	seedRelatedFanwork(t, db, "low-hot", "article", src.ID, 0, 10, true, time.Now().Add(-1*time.Minute), "published")
	seedRelatedFanwork(t, db, "high-hot", "article", src.ID, 0, 100, true, time.Now().Add(-2*time.Minute), "published")
	seedRelatedFanwork(t, db, "mid-hot", "article", src.ID, 0, 50, true, time.Now().Add(-3*time.Minute), "published")

	rec := getRelatedFanworks(t, router, src.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var body relatedFanworksBody
	decodeRelatedFanworksBody(t, rec, &body)
	if body.Total != 3 {
		t.Fatalf("total = %d, want 3; body = %s", body.Total, rec.Body.String())
	}
	wantOrder := []string{"high-hot", "mid-hot", "low-hot"}
	gotTitles := make([]string, 0, len(body.Contents))
	for _, item := range body.Contents {
		gotTitles = append(gotTitles, item.Title)
	}
	for i := range wantOrder {
		if gotTitles[i] != wantOrder[i] {
			t.Fatalf("default sort = %v, want hot order %v", gotTitles, wantOrder)
		}
	}
}

func TestListRelatedFanworksSortNew(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	src := seedRelatedSource(t, db, "orig-j", "original")
	seedRelatedFanwork(t, db, "oldest", "article", src.ID, 0, 999, true, time.Now().Add(-3*time.Minute), "published")
	seedRelatedFanwork(t, db, "middle", "article", src.ID, 0, 100, true, time.Now().Add(-2*time.Minute), "published")
	seedRelatedFanwork(t, db, "newest", "article", src.ID, 0, 1, true, time.Now().Add(-1*time.Minute), "published")

	rec := getRelatedFanworks(t, router, src.ID, "sort=new")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var body relatedFanworksBody
	decodeRelatedFanworksBody(t, rec, &body)
	if body.Total != 3 {
		t.Fatalf("total = %d, want 3; body = %s", body.Total, rec.Body.String())
	}
	wantOrder := []string{"newest", "middle", "oldest"}
	gotTitles := make([]string, 0, len(body.Contents))
	for _, item := range body.Contents {
		gotTitles = append(gotTitles, item.Title)
	}
	for i := range wantOrder {
		if gotTitles[i] != wantOrder[i] {
			t.Fatalf("sort=new = %v, want %v", gotTitles, wantOrder)
		}
	}
}

func TestListRelatedFanworksLimitAliasesPageSize(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	src := seedRelatedSource(t, db, "orig-k", "original")
	seedRelatedFanwork(t, db, "one", "article", src.ID, 0, 10, true, time.Now().Add(-2*time.Minute), "published")
	seedRelatedFanwork(t, db, "two", "article", src.ID, 0, 10, true, time.Now().Add(-1*time.Minute), "published")

	rec := getRelatedFanworks(t, router, src.ID, "limit=1&sort=new")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var body relatedFanworksBody
	decodeRelatedFanworksBody(t, rec, &body)
	if body.PageSize != 1 {
		t.Fatalf("page_size = %d, want 1 (limit alias applied); body = %s", body.PageSize, rec.Body.String())
	}
	if len(body.Contents) != 1 {
		t.Fatalf("len(contents) = %d, want 1; body = %s", len(body.Contents), rec.Body.String())
	}
}

func TestListRelatedFanworksPageSizeWinsOverLimit(t *testing.T) {
	router, db := setupRelatedFanworksRoute(t)

	src := seedRelatedSource(t, db, "orig-l", "original")
	for i := 0; i < 3; i++ {
		seedRelatedFanwork(t, db, fmt.Sprintf("fw-%d", i), "article", src.ID, 0, 10, true, time.Now().Add(time.Duration(-i)*time.Minute), "published")
	}

	rec := getRelatedFanworks(t, router, src.ID, "limit=1&page_size=2&sort=new")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var body relatedFanworksBody
	decodeRelatedFanworksBody(t, rec, &body)
	if body.PageSize != 2 {
		t.Fatalf("page_size = %d, want 2 (page_size wins over limit); body = %s", body.PageSize, rec.Body.String())
	}
	if len(body.Contents) != 2 {
		t.Fatalf("len(contents) = %d, want 2; body = %s", len(body.Contents), rec.Body.String())
	}
}

// --- helpers ---

type relatedFanworksBody struct {
	SourceContentID int64  `json:"source_content_id"`
	SourceZone      string `json:"source_zone"`
	Contents        []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Zone  string `json:"zone"`
	} `json:"contents"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func decodeRelatedFanworksBody(t *testing.T, rec *httptest.ResponseRecorder, body *relatedFanworksBody) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), body); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}
}

func getRelatedFanworks(t *testing.T, router *gin.Engine, sourceID int64, query string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	url := "/api/v1/contents/" + strconv.FormatInt(sourceID, 10) + "/related-fanworks"
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	router.ServeHTTP(rec, req)
	return rec
}

func setupRelatedFanworksRoute(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("ALTER TABLE content_items ADD COLUMN hot_score REAL DEFAULT 0").Error; err != nil {
		t.Fatalf("add hot_score column: %v", err)
	}

	cfg := &config.Config{}
	h := NewContentHandler(db, cfg, nil)

	router := gin.New()
	router.GET("/api/v1/contents/:id/related-fanworks", h.ListRelatedFanworks)

	return router, db
}

func seedRelatedSource(t *testing.T, db *gorm.DB, title, zone string) model.ContentItem {
	t.Helper()
	author := model.User{
		Email:        title + "@example.com",
		Username:     title,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	item := model.ContentItem{
		Title:       title,
		AuthorID:    author.ID,
		Zone:        zone,
		Category:    "game",
		ContentType: "article",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}
	return item
}

func seedBannedRelatedAuthor(t *testing.T, db *gorm.DB, email string) model.User {
	t.Helper()
	author := model.User{
		Email:        email,
		Username:     "banned-author",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
		IsBanned:     true,
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create banned author: %v", err)
	}
	return author
}

func seedRelatedFanwork(t *testing.T, db *gorm.DB, title, contentType string, sourceOriginalID, sourceFanworkID int64, hotScore float64, isPublic bool, createdAt time.Time, status string) model.ContentItem {
	t.Helper()
	author := model.User{
		Email:        title + "@example.com",
		Username:     title,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	return seedRelatedFanworkAs(t, db, title, author.ID, contentType, sourceOriginalID, sourceFanworkID, hotScore, isPublic, createdAt, status)
}

func seedRelatedFanworkAs(t *testing.T, db *gorm.DB, title string, authorID int64, contentType string, sourceOriginalID, sourceFanworkID int64, hotScore float64, isPublic bool, createdAt time.Time, status string) model.ContentItem {
	t.Helper()
	var srcOrigID, srcFanworkID *int64
	if sourceOriginalID != 0 {
		srcOrigID = &sourceOriginalID
	}
	if sourceFanworkID != 0 {
		srcFanworkID = &sourceFanworkID
	}
	item := model.ContentItem{
		Title:            title,
		AuthorID:         authorID,
		Zone:             "fanwork",
		Category:         "game",
		ContentType:      contentType,
		Status:           status,
		IsPublic:         true,
		AllowCopy:        true,
		SourceOriginalID: srcOrigID,
		SourceFanworkID:  srcFanworkID,
		CreatedAt:        createdAt,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create fanwork: %v", err)
	}
	// IsPublic carries a default:true tag, so a zero-value false is omitted
	// from INSERT; force the exact value through an explicit UPDATE.
	if !isPublic {
		if err := db.Model(&model.ContentItem{}).Where("id = ?", item.ID).Update("is_public", false).Error; err != nil {
			t.Fatalf("set is_public: %v", err)
		}
	}
	if err := db.Model(&model.ContentItem{}).Where("id = ?", item.ID).Update("hot_score", hotScore).Error; err != nil {
		t.Fatalf("set hot_score: %v", err)
	}
	return item
}

func softDeleteRelatedContent(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	now := time.Now()
	if err := db.Model(&model.ContentItem{}).Where("id = ?", id).Update("deleted_at", &now).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}
}
