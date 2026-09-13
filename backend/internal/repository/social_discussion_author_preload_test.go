package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// SP-17/T1 (#490)：/social 路径的讨论列表与详情此前缺 Preload("Author")，
// author 序列化为零值 User（前端拿不到昵称头像）。断言 Preload 生效且
// avatar_url 进入 JSON 序列化（悬浮卡数据源）。

func setupSocialDiscussionsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	// discussions 的 gorm 标签 default:NOW() 在 sqlite 不可用，沿用既有测试的 raw DDL
	//（见 discussion_hot_sort_test.go）。
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
	return db
}

func TestSocialDiscussionsSerializeAuthorProfile(t *testing.T) {
	db := setupSocialDiscussionsDB(t)
	author := model.User{
		ID: 9401, Email: "t490-author@seed.omnicraft.local", PasswordHash: "x",
		Username: "t490_author", AvatarURL: "https://oss.example/t490-avatar.png",
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("seed author: %v", err)
	}
	seeded := model.Discussion{
		AuthorID: author.ID,
		Title:    "t490 讨论",
		Body:     "正文",
		Status:   "published",
	}
	if err := db.Create(&seeded).Error; err != nil {
		t.Fatalf("seed discussion: %v", err)
	}

	repo := NewSocialRepository(db)

	t.Run("list preloads author with avatar_url", func(t *testing.T) {
		got, total, err := repo.ListDiscussions(nil, nil, 1, 20, 0)
		if err != nil {
			t.Fatalf("ListDiscussions() error = %v", err)
		}
		if total != 1 || len(got) != 1 {
			t.Fatalf("total/items = %d/%d, want 1/1", total, len(got))
		}
		if got[0].Author.ID != author.ID || got[0].Author.Username != author.Username {
			t.Fatalf("discussion %d author = %+v, want preload of user %d", got[0].ID, got[0].Author, author.ID)
		}
		payload, err := json.Marshal(got[0])
		if err != nil {
			t.Fatalf("marshal discussion: %v", err)
		}
		if !strings.Contains(string(payload), `"avatar_url":"https://oss.example/t490-avatar.png"`) {
			t.Fatalf("discussion payload missing serialized avatar_url: %s", payload)
		}
	})

	t.Run("find preloads author with avatar_url", func(t *testing.T) {
		got, err := repo.FindDiscussion(seeded.ID)
		if err != nil || got == nil {
			t.Fatalf("FindDiscussion() = %#v, %v", got, err)
		}
		if got.Author.ID != author.ID || got.Author.AvatarURL == "" {
			t.Fatalf("discussion %d author profile empty: %+v", got.ID, got.Author)
		}
	})
}
