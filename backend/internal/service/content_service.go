package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
)

var (
	ErrContentNotFound       = errors.New("content not found")
	ErrContentForbidden      = errors.New("forbidden: not content author")
	ErrPublishFrozen         = errors.New("publish permission is temporarily frozen")
	ErrInvalidSourceOriginal = errors.New("source original must be a published original content item")
)

type ContentService struct {
	contentRepo *repository.ContentRepository
	reviewSvc   *ReviewService
	rdb         *redis.Client
	cacheCfg    *config.CacheConfig
}

func NewContentService(contentRepo *repository.ContentRepository) *ContentService {
	return &ContentService{contentRepo: contentRepo}
}

func NewContentServiceWithDeps(contentRepo *repository.ContentRepository, reviewSvc *ReviewService, rdb *redis.Client) *ContentService {
	return &ContentService{contentRepo: contentRepo, reviewSvc: reviewSvc, rdb: rdb}
}

func NewContentServiceWithCache(contentRepo *repository.ContentRepository, reviewSvc *ReviewService, rdb *redis.Client, cacheCfg *config.CacheConfig) *ContentService {
	return &ContentService{contentRepo: contentRepo, reviewSvc: reviewSvc, rdb: rdb, cacheCfg: cacheCfg}
}

type PublishContentInput struct {
	Title            string            `json:"title" binding:"required,min=1,max=500"`
	Description      string            `json:"description"`
	Zone             string            `json:"zone" binding:"required,oneof=fanwork original"`
	IPID             *int64            `json:"ip_id"`
	SourceOriginalID *int64            `json:"source_original_id"`
	Category         string            `json:"category"`
	ContentType      string            `json:"content_type" binding:"required"`
	CoverImageURL    string            `json:"cover_image_url"`
	IsPublic         bool              `json:"is_public"`
	AllowCopy        bool              `json:"allow_copy"`
	Tags             []string          `json:"tags"`
	Attachments      []AttachmentInput `json:"attachments"`
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

	var sourceOriginal *model.ContentItem
	if input.SourceOriginalID != nil {
		source, err := s.contentRepo.FindByID(*input.SourceOriginalID)
		if err != nil {
			return nil, err
		}
		if source == nil {
			return nil, ErrInvalidSourceOriginal
		}
		sourceOriginal = source
	}
	if err := validateSourceOriginalLink(input.Zone, sourceOriginal); err != nil {
		return nil, err
	}

	content := &model.ContentItem{
		Title:            input.Title,
		Description:      input.Description,
		AuthorID:         authorID,
		Zone:             input.Zone,
		IPID:             input.IPID,
		SourceOriginalID: input.SourceOriginalID,
		Category:         input.Category,
		ContentType:      input.ContentType,
		CoverImageURL:    input.CoverImageURL,
		IsPublic:         input.IsPublic,
		AllowCopy:        input.AllowCopy,
		Status:           "pending",
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
			Description: input.Description,
			AuthorID:    authorID,
			Attachments: input.Attachments,
		}
		if err := s.reviewSvc.SubmitForAIReview(context.Background(), reviewInput); err != nil {
			return nil, err
		}
	}

	s.invalidateContentListCache()

	return content, nil
}

func validateSourceOriginalLink(zone string, source *model.ContentItem) error {
	if source == nil {
		return nil
	}
	if zone != "fanwork" {
		return ErrInvalidSourceOriginal
	}
	if source.Zone != "original" || source.Status != "published" {
		return ErrInvalidSourceOriginal
	}
	return nil
}

func (s *ContentService) GetContent(id int64) (*model.ContentItem, error) {
	if s.rdb != nil && s.cacheCfg != nil {
		cacheKey := fmt.Sprintf("cache:content:%d", id)
		var cached model.ContentItem
		if hit, _ := redisclient.GetJSON(context.Background(), cacheKey, &cached); hit {
			return &cached, nil
		}
	}

	content, err := s.contentRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, ErrContentNotFound
	}

	if s.rdb != nil && s.cacheCfg != nil {
		ttl := time.Duration(s.cacheCfg.ContentDetailTTL) * time.Second
		redisclient.SetJSON(context.Background(), fmt.Sprintf("cache:content:%d", id), content, ttl)
	}

	return content, nil
}

