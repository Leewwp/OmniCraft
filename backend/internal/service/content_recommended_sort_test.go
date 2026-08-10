package service

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// GAP #17 后端只读扩展契约：GET /api/v1/contents?sort=recommended 在未传 zone
// （/recommend 推荐页）与 zone=original（/original 推荐排序）时都必须走
// handleRecommended 推荐管线（个性化引擎或 hot 兜底），不得落到仓库层
// created_at 默认排序。

func TestListContentsRecommendedSortRoutesToRecommendedPipeline(t *testing.T) {
	db := setupRecommendedSortTestDB(t)
	repo := repository.NewContentRepository(db)
	svc := NewContentService(repo)

	// A 最新创建但热度最低；B 最旧但热度中等；C 为二创、热度最高。
	now := time.Now()
	a := seedRecommendedSortContent(t, db, "A-original-hot1", "original", 1, now.Add(2*time.Minute))
	b := seedRecommendedSortContent(t, db, "B-original-hot50", "original", 50, now)
	c := seedRecommendedSortContent(t, db, "C-fanwork-hot100", "fanwork", 100, now.Add(1*time.Minute))

	t.Run("cross-zone no zone filter routes to recommended pipeline", func(t *testing.T) {
		got, total, err := svc.ListContents(repository.ListContentsFilter{
			Sort:     "recommended",
			Zone:     "",
			Page:     1,
			PageSize: 20,
		}, 0)
		if err != nil {
			t.Fatalf("ListContents() error = %v", err)
		}
		if total != 3 {
			t.Fatalf("total = %d, want 3", total)
		}
		want := []int64{c.ID, b.ID, a.ID} // hot 降序：二创 C 参与混排
		gotIDs := make([]int64, len(got))
		for i, item := range got {
			gotIDs[i] = item.ID
		}
		assertInt64SliceEqual(t, gotIDs, want)
	})

	t.Run("zone original keeps existing recommended sort contract", func(t *testing.T) {
		got, total, err := svc.ListContents(repository.ListContentsFilter{
			Sort:     "recommended",
			Zone:     "original",
			Page:     1,
			PageSize: 20,
		}, 0)
		if err != nil {
			t.Fatalf("ListContents() error = %v", err)
		}
		if total != 2 {
			t.Fatalf("total = %d, want 2", total)
		}
		want := []int64{b.ID, a.ID}
		gotIDs := make([]int64, len(got))
		for i, item := range got {
			gotIDs[i] = item.ID
		}
		assertInt64SliceEqual(t, gotIDs, want)
	})
}

// #81 服务集成缝：推荐管线无视 category 等筛选条件，携带筛选的
// sort=recommended 请求必须在 handleRecommended 入口降级为 hot（按分类收敛）
// 并输出结构化日志；无筛选的 recommended 不得降级、不得打降级日志。
const recommendedDegradeLogMsg = "content list recommended sort degraded to hot"

func TestListContentsRecommendedWithFiltersDegradesToHotAndLogs(t *testing.T) {
	db := setupRecommendedSortTestDB(t)
	repo := repository.NewContentRepository(db)
	svc := NewContentService(repo)
	svc.SetRecommendationService(NewRecommendationService(db, repository.NewEmbeddingRepository(db), repo, svc, nil, &config.RecommendationConfig{}))

	now := time.Now()
	a := seedRecommendedSortContentCategory(t, db, "A-film-hot50", "original", "film_tv", 50, now)
	b := seedRecommendedSortContentCategory(t, db, "B-film-hot10", "original", "film_tv", 10, now.Add(1*time.Minute))
	seedRecommendedSortContentCategory(t, db, "C-game-hot100", "original", "gaming", 100, now.Add(2*time.Minute))

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	got, total, err := svc.ListContents(repository.ListContentsFilter{
		Zone:     "original",
		Sort:     "recommended",
		Category: "film_tv",
		Page:     1,
		PageSize: 20,
	}, 0)
	if err != nil {
		t.Fatalf("ListContents() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2 (recommended pipeline leaked cross-category content)", total)
	}
	want := []int64{a.ID, b.ID} // hot 降序，仅 film_tv
	gotIDs := make([]int64, len(got))
	for i, item := range got {
		gotIDs[i] = item.ID
	}
	assertInt64SliceEqual(t, gotIDs, want)

	var entry map[string]any
	if decodeErr := json.Unmarshal(logs.Bytes(), &entry); decodeErr != nil {
		t.Fatalf("decode degradation log: %v; output=%q", decodeErr, logs.String())
	}
	if entry["msg"] != recommendedDegradeLogMsg {
		t.Fatalf("log msg = %#v, want %q", entry["msg"], recommendedDegradeLogMsg)
	}
	if entry["category"] != "film_tv" {
		t.Fatalf("log category = %#v, want film_tv", entry["category"])
	}
	if entry["zone"] != "original" {
		t.Fatalf("log zone = %#v, want original", entry["zone"])
	}
}

