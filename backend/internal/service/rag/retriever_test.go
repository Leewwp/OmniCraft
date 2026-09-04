package rag

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"omnicraft/backend/config"
)

type fakeRetrievalProvider struct {
	results []RetrievalCandidate
	err     error
}

func (f fakeRetrievalProvider) Search(context.Context, string, int, int64) ([]RetrievalCandidate, error) {
	return append([]RetrievalCandidate(nil), f.results...), f.err
}

type fakeVectorProvider struct {
	results []RetrievalCandidate
	err     error
}

func (f fakeVectorProvider) Search(context.Context, []float32, int, int64) ([]RetrievalCandidate, error) {
	return append([]RetrievalCandidate(nil), f.results...), f.err
}

type fakeQueryEmbedder struct {
	vector []float32
	err    error
}

func (f fakeQueryEmbedder) GetEmbedding(context.Context, string) ([]float32, error) {
	return f.vector, f.err
}

type fakeVisibility struct {
	hidden map[string]bool
	err    error
	counts *[]int
}

func (f fakeVisibility) FilterVisible(_ context.Context, _ int64, candidates []RetrievalCandidate) ([]RetrievalCandidate, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.counts != nil {
		*f.counts = append(*f.counts, len(candidates))
	}
	visible := make([]RetrievalCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !f.hidden[candidate.ChunkKey] {
			visible = append(visible, candidate)
		}
	}
	return visible, nil
}

func candidate(key string, contentID int64) RetrievalCandidate {
	return RetrievalCandidate{ChunkKey: key, ContentID: contentID, ContentVersion: 1, Title: key}
}

func TestRRFCombinesSameChunkAndUsesStableTieBreak(t *testing.T) {
	keyword := []RetrievalCandidate{candidate("b", 2), candidate("a", 1)}
	vector := []RetrievalCandidate{candidate("a", 1), candidate("b", 2)}

	got := FuseRRF(keyword, vector, 60)
	if len(got) != 2 {
		t.Fatalf("FuseRRF returned %d candidates, want 2", len(got))
	}
	if keys := candidateKeys(got); !reflect.DeepEqual(keys, []string{"a", "b"}) {
		t.Fatalf("stable tie-break keys = %v, want [a b]", keys)
	}
	if got[0].Source != RetrievalSourceHybrid {
		t.Fatalf("same chunk source = %q, want %q", got[0].Source, RetrievalSourceHybrid)
	}
	if got[0].rrfScore <= 1/61.0 {
		t.Fatalf("same chunk score = %v, want accumulated reciprocal ranks", got[0].rrfScore)
	}
}

func TestHybridRetrieverFallsBackToKeywordFallbackWhenPrimaryFails(t *testing.T) {
	r := NewHybridRetriever(
		fakeRetrievalProvider{err: ErrRetrievalUnavailable},
		fakeRetrievalProvider{results: []RetrievalCandidate{candidate("pg", 1)}},
		fakeVectorProvider{results: []RetrievalCandidate{candidate("vec", 2)}},
		fakeQueryEmbedder{vector: []float32{1}},
		fakeVisibility{},
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10},
	)

	got, err := r.Retrieve(context.Background(), "query", 7)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if got.Degraded != RetrievalDegradedKeywordFallback {
		t.Fatalf("degraded = %q, want %q", got.Degraded, RetrievalDegradedKeywordFallback)
	}
	if keys := candidateKeys(got.Candidates); !reflect.DeepEqual(keys, []string{"pg", "vec"}) {
		t.Fatalf("candidate keys = %v, want [pg vec]", keys)
	}
}

type viewerRecordingKeyword struct {
	viewerIDs []int64
	results   []RetrievalCandidate
}

func (p *viewerRecordingKeyword) Search(_ context.Context, _ string, _ int, viewerID int64) ([]RetrievalCandidate, error) {
	p.viewerIDs = append(p.viewerIDs, viewerID)
	return append([]RetrievalCandidate(nil), p.results...), nil
}

