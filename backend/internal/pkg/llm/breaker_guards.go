package llm

import (
	"context"

	"omnicraft/backend/internal/pkg/breaker"
)

// GuardedReranker wraps a Reranker behind a circuit breaker (SP-24 R5). An
// open circuit surfaces breaker.ErrOpen, which the FallbackReranker chain
// and the retriever's degraded-marker path treat exactly like a provider
// failure — the rerank_unavailable semantics stay untouched.
type GuardedReranker struct {
	Inner   Reranker
	Breaker *breaker.Breaker
}

func NewGuardedReranker(inner Reranker, br *breaker.Breaker) *GuardedReranker {
	return &GuardedReranker{Inner: inner, Breaker: br}
}

func (g *GuardedReranker) Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error) {
	var results []RerankResult
	err := g.Breaker.Do(func() error {
		var err error
		results, err = g.Inner.Rerank(ctx, query, documents, topN)
		return err
	})
	return results, err
}

// GuardedImageGenerator wraps the generate_image provider seam behind a
// circuit breaker. An open circuit fails the tool call fast with
// breaker.ErrOpen; the tool layer already renders provider errors to the
// model as a stable tool error outcome.
type GuardedImageGenerator struct {
	Inner   AgentImageGenerator
	Breaker *breaker.Breaker
}

func NewGuardedImageGenerator(inner AgentImageGenerator, br *breaker.Breaker) *GuardedImageGenerator {
	return &GuardedImageGenerator{Inner: inner, Breaker: br}
}

func (g *GuardedImageGenerator) GenerateImage(ctx context.Context, prompt, size string, maxImageBytes int) ([]byte, error) {
	var data []byte
	err := g.Breaker.Do(func() error {
		var err error
		data, err = g.Inner.GenerateImage(ctx, prompt, size, maxImageBytes)
		return err
	})
	return data, err
}
