package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

// ContentNavSummary（#793）：通知 / admin 申诉 / 消息读侧共享的内容导航投影。
// 只取深链所需字段（title 供通知引用块，zone 供路由分流），不扩大取数面。
type ContentNavSummary struct {
	Title string
	Zone  string
}

// ContentNavigationRepository（#793）是批量内容导航取数的最小共享能力：caller
// 按一页去重收集内容 ID 后一次 IN 查询取 id→{title,zone}。路由计算归
// internal/pkg/contentroute 纯函数——SQL 不进纯路由 module，路由不进 SQL；
// handler 也不得借道 NotificationRepository 的私有装饰方法。
type ContentNavigationRepository struct {
	db *gorm.DB
}

func NewContentNavigationRepository(db *gorm.DB) *ContentNavigationRepository {
	return &ContentNavigationRepository{db: db}
}

// SummariesByIDs 返回 id→ContentNavSummary（一次批量查询；不存在的 ID 无条目，
// 与通知装饰既有语义一致，含软删行——内容可见性仍由详情读取规则决定）。
// 查询失败返回空 map（fail-open）：装饰取数故障由 caller 保留旧回退语义，
// 不让列表接口整体失败，也不用空值覆盖已有字段。
func (r *ContentNavigationRepository) SummariesByIDs(ids []int64) map[int64]ContentNavSummary {
	result := map[int64]ContentNavSummary{}
	if r == nil || r.db == nil || len(ids) == 0 {
		return result
	}
	var rows []struct {
		ID    int64
		Title string
		Zone  string
	}
	if err := r.db.Model(&model.ContentItem{}).
		Select("id, title, zone").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.ID] = ContentNavSummary{Title: row.Title, Zone: row.Zone}
	}
	return result
}
