package repository

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// #290 Testing Decision 3：热门讨论排序（ln(回复+1)/龄期衰减）在仓库层验证，
// 含置顶优先与 IP 内搜索过滤；回归既有三种排序的排序语义。
func TestListByIPHotSortPinnedFirstAndSearch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	// discussions 的 gorm 标签 default:NOW() 在 sqlite 不可用，沿用既有测试的 raw DDL
	//（见 social_service_test.go）。
	if err := db.Exec(`
		CREATE TABLE discussions (
			id integer PRIMARY KEY AUTOINCREMENT,
			ip_id integer,
			content_item_id integer,
			author_id integer NOT NULL,
			title text NOT NULL,
			body text,
			status text NOT NULL DEFAULT 'published',
			is_pinned numeric NOT NULL DEFAULT 0,
			view_count integer NOT NULL DEFAULT 0,
			reply_count integer NOT NULL DEFAULT 0,
			last_active_at datetime NOT NULL DEFAULT (datetime('now')),
			created_at datetime,
			updated_at datetime
		)`).Error; err != nil {
		t.Fatalf("create discussions table: %v", err)
	}

	author := &model.User{Email: "hot-sort@example.com", Username: "hotter", PasswordHash: "x", Role: "user"}
	if err := db.Create(author).Error; err != nil {
		t.Fatalf("seed author: %v", err)
	}
	ipID := int64(990)
	now := time.Now()
	seed := func(id int64, title string, pinned bool, replies int, ageHours float64) {
		d := &model.Discussion{
			IPID:       &ipID,
			AuthorID:   author.ID,
			Title:      title,
			Body:       title + " 正文",
			Status:     "published",
			IsPinned:   pinned,
			ReplyCount: replies,
			CreatedAt:  now.Add(-time.Duration(ageHours * float64(time.Hour))),
		}
		d.ID = id
		if err := db.Create(d).Error; err != nil {
			t.Fatalf("seed %s: %v", title, err)
		}
	}
	seed(1, "置顶老帖", true, 0, 24*30)     // 置顶：无论热度都第一
	seed(2, "新鲜热帖", false, 5, 1)        // ln6/(1+1/72) ≈ 1.77
	seed(3, "陈年回复王", false, 100, 24*30) // ln101/(1+720/72) ≈ 0.42
	seed(4, "新鲜零回复", false, 0, 1)       // 0
	// 其他 IP / 未发布行必须排除
	otherIP := int64(991)
	db.Create(&model.Discussion{IPID: &otherIP, AuthorID: author.ID, Title: "别家帖子", Status: "published"})
	db.Create(&model.Discussion{IPID: &ipID, AuthorID: author.ID, Title: "待审帖", Status: "pending"})

	repo := NewDiscussionRepository(db)
	const pageSize = 10

	hot, total, err := repo.ListByIP(ipID, "hot", "", 72, 1, pageSize)
	if err != nil {
		t.Fatalf("hot list: %v", err)
	}
	if total != 4 {
		t.Fatalf("total = %d, want 4 (other ip / pending excluded)", total)
	}
	gotOrder := make([]int64, 0, len(hot))
	for _, d := range hot {
		gotOrder = append(gotOrder, d.ID)
	}
	want := []int64{1, 2, 3, 4}
	for i := range want {
		if gotOrder[i] != want[i] {
			t.Fatalf("hot order = %v, want %v (pinned first, then fresh-with-replies beats old-with-many)", gotOrder, want)
		}
	}

	byReplies, _, err := repo.ListByIP(ipID, "most_replies", "", 72, 1, pageSize)
	if err != nil {
		t.Fatalf("most_replies list: %v", err)
	}
	if byReplies[0].ID != 1 || byReplies[1].ID != 3 {
		t.Fatalf("most_replies order = [%d %d ...], want pinned then reply king", byReplies[0].ID, byReplies[1].ID)
	}

	searched, searchTotal, err := repo.ListByIP(ipID, "hot", "回复王", 72, 1, pageSize)
	if err != nil {
		t.Fatalf("search list: %v", err)
	}
	if searchTotal != 1 || len(searched) != 1 || searched[0].ID != 3 {
		t.Fatalf("q=回复王 should hit only id=3, got total=%d rows=%d", searchTotal, len(searched))
	}
}
