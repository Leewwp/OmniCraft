package repository

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

func setupSocialCommentsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	// in-memory sqlite：钉单连接，避免连接池拿到空库（social_service_test 同款）。
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user model: %v", err)
	}
	// model.Comment 带 GORM `DEFAULT NOW()` 渲染，sqlite DDL 不认，手建表。
	if err := db.Exec(`
		CREATE TABLE comments (
			id integer PRIMARY KEY AUTOINCREMENT,
			content_item_id integer,
			discussion_id integer,
			parent_id integer,
			author_id integer NOT NULL,
			target_type text,
			target_id integer,
			content text,
			body text NOT NULL,
			status text NOT NULL DEFAULT 'published',
			like_count integer NOT NULL DEFAULT 0,
			created_at datetime,
			updated_at datetime
		)`).Error; err != nil {
		t.Fatalf("create comments table: %v", err)
	}
	return db
}

// T45（FIX-29a）：内容评论必须预加载 Author，否则前端昵称头像恒为空。
func TestListCommentsPreloadsAuthor(t *testing.T) {
	db := setupSocialCommentsDB(t)
	author := model.User{
		ID: 9201, Email: "t45-author@seed.omnicraft.local", PasswordHash: "x",
		Username: "t45_author", AvatarURL: "https://oss.example/t45-avatar.png",
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("seed author: %v", err)
	}
	contentID := int64(7301)
	rows := []model.Comment{
		{ContentItemID: &contentID, AuthorID: author.ID, Body: "first", Status: "published"},
		{ContentItemID: &contentID, AuthorID: author.ID, Body: "second", Status: "published"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed comments: %v", err)
	}

	repo := NewSocialRepository(db)
	got, total, err := repo.ListComments(contentID, nil, 1, 20)
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if total != 2 || len(got) != 2 {
		t.Fatalf("total/items = %d/%d, want 2/2", total, len(got))
	}
	for _, comment := range got {
		if comment.Author.ID != author.ID {
			t.Fatalf("comment %d Author.ID = %d, want %d (Preload missing)", comment.ID, comment.Author.ID, author.ID)
		}
		if comment.Author.Username != author.Username || comment.Author.AvatarURL == "" {
			t.Fatalf("comment %d author profile empty: %+v", comment.ID, comment.Author)
		}
	}

	// T03 回归：Author 序列化不得带出 email（匿名视角契约）。
	// 注意 email_verified_at 是合法公开字段（时间戳），断言须精确到
	// email 键本身与邮箱值，不能做裸 "email" 子串匹配。
	payload, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal comments: %v", err)
	}
	if strings.Contains(string(payload), author.Email) {
		t.Fatalf("comment payload leaks author email value: %s", payload)
	}
	if matched, _ := regexp.Match(`"email"\s*:`, payload); matched {
		t.Fatalf("comment payload contains an email key: %s", payload)
	}
}
