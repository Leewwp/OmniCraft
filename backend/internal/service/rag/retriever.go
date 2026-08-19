package rag

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"omnicraft/backend/config"
)

var ErrRetrievalUnavailable = errors.New("retrieval unavailable")

const (
	RetrievalSourceKeyword = "bm25"
	RetrievalSourceVector  = "vector"
	RetrievalSourceHybrid  = "hybrid_rrf"

	RetrievalDegradedKeywordPG   = "keyword_pg"
	RetrievalDegradedKeywordOnly = "keyword_only"
	RetrievalDegradedVectorOnly  = "vector_only"
)

// RetrievalCandidate is the server-owned chunk identity returned by either
// retrieval projection. Scores remain internal and are deliberately absent
// from this contract.
type RetrievalCandidate struct {
	ChunkKey        string
	ContentID       int64
	ContentVersion  int
	ChunkIndex      int
	ChunkingVersion int
	IndexVersion    int
	EmbeddingModel  string
	Title           string
	Heading         string
	Text            string
	SourceStart     int
	SourceEnd       int
	Zone            string
	ContentType     string
	Category        *string
	IP              *int64
	Tags            []string
	Source          string

	rrfScore float64
}

type KeywordRetriever interface {
	Search(ctx context.Context, query string, topK int, viewerID int64) ([]RetrievalCandidate, error)
}

type VectorRetriever interface {
	Search(ctx context.Context, embedding []float32, topK int, viewerID int64) ([]RetrievalCandidate, error)
}

type QueryEmbedder interface {
	GetEmbedding(ctx context.Context, query string) ([]float32, error)
}

// VisibilityFilter is the final viewer-aware database check. Implementations
// must return only candidates that still match published/current chunk truth.
type VisibilityFilter interface {
	FilterVisible(ctx context.Context, viewerID int64, candidates []RetrievalCandidate) ([]RetrievalCandidate, error)
}

type RetrievalResult struct {
	Candidates []RetrievalCandidate
	Degraded   string
}

type HybridRetriever struct {
	openSearch KeywordRetriever
	postgres   KeywordRetriever
	vector     VectorRetriever
	embedder   QueryEmbedder
	visibility VisibilityFilter
	config     config.RAGHybridConfig
}

func NewHybridRetriever(openSearch, postgres KeywordRetriever, vector VectorRetriever, embedder QueryEmbedder, visibility VisibilityFilter, cfg config.RAGHybridConfig) *HybridRetriever {
	return &HybridRetriever{
		openSearch: openSearch,
		postgres:   postgres,
		vector:     vector,
		embedder:   embedder,
		visibility: visibility,
		config:     cfg,
	}
}

func (r *HybridRetriever) Retrieve(ctx context.Context, query string, viewerID int64) (RetrievalResult, error) {
	if r.visibility == nil {
		return RetrievalResult{}, ErrRetrievalUnavailable
	}
	keyword, keywordErr := r.searchKeyword(ctx, query)
	degraded := ""
	if keywordErr != nil && r.postgres != nil {
		keyword, keywordErr = r.postgres.Search(ctx, query, r.topK(r.config.BM25TopK, config.RAGDefaultBM25TopK), viewerID)
		if keywordErr == nil {
			degraded = RetrievalDegradedKeywordPG
		}
	}

	vector, vectorErr := r.searchVector(ctx, query, viewerID)
	if vectorErr != nil {
		if degraded == "" {
			degraded = RetrievalDegradedKeywordOnly
		}
	}

	if keywordErr != nil && vectorErr != nil {
		return RetrievalResult{}, ErrRetrievalUnavailable
	}
	if keywordErr != nil && vectorErr == nil && len(vector) > 0 {
		degraded = RetrievalDegradedVectorOnly
	}

	var ranked []RetrievalCandidate
	switch {
	case len(keyword) > 0 && len(vector) > 0:
		ranked = FuseRRF(keyword, vector, r.topK(r.config.RRFK, config.RAGDefaultRRFK))
	case len(keyword) > 0:
		ranked = markSource(keyword, RetrievalSourceKeyword)
	case len(vector) > 0:
		ranked = markSource(vector, RetrievalSourceVector)
	}

	finalTopK := r.topK(r.config.FinalTopK, config.RAGDefaultFinalTopK)
	if len(ranked) > 0 {
		var err error
		ranked, err = r.filterVisibleWithBackfill(ctx, viewerID, ranked, finalTopK)
		if err != nil {
			return RetrievalResult{}, ErrRetrievalUnavailable
		}
	}
	if len(ranked) > finalTopK {
		ranked = ranked[:finalTopK]
	}
	return RetrievalResult{Candidates: ranked, Degraded: degraded}, nil
}

