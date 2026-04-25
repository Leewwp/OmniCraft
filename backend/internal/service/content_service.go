package service

import (
	"context"
	"errors"
	"strconv"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
)

var (
	ErrContentNotFound  = errors.New("content not found")
	ErrContentForbidden = errors.New("forbidden: not content author")
	ErrPublishFrozen    = errors.New("publish permission is temporarily frozen")
)

type ContentService struct {
	contentRepo *repository.ContentRepository
	reviewSvc   *ReviewService
	rdb         *redis.Client
}

func NewContentService(contentRepo *repository.ContentRepository) *ContentService {
	return &ContentService{contentRepo: contentRepo}
}

func NewContentServiceWithDeps(contentRepo *repository.ContentRepository, reviewSvc *ReviewService, rdb *redis.Client) *ContentService {
	return &ContentService{contentRepo: contentRepo, reviewSvc: reviewSvc, rdb: rdb}
}

type PublishContentInput struct {
	Title         string            `json:"title" binding:"required,min=1,max=500"`
	Zone          string            `json:"zone" binding:"required,oneof=fanwork original"`
	IPID          *int64            `json:"ip_id"`
	Category      string            `json:"category"`
	ContentType   string            `json:"content_type" binding:"required"`
	CoverImageURL string            `json:"cover_image_url"`
	IsPublic      bool              `json:"is_public"`
	AllowCopy     bool              `json:"allow_copy"`
	Tags          []string          `json:"tags"`
	Attachments   []AttachmentInput `json:"attachments"`
}

type AttachmentInput struct {
	FileType    string `json:"file_type"`
	OSSKey      string `json:"oss_key"`
	FileSize    *int64 `json:"file_size"`
	MimeType    string `json:"mime_type"`
	DurationSec *int   `json:"duration_sec"`
	Width       *int   `json:"width"`
	Height      *int   `json:"height"`
	IsPrimary   bool   `json:"is_primary"`
}

func (s *ContentService) PublishContent(input PublishContentInput, authorID int64) (*model.ContentItem, error) {
	if s.rdb != nil {
		freezeKey := "publish_freeze:" + strconv.FormatInt(authorID, 10)
		if frozen, err := s.rdb.Exists(context.Background(), freezeKey).Result(); err == nil && frozen > 0 {
			return nil, ErrPublishFrozen
		}
	}

	content := &model.ContentItem{
		Title:         input.Title,
		AuthorID:      authorID,
		Zone:          input.Zone,
		IPID:          input.IPID,
		Category:      input.Category,
		ContentType:   input.ContentType,
		CoverImageURL: input.CoverImageURL,
		IsPublic:      input.IsPublic,
		AllowCopy:     input.AllowCopy,
		Status:        "pending",
	}

	if err := s.contentRepo.CreateContent(content); err != nil {
		return nil, err
	}

	if len(input.Tags) > 0 {
		tags := make([]model.ContentTag, 0, len(input.Tags))
		for _, tag := range input.Tags {
			if tag != "" {
				tags = append(tags, model.ContentTag{
					ContentItemID: content.ID,
					Tag:           tag,
				})
			}
		}
		if err := s.contentRepo.CreateTags(tags); err != nil {
			return nil, err
		}
	}

	if len(input.Attachments) > 0 {
		attachments := make([]model.ContentAttachment, 0, len(input.Attachments))
		for _, a := range input.Attachments {
			attachments = append(attachments, model.ContentAttachment{
				ContentItemID: content.ID,
				FileType:      a.FileType,
				OSSKey:        a.OSSKey,
				FileSize:      a.FileSize,
				MimeType:      a.MimeType,
				DurationSec:   a.DurationSec,
				Width:         a.Width,
				Height:        a.Height,
				IsPrimary:     a.IsPrimary,
			})
		}
		if err := s.contentRepo.CreateAttachments(attachments); err != nil {
			return nil, err
		}
	}

	if s.reviewSvc != nil {
		reviewInput := SubmitReviewInput{
			TargetType:  "content",
			TargetID:    content.ID,
			ContentType: input.ContentType,
			Title:       input.Title,
			Description: "",
			AuthorID:    authorID,
			Attachments: input.Attachments,
		}
		if err := s.reviewSvc.SubmitForAIReview(context.Background(), reviewInput); err != nil {
			return nil, err
		}
	}

	return content, nil
}

func (s *ContentService) GetContent(id int64) (*model.ContentItem, error) {
	content, err := s.contentRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, ErrContentNotFound
	}
	return content, nil
}

func (s *ContentService) ListContents(filter repository.ListContentsFilter) ([]model.ContentItem, int64, error) {
	return s.contentRepo.ListContents(filter)
}

func (s *ContentService) UpdateContent(id int64, authorID int64, updates map[string]interface{}) error {
	content, err := s.contentRepo.FindByID(id)
	if err != nil || content == nil {
		return ErrContentNotFound
	}
	if content.AuthorID != authorID {
		return ErrContentForbidden
	}
	return s.contentRepo.UpdateContent(id, updates)
}

func (s *ContentService) DeleteContent(id int64, authorID int64) error {
	content, err := s.contentRepo.FindByID(id)
	if err != nil || content == nil {
		return ErrContentNotFound
	}
	if content.AuthorID != authorID {
		return ErrContentForbidden
	}
	return s.contentRepo.DeleteContent(id)
}
