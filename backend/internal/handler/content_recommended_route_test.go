package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
)

// GAP #17 route contract：GET /api/v1/contents?sort=recommended（不带 zone，
// 推荐页数据源）必须返回 200 且走推荐管线：production 接线下 recSvc 恒非空，
// 推荐引擎（或 hot 兜底）仅返回已发布原创；不得落到仓库层跨区 created_at 排序。
func TestListContentsRecommendedSortRouteWithoutZone(t *testing.T) {
	router, db := setupRecommendedSortRoute(t)

	now := time.Now()
	a := seedRecommendedRouteContent(t, db, "route-a-hot1", "original", 1, now.Add(2*time.Minute))
	b := seedRecommendedRouteContent(t, db, "route-b-hot50", "original", 50, now)
	seedRecommendedRouteContent(t, db, "route-c-fanwork-hot100", "fanwork", 100, now.Add(1*time.Minute))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contents?sort=recommended&page=1&page_size=20", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Contents []struct {
			ID   int64  `json:"id"`
			Zone string `json:"zone"`
		} `json:"contents"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}

	if body.Total != 2 {
		t.Fatalf("total = %d, want 2 (originals only); body = %s", body.Total, rec.Body.String())
	}
	gotIDs := make([]int64, 0, len(body.Contents))
	for _, item := range body.Contents {
		if item.Zone != "original" {
			t.Fatalf("zone = %q, want original only (fanwork leaked into recommended feed)", item.Zone)
		}
		gotIDs = append(gotIDs, item.ID)
	}
	want := []int64{b.ID, a.ID} // hot 降序
	if len(gotIDs) != len(want) {
		t.Fatalf("ids = %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("ids = %v, want %v", gotIDs, want)
		}
	}
}

// #81 防御契约：sort=recommended 携带 category 等筛选条件时，服务端必须降级为
// hot 排序并按该分类收敛内容；不得再走推荐管线（推荐管线无视筛选条件）。
func TestListContentsRecommendedWithCategoryDegradesToHot(t *testing.T) {
	router, db := setupRecommendedSortRoute(t)

	now := time.Now()
	a := seedRecommendedRouteContentCategory(t, db, "cat-film-hot50", "original", "film_tv", 50, now)
	b := seedRecommendedRouteContentCategory(t, db, "cat-film-hot10", "original", "film_tv", 10, now.Add(1*time.Minute))
	c := seedRecommendedRouteContentCategory(t, db, "cat-game-hot100", "original", "gaming", 100, now.Add(2*time.Minute))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contents?zone=original&sort=recommended&category=film_tv&page=1&page_size=20", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Contents []struct {
			ID       int64  `json:"id"`
			Category string `json:"category"`
		} `json:"contents"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}

	if body.Total != 2 {
		t.Fatalf("total = %d, want 2 (film_tv only, gaming leaked in); body = %s", body.Total, rec.Body.String())
	}
	gotIDs := make([]int64, 0, len(body.Contents))
	for _, item := range body.Contents {
		if item.Category != "film_tv" {
			t.Fatalf("category = %q, want film_tv only; body = %s", item.Category, rec.Body.String())
		}
		gotIDs = append(gotIDs, item.ID)
	}
	want := []int64{a.ID, b.ID} // hot 降序，且不含更高热的 gaming 内容
	if len(gotIDs) != len(want) {
		t.Fatalf("ids = %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("ids = %v, want %v", gotIDs, want)
		}
	}
	if c.ID == 0 {
		t.Fatal("seed failed: gaming content not created")
	}
}

func setupRecommendedSortRoute(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("ALTER TABLE content_items ADD COLUMN hot_score REAL DEFAULT 0").Error; err != nil {
		t.Fatalf("add hot_score column: %v", err)
	}

	cfg := &config.Config{}
	h := NewContentHandler(db, cfg, nil)

	router := gin.New()
	router.GET("/api/v1/contents", h.ListContents)

	return router, db
}

func seedRecommendedRouteContent(t *testing.T, db *gorm.DB, title, zone string, hotScore float64, createdAt time.Time) model.ContentItem {
	t.Helper()
	return seedRecommendedRouteContentCategory(t, db, title, zone, "game", hotScore, createdAt)
}

func seedRecommendedRouteContentCategory(t *testing.T, db *gorm.DB, title, zone, category string, hotScore float64, createdAt time.Time) model.ContentItem {
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
		Category:    category,
		ContentType: "article",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
		CreatedAt:   createdAt,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	if err := db.Model(&model.ContentItem{}).Where("id = ?", item.ID).Update("hot_score", hotScore).Error; err != nil {
		t.Fatalf("set hot_score: %v", err)
	}
	return item
}
