package repository

import (
	"fmt"

	"gorm.io/gorm"
)

func ApplyContentVisibilityScope(db *gorm.DB, viewerID int64) *gorm.DB {
	db = db.
		Where("content_items.status = ?", "published").
		Where("content_items.deleted_at IS NULL").
		Where("content_items.author_id NOT IN (SELECT id FROM users WHERE is_banned = true OR deleted_at IS NOT NULL)")

	db = db.Where(
		"content_items.is_public = true OR content_items.author_id = ?",
		viewerID,
	)

	return db
}

func ContentVisibilityWhere(viewerID int64) string {
	return fmt.Sprintf(
		"content_items.status = 'published' AND content_items.deleted_at IS NULL AND content_items.author_id NOT IN (SELECT id FROM users WHERE is_banned = true OR deleted_at IS NOT NULL) AND (content_items.is_public = true OR content_items.author_id = %d)",
		viewerID,
	)
}
