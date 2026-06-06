package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type FeedbackRepository struct {
	db *gorm.DB
}

func NewFeedbackRepository(db *gorm.DB) *FeedbackRepository {
	return &FeedbackRepository{db: db}
}

func (r *FeedbackRepository) WithTx(tx *gorm.DB) *FeedbackRepository {
	return &FeedbackRepository{db: tx}
}

func (r *FeedbackRepository) CreateTicket(t *model.FeedbackTicket) error {
	return r.db.Create(t).Error
}

func (r *FeedbackRepository) CreateTicketWithAttachments(t *model.FeedbackTicket, attachments []model.FeedbackAttachment) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return err
		}
		for i := range attachments {
			attachments[i].TicketID = t.ID
			if err := tx.Create(&attachments[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *FeedbackRepository) FindTicketByID(id int64) (*model.FeedbackTicket, error) {
	var t model.FeedbackTicket
	err := r.db.Preload("Replies", func(db *gorm.DB) *gorm.DB {
		return db.Where("is_internal_note = false").Order("created_at ASC")
	}).Preload("Attachments").First(&t, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *FeedbackRepository) FindTicketByIDForAdmin(id int64) (*model.FeedbackTicket, error) {
	var t model.FeedbackTicket
	err := r.db.Preload("Replies", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC")
	}).Preload("Attachments").First(&t, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *FeedbackRepository) ListByUser(userID int64, page, pageSize int) ([]model.FeedbackTicket, int64, error) {
	var total int64
	r.db.Model(&model.FeedbackTicket{}).Where("user_id = ?", userID).Count(&total)
	var tickets []model.FeedbackTicket
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&tickets).Error
	return tickets, total, err
}

func (r *FeedbackRepository) UpdateTicket(t *model.FeedbackTicket) error {
	return r.db.Save(t).Error
}

func (r *FeedbackRepository) CreateReply(rep *model.FeedbackReply) error {
	return r.db.Create(rep).Error
}

func (r *FeedbackRepository) CreateAttachment(a *model.FeedbackAttachment) error {
	return r.db.Create(a).Error
}

func (r *FeedbackRepository) FindAttachmentByOSSKey(ossKey string) (*model.FeedbackAttachment, error) {
	var a model.FeedbackAttachment
	err := r.db.Where("oss_key = ?", ossKey).First(&a).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

type AdminFeedbackFilter struct {
	Status     string
	Category   string
	Priority   string
	AssigneeID *int64
	Page       int
	PageSize   int
}

func (r *FeedbackRepository) ListAdminFeedback(filter AdminFeedbackFilter) ([]model.FeedbackTicket, int64, error) {
	q := r.db.Model(&model.FeedbackTicket{})
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Category != "" {
		q = q.Where("category = ?", filter.Category)
	}
	if filter.Priority != "" {
		q = q.Where("priority = ?", filter.Priority)
	}
	if filter.AssigneeID != nil {
		q = q.Where("assignee_admin_id = ?", *filter.AssigneeID)
	}

	var total int64
	q.Count(&total)

	var tickets []model.FeedbackTicket
	err := q.Preload("Attachments").
		Order("created_at DESC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).
		Find(&tickets).Error
	return tickets, total, err
}

func (r *FeedbackRepository) CountByStatus(status string) (int64, error) {
	var count int64
	err := r.db.Model(&model.FeedbackTicket{}).Where("status = ?", status).Count(&count).Error
	return count, err
}
