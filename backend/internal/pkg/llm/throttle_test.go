package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"

	"omnicraft/backend/config"
	"testing"
	"time"
)

// gatedProvider blocks every call until release is closed while tracking the
// high-water mark of concurrent executions. One token is pushed onto enter
// per call start so tests can wait for a specific number of in-flight calls.
type gatedProvider struct {
	enter    chan struct{}
	release  chan struct{}
	inFlight atomic.Int32
	maxSeen  atomic.Int32
}

func newGatedProvider() *gatedProvider {
	return &gatedProvider{enter: make(chan struct{}, 16), release: make(chan struct{})}
}

func (p *gatedProvider) signalEnter() {
	select {
	case p.enter <- struct{}{}:
	default:
	}
}

func (p *gatedProvider) trackPeak(n int32) {
	for {
		old := p.maxSeen.Load()
		if n <= old || p.maxSeen.CompareAndSwap(old, n) {
			return
		}
	}
}

func (p *gatedProvider) Chat(ctx context.Context, _ ChatRequest) (*ChatResponse, error) {
	p.trackPeak(p.inFlight.Add(1))
	defer p.inFlight.Add(-1)
	p.signalEnter()
	select {
	case <-p.release:
		return &ChatResponse{Content: "ok"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *gatedProvider) ChatStream(ctx context.Context, req ChatRequest, handler func(delta ChatDelta) error) error {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}
	return handler(ChatDelta{Content: resp.Content, Done: true})
}

func (p *gatedProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return []float32{1}, nil
}

func startChat(ctx context.Context, p LLMProvider) chan struct {
	resp *ChatResponse
	err  error
} {
	done := make(chan struct {
		resp *ChatResponse
		err  error
	}, 1)
	go func() {
		resp, err := p.Chat(ctx, ChatRequest{})
		done <- struct {
			resp *ChatResponse
			err  error
		}{resp, err}
	}()
	return done
}

func TestDisabledThrottleWrapsNothing(t *testing.T) {
	inner := newGatedProvider()
	var throttle *ProviderThrottle = NewProviderThrottle(0)
	if got := throttle.Wrap("minimax", inner); got != LLMProvider(inner) {
		t.Fatal("disabled throttle must return the inner provider unchanged")
	}
}

func TestThrottleSerializesBeyondLimit(t *testing.T) {
	inner := newGatedProvider()
	throttle := NewProviderThrottle(1)
	guarded := throttle.Wrap("minimax", inner)

	first := startChat(context.Background(), guarded)
	<-inner.enter // first call is inside the provider

	// Second call queues behind the single slot; it must not fail.
	second := startChat(context.Background(), guarded)
	select {
	case r := <-second:
		t.Fatalf("second call must queue, not complete early: %+v", r)
	case <-time.After(50 * time.Millisecond):
	}

	close(inner.release)
	if r := <-first; r.err != nil {
		t.Fatalf("first call failed: %v", r.err)
	}
	if r := <-second; r.err != nil || r.resp.Content != "ok" {
		t.Fatalf("queued call must complete after release: %+v", r)
	}
	if got := inner.maxSeen.Load(); got > 1 {
		t.Fatalf("limit 1 violated: %d concurrent executions", got)
	}
}

func TestThrottleCancelledWhileQueuing(t *testing.T) {
	inner := newGatedProvider()
	throttle := NewProviderThrottle(1)
	guarded := throttle.Wrap("deepseek", inner)

	first := startChat(context.Background(), guarded)
	<-inner.enter

	qctx, qcancel := context.WithCancel(context.Background())
	second := startChat(qctx, guarded)
	time.Sleep(30 * time.Millisecond)
	qcancel()

	select {
	case r := <-second:
		if !errors.Is(r.err, context.Canceled) {
			t.Fatalf("queued call under a cancelled ctx must surface ctx error, got %+v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled queued call must return promptly")
	}
	close(inner.release)
	<-first
}

func TestSameNameSharesSlotsDifferentNamesIndependent(t *testing.T) {
	inner := newGatedProvider()
	throttle := NewProviderThrottle(1)
	a := throttle.Wrap("minimax", inner)
	b := throttle.Wrap("minimax", inner)
	c := throttle.Wrap("deepseek", inner)

	first := startChat(context.Background(), a)
	<-inner.enter

	// Same name: the second minimax call queues behind the first.
	second := startChat(context.Background(), b)
	select {
	case r := <-second:
		t.Fatalf("same-name call must share the semaphore: %+v", r)
	case <-time.After(50 * time.Millisecond):
	}

	// Different name: deepseek proceeds on its own slot.
	third := startChat(context.Background(), c)
	<-inner.enter

	close(inner.release)
	<-first
	<-second
	<-third
}

func TestThrottleGuardsStreaming(t *testing.T) {
	inner := newGatedProvider()
	throttle := NewProviderThrottle(1)
	guarded := throttle.Wrap("minimax", inner)

	streamDone := make(chan error, 1)
	go func() {
		streamDone <- guarded.ChatStream(context.Background(), ChatRequest{}, func(delta ChatDelta) error {
			return nil
		})
	}()
	<-inner.enter

	// While the stream holds the slot, a chat call on the same name queues.
	chatDone := startChat(context.Background(), guarded)
	select {
	case r := <-chatDone:
		t.Fatalf("chat must queue behind the streaming call: %+v", r)
	case <-time.After(50 * time.Millisecond):
	}

	close(inner.release)
	if err := <-streamDone; err != nil {
		t.Fatalf("stream failed: %v", err)
	}
	if r := <-chatDone; r.err != nil {
		t.Fatalf("queued chat failed: %v", r.err)
	}
}

func TestEmbeddingPassesThroughGuard(t *testing.T) {
	inner := newGatedProvider()
	throttle := NewProviderThrottle(1)
	guarded := throttle.Wrap("minimax", inner)

	first := startChat(context.Background(), guarded)
	<-inner.enter

	// Embedding is unguarded by design: it completes while the chat slot is held.
	embCtx, embCancel := context.WithTimeout(context.Background(), time.Second)
	defer embCancel()
	if _, err := guarded.GetEmbedding(embCtx, "text"); err != nil {
		t.Fatalf("embedding must not queue behind chat: %v", err)
	}

	close(inner.release)
	<-first
}

var _ LLMProvider = (*gatedProvider)(nil)

// SP-24 R6 penetration test through the config seam: an enabled
// resilience.llm_concurrency must actually wrap the constructed provider and
// serialize its chat calls — a type assertion alone (registration ≠ wiring)
// is not enough.
func TestNewProviderThrottleWiringServesAndLimits(t *testing.T) {
	var mu sync.Mutex
	inFlight, maxInFlight := 0, 0
	firstArrived := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		arrived := inFlight
		mu.Unlock()
		if arrived == 1 {
			select {
			case firstArrived <- struct{}{}:
			default:
			}
		}
		<-release
		mu.Lock()
		inFlight--
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	t.Run("disabled keeps the bare surface", func(t *testing.T) {
		cfg := &config.Config{Agent: config.AgentConfig{
			LLMProvider: "openai_compat", LLMModel: "m", LLMAPIKey: "k", LLMAPIBase: server.URL,
		}}
		if _, ok := NewProvider(cfg).(*ThrottledProvider); ok {
			t.Fatal("disabled throttle must not wrap the provider")
		}
	})

	t.Run("enabled wraps and serializes", func(t *testing.T) {
		cfg := &config.Config{
			Agent: config.AgentConfig{
				LLMProvider: "openai_compat", LLMModel: "m", LLMAPIKey: "k", LLMAPIBase: server.URL,
			},
			Resilience: config.ResilienceConfig{LLMConcurrency: config.LLMConcurrencyConfig{
				Enabled: true, MaxPerProvider: 1,
			}},
		}
		p := NewProvider(cfg)
		if _, ok := p.(*ThrottledProvider); !ok {
			t.Fatal("enabled throttle must wrap the single-provider surface")
		}

		ctx := context.Background()
		first := startChat(ctx, p)
		<-firstArrived // first request holds the only slot inside the server

		second := startChat(ctx, p)
		select {
		case <-second:
			t.Fatal("second chat must queue behind the limit-1 slot")
		case <-time.After(50 * time.Millisecond):
		}

		close(release)
		if r := <-first; r.err != nil {
			t.Fatalf("first chat failed: %v", r.err)
		}
		if r := <-second; r.err != nil || r.resp.Content != "ok" {
			t.Fatalf("queued chat must complete: %+v", r)
		}

		mu.Lock()
		peak := maxInFlight
		mu.Unlock()
		if peak != 1 {
			t.Fatalf("server saw %d concurrent requests, want 1", peak)
		}
	})
}
