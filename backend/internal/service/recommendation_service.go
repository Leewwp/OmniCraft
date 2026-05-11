package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"omnicraft/backend/config"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type RecommendationService struct {
	db           *gorm.DB
	embeddingRepo *repository.EmbeddingRepository
	contentRepo   *repository.ContentRepository
	contentSvc    *ContentService
	rdb           *redis.Client
	cfg           *config.RecommendationConfig
}

func NewRecommendationService(
	db *gorm.DB,
	embeddingRepo *repository.EmbeddingRepository,
	contentRepo *repository.ContentRepository,
	contentSvc *ContentService,
	rdb *redis.Client,
	cfg *config.RecommendationConfig,
) *RecommendationService {
	return &RecommendationService{
		db:           db,
		embeddingRepo: embeddingRepo,
		contentRepo:   contentRepo,
		contentSvc:    contentSvc,
		rdb:           rdb,
		cfg:           cfg,
	}
}

func (s *RecommendationService) Recommend(ctx context.Context, userID int64, page, pageSize int) ([]ContentItemWithScore, int64, error) {
	if s.cfg == nil || !s.cfg.Enabled {
		return s.fallbackToHot(ctx, page, pageSize)
	}

	personalizationWeight := s.cfg.PersonalizationWeight
	if personalizationWeight <= 0 {
		personalizationWeight = 0.6
	}
	minInteraction := s.cfg.MinInteractionForPersonalize
	if minInteraction <= 0 {
		minInteraction = 10
	}
	topK := s.cfg.EmbeddingTopk
	if topK <= 0 {
		topK = 200
	}

	cacheKey := fmt.Sprintf("rec:original:%d:%d", userID, page)
	if s.rdb != nil {
		var cached struct {
			Items []ContentItemWithScore `json:"items"`
			Total int64                  `json:"total"`
		}
		if hit, _ := redisclient.GetJSON(ctx, cacheKey, &cached); hit {
			return cached.Items, cached.Total, nil
		}
	}

	if s.isColdStart(userID, minInteraction) {
		return s.fallbackToHot(ctx, page, pageSize)
	}

	profile, err := s.buildUserProfile(ctx, userID)
	if err != nil || len(profile) == 0 {
		log.Printf("[rec] profile build failed for user %d: %v, falling back to hot", userID, err)
		return s.fallbackToHot(ctx, page, pageSize)
	}

	candidates, err := s.embeddingRepo.VectorSearch(profile, topK*2)
	if err != nil {
		log.Printf("[rec] vector search failed: %v, falling back to hot", err)
		return s.fallbackToHot(ctx, page, pageSize)
	}

	ids := make([]int64, 0, len(candidates))
	candidateScores := make(map[int64]float64, len(candidates))
	for _, c := range candidates {
		ids = append(ids, c.ContentItemID)
		candidateScores[c.ContentItemID] = float64(c.Score)
	}

	scored, err := s.computeFinalScores(ctx, ids, candidateScores, personalizationWeight)
	if err != nil {
		log.Printf("[rec] score computation failed: %v", err)
		return s.fallbackToHot(ctx, page, pageSize)
	}

	total := int64(len(scored))
	start := (page - 1) * pageSize
	if start >= len(scored) {
		return []ContentItemWithScore{}, total, nil
	}
	end := start + pageSize
	if end > len(scored) {
		end = len(scored)
	}
	paged := scored[start:end]

	if s.rdb != nil && s.cfg.RefreshIntervalH > 0 {
		ttl := time.Duration(s.cfg.RefreshIntervalH) * time.Hour
		cached := struct {
			Items []ContentItemWithScore `json:"items"`
			Total int64                  `json:"total"`
		}{Items: paged, Total: total}
		if err := redisclient.SetJSON(ctx, cacheKey, cached, ttl); err != nil {
			log.Printf("[rec] cache write failed: %v", err)
		}
	}

	return paged, total, nil
}

type ContentItemWithScore struct {
	Item  ContentItemBrief `json:"item"`
	Score float64          `json:"score"`
}

type ContentItemBrief struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	CoverURL    string `json:"cover_image_url"`
	AuthorID    int64  `json:"author_id"`
	AuthorName  string `json:"author_name"`
	LikeCount   int    `json:"like_count"`
	ViewCount   int64  `json:"view_count"`
	ContentType string `json:"content_type"`
	Category    string `json:"category"`
	Zone        string `json:"zone"`
}

func (s *RecommendationService) isColdStart(userID int64, threshold int) bool {
	if s.db == nil {
		return true
	}
	var count int64
	s.db.Raw(`SELECT COUNT(*) FROM (
		SELECT content_item_id FROM browse_history WHERE user_id = ? LIMIT ?
		UNION
		SELECT content_item_id FROM favorites WHERE user_id = ? LIMIT ?
		UNION
		SELECT target_id FROM reactions WHERE user_id = ? AND target_type = 'content' AND reaction = 'like' LIMIT ?
	) t`, userID, threshold, userID, threshold, userID, threshold).Scan(&count)
	return count < int64(threshold)
}

