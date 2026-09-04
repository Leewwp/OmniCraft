package service

import (
	"testing"

	"omnicraft/backend/internal/model"
)

// #318: PublishContent 显式 is_public=false / allow_copy=false 必须原样落库。
// 修复前 model 两字段带 gorm default:true 标签，GORM 在 INSERT 剔除零值字段，
// 显式 false 被数据库默认 true 静默覆盖——受限内容发布后变公开（#316 注入
// 被迫补偿 400 条受限条目）。夹具镜像生产行为，不再依赖隐式默认。
func TestPublishContentPersistsExplicitRestrictedFlags(t *testing.T) {
	svc, _, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()

	published, err := svc.PublishContent(PublishContentInput{
		Title:       "restricted article",
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		IsPublic:    false,
		AllowCopy:   false,
	}, 1)
	if err != nil {
		t.Fatalf("publish restricted content: %v", err)
	}

	var stored model.ContentItem
	if err := svc.contentRepo.DB().First(&stored, published.ID).Error; err != nil {
		t.Fatalf("load stored content: %v", err)
	}
	if stored.IsPublic {
		t.Fatal("explicit is_public=false must persist, got true (GORM default-zero-skip drift, #318)")
	}
	if stored.AllowCopy {
		t.Fatal("explicit allow_copy=false must persist, got true (same default-zero-skip class as #318)")
	}
}