// The lexical primary must receive the real viewer identity so viewer-scoped
// backends (Postgres FTS) pre-filter restricted content instead of ranking it
// first and filtering after the fact.
func TestHybridRetrieverPassesViewerIdentityToLexicalPrimary(t *testing.T) {
	primary := &viewerRecordingKeyword{results: []RetrievalCandidate{candidate("pg", 1)}}
	r := NewHybridRetriever(
		primary,
		fakeRetrievalProvider{results: []RetrievalCandidate{candidate("os", 2)}},
		fakeVectorProvider{results: []RetrievalCandidate{candidate("vec", 3)}},
		fakeQueryEmbedder{vector: []float32{1}},
		fakeVisibility{},
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10},
	)

	got, err := r.Retrieve(context.Background(), "query", 42)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if got.Degraded != "" {
		t.Fatalf("primary success must not be marked degraded, got %q", got.Degraded)
	}
	if len(primary.viewerIDs) != 1 || primary.viewerIDs[0] != 42 {
		t.Fatalf("lexical primary viewer ids = %v, want [42]", primary.viewerIDs)
	}
}

func TestHybridRetrieverFailsClosedWithoutVisibilityFilter(t *testing.T) {
	r := NewHybridRetriever(
		fakeRetrievalProvider{results: []RetrievalCandidate{candidate("keyword", 1)}},
		fakeRetrievalProvider{},
		fakeVectorProvider{},
		fakeQueryEmbedder{err: errors.New("offline")},
		nil,
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10},
	)

	if _, err := r.Retrieve(context.Background(), "query", 7); !errors.Is(err, ErrRetrievalUnavailable) {
		t.Fatalf("Retrieve() error = %v, want ErrRetrievalUnavailable", err)
	}
}

func TestHybridRetrieverReportsVectorOnlyWhenKeywordProvidersFail(t *testing.T) {
	r := NewHybridRetriever(
		fakeRetrievalProvider{err: ErrRetrievalUnavailable},
		fakeRetrievalProvider{err: ErrRetrievalUnavailable},
		fakeVectorProvider{results: []RetrievalCandidate{candidate("vector", 1)}},
		fakeQueryEmbedder{vector: []float32{1}},
		fakeVisibility{},
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10},
	)

	got, err := r.Retrieve(context.Background(), "query", 7)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if got.Degraded != RetrievalDegradedVectorOnly {
		t.Fatalf("degraded = %q, want %q", got.Degraded, RetrievalDegradedVectorOnly)
	}
}

func TestHybridRetrieverEmbeddingFailureReturnsKeywordOnly(t *testing.T) {
	r := NewHybridRetriever(
		fakeRetrievalProvider{results: []RetrievalCandidate{candidate("keyword", 1)}},
		fakeRetrievalProvider{},
		fakeVectorProvider{err: errors.New("provider offline")},
		fakeQueryEmbedder{err: errors.New("provider offline")},
		fakeVisibility{},
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10},
	)

	got, err := r.Retrieve(context.Background(), "query", 7)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if got.Degraded != RetrievalDegradedKeywordOnly {
		t.Fatalf("degraded = %q, want %q", got.Degraded, RetrievalDegradedKeywordOnly)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].Source != RetrievalSourceKeyword {
		t.Fatalf("keyword-only candidates = %#v", got.Candidates)
	}
}

func TestHybridRetrieverMarksKeywordOnlyWhenEmbeddingFailsWithNoKeywordHits(t *testing.T) {
	r := NewHybridRetriever(
		fakeRetrievalProvider{},
		fakeRetrievalProvider{},
		fakeVectorProvider{err: errors.New("provider offline")},
		fakeQueryEmbedder{err: errors.New("provider offline")},
		fakeVisibility{},
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10},
	)

	got, err := r.Retrieve(context.Background(), "query", 7)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if got.Degraded != RetrievalDegradedKeywordOnly {
		t.Fatalf("degraded = %q, want %q", got.Degraded, RetrievalDegradedKeywordOnly)
	}
	if len(got.Candidates) != 0 {
		t.Fatalf("candidates = %#v, want empty", got.Candidates)
	}
}

