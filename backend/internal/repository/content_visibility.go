package repository

import (
	"gorm.io/gorm"
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
