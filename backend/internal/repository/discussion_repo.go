package repository

import (
	"fmt"
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type DiscussionRepository struct {
	db *gorm.DB
}

func NewDiscussionRepository(db *gorm.DB) *DiscussionRepository {
	return &DiscussionRepository{db: db}
}

// ListByIP returns one page of an IP's discussions. sort: latest_reply (默认)
// / newest_post / most_replies / hot（log(回复数+1)/龄期衰减，#290）。
// query（IP 内搜索）在标题/正文上做包含匹配；置顶帖在所有排序下优先。
func (r *DiscussionRepository) ListByIP(ipID int64, sort, query string, hotDecayHours float64, page, pageSize int) ([]model.Discussion, int64, error) {
	base := r.db.Model(&model.Discussion{}).Where("ip_id = ? AND status = ?", ipID, "published")
	if query != "" {
		like := "%" + query + "%"
		base = base.Where("title LIKE ? OR body LIKE ?", like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	q := base.Session(&gorm.Session{})
	switch sort {
	case "newest_post":
		q = q.Order("is_pinned DESC, created_at DESC")
	case "most_replies":
		q = q.Order("is_pinned DESC, reply_count DESC")
	case "hot":
		// score = ln(reply_count+1) / (1 + age_hours / decay) —— 回复越多越热，
		// 龄期按 config.discussion.hot_decay_hours 衰减；方言差异分别表达。
		if hotDecayHours <= 0 {
			hotDecayHours = 72
		}
		if r.db.Dialector.Name() == "sqlite" {
			q = q.Order(fmt.Sprintf(
				"is_pinned DESC, (ln(reply_count + 1) / (1 + ((strftime('%%s','now') - strftime('%%s', created_at)) / 3600.0) / %f)) DESC",
				hotDecayHours))
		} else {
			q = q.Order(fmt.Sprintf(
				"is_pinned DESC, (ln(reply_count + 1) / (1 + (EXTRACT(EPOCH FROM (now() - created_at)) / 3600.0) / %f)) DESC",
				hotDecayHours))
		}
	default:
		q = q.Order("is_pinned DESC, last_active_at DESC")
	}

	var discussions []model.Discussion
	err := q.Preload("Author").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&discussions).Error
	return discussions, total, err
}

func (r *DiscussionRepository) GetByID(id int64) (*model.Discussion, error) {
	var d model.Discussion
	err := r.db.Preload("Author").First(&d, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *DiscussionRepository) Create(d *model.Discussion) error {
	return r.db.Create(d).Error
}

func (r *DiscussionRepository) Update(id int64, updates map[string]interface{}) error {
	return r.db.Model(&model.Discussion{}).Where("id = ?", id).Updates(updates).Error
}

func (r *DiscussionRepository) Delete(id int64) error {
	return r.db.Delete(&model.Discussion{}, id).Error
}

func (r *DiscussionRepository) Pin(id int64, pinned bool) error {
	return r.db.Model(&model.Discussion{}).Where("id = ?", id).Update("is_pinned", pinned).Error
}

func (r *DiscussionRepository) IncrementReplyCount(id int64) error {
	return r.db.Model(&model.Discussion{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"reply_count":    gorm.Expr("reply_count + 1"),
			"last_active_at": gorm.Expr("NOW()"),
		}).Error
}

func (r *DiscussionRepository) SearchByKeyword(ipID int64, keyword string, page, pageSize int) ([]model.Discussion, error) {
	var discussions []model.Discussion
	err := r.db.Where("ip_id = ? AND to_tsvector('simple', title || ' ' || body) @@ plainto_tsquery('simple', ?)", ipID, keyword).
		Order("last_active_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&discussions).Error
	return discussions, err
}

func (r *DiscussionRepository) ListByUser(userID int64, page, pageSize int) ([]model.Discussion, int64, error) {
	commentSubq := r.db.Model(&model.Comment{}).
		Select("discussion_id").
		Where("author_id = ? AND discussion_id IS NOT NULL", userID)

	base := r.db.Where(
		"author_id = ? OR id IN (?)",
		userID,
		commentSubq,
	)

	var total int64
	base.Model(&model.Discussion{}).Count(&total)

	var discussions []model.Discussion
	err := base.Preload("Author").
		Order("last_active_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&discussions).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list discussions by user %d: %w", userID, err)
	}
	return discussions, total, nil
}
