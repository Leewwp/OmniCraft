package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type PRRepository struct {
	db *gorm.DB
}

func NewPRRepository(db *gorm.DB) *PRRepository {
	return &PRRepository{db: db}
}

func (r *PRRepository) CreatePR(pr *model.PullRequest) error {
	return r.db.Create(pr).Error
}

func (r *PRRepository) FindByID(id int64) (*model.PullRequest, error) {
	var pr model.PullRequest
	err := r.db.First(&pr, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &pr, nil
}

func (r *PRRepository) ListByContent(contentID int64, status string) ([]model.PullRequest, error) {
	var prs []model.PullRequest
	q := r.db.Where("content_item_id = ?", contentID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("created_at DESC").Find(&prs).Error
	return prs, err
}

func (r *PRRepository) ListByContentPaged(contentID int64, status string, page, pageSize int) ([]model.PullRequest, int64, error) {
	var total int64
	q := r.db.Model(&model.PullRequest{}).Where("content_item_id = ?", contentID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var prs []model.PullRequest
	err := r.db.Where("content_item_id = ?", contentID).
		Scopes(func(db *gorm.DB) *gorm.DB {
			if status != "" {
				return db.Where("status = ?", status)
			}
			return db
		}).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&prs).Error
	return prs, total, err
}

func (r *PRRepository) UpdateStatus(id int64, status string, extra map[string]interface{}) error {
	updates := map[string]interface{}{"status": status}
	for k, v := range extra {
		updates[k] = v
	}
	return r.db.Model(&model.PullRequest{}).Where("id = ?", id).Updates(updates).Error
}

func (r *PRRepository) IsBlocked(authorID int64, submitterID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.AuthorBlocklist{}).
		Where("author_id = ? AND blocked_id = ?", authorID, submitterID).
		Count(&count).Error
	return count > 0, err
}

func (r *PRRepository) UpsertContributor(contentID int64, userID int64) error {
	return r.db.Exec(`
		INSERT INTO content_contributors (content_item_id, user_id, pr_count, first_at)
		VALUES (?, ?, 1, NOW())
		ON CONFLICT (content_item_id, user_id)
		DO UPDATE SET pr_count = content_contributors.pr_count + 1
	`, contentID, userID).Error
}

func (r *PRRepository) ListContributors(contentID int64) ([]model.ContentContributor, error) {
	var contributors []model.ContentContributor
	err := r.db.Where("content_item_id = ?", contentID).Order("pr_count DESC").Find(&contributors).Error
	return contributors, err
}

func (r *PRRepository) BlockUser(authorID int64, blockedID int64) error {
	blocklist := model.AuthorBlocklist{AuthorID: authorID, BlockedID: blockedID}
	return r.db.FirstOrCreate(&blocklist, model.AuthorBlocklist{AuthorID: authorID, BlockedID: blockedID}).Error
}

func (r *PRRepository) UnblockUser(authorID int64, blockedID int64) error {
	return r.db.Where("author_id = ? AND blocked_id = ?", authorID, blockedID).
		Delete(&model.AuthorBlocklist{}).Error
}
