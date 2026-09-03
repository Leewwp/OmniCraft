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

func (r *SocialRepository) DB() *gorm.DB {
	return r.db
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
	err := q.Preload("Author").Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&comments).Error
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

// ListCommentsByParentIDs returns published children of the given comments in
// one query (T46: discussion detail ships top-level page plus its children so
// the client can nest without extra round-trips).
func (r *SocialRepository) ListCommentsByParentIDs(parentIDs []int64) ([]model.Comment, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}
	var children []model.Comment
	err := r.db.Model(&model.Comment{}).
		Where("parent_id IN ? AND status = ?", parentIDs, "published").
		Preload("Author").Order("created_at ASC").
		Find(&children).Error
	return children, err
}

func (r *SocialRepository) DeleteComment(id int64) error {
	return r.db.Model(&model.Comment{}).Where("id = ?", id).Update("status", "hidden").Error
}

func (r *SocialRepository) EditComment(id int64, body string) error {
	return r.db.Model(&model.Comment{}).Where("id = ?", id).Updates(map[string]interface{}{
		"body":       body,
		"updated_at": gorm.Expr("NOW()"),
	}).Error
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
	err := q.Order("last_active_at DESC").Offset(offset).Limit(pageSize).Find(&discussions).Error
	return discussions, total, err
}

func (r *SocialRepository) UpsertReaction(reaction *model.Reaction) (string, error) {
	action := ""
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.Reaction
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"user_id = ? AND target_type = ? AND target_id = ?",
			reaction.UserID, reaction.TargetType, reaction.TargetID,
		).First(&existing).Error

		if err == gorm.ErrRecordNotFound {
			if err := tx.Create(reaction).Error; err != nil {
				return err
			}
			action = "created"
			return nil
		}
		if err != nil {
			return err
		}

		if existing.Reaction == reaction.Reaction {
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
			action = "removed"
			return nil
		}

		if err := tx.Model(&existing).Update("reaction", reaction.Reaction).Error; err != nil {
			return err
		}
		action = "updated"
		return nil
	})
	return action, err
}

func (r *SocialRepository) GetReactionCounts(targetType string, targetID int64) (int64, int64, error) {
	var results []struct {
		Reaction string
		Count    int64
	}
	err := r.db.Model(&model.Reaction{}).
		Select("reaction, COUNT(*) as count").
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Group("reaction").
		Find(&results).Error
	if err != nil {
		return 0, 0, err
	}

	var likes, dislikes int64
	for _, r := range results {
		switch r.Reaction {
		case "like":
			likes = r.Count
		case "dislike":
			dislikes = r.Count
		}
	}
	return likes, dislikes, nil
}

func (r *SocialRepository) GetViewerReaction(userID int64, targetType string, targetID int64) (*string, error) {
	if userID <= 0 {
		return nil, nil
	}
	var reaction model.Reaction
	err := r.db.Select("reaction").Where(
		"user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID,
	).First(&reaction).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	value := reaction.Reaction
	return &value, nil
}

func (r *SocialRepository) CreateReport(report *model.Report) error {
	return r.db.Create(report).Error
}

func (r *SocialRepository) FindReportByUserAndTarget(reporterID int64, targetType string, targetID int64) (*model.Report, error) {
	var report model.Report
	err := r.db.Where("reporter_id = ? AND target_type = ? AND target_id = ?", reporterID, targetType, targetID).First(&report).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *SocialRepository) CountReports(targetType string, targetID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.Report{}).Where("target_type = ? AND target_id = ?", targetType, targetID).Count(&count).Error
	return count, err
}
