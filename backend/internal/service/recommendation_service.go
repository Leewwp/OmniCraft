package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type RecommendationService struct {
	db            *gorm.DB
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
		db:            db,
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
	multiplier := s.cfg.EmbeddingMultiplier
	if multiplier <= 0 {
		multiplier = 2
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

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	if s.isColdStart(userID, minInteraction) {
		return s.fallbackToHot(ctx, page, pageSize)
	}

	profile, err := s.buildUserProfile(ctx, userID)
	if err != nil || len(profile) == 0 {
		slog.Error("[rec] profile build failed, falling back to hot", "user_id", userID, "error", err)
		return s.fallbackToHot(ctx, page, pageSize)
	}

	candidates, err := s.embeddingRepo.VectorSearch(profile, topK*multiplier)
	if err != nil {
		slog.Error("[rec] vector search failed, falling back to hot", "error", err)
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
		slog.Error("[rec] score computation failed", "error", err)
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
			slog.Error("[rec] cache write failed", "error", err)
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
	CoverWidth  *int   `json:"cover_width,omitempty"`
	CoverHeight *int   `json:"cover_height,omitempty"`
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
		SELECT content_item_id FROM (
			SELECT content_item_id FROM browse_history WHERE user_id = ?
			UNION
			SELECT ci.content_item_id
			FROM collection_items ci
			JOIN collections c ON c.id = ci.collection_id
			WHERE c.user_id = ? AND c.deleted_at IS NULL
			UNION
			SELECT target_id AS content_item_id FROM reactions WHERE user_id = ? AND target_type = 'content' AND reaction = 'like'
		) interactions
		LIMIT ?
	) t`, userID, userID, userID, threshold).Scan(&count)
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
	if err := s.db.Raw(`
		SELECT CAST(ce.embedding AS text) AS embedding
		FROM browse_history bh
		JOIN content_embeddings ce ON ce.content_item_id = bh.content_item_id
		WHERE bh.user_id = ?
		ORDER BY bh.viewed_at DESC
		LIMIT 50
	`, userID).Scan(&rows).Error; err != nil {
		slog.Error("[rec] browse history query failed", "error", err)
	}

	// #74: 收藏信号只来自收藏成员关系（活动收藏集），不再读取旧 favorites 表。
	var favRows []embeddingRow
	if err := s.db.Raw(`
		SELECT CAST(ce.embedding AS text) AS embedding
		FROM collection_items ci
		JOIN collections c ON c.id = ci.collection_id
		JOIN content_embeddings ce ON ce.content_item_id = ci.content_item_id
		WHERE c.user_id = ? AND c.deleted_at IS NULL
		LIMIT 50
	`, userID).Scan(&favRows).Error; err != nil {
		slog.Error("[rec] collection membership query failed", "error", err)
	}

	var likeRows []embeddingRow
	if err := s.db.Raw(`
		SELECT CAST(ce.embedding AS text) AS embedding
		FROM reactions r
		JOIN content_embeddings ce ON ce.content_item_id = r.target_id
		WHERE r.user_id = ? AND r.target_type = 'content' AND r.reaction = 'like'
		LIMIT 50
	`, userID).Scan(&likeRows).Error; err != nil {
		slog.Error("[rec] reactions query failed", "error", err)
	}

	if len(rows) == 0 && len(favRows) == 0 && len(likeRows) == 0 {
		return nil, fmt.Errorf("no interaction data for user %d", userID)
	}

	var sum []float64
	var totalWeight float64

	accumulate := func(r embeddingRow, weight float64) {
		vec := parseEmbeddingJSON(r.Embedding)
		if vec == nil {
			return
		}
		if sum == nil {
			sum = make([]float64, len(vec))
		}
		if len(vec) != len(sum) {
			slog.Warn("[rec] embedding dimension mismatch", "expected", len(sum), "got", len(vec))
			return
		}
		for i, v := range vec {
			sum[i] += float64(v) * weight
		}
		totalWeight += weight
	}

	for _, r := range rows {
		accumulate(r, 1.0)
	}
	for _, r := range favRows {
		accumulate(r, 2.0)
	}
	for _, r := range likeRows {
		accumulate(r, 1.5)
	}

	if totalWeight == 0 || sum == nil {
		return nil, fmt.Errorf("failed to build profile vector")
	}

	profile := make([]float32, len(sum))
	for i, v := range sum {
		profile[i] = float32(v / totalWeight)
	}

	var norm float64
	for _, v := range profile {
		norm += float64(v * v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range profile {
			profile[i] = float32(float64(profile[i]) / norm)
		}
	}

	return profile, nil
}

func (s *RecommendationService) computeFinalScores(ctx context.Context, contentIDs []int64, simScores map[int64]float64, alpha float64) ([]ContentItemWithScore, error) {
	hotScores := make(map[int64]float64, len(contentIDs))
	if s.rdb != nil && len(contentIDs) > 0 {
		pipe := s.rdb.Pipeline()
		cmds := make([]*redis.FloatCmd, len(contentIDs))
		for i, id := range contentIDs {
			cmds[i] = pipe.ZScore(ctx, "rank:hot:contents", fmt.Sprintf("%d", id))
		}
		_, _ = pipe.Exec(ctx)
		for i, id := range contentIDs {
			score, err := cmds[i].Result()
			if err == nil {
				hotScores[id] = score
			}
		}
	}

	contents, err := s.batchFetchContents(contentIDs)
	if err != nil {
		return nil, fmt.Errorf("batch fetch failed: %w", err)
	}

	items := make([]ContentItemWithScore, 0, len(contents))
	for _, content := range contents {
		if content.Status != "published" || content.Zone != "original" {
			continue
		}

		sim := simScores[content.ID]
		hot := hotScores[content.ID]
		final := alpha*sim + (1-alpha)*hot

		authorName := content.Author.Username

		items = append(items, ContentItemWithScore{
			Item: ContentItemBrief{
				ID:          content.ID,
				Title:       content.Title,
				CoverURL:    content.CoverImageURL,
				CoverWidth:  content.CoverWidth,
				CoverHeight: content.CoverHeight,
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

	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})

	return items, nil
}

func (s *RecommendationService) batchFetchContents(ids []int64) ([]model.ContentItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var contents []model.ContentItem
	err := s.contentRepo.DB().
		Preload("Author").
		Where("id IN ?", ids).
		Find(&contents).Error
	return contents, err
}

func (s *RecommendationService) fallbackToHot(ctx context.Context, page, pageSize int) ([]ContentItemWithScore, int64, error) {
	filter := repository.ListContentsFilter{
		Zone:     "original",
		Sort:     "hot",
		Page:     page,
		PageSize: pageSize,
	}
	contents, total, err := s.contentSvc.ListContents(filter, 0)
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
				CoverWidth:  c.CoverWidth,
				CoverHeight: c.CoverHeight,
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

func (s *RecommendationService) InvalidateUserCache(ctx context.Context, userID int64) {
	if s.rdb == nil {
		return
	}
	redisclient.DeleteByPattern(ctx, fmt.Sprintf("rec:original:%d:*", userID))
}

func (s *RecommendationService) FillMissingEmbeddings(ctx context.Context, llmProvider interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}) error {
	if s.db == nil || s.embeddingRepo == nil {
		return fmt.Errorf("db or embedding repo not available")
	}

	var missing []struct {
		ID          int64  `gorm:"column:id"`
		Title       string `gorm:"column:title"`
		Description string `gorm:"column:description"`
	}
	s.db.Raw(`
		SELECT ci.id, ci.title, COALESCE(ci.description, '') AS description
		FROM content_items ci
		LEFT JOIN content_embeddings ce ON ce.content_item_id = ci.id
		WHERE ci.zone = 'original' AND ci.status = 'published' AND ce.content_item_id IS NULL
		LIMIT 50
	`).Scan(&missing)

	if len(missing) == 0 {
		return nil
	}

	slog.Info("[rec] filling missing content embeddings", "count", len(missing))
	for _, m := range missing {
		text := m.Title
		if m.Description != "" {
			text += " " + m.Description
		}
		embedding, err := llmProvider.GetEmbedding(ctx, text)
		if err != nil {
			slog.Error("[rec] embedding failed", "content_id", m.ID, "error", err)
			continue
		}
		if err := s.embeddingRepo.UpsertEmbedding(m.ID, embedding); err != nil {
			slog.Error("[rec] upsert embedding failed", "content_id", m.ID, "error", err)
		}
	}

	return nil
}

func parseEmbeddingJSON(s string) []float32 {
	if s == "" || s[0] != '[' {
		return nil
	}
	var vals []float64
	if err := json.Unmarshal([]byte(s), &vals); err != nil {
		return nil
	}
	result := make([]float32, len(vals))
	for i, v := range vals {
		result[i] = float32(v)
	}
	return result
}

func EstimateEmbeddingDim(db *gorm.DB) int {
	var dim int
	db.Raw("SELECT COALESCE((SELECT vector_dims(embedding) FROM content_embeddings LIMIT 1), 0)").Scan(&dim)
	return dim
}
