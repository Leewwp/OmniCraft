package rag

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/llm"
)

var ErrRetrievalUnavailable = errors.New("retrieval unavailable")

const (
	RetrievalSourceKeyword = "bm25"
	RetrievalSourceVector  = "vector"
	RetrievalSourceHybrid  = "hybrid_rrf"

	RetrievalDegradedKeywordPG        = "keyword_pg"
	RetrievalDegradedKeywordOnly      = "keyword_only"
	RetrievalDegradedVectorOnly       = "vector_only"
	RetrievalDegradedRerankUnavailable = "rerank_unavailable"
)

// defaultRerankInputTopK bounds how many RRF-fused candidates are handed to
// the reranker when no explicit input top-k is configured (spec: top-20).
const defaultRerankInputTopK = 20

// rerankDocumentTextRunes bounds the chunk text portion of a rerank document
// so one oversized chunk cannot dominate the rerank request payload.
const rerankDocumentTextRunes = 512

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
	// ExpandedQueries carries the query-expansion terms actually used (A-03);
	// empty when expansion is disabled or produced nothing. Display-only.
	ExpandedQueries []string
}

// QueryExpander turns one colloquial user query into additional retrieval
// terms. Implementations swallow their own errors and return nil/empty on
// failure: expansion is an enhancement, never a gate.
type QueryExpander interface {
	Expand(ctx context.Context, query string) []string
}

