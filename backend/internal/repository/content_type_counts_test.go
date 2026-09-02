package repository

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// #290 IP 详情分享 tab 的媒体类型 chips 计数分面：与列表同一可见性语义
// （published + 公开 + zone=fanwork + 本 IP），不受当前选中 type 影响，
// 可选标题搜索与列表一致。
func TestCountByTypeWithinIP(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	author := &model.User{Email: "facet@example.com", Username: "facet", PasswordHash: "x", Role: "user"}
	if err := db.Create(author).Error; err != nil {
		t.Fatalf("seed author: %v", err)
	}
	ip := &model.IP{Name: "Facet IP", Slug: "facet-ip", Status: "published"}
	if err := db.Create(ip).Error; err != nil {
		t.Fatalf("seed ip: %v", err)
	}
	other := &model.IP{Name: "Other IP", Slug: "other-ip", Status: "published"}
	if err := db.Create(other).Error; err != nil {
		t.Fatalf("seed other ip: %v", err)
	}

	seed := func(title, zone, contentType, status string, ipID int64) *model.ContentItem {
		item := &model.ContentItem{
			Title:       title,
			AuthorID:    author.ID,
			Zone:        zone,
			IPID:        &ipID,
			ContentType: contentType,
			Status:      status,
			IsPublic:    true,
		}
		if err := db.Create(item).Error; err != nil {
			t.Fatalf("seed content %s: %v", title, err)
		}
		return item
	}
	seed("星尘图精选", "fanwork", "image", "published", ip.ID)
	seed("星尘视频日志", "fanwork", "video", "published", ip.ID)
	seed("未发布图", "fanwork", "image", "pending", ip.ID)     // 非公开可见性排除
	seed("原创视频", "original", "video", "published", ip.ID) // 非二创 zone 排除
	seed("别家图", "fanwork", "image", "published", other.ID) // 其他 IP 排除

	repo := NewContentRepository(db)

	counts, err := repo.CountByTypeWithinIP(ip.ID, "")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if got := counts["image"]; got != 1 {
		t.Fatalf("image count = %d, want 1 (pending/other-ip excluded)", got)
	}
	if got := counts["video"]; got != 1 {
		t.Fatalf("video count = %d, want 1 (original zone excluded)", got)
	}
	if _, ok := counts["article"]; ok {
		t.Fatalf("unexpected article bucket: %+v", counts)
	}

	searched, err := repo.CountByTypeWithinIP(ip.ID, "视频")
	if err != nil {
		t.Fatalf("count with q: %v", err)
	}
	if got := searched["video"]; got != 1 {
		t.Fatalf("video search count = %d, want 1", got)
	}
	if got := searched["image"]; got != 0 {
		t.Fatalf("image search count = %d, want 0", got)
	}
}
