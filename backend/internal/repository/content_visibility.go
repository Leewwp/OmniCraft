package repository

import (
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

func ApplyContentVisibilityScope(db *gorm.DB, viewerID int64) *gorm.DB {
	db = db.
		Where("content_items.status = ?", "published").
		Where("content_items.deleted_at IS NULL").
		Where("content_items.author_id NOT IN (SELECT id FROM users WHERE is_banned = true OR deleted_at IS NOT NULL)").
		Where("content_items.ip_id IS NULL OR content_items.ip_id NOT IN (SELECT id FROM ips WHERE status = ?)", "banned")

	db = db.Where(
		"content_items.is_public = ? OR content_items.author_id = ?",
		true, viewerID,
	)

	return db
}

func ContentVisibilitySQL(viewerID int64) (string, []interface{}) {
	return "content_items.status = ? AND content_items.deleted_at IS NULL AND content_items.author_id NOT IN (SELECT id FROM users WHERE is_banned = true OR deleted_at IS NOT NULL) AND (content_items.ip_id IS NULL OR content_items.ip_id NOT IN (SELECT id FROM ips WHERE status = ?)) AND (content_items.is_public = ? OR content_items.author_id = ?)",
		[]interface{}{"published", "banned", true, viewerID}
}

// ContentVisibleToViewer is the single-row form of the anonymous visibility
// scope, for detail boundaries and content-derived listings (versions, PRs,
// comments, source enrichment) that already hold the row and need a gate
// before serving derived data (#446 / SP-16 P0).
func ContentVisibleToViewer(db *gorm.DB, content *model.ContentItem, viewerID int64) bool {
	if content == nil {
		return false
	}
	var visible int64
	sql, args := ContentVisibilitySQL(viewerID)
	if err := db.Model(&model.ContentItem{}).
		Where("id = ?", content.ID).
		Where(sql, args...).
		Count(&visible).Error; err != nil {
		return false
	}
	return visible > 0
}