func (s *RecommendationService) buildUserProfile(ctx context.Context, userID int64) ([]float32, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db not available")
	}

	type embeddingRow struct {
		Embedding string `gorm:"column:embedding"`
	}

	var rows []embeddingRow

	s.db.Raw(`
		SELECT ce.embedding::text as embedding
		FROM browse_history bh
		JOIN content_embeddings ce ON ce.content_item_id = bh.content_item_id
		WHERE bh.user_id = ?
		ORDER BY bh.viewed_at DESC
		LIMIT 50
	`, userID).Scan(&rows)

	favRows := []embeddingRow{}
	s.db.Raw(`
		SELECT ce.embedding::text as embedding
		FROM favorites f
		JOIN content_embeddings ce ON ce.content_item_id = f.content_item_id
		WHERE f.user_id = ?
		LIMIT 50
	`, userID).Scan(&favRows)

	likeRows := []embeddingRow{}
	s.db.Raw(`
		SELECT ce.embedding::text as embedding
		FROM reactions r
		JOIN content_embeddings ce ON ce.content_item_id = r.target_id
		WHERE r.user_id = ? AND r.target_type = 'content' AND r.reaction = 'like'
		LIMIT 50
	`, userID).Scan(&likeRows)

	if len(rows) == 0 && len(favRows) == 0 && len(likeRows) == 0 {
		return nil, fmt.Errorf("no interaction data for user %d", userID)
	}

	var sum []float64
	var totalWeight float64

	for _, r := range rows {
		vec := parseEmbedding(r.Embedding)
		if vec == nil {
			continue
		}
		if sum == nil {
			sum = make([]float64, len(vec))
		}
		for i, v := range vec {
			sum[i] += float64(v)
		}
		totalWeight += 1
	}

	for _, r := range favRows {
		vec := parseEmbedding(r.Embedding)
		if vec == nil || sum == nil {
			continue
		}
		for i, v := range vec {
			sum[i] += float64(v) * 2
		}
		totalWeight += 2
	}

	for _, r := range likeRows {
		vec := parseEmbedding(r.Embedding)
		if vec == nil || sum == nil {
			continue
		}
		for i, v := range vec {
			sum[i] += float64(v) * 1.5
		}
		totalWeight += 1.5
	}

	if totalWeight == 0 || sum == nil {
		return nil, fmt.Errorf("failed to build profile vector")
	}

	profile := make([]float32, len(sum))
	for i, v := range sum {
		profile[i] = float32(v / totalWeight)
	}

	return profile, nil
}

func (s *RecommendationService) computeFinalScores(ctx context.Context, contentIDs []int64, simScores map[int64]float64, alpha float64) ([]ContentItemWithScore, error) {
	hotScores := make(map[int64]float64)
	if s.rdb != nil {
		for _, id := range contentIDs {
			score, err := s.rdb.ZScore(ctx, "rank:hot:contents", fmt.Sprintf("%d", id)).Result()
			if err == nil {
				hotScores[id] = score
			}
		}
	}

	items := make([]ContentItemWithScore, 0, len(contentIDs))
	for _, id := range contentIDs {
		sim := simScores[id]
		hot := hotScores[id]

		final := alpha*sim + (1-alpha)*hot

		content, err := s.contentRepo.FindByID(id)
		if err != nil || content == nil || content.Status != "published" {
			continue
		}
		if content.Zone != "original" {
			continue
		}

		authorName := ""
		if content.Author.Username != "" {
			authorName = content.Author.Username
		}

		items = append(items, ContentItemWithScore{
			Item: ContentItemBrief{
				ID:          content.ID,
				Title:       content.Title,
				CoverURL:    content.CoverImageURL,
				AuthorID:    content.AuthorID,
				AuthorName:  authorName,
				LikeCount:   content.LikeCount,
				ViewCount:   content.ViewCount,
				ContentType: content.ContentType,
				Category:    content.Category,
				Zone:        content.Zone,
			},
			Score: final,
		})
	}

	sortByScore(items)
	return items, nil
}

func (s *RecommendationService) fallbackToHot(ctx context.Context, page, pageSize int) ([]ContentItemWithScore, int64, error) {
	filter := repository.ListContentsFilter{
		Zone:     "original",
		Sort:     "hot",
		Page:     page,
		PageSize: pageSize,
	}
	contents, total, err := s.contentSvc.ListContents(filter)
	if err != nil {
		return nil, 0, err
	}
	items := make([]ContentItemWithScore, 0, len(contents))
	for _, c := range contents {
		items = append(items, ContentItemWithScore{
			Item: ContentItemBrief{
				ID:          c.ID,
				Title:       c.Title,
				CoverURL:    c.CoverImageURL,
				AuthorID:    c.AuthorID,
				AuthorName:  c.Author.Username,
				LikeCount:   c.LikeCount,
				ViewCount:   c.ViewCount,
				ContentType: c.ContentType,
				Category:    c.Category,
				Zone:        c.Zone,
			},
		})
	}
	return items, total, nil
}

func (s *RecommendationService) RecommendForAnonymous(ctx context.Context, page, pageSize int) ([]ContentItemWithScore, int64, error) {
	return s.fallbackToHot(ctx, page, pageSize)
}

func sortByScore(items []ContentItemWithScore) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Score > items[i].Score {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func parseEmbedding(s string) []float32 {
	if s == "" || s[0] != '[' {
		return nil
	}
	var vals []float32
	num := float32(0)
	sign := float32(1)
	inNum := false
	afterDecimal := false
	decDiv := float32(1)
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			inNum = true
			if afterDecimal {
				decDiv *= 10
				num += float32(c-'0') / decDiv
			} else {
				num = num*10 + float32(c-'0')
			}
		case c == '.':
			afterDecimal = true
		case c == '-':
			sign = -1
		case c == ',' || c == ']':
			if inNum {
				vals = append(vals, sign*num)
			}
			num = 0
			sign = 1
			inNum = false
			afterDecimal = false
			decDiv = 1
		case c == ' ' || c == 'e' || c == 'E':
			continue
		}
	}
	return vals
}

func EstimateEmbeddingDim(db *gorm.DB) int {
	var dim int
	db.Raw("SELECT COALESCE((SELECT vector_dims(embedding) FROM content_embeddings LIMIT 1), 0)").Scan(&dim)
	return dim
}

var _ = math.Log10
