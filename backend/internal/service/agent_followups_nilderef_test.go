package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability/agenttrace"
	"omnicraft/backend/internal/pkg/llm"
)

// SP-21 T8 live-smoke regression: when the follow-up side call returns an
// error (nil response), the deferred node End dereferenced resp.Content and
// panicked inside GoSafe — the node then stayed RUNNING forever. The node
// must close as ERROR and the call must return nil.
func TestGenerateFollowUpsClosesNodeOnProviderError(t *testing.T) {
	store := &nilderefStore{}
	w := agenttrace.NewWriter(store, agenttrace.Options{
		Enabled:        true,
		SampleRatio:    1,
		ChannelSize:    64,
		FlushInterval:  50 * time.Millisecond,
		FlushBatchSize: 10,
	})
	w.Start(context.Background())
	defer w.Stop(context.Background())

	rec := agenttrace.NewTurnRecorder(w, "trace-nilderef-1")
	provider := &followUpTestProvider{chatErr: errors.New("provider down")}

	items := generateFollowUps(context.Background(), rec, nil, provider, "trace-nilderef-1", "问题", nil, "前缀")

	if items != nil {
		t.Fatalf("expected nil items on provider error, got %v", items)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if n := store.lookupNode("follow_ups"); n != nil && n.Status == model.AgentTraceStatusError {
			if n.ErrorCode != "follow_ups_call_failed" {
				t.Fatalf("error code = %q, want follow_ups_call_failed", n.ErrorCode)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("follow_ups node never closed as ERROR (stuck RUNNING or lost)")
}

type nilderefStore struct {
	mu    sync.Mutex
	nodes []model.AgentTraceNode
}

func (s *nilderefStore) UpsertRuns(_ context.Context, _ []model.AgentTraceRun) error { return nil }

func (s *nilderefStore) UpsertNodes(_ context.Context, nodes []model.AgentTraceNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes = append(s.nodes, nodes...)
	return nil
}

func (s *nilderefStore) PurgeBefore(_ context.Context, _ time.Time) (int64, error) { return 0, nil }

func (s *nilderefStore) lookupNode(key string) *model.AgentTraceNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	var last *model.AgentTraceNode
	for i := range s.nodes {
		if s.nodes[i].NodeKey == key {
			last = &s.nodes[i]
		}
	}
	return last
}

var _ agenttrace.Store = (*nilderefStore)(nil)
var _ llm.LLMProvider = (*followUpTestProvider)(nil)
