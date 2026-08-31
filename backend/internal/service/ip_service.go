package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
)

var (
	ErrIPNotFound  = errors.New("ip not found")
	ErrIPSlugTaken = errors.New("slug already taken")
	ErrIPForbidden = errors.New("forbidden")
)

type IPService struct {
	ipRepo        *repository.IPRepository
	rdb           *redis.Client
	cacheCfg      *config.CacheConfig
	reviewSvc     *ReviewService
	queueProducer queue.Producer
}

func NewIPService(ipRepo *repository.IPRepository) *IPService {
	return &IPService{ipRepo: ipRepo}
}

func NewIPServiceWithCache(ipRepo *repository.IPRepository, rdb *redis.Client, cacheCfg *config.CacheConfig) *IPService {
	return &IPService{ipRepo: ipRepo, rdb: rdb, cacheCfg: cacheCfg}
}

func NewIPServiceWithReview(ipRepo *repository.IPRepository, rdb *redis.Client, cacheCfg *config.CacheConfig, reviewSvc *ReviewService) *IPService {
	return &IPService{ipRepo: ipRepo, rdb: rdb, cacheCfg: cacheCfg, reviewSvc: reviewSvc}
}

func (s *IPService) SetQueueProducer(p queue.Producer) {
	s.queueProducer = p
}

