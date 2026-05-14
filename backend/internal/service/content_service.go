package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/recovery"
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
	ossSvc      *OSSService
	recSvc      *RecommendationService
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

func NewContentServiceWithOSS(contentRepo *repository.ContentRepository, reviewSvc *ReviewService, rdb *redis.Client, cacheCfg *config.CacheConfig, ossSvc *OSSService) *ContentService {
	return &ContentService{contentRepo: contentRepo, reviewSvc: reviewSvc, rdb: rdb, cacheCfg: cacheCfg, ossSvc: ossSvc}
}

func (s *ContentService) SetRecommendationService(recSvc *RecommendationService) {
	s.recSvc = recSvc
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
		if err := s.reviewSvc.SubmitForAIReview(context.Background(), reviewInput); err != nil && !errors.Is(err, aliyun.ErrGreenNotConfigured) {
			return nil, err
		}
	}

	if input.ContentType == "video" && content.CoverImageURL == "" {
		s.triggerVideoSnapshot(content.ID, input.Attachments)
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

func (s *ContentService) ListContents(filter repository.ListContentsFilter, viewerID int64) ([]model.ContentItem, int64, error) {
	if filter.Sort == "recommended" && filter.Zone == "original" {
		return s.handleRecommended(filter, viewerID)
	}
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

func (s *ContentService) handleRecommended(filter repository.ListContentsFilter, viewerID int64) ([]model.ContentItem, int64, error) {
	if s.recSvc == nil {
		filter.Sort = "hot"
		return s.ListContents(filter, viewerID)
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	ctx := context.Background()

	if viewerID > 0 {
		items, total, err := s.recSvc.Recommend(ctx, viewerID, page, pageSize)
		if err != nil {
			filter.Sort = "hot"
			return s.ListContents(filter, viewerID)
		}
		return scoredToContentItems(items), total, nil
	}

	items, total, err := s.recSvc.RecommendForAnonymous(ctx, page, pageSize)
	if err != nil {
		filter.Sort = "hot"
		return s.ListContents(filter, viewerID)
	}
	return scoredToContentItems(items), total, nil
}

func scoredToContentItems(scored []ContentItemWithScore) []model.ContentItem {
	items := make([]model.ContentItem, 0, len(scored))
	for _, s := range scored {
		items = append(items, model.ContentItem{
			ID:            s.Item.ID,
			Title:         s.Item.Title,
			CoverImageURL: s.Item.CoverURL,
			AuthorID:      s.Item.AuthorID,
			Author:        model.User{Username: s.Item.AuthorName},
			LikeCount:     s.Item.LikeCount,
			ViewCount:     s.Item.ViewCount,
			ContentType:   s.Item.ContentType,
			Category:      s.Item.Category,
			Zone:          s.Item.Zone,
		})
	}
	return items
}

func (s *ContentService) getHotContents(ctx context.Context, filter repository.ListContentsFilter) ([]model.ContentItem, error) {
	if s.rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}

	members, err := s.rdb.ZRevRange(ctx, "rank:hot:contents", 0, 199).Result()
	if err != nil || len(members) == 0 {
		return nil, err
	}

	ids := make([]int64, 0, len(members))
	for _, member := range members {
		var id int64
		fmt.Sscanf(member, "%d", &id)
		ids = append(ids, id)
	}

	allContents, err := s.contentRepo.BatchGetByIDs(ids)
	if err != nil {
		return nil, err
	}

	contentMap := make(map[int64]model.ContentItem, len(allContents))
	for _, c := range allContents {
		contentMap[c.ID] = c
	}

	contents := make([]model.ContentItem, 0, len(members))
	for _, id := range ids {
		content, ok := contentMap[id]
		if !ok {
			continue
		}
		if filter.Zone != "" && content.Zone != filter.Zone {
			continue
		}
		contents = append(contents, content)
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
	if ipID != nil && *ipID != 0 {
		s.rdb.ZIncrBy(ctx, "rank:hot:ips", delta, fmt.Sprintf("%d", *ipID))
	}
}

func (s *ContentService) UpdateHotRank(ctx context.Context, trendingWindowDays int, hotDecayHours float64) error {
	if s.rdb == nil || s.contentRepo == nil {
		return fmt.Errorf("redis or repo not available")
	}

	since := time.Now().AddDate(0, 0, -trendingWindowDays)
	contents, _, err := s.contentRepo.ListContents(repository.ListContentsFilter{
		Zone:     "original",
		Status:   "published",
		TimeRange: "all",
		Page:     1,
		PageSize: 10000,
	})
	if err != nil {
		return fmt.Errorf("failed to list contents: %w", err)
	}

	pipe := s.rdb.Pipeline()
	for _, c := range contents {
		if c.CreatedAt.Before(since) {
			continue
		}
		ageHours := time.Since(c.CreatedAt).Hours()
		hotScore := computeHotScore(float64(c.ViewCount), float64(c.LikeCount), ageHours, hotDecayHours)
		pipe.ZAdd(ctx, "rank:hot:contents", redis.Z{Score: hotScore, Member: fmt.Sprintf("%d", c.ID)})
	}
	_, err = pipe.Exec(ctx)
	return err
}

func computeHotScore(views, likes, ageHours, decayHours float64) float64 {
	if decayHours <= 0 {
		decayHours = 48
	}
	popularity := 1 + views + likes*3
	if popularity < 1 {
		popularity = 1
	}
	return math.Log10(popularity) * timeDecay(ageHours, decayHours)
}


func timeDecay(ageHours, halfLifeHours float64) float64 {
	exponent := -ageHours / halfLifeHours
	return math.Pow(2, exponent)
}

func (s *ContentService) FlushViewCounts(ctx context.Context) error {
	if s.rdb == nil {
		return nil
	}

	var cursor uint64
	batch := make(map[int64]int64)

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
			batch[id] += count
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if len(batch) > 0 {
		return s.contentRepo.BatchIncrViewCounts(batch)
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

func (s *ContentService) triggerVideoSnapshot(contentID int64, attachments []AttachmentInput) {
	if s.ossSvc == nil {
		return
	}

	var videoKey string
	for _, a := range attachments {
		if a.FileType == "video" && a.OSSKey != "" {
			videoKey = a.OSSKey
			break
		}
	}
	if videoKey == "" {
		return
	}

	recovery.GoSafe(func() {
		ctx := context.Background()
		snapshotURL, err := s.ossSvc.GenerateVideoSnapshotURL(ctx, videoKey)
		if err != nil {
			slog.Error("video snapshot failed", "content_id", contentID, "error", err)
			return
		}
		if err := s.contentRepo.UpdateContent(contentID, map[string]interface{}{
			"cover_image_url": snapshotURL,
		}); err != nil {
			slog.Error("failed to update cover_image_url", "content_id", contentID, "error", err)
			return
		}
		s.invalidateContentCache(contentID)
	})
}
