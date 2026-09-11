package rageval

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"omnicraft/backend/internal/repository"
	ragservice "omnicraft/backend/internal/service/rag"
)

// ProductionKeywordAdapter measures the existing keyword-only retrieval path
// (tsvector + ILIKE + content visibility scope) via
// repository.SearchRepository.SearchContents. Retrieved rows carry the
// latest content version so citation identity resolves fully.
type ProductionKeywordAdapter struct {
	Search *repository.SearchRepository
	DB     *gorm.DB
}

// Retrieve implements KeywordRetriever for the production keyword path.
func (a *ProductionKeywordAdapter) Retrieve(ctx context.Context, query string, viewerID int64, topK int) ([]Retrieved, error) {
	results, _, err := a.Search.SearchContents(query, "", "", "", nil, "", "all", 1, topK, viewerID)
	if err != nil {
		return nil, fmt.Errorf("keyword search: %w", err)
	}
	hits := make([]Retrieved, 0, len(results))
	for _, r := range results {
		hits = append(hits, Retrieved{ContentID: r.ID, Score: r.Score})
	}
	enrichVersions(ctx, a.DB, hits)
	return hits, nil
}

// ProductionVectorAdapter measures the existing vector-only retrieval path
// (raw pgvector cosine over content_embeddings, no visibility predicates)
// via repository.EmbeddingRepository.VectorSearch. Query embeddings come
// from the local-dev deterministic standin (no provider credentials in local
// development mode); the artifact environment marks this explicitly.
type ProductionVectorAdapter struct {
	Embeddings    *repository.EmbeddingRepository
	DB            *gorm.DB
	Dims          int
	QueryEmbedder ragservice.QueryEmbedder
}

// Retrieve implements Retriever for the production pgvector path. viewerID is
// deliberately ignored: the existing VectorSearch has no visibility scope, so
// the baseline measures the raw as-is behaviour (leak count included).
func (a *ProductionVectorAdapter) Retrieve(ctx context.Context, query string, viewerID int64, topK int) ([]Retrieved, error) {
	embedding := DeterministicEmbedding(query, a.Dims)
	if a.QueryEmbedder != nil {
		var err error
		embedding, err = a.QueryEmbedder.GetEmbedding(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("query embedding: %w", err)
		}
	}
	results, err := a.Embeddings.VectorSearch(embedding, topK)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	hits := make([]Retrieved, 0, len(results))
	for _, r := range results {
		hits = append(hits, Retrieved{ContentID: r.ContentItemID, Score: r.Score})
	}
	enrichVersions(ctx, a.DB, hits)
	return hits, nil
}

// enrichVersions fills ContentVersion with the latest version number of each
// content row (0 stays unknown when no version exists). Version lookups are
// best-effort: a missing row simply leaves the version unknown.
func enrichVersions(ctx context.Context, db *gorm.DB, hits []Retrieved) {
	if db == nil || len(hits) == 0 {
		return
	}
	ids := make([]int64, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ContentID)
	}
	var versions []struct {
		ContentItemID int64
		VersionNumber int64
	}
	if err := db.WithContext(ctx).
		Table("content_versions").
		Select("content_item_id, version_number").
		Where("is_latest = ? AND content_item_id IN ?", true, ids).
		Scan(&versions).Error; err != nil {
		return
	}
	byID := make(map[int64]int64, len(versions))
	for _, v := range versions {
		byID[v.ContentItemID] = v.VersionNumber
	}
	for i := range hits {
		hits[i].ContentVersion = byID[hits[i].ContentID]
	}
}