func (s *ContentService) ListContents(filter repository.ListContentsFilter) ([]model.ContentItem, int64, error) {
	if s.rdb != nil && s.cacheCfg != nil && filter.Sort == "hot" && filter.Zone != "" && filter.Category == "" && filter.ContentType == "" && filter.Tags == nil && filter.TimeRange == "all" {
		contents, err := s.getHotContents(context.Background(), filter)
		if err == nil && len(contents) > 0 {
			total := int64(len(contents))
			page := filter.Page
			if page < 1 {
				page = 1
			}
			pageSize := filter.PageSize
			if pageSize < 1 || pageSize > 100 {
				pageSize = 20
			}
			start := (page - 1) * pageSize
			if start >= len(contents) {
				return []model.ContentItem{}, total, nil
			}
			end := start + pageSize
			if end > len(contents) {
				end = len(contents)
			}
			return contents[start:end], total, nil
		}
	}

	if s.rdb != nil && s.cacheCfg != nil {
		cacheKey := redisclient.ListCacheKey("content", filter)
		var result struct {
			Contents []model.ContentItem `json:"contents"`
			Total    int64               `json:"total"`
		}
		if hit, _ := redisclient.GetJSON(context.Background(), cacheKey, &result); hit {
			return result.Contents, result.Total, nil
		}
	}

	contents, total, err := s.contentRepo.ListContents(filter)
	if err != nil {
		return nil, 0, err
	}

	if s.rdb != nil && s.cacheCfg != nil {
		cacheKey := redisclient.ListCacheKey("content", filter)
		ttl := time.Duration(s.cacheCfg.ContentListTTL) * time.Second
		cached := struct {
			Contents []model.ContentItem `json:"contents"`
			Total    int64               `json:"total"`
		}{Contents: contents, Total: total}
		redisclient.SetJSON(context.Background(), cacheKey, cached, ttl)
	}

	return contents, total, nil
}

func (s *ContentService) getHotContents(ctx context.Context, filter repository.ListContentsFilter) ([]model.ContentItem, error) {
	if s.rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}

	members, err := s.rdb.ZRevRange(ctx, "rank:hot:contents", 0, 199).Result()
	if err != nil || len(members) == 0 {
		return nil, err
	}

	contents := make([]model.ContentItem, 0, len(members))
	for _, member := range members {
		var id int64
		fmt.Sscanf(member, "%d", &id)
		content, err := s.GetContent(id)
		if err != nil || content == nil {
			continue
		}
		if filter.Zone != "" && content.Zone != filter.Zone {
			continue
		}
		contents = append(contents, *content)
	}

	return contents, nil
}

func (s *ContentService) UpdateContent(id int64, authorID int64, updates map[string]interface{}) error {
	content, err := s.contentRepo.FindByID(id)
	if err != nil || content == nil {
		return ErrContentNotFound
	}
	if content.AuthorID != authorID {
		return ErrContentForbidden
	}
	if err := s.contentRepo.UpdateContent(id, updates); err != nil {
		return err
	}

	s.invalidateContentCache(id)
	s.invalidateContentListCache()

	return nil
}

func (s *ContentService) DeleteContent(id int64, authorID int64) error {
	content, err := s.contentRepo.FindByID(id)
	if err != nil || content == nil {
		return ErrContentNotFound
	}
	if content.AuthorID != authorID {
		return ErrContentForbidden
	}
	if err := s.contentRepo.DeleteContent(id); err != nil {
		return err
	}

	s.invalidateContentCache(id)
	s.invalidateContentListCache()

	return nil
}

func (s *ContentService) IncrViewCount(id int64) {
	if s.rdb != nil {
		ctx := context.Background()
		contentKey := fmt.Sprintf("view_count:%d", id)
		s.rdb.Incr(ctx, contentKey)
		s.rdb.ZIncrBy(ctx, "rank:hot:contents", 1, fmt.Sprintf("%d", id))
	}
}

func (s *ContentService) incrementHotRank(contentID int64, ipID *int64, delta float64) {
	if s.rdb == nil {
		return
	}
	ctx := context.Background()
	s.rdb.ZIncrBy(ctx, "rank:hot:contents", delta, fmt.Sprintf("%d", contentID))
	if ipID != nil {
		s.rdb.ZIncrBy(ctx, "rank:hot:ips", delta, fmt.Sprintf("%d", *ipID))
	}
}

func (s *ContentService) FlushViewCounts(ctx context.Context) error {
	if s.rdb == nil {
		return nil
	}

	var cursor uint64
	for {
		keys, nextCursor, err := s.rdb.Scan(ctx, cursor, "view_count:*", 100).Result()
		if err != nil {
			return err
		}

		for _, key := range keys {
			val, err := s.rdb.GetDel(ctx, key).Result()
			if err != nil {
				continue
			}
			count, err := strconv.ParseInt(val, 10, 64)
			if err != nil || count <= 0 {
				continue
			}

			var id int64
			fmt.Sscanf(key, "view_count:%d", &id)

			s.contentRepo.IncrViewCountBy(id, count)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

func (s *ContentService) invalidateContentCache(id int64) {
	if s.rdb == nil {
		return
	}
	ctx := context.Background()
	s.rdb.Del(ctx, fmt.Sprintf("cache:content:%d", id))
}

func (s *ContentService) invalidateContentListCache() {
	if s.rdb == nil {
		return
	}
	redisclient.DeleteByPattern(context.Background(), "cache:content:list:*")
}