func TestHybridRetrieverReturnsVectorOnlyWhenKeywordProvidersAreEmpty(t *testing.T) {
	r := NewHybridRetriever(
		fakeRetrievalProvider{},
		fakeRetrievalProvider{},
		fakeVectorProvider{results: []RetrievalCandidate{candidate("vector", 1)}},
		fakeQueryEmbedder{vector: []float32{1}},
		fakeVisibility{},
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10},
	)

	got, err := r.Retrieve(context.Background(), "query", 7)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if got.Degraded != "" {
		t.Fatalf("degraded = %q, want empty for one-sided empty result", got.Degraded)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].Source != RetrievalSourceVector {
		t.Fatalf("vector-only candidates = %#v", got.Candidates)
	}
}

func TestHybridRetrieverReturnsUnavailableWhenBothSidesFail(t *testing.T) {
	r := NewHybridRetriever(
		fakeRetrievalProvider{err: ErrRetrievalUnavailable},
		fakeRetrievalProvider{err: ErrRetrievalUnavailable},
		fakeVectorProvider{err: ErrRetrievalUnavailable},
		fakeQueryEmbedder{err: ErrRetrievalUnavailable},
		fakeVisibility{},
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10},
	)

	if _, err := r.Retrieve(context.Background(), "query", 7); !errors.Is(err, ErrRetrievalUnavailable) {
		t.Fatalf("Retrieve() error = %v, want ErrRetrievalUnavailable", err)
	}
}

func TestHybridRetrieverFailsClosedWhenVisibilityRevalidationFails(t *testing.T) {
	r := NewHybridRetriever(
		fakeRetrievalProvider{results: []RetrievalCandidate{candidate("keyword", 1)}},
		fakeRetrievalProvider{},
		fakeVectorProvider{},
		fakeQueryEmbedder{err: ErrRetrievalUnavailable},
		fakeVisibility{err: errors.New("database offline")},
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10},
	)

	if _, err := r.Retrieve(context.Background(), "query", 7); !errors.Is(err, ErrRetrievalUnavailable) {
		t.Fatalf("Retrieve() error = %v, want ErrRetrievalUnavailable", err)
	}
}

func TestHybridRetrieverVisibilityFilteringBackfillsInRankOrder(t *testing.T) {
	var visibilityCallSizes []int
	r := NewHybridRetriever(
		fakeRetrievalProvider{results: []RetrievalCandidate{candidate("one", 1), candidate("two", 2), candidate("three", 3), candidate("four", 4)}},
		fakeRetrievalProvider{},
		fakeVectorProvider{},
		fakeQueryEmbedder{err: errors.New("offline")},
		fakeVisibility{hidden: map[string]bool{"one": true}, counts: &visibilityCallSizes},
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 2},
	)

	got, err := r.Retrieve(context.Background(), "query", 7)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if keys := candidateKeys(got.Candidates); !reflect.DeepEqual(keys, []string{"two", "three"}) {
		t.Fatalf("backfilled keys = %v, want [two three]", keys)
	}
	if !reflect.DeepEqual(visibilityCallSizes, []int{4}) {
		t.Fatalf("visibility call sizes = %v, want [4] for fused candidates within top 20", visibilityCallSizes)
	}
}

func TestHybridRetrieverBackfillsBelowVisibilityWindowOneAtATime(t *testing.T) {
	var visibilityCallSizes []int
	candidates := make([]RetrievalCandidate, 0, 22)
	for i := 1; i <= 22; i++ {
		candidates = append(candidates, candidate(fmt.Sprintf("candidate-%02d", i), int64(i)))
	}
	r := NewHybridRetriever(
		fakeRetrievalProvider{results: candidates},
		fakeRetrievalProvider{},
		fakeVectorProvider{},
		fakeQueryEmbedder{err: errors.New("offline")},
		fakeVisibility{hidden: map[string]bool{}, counts: &visibilityCallSizes},
		config.RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 21},
	)

	got, err := r.Retrieve(context.Background(), "query", 7)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if len(got.Candidates) != 21 {
		t.Fatalf("candidate count = %d, want 21", len(got.Candidates))
	}
	if !reflect.DeepEqual(visibilityCallSizes, append([]int{20}, 1)) {
		t.Fatalf("visibility call sizes = %v, want [20 1]", visibilityCallSizes)
	}
}

func candidateKeys(candidates []RetrievalCandidate) []string {
	keys := make([]string, len(candidates))
	for i, candidate := range candidates {
		keys[i] = candidate.ChunkKey
	}
	return keys
}
