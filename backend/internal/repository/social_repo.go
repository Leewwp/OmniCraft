package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SocialRepository struct {
	db *gorm.DB
}

func NewSocialRepository(db *gorm.DB) *SocialRepository {
	return &SocialRepository{db: db}
}

func (r *SocialRepository) CreateComment(c *model.Comment) error {
	return r.db.Create(c).Error
}

func (r *SocialRepository) FindComment(id int64) (*model.Comment, error) {
	var c model.Comment
	err := r.db.First(&c, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *SocialRepository) ListComments(contentID int64, parentID *int64, page, pageSize int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64
	q := r.db.Model(&model.Comment{}).Where("content_item_id = ? AND status = ?", contentID, "published")
	if parentID == nil {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *parentID)
	}
	q.Count(&total)
	offset := (page - 1) * pageSize
	err := q.Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&comments).Error
	return comments, total, err
}

func (r *SocialRepository) ListCommentsByTarget(targetType string, targetID int64, page, pageSize int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64
	var q *gorm.DB
	if targetType == "discussion" {
		q = r.db.Model(&model.Comment{}).
			Where("discussion_id = ? AND parent_id IS NULL AND status = ?", targetID, "published")
	} else {
		q = r.db.Model(&model.Comment{}).
			Where("target_type = ? AND target_id = ? AND parent_id IS NULL AND status = ?", targetType, targetID, "published")
	}
	q.Count(&total)
	err := q.Preload("Author").Order("created_at ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&comments).Error
	return comments, total, err
}

func (r *SocialRepository) DeleteComment(id int64) error {
	return r.db.Model(&model.Comment{}).Where("id = ?", id).Update("status", "hidden").Error
}

func (r *SocialRepository) CreateDiscussion(d *model.Discussion) error {
	return r.db.Create(d).Error
}

func (r *SocialRepository) FindDiscussion(id int64) (*model.Discussion, error) {
	var d model.Discussion
	err := r.db.First(&d, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *SocialRepository) ListDiscussions(ipID *int64, contentID *int64, page, pageSize int) ([]model.Discussion, int64, error) {
	var discussions []model.Discussion
	var total int64
	q := r.db.Model(&model.Discussion{}).Where("status = ?", "published")
	if ipID != nil {
		q = q.Where("ip_id = ?", *ipID)
	}
	if contentID != nil {
		q = q.Where("content_item_id = ?", *contentID)
	}
	q.Count(&total)
	offset := (page - 1) * pageSize
	err := q.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&discussions).Error
	return discussions, total, err
}

func (r *SocialRepository) UpsertReaction(reaction *model.Reaction) (string, error) {
	var existing model.Reaction
	err := r.db.Where("user_id = ? AND target_type = ? AND target_id = ?",
		reaction.UserID, reaction.TargetType, reaction.TargetID).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		if err2 := r.db.Create(reaction).Error; err2 != nil {
			return "", err2
		}
		return "created", nil
	}
	if err != nil {
		return "", err
	}

	if existing.Reaction == reaction.Reaction {
		r.db.Delete(&existing)
		return "removed", nil
	}

	r.db.Model(&existing).Update("reaction", reaction.Reaction)
	return "updated", nil
}

func (r *SocialRepository) GetReactionCounts(targetType string, targetID int64) (int64, int64, error) {
	var likes, dislikes int64
	r.db.Model(&model.Reaction{}).Where("target_type = ? AND target_id = ? AND reaction = ?", targetType, targetID, "like").Count(&likes)
	r.db.Model(&model.Reaction{}).Where("target_type = ? AND target_id = ? AND reaction = ?", targetType, targetID, "dislike").Count(&dislikes)
	return likes, dislikes, nil
}

func (r *SocialRepository) CreateReport(report *model.Report) error {
	return r.db.Create(report).Error
}

func (r *SocialRepository) CountReports(targetType string, targetID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.Report{}).Where("target_type = ? AND target_id = ?", targetType, targetID).Count(&count).Error
	return count, err
}

func (r *SocialRepository) CreateFavorite(userID, contentID int64) error {
	fav := model.Favorite{UserID: userID, ContentItemID: contentID}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&fav).Error
}

func (r *SocialRepository) DeleteFavorite(userID, contentID int64) error {
	return r.db.Where("user_id = ? AND content_item_id = ?", userID, contentID).
		Delete(&model.Favorite{}).Error
}

func (r *SocialRepository) ListFavoritesByUser(userID int64, page, pageSize int) ([]model.Favorite, int64, error) {
	var favs []model.Favorite
	var total int64
	q := r.db.Model(&model.Favorite{}).Where("user_id = ?", userID)
	q.Count(&total)
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&favs).Error
	return favs, total, err
}
