package scheduler

import (
	"log/slog"
	"time"

	"omnicraft/backend/internal/pkg/recovery"

	"gorm.io/gorm"
)

type TagUsageSync struct {
	db *gorm.DB
}

func NewTagUsageSync(db *gorm.DB) *TagUsageSync {
	return &TagUsageSync{db: db}
}

func (s *TagUsageSync) Start() {
	recovery.GoSafe(func() {
		s.Run()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			s.Run()
		}
	})
}

func (s *TagUsageSync) Run() {
	slog.Info("[TagUsageSync] Starting tag usage count sync")
	result := s.db.Exec(`
		UPDATE tags t
		SET usage_count = (
			SELECT COUNT(*)
			FROM content_tags ct
			JOIN content_items ci ON ci.id = ct.content_item_id
			WHERE ct.tag = t.name AND ci.status = 'published'
		)
	`)
	if result.Error != nil {
		slog.Error("[TagUsageSync] Failed to sync usage counts", "error", result.Error)
		return
	}
	slog.Info("[TagUsageSync] Completed", "tags_updated", result.RowsAffected)
}