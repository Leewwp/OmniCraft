package rag

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/internal/pkg/auxcache"
	"omnicraft/backend/internal/pkg/llm"
)

type countingExpansionProvider struct {
	content string
	err     error
	calls   int
}

func (p *countingExpansionProvider) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return &llm.ChatResponse{Content: p.content}, nil
}

func newExpanderTestCache(t *testing.T, ttl time.Duration) *auxcache.Cache {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return auxcache.New("expander", "omnicraft:auxcache:expander:", ttl, client)
}

// SP-24 R6: an identical query reuses the cached expansion terms; the
// provider is called once.
func TestExpanderCacheReusesTermsForIdenticalQuery(t *testing.T) {
	provider := &countingExpansionProvider{content: `["攻略","教程"]`}
	expander := NewLLMQueryExpander(provider)
	expander.SetResultCache(newExpanderTestCache(t, time.Minute))

	first := expander.Expand(context.Background(), "找原神攻略")
	second := expander.Expand(context.Background(), "找原神攻略")
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("terms = %v / %v", first, second)
	}
	if provider.calls != 1 {
		t.Fatalf("identical query must hit the cache: expected 1 call, got %d", provider.calls)
	}
}

// SP-24 R6: without a cache (the default-off wiring) every expansion calls
// the provider — zero behavior change.
func TestExpanderWithoutCacheCallsEveryTime(t *testing.T) {
	provider := &countingExpansionProvider{content: `["攻略","教程"]`}
	expander := NewLLMQueryExpander(provider)

	_ = expander.Expand(context.Background(), "找原神攻略")
	_ = expander.Expand(context.Background(), "找原神攻略")
	if provider.calls != 2 {
		t.Fatalf("no cache wired: expected 2 calls, got %d", provider.calls)
	}
}

// SP-24 R6: distinct queries never collide on one cache entry.
func TestExpanderCacheSeparatesQueries(t *testing.T) {
	provider := &countingExpansionProvider{content: `["攻略","教程"]`}
	expander := NewLLMQueryExpander(provider)
	expander.SetResultCache(newExpanderTestCache(t, time.Minute))

	_ = expander.Expand(context.Background(), "找原神攻略")
	_ = expander.Expand(context.Background(), "推荐几个设定")
	if provider.calls != 2 {
		t.Fatalf("distinct queries must not share entries: expected 2 calls, got %d", provider.calls)
	}
}

// SP-24 R6: a provider failure produces no cache entry, so the retry after
// recovery re-invokes the provider instead of a pinned empty expansion.
func TestExpanderFailureIsNotCached(t *testing.T) {
	provider := &countingExpansionProvider{err: errors.New("llm down")}
	expander := NewLLMQueryExpander(provider)
	expander.SetResultCache(newExpanderTestCache(t, time.Minute))

	if terms := expander.Expand(context.Background(), "找原神攻略"); terms != nil {
		t.Fatalf("expected nil terms on failure, got %v", terms)
	}
	provider.err = nil
	provider.content = `["攻略"]`
	terms := expander.Expand(context.Background(), "找原神攻略")
	if len(terms) != 1 {
		t.Fatalf("recovered expansion must reach the provider: %v", terms)
	}
	if provider.calls != 2 {
		t.Fatalf("failures must not be cached: expected 2 calls, got %d", provider.calls)
	}
}

// SP-24 R6: entries expire at the configured TTL and the next call expands
// again.
func TestExpanderCacheTTLExpiry(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := auxcache.New("expander", "omnicraft:auxcache:expander:", 2*time.Second, client)

	provider := &countingExpansionProvider{content: `["攻略"]`}
	expander := NewLLMQueryExpander(provider)
	expander.SetResultCache(cache)

	_ = expander.Expand(context.Background(), "找原神攻略")
	mr.FastForward(3 * time.Second)
	_ = expander.Expand(context.Background(), "找原神攻略")
	if provider.calls != 2 {
		t.Fatalf("expired entry must re-expand: expected 2 calls, got %d", provider.calls)
	}
}
