package rag

import (
	"context"
	"errors"
	"strings"
	"testing"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/llm"
)

type staticExpander struct {
	terms []string
}

func (e staticExpander) Expand(context.Context, string) []string {
	return append([]string(nil), e.terms...)
}

type recordingKeywordProvider struct {
	results  []RetrievalCandidate
	queries  []string
	perQuery map[string][]RetrievalCandidate
}

func (p *recordingKeywordProvider) Search(_ context.Context, query string, _ int, _ int64) ([]RetrievalCandidate, error) {
	p.queries = append(p.queries, query)
	if p.perQuery != nil {
		if results, ok := p.perQuery[query]; ok {
			return append([]RetrievalCandidate(nil), results...), nil
		}
		return nil, nil
	}
	return append([]RetrievalCandidate(nil), p.results...), nil
}

type recordingVectorProvider struct {
	perEmbedding map[int][]RetrievalCandidate
	calls        [][]float32
}

func (p *recordingVectorProvider) Search(_ context.Context, embedding []float32, _ int, _ int64) ([]RetrievalCandidate, error) {
	p.calls = append(p.calls, embedding)
	if p.perEmbedding != nil {
		if results, ok := p.perEmbedding[int(embedding[0])]; ok {
			return append([]RetrievalCandidate(nil), results...), nil
		}
		return nil, nil
	}
	return nil, nil
}

// A-03: query expansion fans the retrieval out to the original query plus
// every expansion term on both paths, and the expanded terms ride along in
// the result for display.
func TestHybridRetrieverExpansionFansOutBothPaths(t *testing.T) {
	keyword := &recordingKeywordProvider{perQuery: map[string][]RetrievalCandidate{
		"原查询": {candidate("k1", 1)},
		"扩展词": {candidate("k2", 2)},
	}}
	vector := &recordingVectorProvider{}
	embedder := &mappingQueryEmbedder{mapping: map[string][]float32{
		"原查询": {1}, "扩展词": {2},
	}}
	vector.perEmbedding = map[int][]RetrievalCandidate{
		1: {candidate("v1", 3)},
		2: {candidate("v2", 4)},
	}
	visibility := fakeVisibility{}

	r := NewHybridRetriever(keyword, nil, vector, embedder, visibility, config.RAGHybridConfig{})
	r.SetQueryExpander(staticExpander{terms: []string{"扩展词"}})

	result, err := r.Retrieve(context.Background(), "原查询", 7)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if got := strings.Join(keyword.queries, ","); got != "原查询,扩展词" {
		t.Fatalf("keyword queries = %q, want fan-out", got)
	}
	if len(vector.calls) != 2 {
		t.Fatalf("vector calls = %d, want 2 (one per sibling query)", len(vector.calls))
	}
	if len(result.ExpandedQueries) != 1 || result.ExpandedQueries[0] != "扩展词" {
		t.Fatalf("ExpandedQueries = %v", result.ExpandedQueries)
	}
	keys := make(map[string]bool)
	for _, c := range result.Candidates {
		keys[c.ChunkKey] = true
	}
	for _, want := range []string{"k1", "k2", "v1", "v2"} {
		if !keys[want] {
			t.Fatalf("candidate %s missing from %+v", want, result.Candidates)
		}
	}
}

// mappingQueryEmbedder maps distinct queries to distinct marker vectors.
type mappingQueryEmbedder struct {
	mapping map[string][]float32
	err     error
}

func (e *mappingQueryEmbedder) GetEmbedding(_ context.Context, text string) ([]float32, error) {
	if e.err != nil {
		return nil, e.err
	}
	if v, ok := e.mapping[text]; ok {
		return v, nil
	}
	return []float32{0}, nil
}

// A-03: the sibling queries of an expanded retrieval are embedded in ONE
// batch call when the embedder supports it.
func TestHybridRetrieverUsesBatchEmbeddingForSiblingQueries(t *testing.T) {
	embedder := &recordingBatchEmbedder{vectors: map[string][]float32{"q": {1}, "t": {2}}}
	r := NewHybridRetriever(
		&recordingKeywordProvider{},
		nil,
		&recordingVectorProvider{},
		embedder,
		fakeVisibility{},
		config.RAGHybridConfig{},
	)
	r.SetQueryExpander(staticExpander{terms: []string{"t"}})
	if _, err := r.Retrieve(context.Background(), "q", 7); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if embedder.batchCalls != 1 || embedder.singleCalls != 0 {
		t.Fatalf("embedder calls = batch %d / single %d, want 1/0", embedder.batchCalls, embedder.singleCalls)
	}
}

type recordingBatchEmbedder struct {
	vectors     map[string][]float32
	batchCalls  int
	singleCalls int
}

func (e *recordingBatchEmbedder) GetEmbedding(_ context.Context, text string) ([]float32, error) {
	e.singleCalls++
	return e.vectors[text], nil
}

func (e *recordingBatchEmbedder) GetEmbeddings(_ context.Context, texts []string) ([][]float32, error) {
	e.batchCalls++
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = e.vectors[text]
	}
	return out, nil
}

type fakeReranker struct {
	results []llm.RerankResult
	err     error
	queries []string
	docs    [][]string
}

func (f *fakeReranker) Rerank(_ context.Context, query string, documents []string, _ int) ([]llm.RerankResult, error) {
	f.queries = append(f.queries, query)
	f.docs = append(f.docs, documents)
	return f.results, f.err
}

