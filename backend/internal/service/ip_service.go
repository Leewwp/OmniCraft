package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
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
	ipRepo    *repository.IPRepository
	rdb       *redis.Client
	cacheCfg  *config.CacheConfig
	reviewSvc *ReviewService
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

type CreateIPInput struct {
	Name        string   `json:"name" binding:"required,min=1,max=255"`
	Description string   `json:"description"`
	CoverURL    string   `json:"cover_url"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
}

func (s *IPService) CreateIP(input CreateIPInput, creatorID int64) (*model.IP, error) {
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
	}

	if err := s.ipRepo.CreateIP(ip); err != nil {
		return nil, err
	}

	s.invalidateIPListCache()
	s.submitIPForAIReview(ip, creatorID)

	return ip, nil
}

func (s *IPService) submitIPForAIReview(ip *model.IP, creatorID int64) {
	if s.reviewSvc == nil || ip == nil {
		return
	}
	go func() {
		err := s.reviewSvc.SubmitForAIReview(context.Background(), SubmitReviewInput{
			TargetType:  "ip",
			TargetID:    ip.ID,
			Title:       ip.Name,
			Description: ip.Description,
			AuthorID:    creatorID,
		})
		if err != nil && !errors.Is(err, aliyun.ErrGreenNotConfigured) {
			log.Printf("ip ai review failed for ip_id=%d: %v", ip.ID, err)
		}
	}()
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

	ips := make([]model.IP, 0, len(members))
	for _, m := range members {
		var id int64
		fmt.Sscanf(m.Member.(string), "%d", &id)
		ip, err := s.GetIP(id)
		if err != nil || ip == nil {
			continue
		}
		ips = append(ips, *ip)
	}
	return ips, nil
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
