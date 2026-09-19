package llm

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// slowQueueWait is the threshold beyond which a throttled call logs its
// queue wait (SP-24 R6: 排队不失败，仅排队延迟进日志). It is deliberately
// not a config knob — it gates a log line, not behavior.
const slowQueueWait = 100 * time.Millisecond

// ProviderThrottle hands out per-provider concurrency slots (SP-24 R6).
// Providers registered under the same name share one semaphore: the quota
// being protected belongs to the provider account, not the model entry, so
// e.g. the legacy primary MiniMax adapter and a minimax registry entry
// throttle together.
//
// A disabled throttle (limit <= 0, the default) makes Wrap return inner
// unchanged — the R6 zero-behavior-change contract.
type ProviderThrottle struct {
	limit    int
	slowWait time.Duration
	now      func() time.Time

	mu    sync.Mutex
	slots map[string]chan struct{}
}

// NewProviderThrottle builds the shared throttle. limit <= 0 returns nil,
// whose Wrap is the identity — call sites never branch.
func NewProviderThrottle(limit int) *ProviderThrottle {
	if limit <= 0 {
		return nil
	}
	return &ProviderThrottle{
		limit:    limit,
		slowWait: slowQueueWait,
		now:      time.Now,
		slots:    make(map[string]chan struct{}),
	}
}

// Wrap guards inner's chat calls behind the named provider's semaphore.
// The name is normalized so config variants like "MiniMax" and "minimax"
// share one slot pool. Queueing blocks (never fails) until a slot frees or
// ctx is done; waits beyond slowWait are logged with the provider name.
func (t *ProviderThrottle) Wrap(name string, inner LLMProvider) LLMProvider {
	if t == nil || inner == nil {
		return inner
	}
	name = strings.ToLower(strings.TrimSpace(name))
	return &ThrottledProvider{
		inner:    inner,
		name:     name,
		slots:    t.semaphore(name),
		slowWait: t.slowWait,
		now:      t.now,
	}
}

func (t *ProviderThrottle) semaphore(name string) chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ch, ok := t.slots[name]; ok {
		return ch
	}
	ch := make(chan struct{}, t.limit)
	t.slots[name] = ch
	return ch
}

// ThrottledProvider is the per-provider concurrency guard. Only Chat and
// ChatStream are bounded: chat calls are the burst source (main answer,
// tools, titles, expansion), while embeddings ride batch endpoints with
// their own pacing and sit on the latency-sensitive retrieval path.
type ThrottledProvider struct {
	inner    LLMProvider
	name     string
	slots    chan struct{}
	slowWait time.Duration
	now      func() time.Time
}

func (p *ThrottledProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if err := p.acquire(ctx); err != nil {
		return nil, err
	}
	defer p.release()
	return p.inner.Chat(ctx, req)
}

func (p *ThrottledProvider) ChatStream(ctx context.Context, req ChatRequest, handler func(delta ChatDelta) error) error {
	if err := p.acquire(ctx); err != nil {
		return err
	}
	defer p.release()
	return p.inner.ChatStream(ctx, req, handler)
}

// GetEmbedding passes through unguarded by design; see ThrottledProvider.
func (p *ThrottledProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	return p.inner.GetEmbedding(ctx, text)
}

func (p *ThrottledProvider) acquire(ctx context.Context) error {
	started := p.now()
	select {
	case p.slots <- struct{}{}:
		if wait := p.now().Sub(started); wait > p.slowWait {
			slog.Warn("llm provider concurrency queue wait", "provider", p.name, "wait_ms", wait.Milliseconds())
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *ThrottledProvider) release() {
	<-p.slots
}