type HybridRetriever struct {
	openSearch KeywordRetriever
	postgres   KeywordRetriever
	vector     VectorRetriever
	embedder   QueryEmbedder
	visibility VisibilityFilter
	config     config.RAGHybridConfig
	expander   QueryExpander
	reranker   llm.Reranker
	rerankIn   int
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

// SetQueryExpander enables query expansion (A-03): each retrieval fans out to
// the original query plus the expanded terms on both the lexical and the
// vector path. A nil expander disables expansion.
func (r *HybridRetriever) SetQueryExpander(expander QueryExpander) {
	r.expander = expander
}

// SetReranker enables reranking (A-03): the RRF-fused top inputTopK candidates
// are reranked (query relevance) before the final top-k cut. A nil reranker
// disables reranking; inputTopK <= 0 falls back to defaultRerankInputTopK.
func (r *HybridRetriever) SetReranker(reranker llm.Reranker, inputTopK int) {
	r.reranker = reranker
	r.rerankIn = inputTopK
}

// Retrieve runs the hybrid pipeline over one user query (A-03: optionally
// expanded into several sibling queries). Degradation semantics aggregate
// across the sibling queries and keep the historical priorities: postgres
// keyword fallback < keyword-only/vector-only < total unavailability; a
// rerank failure degrades last and never masks retrieval degradation.
func (r *HybridRetriever) Retrieve(ctx context.Context, query string, viewerID int64) (RetrievalResult, error) {
	if r.visibility == nil {
		return RetrievalResult{}, ErrRetrievalUnavailable
	}
	query = strings.TrimSpace(query)
	queries := []string{query}
	expanded := []string(nil)
	if r.expander != nil && query != "" {
		expanded = r.expander.Expand(ctx, query)
		queries = append(queries, expanded...)
	}

	degraded := ""
	bm25TopK := r.topK(r.config.BM25TopK, config.RAGDefaultBM25TopK)
	keywordLists := make([][]RetrievalCandidate, 0, len(queries))
	keywordFailures := 0
	keywordPGFallback := false
	for _, q := range queries {
		keyword, keywordErr := r.searchKeyword(ctx, q)
		if keywordErr != nil && r.postgres != nil {
			keyword, keywordErr = r.postgres.Search(ctx, q, bm25TopK, viewerID)
			if keywordErr == nil {
				keywordPGFallback = true
			}
		}
		if keywordErr != nil {
			keywordFailures++
			continue
		}
		if len(keyword) > 0 {
			keywordLists = append(keywordLists, keyword)
		}
	}
	// The keyword side counts as failed only when every sibling query failed
	// (partial expansion failures keep the surviving lists in play).
	keywordSideFailed := len(queries) > 0 && keywordFailures == len(queries)
	if keywordPGFallback {
		degraded = RetrievalDegradedKeywordPG
	}

	vectorLists, vectorErr := r.searchVectors(ctx, queries, viewerID)
	if vectorErr != nil {
		if degraded == "" {
			degraded = RetrievalDegradedKeywordOnly
		}
	}

	if keywordSideFailed && vectorErr != nil {
		return RetrievalResult{}, ErrRetrievalUnavailable
	}
	if keywordSideFailed && vectorErr == nil && len(vectorLists) > 0 {
		degraded = RetrievalDegradedVectorOnly
	}

	ranked := r.fuseLists(keywordLists, vectorLists)
	if r.reranker != nil && len(ranked) > 1 {
		inputTopK := r.rerankIn
		if inputTopK <= 0 {
			inputTopK = defaultRerankInputTopK
		}
		pool := ranked
		if len(pool) > inputTopK {
			pool = pool[:inputTopK]
		}
		ordered, err := r.rerankCandidates(ctx, query, pool)
		if err == nil && len(ordered) > 0 {
			ranked = ordered
		} else if err != nil && degraded == "" {
			degraded = RetrievalDegradedRerankUnavailable
		}
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
	return RetrievalResult{Candidates: ranked, Degraded: degraded, ExpandedQueries: expanded}, nil
}

// fuseLists marks and fuses the per-query keyword/vector lists. A single
// non-empty list keeps its pure source (historical contract); multiple lists
// go through multi-list RRF.
func (r *HybridRetriever) fuseLists(keywordLists, vectorLists [][]RetrievalCandidate) []RetrievalCandidate {
	lists := make([]rankedList, 0, len(keywordLists)+len(vectorLists))
	for _, l := range keywordLists {
		if len(l) > 0 {
			lists = append(lists, rankedList{candidates: l, source: RetrievalSourceKeyword})
		}
	}
	for _, l := range vectorLists {
		if len(l) > 0 {
			lists = append(lists, rankedList{candidates: l, source: RetrievalSourceVector})
		}
	}
	switch len(lists) {
	case 0:
		return nil
	case 1:
		return markSource(lists[0].candidates, lists[0].source)
	default:
		return FuseRRFLists(lists, r.topK(r.config.RRFK, config.RAGDefaultRRFK))
	}
}

// searchVectors embeds every query and runs one vector search per query. All
// sibling queries are embedded in a single batch call when the embedder
// supports it (DashScope v4 batch); a single query keeps the historical
// single-text embedding wire format.
func (r *HybridRetriever) searchVectors(ctx context.Context, queries []string, viewerID int64) ([][]RetrievalCandidate, error) {
	if r.vector == nil || r.embedder == nil {
		return nil, ErrRetrievalUnavailable
	}
	topK := r.topK(r.config.VectorTopK, config.RAGDefaultVectorTopK)
	if len(queries) == 0 {
		return nil, nil
	}
	embeddings, err := r.embedQueries(ctx, queries)
	if err != nil {
		return nil, err
	}
	lists := make([][]RetrievalCandidate, 0, len(queries))
	failures := 0
	for i := range queries {
		results, searchErr := r.vector.Search(ctx, embeddings[i], topK, viewerID)
		if searchErr != nil {
			failures++
			continue
		}
		if len(results) > 0 {
			lists = append(lists, results)
		}
	}
	if failures == len(queries) {
		return lists, errors.New("vector search failed for every query")
	}
	return lists, nil
}

// batchQueryEmbedder is the optional batch capability of the query embedder.
type batchQueryEmbedder interface {
	GetEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
}

func (r *HybridRetriever) embedQueries(ctx context.Context, queries []string) ([][]float32, error) {
	if len(queries) == 1 {
		// Historical single-text wire format (legacy embo-01 contract).
		vector, err := r.embedder.GetEmbedding(ctx, queries[0])
		if err != nil {
			return nil, err
		}
		return [][]float32{vector}, nil
	}
	if batch, ok := r.embedder.(batchQueryEmbedder); ok {
		vectors, err := batch.GetEmbeddings(ctx, queries)
		if err == nil && len(vectors) == len(queries) {
			return vectors, nil
		}
		if err != nil {
			return nil, err
		}
	}
	vectors := make([][]float32, len(queries))
	for i, q := range queries {
		vector, err := r.embedder.GetEmbedding(ctx, q)
		if err != nil {
			return nil, err
		}
		vectors[i] = vector
	}
	return vectors, nil
}

// rerankCandidates reranks the fused pool by query relevance and marks every
// surviving candidate as hybrid_rrf-sourced (post-fusion ordering).
func (r *HybridRetriever) rerankCandidates(ctx context.Context, query string, pool []RetrievalCandidate) ([]RetrievalCandidate, error) {
	documents := make([]string, len(pool))
	for i := range pool {
		documents[i] = rerankDocument(pool[i])
	}
	results, err := r.reranker.Rerank(ctx, query, documents, len(pool))
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	ordered := make([]RetrievalCandidate, 0, len(results))
	seen := make(map[string]bool, len(results))
	for _, result := range results {
		if result.Index < 0 || result.Index >= len(pool) {
			continue
		}
		candidate := pool[result.Index]
		key := stableChunkKey(candidate)
		if seen[key] {
			continue
		}
		seen[key] = true
		candidate.Source = RetrievalSourceHybrid
		ordered = append(ordered, candidate)
	}
	return ordered, nil
}

// rerankDocument renders one candidate as a rerank input document: title,
// heading and a bounded text excerpt.
func rerankDocument(candidate RetrievalCandidate) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{candidate.Title, candidate.Heading} {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	text := strings.TrimSpace(candidate.Text)
	if text != "" {
		runes := []rune(text)
		if len(runes) > rerankDocumentTextRunes {
			runes = runes[:rerankDocumentTextRunes]
		}
		parts = append(parts, string(runes))
	}
	return strings.Join(parts, "\n")
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

// rankedList is one ranked contribution to the RRF fusion, tagged with the
// retrieval path it came from.
type rankedList struct {
	candidates []RetrievalCandidate
	source     string
}

// FuseRRFLists is the N-list generalization of FuseRRF (A-03 query expansion
// fans out to one list per sibling query per path): every list contributes
// reciprocal-rank scores by stable chunk key, and a candidate found in
// several lists is marked hybrid.
func FuseRRFLists(lists []rankedList, k int) []RetrievalCandidate {
	if k <= 0 {
		k = config.RAGDefaultRRFK
	}
	merged := make(map[string]RetrievalCandidate)
	for _, list := range lists {
		for rank, candidate := range list.candidates {
			candidate.ChunkKey = stableChunkKey(candidate)
			current, ok := merged[candidate.ChunkKey]
			if !ok {
				candidate.Source = list.source
				candidate.rrfScore = 1 / float64(k+rank+1)
				merged[candidate.ChunkKey] = candidate
				continue
			}
			current.rrfScore += 1 / float64(k+rank+1)
			current.Source = RetrievalSourceHybrid
			merged[candidate.ChunkKey] = current
		}
	}
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
