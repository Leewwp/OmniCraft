package rag

import (
	"context"

	"omnicraft/backend/internal/pkg/breaker"
)

// GuardedKeywordRetriever wraps a lexical channel behind a circuit breaker
// (SP-24 R5). An open circuit surfaces breaker.ErrOpen, which the hybrid
// retriever already treats as that channel failing — the keyword-fallback /
// keyword-only degradation markers stay untouched.
type GuardedKeywordRetriever struct {
	Inner   KeywordRetriever
	Breaker *breaker.Breaker
}

func NewGuardedKeywordRetriever(inner KeywordRetriever, br *breaker.Breaker) *GuardedKeywordRetriever {
	return &GuardedKeywordRetriever{Inner: inner, Breaker: br}
}

func (g *GuardedKeywordRetriever) Search(ctx context.Context, query string, topK int, viewerID int64) ([]RetrievalCandidate, error) {
	var results []RetrievalCandidate
	err := g.Breaker.Do(func() error {
		var err error
		results, err = g.Inner.Search(ctx, query, topK, viewerID)
		return err
	})
	return results, err
}