func TestListContentsRecommendedWithoutFiltersKeepsRecommendedAndSilent(t *testing.T) {
	db := setupRecommendedSortTestDB(t)
	repo := repository.NewContentRepository(db)
	svc := NewContentService(repo)
	svc.SetRecommendationService(NewRecommendationService(db, repository.NewEmbeddingRepository(db), repo, svc, nil, &config.RecommendationConfig{}))

	now := time.Now()
	seedRecommendedSortContentCategory(t, db, "A-original-hot1", "original", "film_tv", 1, now.Add(2*time.Minute))
	seedRecommendedSortContentCategory(t, db, "B-original-hot50", "original", "game", 50, now)

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	_, total, err := svc.ListContents(repository.ListContentsFilter{
		Zone:     "original",
		Sort:     "recommended",
		Page:     1,
		PageSize: 20,
	}, 0)
	if err != nil {
		t.Fatalf("ListContents() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2 (recommended contract changed without filters)", total)
	}
	if logs.Len() != 0 {
		t.Fatalf("unexpected degradation log without filters: %s", logs.String())
	}
}

func TestListContentsRecommendedDegradesForAnyFilterCondition(t *testing.T) {
	filterVariants := []struct {
		name   string
		mutate func(f *repository.ListContentsFilter)
	}{
		{"content_type", func(f *repository.ListContentsFilter) { f.ContentType = "article" }},
		{"content_types", func(f *repository.ListContentsFilter) { f.ContentTypes = []string{"article", "image"} }},
		{"tags", func(f *repository.ListContentsFilter) { f.Tags = []string{"vibe"} }},
		{"author_id", func(f *repository.ListContentsFilter) { id := int64(42); f.AuthorID = &id }},
		{"ip_id", func(f *repository.ListContentsFilter) { id := int64(7); f.IPID = &id }},
		{"source_original_id", func(f *repository.ListContentsFilter) { id := int64(3); f.SourceOriginalID = &id }},
		{"time_range", func(f *repository.ListContentsFilter) { f.TimeRange = "week" }},
	}
	for _, tc := range filterVariants {
		t.Run(tc.name, func(t *testing.T) {
			db := setupRecommendedSortTestDB(t)
			repo := repository.NewContentRepository(db)
			svc := NewContentService(repo)

			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previousLogger) })

			filter := repository.ListContentsFilter{
				Zone:     "original",
				Sort:     "recommended",
				Page:     1,
				PageSize: 20,
			}
			tc.mutate(&filter)

			if _, _, err := svc.ListContents(filter, 0); err != nil {
				t.Fatalf("ListContents() error = %v", err)
			}

			var entry map[string]any
			if decodeErr := json.Unmarshal(logs.Bytes(), &entry); decodeErr != nil {
				t.Fatalf("decode degradation log: %v; output=%q", decodeErr, logs.String())
			}
			if entry["msg"] != recommendedDegradeLogMsg {
				t.Fatalf("log msg = %#v, want %q; output=%q", entry["msg"], recommendedDegradeLogMsg, logs.String())
			}
		})
	}
}

func setupRecommendedSortTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.ContentTag{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	// 生产 schema 有 hot_score（迁移维护），测试内存库手动补齐该列。
	if err := db.Exec("ALTER TABLE content_items ADD COLUMN hot_score REAL DEFAULT 0").Error; err != nil {
		t.Fatalf("add hot_score column: %v", err)
	}
	return db
}

func seedRecommendedSortContent(t *testing.T, db *gorm.DB, title, zone string, hotScore float64, createdAt time.Time) model.ContentItem {
	t.Helper()
	return seedRecommendedSortContentCategory(t, db, title, zone, "game", hotScore, createdAt)
}

func seedRecommendedSortContentCategory(t *testing.T, db *gorm.DB, title, zone, category string, hotScore float64, createdAt time.Time) model.ContentItem {
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

func assertInt64SliceEqual(t *testing.T, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %v, want %v", got, want)
		}
	}
}
