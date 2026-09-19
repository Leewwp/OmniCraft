package rag

import (
	"context"
	"errors"
	"testing"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/breaker"
	"omnicraft/backend/internal/pkg/llm"
)

// SP-24 R5 OpenSearch mount: an open lexical channel surfaces
// breaker.ErrOpen, which the hybrid retriever already treats as that
// channel failing (degradation markers unchanged).
func TestGuardedKeywordRetrieverOpenSkipsChannel(t *testing.T) {
	br := breaker.New("opensearch", breaker.Config{FailureThreshold: 1, OpenTimeout: 30 * 1e9}, nil)
	if err := br.Do(func() error { return errors.New("es down") }); err == nil {
		t.Fatal("trip failure must propagate")
	}
	g := NewGuardedKeywordRetriever(nil, br) // inner intentionally nil
	if _, err := g.Search(context.Background(), "q", 10, 0); !errors.Is(err, breaker.ErrOpen) {
		t.Fatalf("open circuit = %v, want breaker.ErrOpen", err)
	}
}

// End-to-end mount semantics: a reranker whose breaker is open behaves like
// a failing reranker — the retrieval keeps RRF order and degrades with the
// existing rerank_unavailable marker instead of erroring.
func TestHybridRetrieverGuardedRerankerOpenDegradesNotFails(t *testing.T) {
	keyword := &recordingKeywordProvider{results: []RetrievalCandidate{
		candidate("a", 1), candidate("b", 2), candidate("c", 3),
	}}
	br := breaker.New("rerank", breaker.Config{FailureThreshold: 1, OpenTimeout: 30 * 1e9}, nil)
	if err := br.Do(func() error { return errors.New("rerank api down") }); err == nil {
		t.Fatal("trip failure must propagate")
	}
	r := NewHybridRetriever(keyword, nil, &recordingVectorProvider{}, fakeQueryEmbedder{vector: []float32{1}}, fakeVisibility{}, config.RAGHybridConfig{})
	r.SetReranker(llm.NewGuardedReranker(&fakeReranker{results: []llm.RerankResult{
		{Index: 2, RelevanceScore: 0.9},
	}}, br), 20)

	result, err := r.Retrieve(context.Background(), "q", 7)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(result.Candidates) != 3 || result.Candidates[0].ChunkKey != "a" {
		t.Fatalf("open rerank breaker must keep RRF order, got %+v", result.Candidates)
	}
	if result.Degraded != RetrievalDegradedRerankUnavailable {
		t.Fatalf("degraded = %q, want rerank_unavailable", result.Degraded)
	}
}