func (r *HybridRetriever) filterVisibleWithBackfill(ctx context.Context, viewerID int64, ranked []RetrievalCandidate, target int) ([]RetrievalCandidate, error) {
	checkCount := len(ranked)
	if checkCount > 20 {
		checkCount = 20
	}
	visible, err := r.visibility.FilterVisible(ctx, viewerID, ranked[:checkCount])
	if err != nil {
		return nil, err
	}
	visibleByKey := make(map[string]RetrievalCandidate, len(visible))
	for _, candidate := range visible {
		visibleByKey[candidate.ChunkKey] = candidate
	}
	filtered := make([]RetrievalCandidate, 0, target)
	for _, candidate := range ranked[:checkCount] {
		if checked, ok := visibleByKey[candidate.ChunkKey]; ok {
			filtered = append(filtered, checked)
		}
	}
	for _, candidate := range ranked[checkCount:] {
		if len(filtered) >= target {
			break
		}
		visible, err := r.visibility.FilterVisible(ctx, viewerID, []RetrievalCandidate{candidate})
		if err != nil {
			return nil, err
		}
		if len(visible) == 1 {
			filtered = append(filtered, visible[0])
		}
	}
	return filtered, nil
}

func (r *HybridRetriever) searchKeyword(ctx context.Context, query string) ([]RetrievalCandidate, error) {
	if r.openSearch == nil {
		return nil, ErrRetrievalUnavailable
	}
	return r.openSearch.Search(ctx, query, r.topK(r.config.BM25TopK, config.RAGDefaultBM25TopK), 0)
}

func (r *HybridRetriever) searchVector(ctx context.Context, query string, viewerID int64) ([]RetrievalCandidate, error) {
	if r.vector == nil || r.embedder == nil {
		return nil, ErrRetrievalUnavailable
	}
	embedding, err := r.embedder.GetEmbedding(ctx, query)
	if err != nil {
		return nil, err
	}
	return r.vector.Search(ctx, embedding, r.topK(r.config.VectorTopK, config.RAGDefaultVectorTopK), viewerID)
}

func (r *HybridRetriever) topK(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func markSource(candidates []RetrievalCandidate, source string) []RetrievalCandidate {
	marked := append([]RetrievalCandidate(nil), candidates...)
	for i := range marked {
		marked[i].Source = source
	}
	return marked
}

// FuseRRF merges candidates by their stable chunk key. A candidate present in
// both lists receives both reciprocal-rank contributions.
func FuseRRF(keyword, vector []RetrievalCandidate, k int) []RetrievalCandidate {
	if k <= 0 {
		k = config.RAGDefaultRRFK
	}
	merged := make(map[string]RetrievalCandidate, len(keyword)+len(vector))
	add := func(candidates []RetrievalCandidate, source string) {
		for rank, candidate := range candidates {
			candidate.ChunkKey = stableChunkKey(candidate)
			current, ok := merged[candidate.ChunkKey]
			if !ok {
				candidate.Source = source
				candidate.rrfScore = 1 / float64(k+rank+1)
				merged[candidate.ChunkKey] = candidate
				continue
			}
			current.rrfScore += 1 / float64(k+rank+1)
			current.Source = RetrievalSourceHybrid
			merged[candidate.ChunkKey] = current
		}
	}
	add(keyword, RetrievalSourceKeyword)
	add(vector, RetrievalSourceVector)

	result := make([]RetrievalCandidate, 0, len(merged))
	for _, candidate := range merged {
		result = append(result, candidate)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].rrfScore == result[j].rrfScore {
			return result[i].ChunkKey < result[j].ChunkKey
		}
		return result[i].rrfScore > result[j].rrfScore
	})
	return result
}

func stableChunkKey(candidate RetrievalCandidate) string {
	if candidate.ChunkKey != "" {
		return candidate.ChunkKey
	}
	return fmt.Sprintf("content:%d/version:%d/span:%d-%d", candidate.ContentID, candidate.ContentVersion, candidate.SourceStart, candidate.SourceEnd)
}
