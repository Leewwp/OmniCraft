package repository

// #754 A：ListIPs 显式浏览口径在真实 SQL（sqlite 内存库）上验证——
// newest/most_contents 排序、most_contents 同分 id 稳定、approved 与
// category 过滤先于 LIMIT。不依赖 fake seam。

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

func openIPBrowseTestDB(t *testing.T) *IPRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.IP{}, &model.User{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewIPRepository(db)
}

func TestListIPsBrowseNewestOrdersByCreatedAtDesc(t *testing.T) {
	repo := openIPBrowseTestDB(t)
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	seed := []model.IP{
		{ID: 101, Name: "旧 IP", Slug: "old", Status: "approved", CreatedAt: base.Add(-72 * time.Hour)},
		{ID: 102, Name: "新 IP", Slug: "new", Status: "approved", CreatedAt: base},
		{ID: 103, Name: "中 IP", Slug: "mid", Status: "approved", CreatedAt: base.Add(-24 * time.Hour)},
	}
	for i := range seed {
		if err := repo.db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	ips, total, err := repo.ListIPs(ListIPsFilter{Status: "approved", Sort: "newest", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 || len(ips) != 3 {
		t.Fatalf("total=%d len=%d, want 3/3", total, len(ips))
	}
	want := []int64{102, 103, 101}
	for i, ip := range ips {
		if ip.ID != want[i] {
			t.Fatalf("newest order[%d] = %d, want %d", i, ip.ID, want[i])
		}
	}
}

func TestListIPsBrowseMostContentsDeterministicTieBreak(t *testing.T) {
	repo := openIPBrowseTestDB(t)
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	// 三个 IP：内容量 2 / 2 / 0。前两个同分（都 2 条已发布），id 大者在前。
	ips := []model.IP{
		{ID: 201, Name: "双内容甲", Slug: "two-a", Status: "approved", CreatedAt: base},
		{ID: 202, Name: "双内容乙", Slug: "two-b", Status: "approved", CreatedAt: base.Add(-time.Hour)},
		{ID: 203, Name: "零内容", Slug: "zero", Status: "approved", CreatedAt: base.Add(-2 * time.Hour)},
	}
	for i := range ips {
		if err := repo.db.Create(&ips[i]).Error; err != nil {
			t.Fatalf("seed ip: %v", err)
		}
	}
	contents := []model.ContentItem{
		{IPID: int64From(201), Title: "甲-1", Zone: "original", ContentType: "illustration", Status: "published", CreatedAt: base, UpdatedAt: base},
		{IPID: int64From(201), Title: "甲-2", Zone: "original", ContentType: "illustration", Status: "published", CreatedAt: base, UpdatedAt: base},
		{IPID: int64From(202), Title: "乙-1", Zone: "fanwork", ContentType: "illustration", Status: "published", CreatedAt: base, UpdatedAt: base},
		{IPID: int64From(202), Title: "乙-2", Zone: "fanwork", ContentType: "illustration", Status: "published", CreatedAt: base, UpdatedAt: base},
		{IPID: int64From(203), Title: "零-1", Zone: "original", ContentType: "illustration", Status: "published", CreatedAt: base, UpdatedAt: base},
	}
	for i := range contents {
		if err := repo.db.Create(&contents[i]).Error; err != nil {
			t.Fatalf("seed content: %v", err)
		}
	}
	for run := 0; run < 3; run++ {
		got, _, err := repo.ListIPs(ListIPsFilter{Status: "approved", Sort: "most_contents", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("list run %d: %v", run, err)
		}
		want := []int64{202, 201, 203}
		if len(got) != len(want) {
			t.Fatalf("run %d len=%d want %d", run, len(got), len(want))
		}
		for i := range want {
			if got[i].ID != want[i] {
				t.Fatalf("run %d most_contents order[%d] = %d, want %d (deterministic tie-break)", run, i, got[i].ID, want[i])
			}
		}
	}
}

// approved 与 category 过滤必须发生在 LIMIT 之前（LIMIT 不放大预算，也不把
// 非 approved 行挤进结果）。
func TestListIPsBrowseFiltersBeforeLimit(t *testing.T) {
	repo := openIPBrowseTestDB(t)
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	seed := []model.IP{
		{ID: 301, Name: "游戏新", Slug: "game-new", Category: "game", Status: "approved", CreatedAt: base},
		{ID: 302, Name: "游戏旧", Slug: "game-old", Category: "game", Status: "approved", CreatedAt: base.Add(-time.Hour)},
		{ID: 303, Name: "动漫", Slug: "anime", Category: "anime", Status: "approved", CreatedAt: base.Add(-2 * time.Hour)},
		{ID: 304, Name: "待审", Slug: "pending", Category: "game", Status: "pending", CreatedAt: base},
		{ID: 305, Name: "被封", Slug: "banned", Category: "game", Status: "banned", CreatedAt: base},
	}
	for i := range seed {
		if err := repo.db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	got, total, err := repo.ListIPs(ListIPsFilter{Status: "approved", Category: "game", Sort: "newest", Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Fatalf("category+approved total = %d, want 2 (pending/banned excluded before limit)", total)
	}
	if len(got) != 1 || got[0].ID != 301 {
		t.Fatalf("limit-after-filter order broken: %+v", got)
	}
}

func int64From(v int64) *int64 { return &v }