// A-03: rerank reorders the fused pool by relevance and keeps every
// candidate marked as hybrid-sourced after fusion.
func TestHybridRetrieverRerankReordersPool(t *testing.T) {
	keyword := &recordingKeywordProvider{results: []RetrievalCandidate{
		candidate("a", 1), candidate("b", 2), candidate("c", 3),
	}}
	vector := &recordingVectorProvider{}
	r := NewHybridRetriever(keyword, nil, vector, fakeQueryEmbedder{vector: []float32{1}}, fakeVisibility{}, config.RAGHybridConfig{})
	// relevance order: c, a, b
	r.SetReranker(&fakeReranker{results: []llm.RerankResult{
		{Index: 2, RelevanceScore: 0.9}, {Index: 0, RelevanceScore: 0.5}, {Index: 1, RelevanceScore: 0.2},
	}}, 20)

	result, err := r.Retrieve(context.Background(), "q", 7)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(result.Candidates) != 3 || result.Candidates[0].ChunkKey != "c" || result.Candidates[1].ChunkKey != "a" {
		t.Fatalf("rerank order = %+v", result.Candidates)
	}
	if result.Degraded != "" {
		t.Fatalf("successful rerank must not degrade: %q", result.Degraded)
	}
}

// A-03 degradation: a rerank failure keeps the RRF order and marks the
// result rerank_unavailable.
func TestHybridRetrieverRerankFailureKeepsRRFOrder(t *testing.T) {
	keyword := &recordingKeywordProvider{results: []RetrievalCandidate{
		candidate("a", 1), candidate("b", 2),
	}}
	r := NewHybridRetriever(keyword, nil, &recordingVectorProvider{}, fakeQueryEmbedder{vector: []float32{1}}, fakeVisibility{}, config.RAGHybridConfig{})
	r.SetReranker(&fakeReranker{err: errors.New("rerank down")}, 20)

	result, err := r.Retrieve(context.Background(), "q", 7)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if result.Degraded != RetrievalDegradedRerankUnavailable {
		t.Fatalf("degraded = %q, want rerank_unavailable", result.Degraded)
	}
	if result.Candidates[0].ChunkKey != "a" {
		t.Fatalf("RRF order must survive rerank failure: %+v", result.Candidates)
	}
}

// A-03: multi-list RRF fuses N keyword/vector lists; overlapping candidates
// accumulate reciprocal-rank contributions across lists.
func TestFuseRRFListsAccumulatesAcrossLists(t *testing.T) {
	lists := []rankedList{
		{candidates: []RetrievalCandidate{candidate("x", 1), candidate("y", 2)}, source: RetrievalSourceKeyword},
		{candidates: []RetrievalCandidate{candidate("y", 2)}, source: RetrievalSourceVector},
		{candidates: []RetrievalCandidate{candidate("z", 3)}, source: RetrievalSourceKeyword},
	}
	fused := FuseRRFLists(lists, 60)
	if len(fused) != 3 {
		t.Fatalf("fused = %d candidates, want 3", len(fused))
	}
	if fused[0].ChunkKey != "y" {
		t.Fatalf("overlapping candidate must outrank singles: %+v", fused)
	}
	if fused[0].Source != RetrievalSourceHybrid {
		t.Fatalf("overlap must be marked hybrid, got %q", fused[0].Source)
	}
}

// A-03: the expander parses fenced JSON arrays, drops duplicates of the
// original query and caps the term count.
func TestParseExpansionTermsSanitizes(t *testing.T) {
	terms := parseExpansionTerms("```json\n[\"原查询\", \" 好词 \", \"好词\", \"\", \"词3\", \"词4\", \"词5\", \"词6\"]\n```", "原查询")
	want := []string{"好词", "词3", "词4", "词5", "词6"}
	if len(terms) != len(want) {
		t.Fatalf("terms = %v, want %v", terms, want)
	}
	for i := range want {
		if terms[i] != want[i] {
			t.Fatalf("terms = %v, want %v", terms, want)
		}
	}
	if parseExpansionTerms("not json at all", "q") != nil {
		t.Fatal("unparseable reply must yield nil")
	}
}

type staticExpansionChatProvider struct {
	content string
	err     error
}

func (p staticExpansionChatProvider) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	if p.err != nil {
		return nil, p.err
	}
	return &llm.ChatResponse{Content: p.content}, nil
}

// A-03: an expander provider failure is swallowed (expansion is an
// enhancement, never a gate).
func TestLLMQueryExpanderSwallowsProviderFailure(t *testing.T) {
	expander := NewLLMQueryExpander(staticExpansionChatProvider{err: errors.New("llm down")})
	if terms := expander.Expand(context.Background(), "找原神攻略"); terms != nil {
		t.Fatalf("expected nil terms on failure, got %v", terms)
	}
	parsed := NewLLMQueryExpander(staticExpansionChatProvider{content: `["攻略","教程"]`})
	terms := parsed.Expand(context.Background(), "找原神攻略")
	if len(terms) != 2 || terms[0] != "攻略" {
		t.Fatalf("terms = %v", terms)
	}
}

// A-03: the chunk embedder prefers the batch endpoint and preserves order.
func TestProviderChunkEmbedderPrefersBatch(t *testing.T) {
	provider := &recordingBatchEmbedder{vectors: map[string][]float32{"a": {1}, "b": {2}, "c": {3}}}
	embedder := NewProviderChunkEmbedder(provider)
	vectors, err := embedder.Embed(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vectors) != 3 || vectors[0][0] != 1 || vectors[2][0] != 3 {
		t.Fatalf("vectors = %v", vectors)
	}
	if provider.batchCalls != 1 || provider.singleCalls != 0 {
		t.Fatalf("calls = batch %d / single %d", provider.batchCalls, provider.singleCalls)
	}
}