type CreateIPInput struct {
	Name        string   `json:"name" binding:"required,min=1,max=255"`
	Description string   `json:"description"`
	CoverURL    string   `json:"cover_url"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
}

// iptagMaxLen mirrors ip_tags.tag VARCHAR(50) (migration 005).
const iptagMaxLen = 50

// normalizeIPTags trims each tag, drops empties, truncates to the column
// length and removes duplicates (case-sensitive), preserving first-seen order.
func normalizeIPTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		if runes := []rune(trimmed); len(runes) > iptagMaxLen {
			trimmed = string(runes[:iptagMaxLen])
		}
		if _, dup := seen[trimmed]; dup {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func (s *IPService) CreateIP(ctx context.Context, input CreateIPInput, creatorID int64) (*model.IP, error) {
	slug := generateSlug(input.Name)

	existing, err := s.ipRepo.FindBySlug(slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		slug = fmt.Sprintf("%s-%d", slug, creatorID)
	}

	ip := &model.IP{
		Name:        input.Name,
		Slug:        slug,
		Description: input.Description,
		CoverURL:    input.CoverURL,
		Category:    input.Category,
		CreatorID:   &creatorID,
		Status:      "pending",
		Tags:        normalizeIPTags(input.Tags),
	}

	// The IP row and its ip_tags rows commit atomically; the search vector
	// trigger (trg_ip_tags_search_vector) fires inside the same transaction.
	err = s.ipRepo.Transaction(func(txRepo *repository.IPRepository) error {
		if err := txRepo.CreateIP(ip); err != nil {
			return err
		}
		tagRows := make([]model.IPTag, 0, len(ip.Tags))
		for _, tag := range ip.Tags {
			tagRows = append(tagRows, model.IPTag{IPID: ip.ID, Tag: tag})
		}
		return txRepo.CreateTags(tagRows)
	})
	if err != nil {
		return nil, err
	}

	s.invalidateIPListCache()
	s.submitIPForAIReview(ctx, ip, creatorID)

	return ip, nil
}

func (s *IPService) submitIPForAIReview(ctx context.Context, ip *model.IP, creatorID int64) {
	if s.reviewSvc == nil || ip == nil {
		return
	}
	reviewInput := ipReviewInput(ip, creatorID)
	detachedCtx := context.WithoutCancel(ctx)
	if _, ok := s.queueProducer.(*queue.NoopProducer); !ok && s.queueProducer != nil {
		recovery.GoSafe(func() {
			payload, _ := json.Marshal(map[string]interface{}{
				"action":          "submit_ai_review",
				"target_type":     reviewInput.TargetType,
				"target_id":       reviewInput.TargetID,
				"title":           reviewInput.Title,
				"description":     reviewInput.Description,
				"author_id":       reviewInput.AuthorID,
				"cover_image_url": reviewInput.CoverImageURL,
			})
			if err := s.queueProducer.Publish(detachedCtx, "ip.review", payload); err != nil {
				slog.Error("failed to publish ip.review message", "ip_id", ip.ID, "error", err)
			}
		})
	} else {
		recovery.GoSafe(func() {
			err := s.reviewSvc.SubmitForAIReview(detachedCtx, reviewInput)
			if err != nil && !errors.Is(err, aliyun.ErrGreenNotConfigured) {
				slog.Error("ip ai review failed", "ip_id", ip.ID, "error", err)
			}
		})
	}
}

// ipReviewInput assembles the review input for an IP. IPs stay on the pure
// text synchronous path: no attachments, no async video callback. The cover
// enters the image review input only (decision 6, B3).
func ipReviewInput(ip *model.IP, creatorID int64) SubmitReviewInput {
	return SubmitReviewInput{
		TargetType:    "ip",
		TargetID:      ip.ID,
		Title:         ip.Name,
		Description:   ip.Description,
		AuthorID:      creatorID,
		CoverImageURL: ip.CoverURL,
	}
}

func (s *IPService) GetIP(id int64) (*model.IP, error) {
	if s.rdb != nil && s.cacheCfg != nil {
		cacheKey := fmt.Sprintf("cache:ip:%d", id)
		var cached model.IP
		if hit, _ := redisclient.GetJSON(context.Background(), cacheKey, &cached); hit {
			return &cached, nil
		}
	}

	ip, err := s.ipRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if ip == nil {
		return nil, ErrIPNotFound
	}

	// Backfill the gorm:"-" Tags field from ip_tags; the detail cache stores
	// the assembled shape so hits and misses return the same contract.
	tagRows, err := s.ipRepo.GetTags(id)
	if err != nil {
		slog.Error("failed to get ip tags", "ip_id", id, "error", err)
	} else if len(tagRows) > 0 {
		ip.Tags = make([]string, 0, len(tagRows))
		for _, row := range tagRows {
			ip.Tags = append(ip.Tags, row.Tag)
		}
	}

	if s.rdb != nil && s.cacheCfg != nil {
		ttl := time.Duration(s.cacheCfg.IPDetailTTL) * time.Second
		redisclient.SetJSON(context.Background(), fmt.Sprintf("cache:ip:%d", id), ip, ttl)
	}

	return ip, nil
}

func (s *IPService) ListIPs(filter repository.ListIPsFilter) ([]model.IP, int64, error) {
	if s.rdb != nil && s.cacheCfg != nil {
		cacheKey := redisclient.ListCacheKey("ip", filter)
		var result struct {
			IPs   []model.IP `json:"ips"`
			Total int64      `json:"total"`
		}
		if hit, _ := redisclient.GetJSON(context.Background(), cacheKey, &result); hit {
			return result.IPs, result.Total, nil
		}
	}

	ips, total, err := s.ipRepo.ListIPs(filter)
	if err != nil {
		return nil, 0, err
	}

	if s.rdb != nil && s.cacheCfg != nil {
		cacheKey := redisclient.ListCacheKey("ip", filter)
		ttl := time.Duration(s.cacheCfg.IPListTTL) * time.Second
		cached := struct {
			IPs   []model.IP `json:"ips"`
			Total int64      `json:"total"`
		}{IPs: ips, Total: total}
		redisclient.SetJSON(context.Background(), cacheKey, cached, ttl)
	}

	return ips, total, nil
}

func (s *IPService) ApproveIP(id int64) error {
	ip, err := s.ipRepo.FindByID(id)
	if err != nil || ip == nil {
		return ErrIPNotFound
	}
	if err := s.ipRepo.UpdateStatus(id, "approved"); err != nil {
		return err
	}
	s.invalidateIPCache(id)
	s.invalidateIPListCache()
	return nil
}

func (s *IPService) RejectIP(id int64) error {
	ip, err := s.ipRepo.FindByID(id)
	if err != nil || ip == nil {
		return ErrIPNotFound
	}
	if err := s.ipRepo.UpdateStatus(id, "rejected"); err != nil {
		return err
	}
	s.invalidateIPCache(id)
	s.invalidateIPListCache()
	return nil
}

func (s *IPService) BanIP(id int64) error {
	if err := s.ipRepo.BanIPAndContents(id); err != nil {
		return err
	}
	s.invalidateIPCache(id)
	s.invalidateIPListCache()
	return nil
}

func (s *IPService) InvalidateIPCacheForAdmin(id int64) {
	s.invalidateIPCache(id)
	s.invalidateIPListCache()
}

func (s *IPService) invalidateIPCache(id int64) {
	if s.rdb == nil {
		return
	}
	ctx := context.Background()
	s.rdb.Del(ctx, fmt.Sprintf("cache:ip:%d", id))
}

func (s *IPService) invalidateIPListCache() {
	if s.rdb == nil {
		return
	}
	redisclient.DeleteByPattern(context.Background(), "cache:ip:list:*")
}

func (s *IPService) UpdateHotRank(ipID int64, increment float64) {
	if s.rdb == nil {
		return
	}
	ctx := context.Background()
	s.rdb.ZIncrBy(ctx, "rank:hot:ips", increment, fmt.Sprintf("%d", ipID))
}

type HotIPEntry struct {
	IP    model.IP `json:"ip"`
	Score float64  `json:"score"`
}

func (s *IPService) GetHotIPs(ctx context.Context, limit int) ([]model.IP, error) {
	if s.rdb == nil {
		ips, _, err := s.ipRepo.ListIPs(repository.ListIPsFilter{
			Sort:     "most_content",
			Page:     1,
			PageSize: limit,
		})
		return ips, err
	}

	members, err := s.rdb.ZRevRangeWithScores(ctx, "rank:hot:ips", 0, int64(limit-1)).Result()
	if err != nil || len(members) == 0 {
		ips, _, err := s.ipRepo.ListIPs(repository.ListIPsFilter{
			Sort:     "most_content",
			Page:     1,
			PageSize: limit,
		})
		return ips, err
	}

	ids := make([]int64, 0, len(members))
	for _, m := range members {
		var id int64
		fmt.Sscanf(m.Member.(string), "%d", &id)
		ids = append(ids, id)
	}

	return s.ipRepo.BatchGetByIDs(ids)
}

// Ensure json import is used for cache serialization
var _ = json.Marshal

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]`)
var multiDash = regexp.MustCompile(`-+`)

func generateSlug(name string) string {
	lower := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return '-'
		}
		return unicode.ToLower(r)
	}, name)

	slug := nonAlphanumeric.ReplaceAllString(lower, "")
	slug = multiDash.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if slug == "" {
		slug = fmt.Sprintf("ip-%d", len(name))
	}
	if len(slug) > 200 {
		slug = slug[:200]
	}
	return slug
}
