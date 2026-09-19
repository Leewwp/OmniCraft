package llm

import (
	"context"
	"errors"
	"testing"

	"omnicraft/backend/internal/pkg/breaker"
)

type stubReranker struct {
	calls int
	err   error
}

func (s *stubReranker) Rerank(context.Context, string, []string, int) ([]RerankResult, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return []RerankResult{{Index: 0, RelevanceScore: 1}}, nil
}

type stubImageGenerator struct {
	calls int
}

func (s *stubImageGenerator) GenerateImage(context.Context, string, string, int) ([]byte, error) {
	s.calls++
	return []byte("png"), nil
}

// SP-24 R5 rerank mount: threshold failures open the circuit and further
// Rerank calls fail fast with breaker.ErrOpen without touching the provider.
func TestGuardedRerankerOpensAfterFailures(t *testing.T) {
	inner := &stubReranker{err: errors.New("provider down")}
	br := breaker.New("rerank", breaker.Config{FailureThreshold: 2, OpenTimeout: 30 * 1e9}, nil)
	g := NewGuardedReranker(inner, br)

	for i := 0; i < 2; i++ {
		if _, err := g.Rerank(context.Background(), "q", []string{"d"}, 1); err == nil {
			t.Fatal("provider failure must propagate")
		}
	}
	if _, err := g.Rerank(context.Background(), "q", []string{"d"}, 1); !errors.Is(err, breaker.ErrOpen) {
		t.Fatalf("open circuit = %v, want breaker.ErrOpen", err)
	}
	if inner.calls != 2 {
		t.Fatalf("provider called %d times, want 2 (third call must be skipped)", inner.calls)
	}
}

// SP-24 R5 image mount: an open circuit fails the tool call fast instead of
// paying another CogView round-trip.
func TestGuardedImageGeneratorOpenSkipsProvider(t *testing.T) {
	inner := &stubImageGenerator{}
	br := breaker.New("image", breaker.Config{FailureThreshold: 1, OpenTimeout: 30 * 1e9}, nil)
	g := NewGuardedImageGenerator(inner, br)

	if _, err := g.GenerateImage(context.Background(), "p", "1024x1024", 1<<20); err != nil {
		t.Fatalf("first call must succeed: %v", err)
	}
	// Force the circuit open via a failing stub swap is overkill: trip it by
	// recording a failure through a second guarded view of the same breaker.
	if err := br.Do(func() error { return errors.New("trip") }); err == nil {
		t.Fatal("trip failure must propagate")
	}
	if _, err := g.GenerateImage(context.Background(), "p", "1024x1024", 1<<20); !errors.Is(err, breaker.ErrOpen) {
		t.Fatalf("open circuit = %v, want breaker.ErrOpen", err)
	}
	if inner.calls != 1 {
		t.Fatalf("provider called %d times, want 1", inner.calls)
	}
}
