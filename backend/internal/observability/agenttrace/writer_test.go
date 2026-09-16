package agenttrace

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"omnicraft/backend/internal/model"
)

// fakeStore captures upserts keyed by trace_id / (trace_id, node_key) so the
// latest record per key is inspectable, like the ON CONFLICT table state.
type fakeStore struct {
	mu       sync.Mutex
	runs     map[string]model.AgentTraceRun
	nodes    map[nodeKey]model.AgentTraceNode
	runCalls int
	err      error
	purged   int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{runs: map[string]model.AgentTraceRun{}, nodes: map[nodeKey]model.AgentTraceNode{}}
}

func (f *fakeStore) UpsertRuns(_ context.Context, runs []model.AgentTraceRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runCalls++
	if f.err != nil {
		return f.err
	}
	for _, r := range runs {
		f.runs[r.TraceID] = r
	}
	return nil
}

func (f *fakeStore) UpsertNodes(_ context.Context, nodes []model.AgentTraceNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	for _, n := range nodes {
		f.nodes[nodeKey{trace: n.TraceID, key: n.NodeKey}] = n
	}
	return nil
}

func (f *fakeStore) PurgeBefore(_ context.Context, _ time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.purged, nil
}

func testOptions() Options {
	return Options{
		Enabled:        true,
		SampleRatio:    1,
		ChannelSize:    128,
		FlushInterval:  200 * time.Millisecond,
		FlushBatchSize: 50,
		DigestMaxRunes: 500,
		RetentionDays:  30,
	}
}

func runStart(trace string) model.AgentTraceRun {
	return model.AgentTraceRun{
		TraceID:   trace,
		Status:    model.AgentTraceStatusRunning,
		StartedAt: time.Now().Add(-2 * time.Second),
		UserID:    int64Ptr(7),
		Surface:   "global",
	}
}

func runEnd(trace string, status string) model.AgentTraceRun {
	end := time.Now()
	dur := int64(1500)
	ttft := int64(480)
	return model.AgentTraceRun{
		TraceID:     trace,
		Status:      status,
		EndedAt:     &end,
		DurationMs:  &dur,
		TTFTMs:      &ttft,
		AnswerKind:  "grounded_content",
		Model:       "minimax-m3",
		PromptName:  "agent_system",
	}
}

func nodeStart(trace, key string) model.AgentTraceNode {
	return model.AgentTraceNode{
		TraceID:   trace,
		NodeKey:   key,
		NodeType:  "llm",
		Status:    model.AgentTraceStatusRunning,
		StartedAt: time.Now().Add(-1 * time.Second),
	}
}

func nodeEnd(trace, key string) model.AgentTraceNode {
	end := time.Now()
	dur := int64(900)
	tin, tout := int64(1200), int64(350)
	return model.AgentTraceNode{
		TraceID:         trace,
		NodeKey:         key,
		Status:          model.AgentTraceStatusSuccess,
		EndedAt:         &end,
		DurationMs:      &dur,
		Model:           "minimax-m3",
		PromptDigest:    "system prompt digest",
		CompletionDigest: "answer digest",
		TokensIn:        &tin,
		TokensOut:       &tout,
	}
}

func int64Ptr(v int64) *int64 { return &v }

// TestWriterMergesStartAndTerminalBeforeFlush: a start and its terminal
// update landing before a flush must collapse into ONE buffered row carrying
// both start identity (user/surface/started_at) and terminal fields.
func TestWriterMergesStartAndTerminalBeforeFlush(t *testing.T) {
	store := newFakeStore()
	w := NewWriter(store, testOptions())

	if !w.RecordRun(runStart("t1")) {
		t.Fatal("run start dropped")
	}
	if !w.RecordRun(runEnd("t1", model.AgentTraceStatusSuccess)) {
		t.Fatal("run end dropped")
	}
	if !w.RecordNode(nodeStart("t1", "llm_round_1")) {
		t.Fatal("node start dropped")
	}
	if !w.RecordNode(nodeEnd("t1", "llm_round_1")) {
		t.Fatal("node end dropped")
	}
	// Records sit in the channel until the flusher tick drains them into the
	// merge buffer; drive that step explicitly.
	w.drain()

	st := w.Stats()
	if st.BufferedRuns != 1 || st.BufferedNodes != 1 {
		t.Fatalf("buffered = (%d runs, %d nodes), want (1, 1)", st.BufferedRuns, st.BufferedNodes)
	}

	w.finalFlush()
	got := store.runs["t1"]
	if got.Status != model.AgentTraceStatusSuccess {
		t.Fatalf("merged run status = %q, want SUCCESS", got.Status)
	}
	if got.UserID == nil || *got.UserID != 7 {
		t.Fatal("merged run lost start identity (user_id)")
	}
	if got.Surface != "global" {
		t.Fatal("merged run lost start surface")
	}
	if got.TTFTMs == nil || *got.TTFTMs != 480 {
		t.Fatal("merged run lost ttft_ms")
	}
	if got.AnswerKind != "grounded_content" {
		t.Fatal("merged run lost answer_kind")
	}
	node := store.nodes[nodeKey{trace: "t1", key: "llm_round_1"}]
	if node.Status != model.AgentTraceStatusSuccess || node.TokensOut == nil || *node.TokensOut != 350 {
		t.Fatalf("merged node = %+v", node)
	}
	if store.runCalls != 1 {
		t.Fatalf("flush call count = %d, want 1 (single merged batch)", store.runCalls)
	}
}

// TestWriterTerminalUpdateAfterFlush: when the RUNNING row was already
// flushed, the terminal fragment must merge onto the remembered flushed
// snapshot — the store receives a COMPLETE row (start identity + terminal
// fields), never a partial one that would clobber identity columns.
func TestWriterTerminalUpdateAfterFlush(t *testing.T) {
	store := newFakeStore()
	w := NewWriter(store, testOptions())

	w.RecordRun(runStart("t2"))
	w.finalFlush()
	if got := store.runs["t2"]; got.Status != model.AgentTraceStatusRunning {
		t.Fatalf("first flush status = %q, want RUNNING", got.Status)
	}

	w.RecordRun(runEnd("t2", model.AgentTraceStatusError))
	w.finalFlush()
	got := store.runs["t2"]
	if got.Status != model.AgentTraceStatusError {
		t.Fatalf("terminal flush status = %q, want ERROR", got.Status)
	}
	if got.UserID == nil || *got.UserID != 7 || got.Surface != "global" {
		t.Fatalf("terminal fragment must merge onto the flushed start identity: %+v", got)
	}
	if got.TTFTMs == nil || *got.TTFTMs != 480 {
		t.Fatalf("terminal fields missing after merge: %+v", got)
	}
	if got.StartedAt.IsZero() {
		t.Fatal("merged row lost started_at")
	}

	// Same guarantee for nodes.
	w.RecordNode(nodeStart("t2", "tool_search_1"))
	w.finalFlush()
	w.RecordNode(func() model.AgentTraceNode {
		n := nodeEnd("t2", "tool_search_1")
		n.NodeType = "tool"
		return n
	}())
	w.finalFlush()
	node := store.nodes[nodeKey{trace: "t2", key: "tool_search_1"}]
	if node.Status != model.AgentTraceStatusSuccess || node.NodeType != "tool" || node.TokensIn == nil {
		t.Fatalf("late node fragment not merged onto flushed snapshot: %+v", node)
	}
}

// TestWriterBackpressureDropsAndCounts: a full channel must never block the
// caller; excess records are counted as dropped.
func TestWriterBackpressureDropsAndCounts(t *testing.T) {
	store := newFakeStore()
	opts := testOptions()
	opts.ChannelSize = 128
	w := NewWriter(store, opts)

	accepted := 0
	for i := 0; i < 500; i++ {
		if w.RecordRun(runStart(fmt.Sprintf("drop-%d", i))) {
			accepted++
		}
	}
	if accepted != 128 {
		t.Fatalf("accepted = %d, want 128 (channel capacity)", accepted)
	}
	st := w.Stats()
	if st.DroppedRuns != 372 {
		t.Fatalf("dropped_runs = %d, want 372", st.DroppedRuns)
	}
}

// TestWriterBufferCapDropsNewRecords: when the merge buffer is at capacity
// (records enqueued while the flusher is not draining), new records are
// dropped rather than growing memory without bound — both at the enqueue
// gate and inside apply (the drain funnel).
func TestWriterBufferCapDropsNewRecords(t *testing.T) {
	store := newFakeStore()
	opts := testOptions()
	opts.ChannelSize = 64
	w := NewWriter(store, opts)

	// Fill the merge buffer exactly to capacity.
	for i := 0; i < 64; i++ {
		w.apply(record{run: ptr(runStart(fmt.Sprintf("cap-%d", i)))})
	}
	if w.RecordRun(runStart("cap-overflow")) {
		t.Fatal("record accepted with full merge buffer, want dropped")
	}
	// A record draining from the channel hits the same bound.
	w.apply(record{node: ptr(nodeStart("cap-x", "n1"))})
	st := w.Stats()
	if st.DroppedRuns != 1 || st.DroppedNodes != 1 {
		t.Fatalf("drops = (%d runs, %d nodes), want (1, 1)", st.DroppedRuns, st.DroppedNodes)
	}
}

func ptr[T any](v T) *T { return &v }

// TestWriterDisabledDropsEverything: disabled writers (and nil writers)
// reject every record and never panic.
func TestWriterDisabledDropsEverything(t *testing.T) {
	w := NewWriter(newFakeStore(), Options{Enabled: false})
	if w.RecordRun(runStart("x")) || w.RecordNode(nodeStart("x", "n")) {
		t.Fatal("disabled writer accepted a record")
	}
	var nilWriter *Writer
	if nilWriter.RecordRun(runStart("x")) || nilWriter.Enabled() || nilWriter.Sampled("x") {
		t.Fatal("nil writer must be inert")
	}
	nilWriter.Start(context.Background())
	nilWriter.Stop(context.Background())
}

// TestWriterDigestTruncation: digests are bounded to DigestMaxRunes unless
// KeepFullPrompt is set; max 0 stores no digest at all.
func TestWriterDigestTruncation(t *testing.T) {
	store := newFakeStore()
	opts := testOptions()
	opts.DigestMaxRunes = 10
	w := NewWriter(store, opts)

	longRunes := make([]rune, 100)
	for i := range longRunes {
		longRunes[i] = '字'
	}
	long := string(longRunes)
	n := nodeStart("t3", "n1")
	n.PromptDigest = long
	n.CompletionDigest = long
	w.RecordNode(n)
	w.finalFlush()

	got := store.nodes[nodeKey{trace: "t3", key: "n1"}]
	if runeCount := len([]rune(got.PromptDigest)); runeCount != 10 {
		t.Fatalf("prompt digest runes = %d, want 10", runeCount)
	}
	if got.CompletionDigest != string([]rune(long)[:10]) {
		t.Fatal("completion digest not truncated")
	}

	full := NewWriter(newFakeStore(), func() Options {
		o := testOptions()
		o.DigestMaxRunes = 10
		o.KeepFullPrompt = true
		return o
	}())
	fullStore := full.store.(*fakeStore)
	n2 := nodeStart("t4", "n1")
	n2.PromptDigest = long
	full.RecordNode(n2)
	full.finalFlush()
	if got := fullStore.nodes[nodeKey{trace: "t4", key: "n1"}]; len([]rune(got.PromptDigest)) != 100 {
		t.Fatal("KeepFullPrompt must bypass truncation")
	}
}

// TestWriterSamplingDeterministic: the sampling decision is stable per trace
// id and honors the ratio bounds.
func TestWriterSamplingDeterministic(t *testing.T) {
	all := NewWriter(newFakeStore(), testOptions())
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("trace-%d", i)
		if !all.Sampled(id) {
			t.Fatalf("ratio 1.0 must sample everything (%s)", id)
		}
	}
	none := NewWriter(newFakeStore(), func() Options {
		o := testOptions()
		o.SampleRatio = 0
		return o
	}())
	if none.Sampled("anything") {
		t.Fatal("ratio 0 must sample nothing")
	}
	half := NewWriter(newFakeStore(), func() Options {
		o := testOptions()
		o.SampleRatio = 0.5
		return o
	}())
	sampled := 0
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("half-%d", i)
		a, b := half.Sampled(id), half.Sampled(id)
		if a != b {
			t.Fatalf("sampling must be deterministic per trace id (%s)", id)
		}
		if a {
			sampled++
		}
	}
	if sampled == 0 || sampled == 1000 {
		t.Fatalf("ratio 0.5 sampled %d/1000, want a mix", sampled)
	}
}

// TestWriterFlushErrorCountedAndDropped: a failing store must not wedge the
// writer; the batch is counted and dropped.
func TestWriterFlushErrorCountedAndDropped(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("db down")
	w := NewWriter(store, testOptions())

	w.RecordRun(runStart("t5"))
	w.finalFlush()

	st := w.Stats()
	if st.FlushErrors != 1 {
		t.Fatalf("flush_errors = %d, want 1", st.FlushErrors)
	}
	if st.BufferedRuns != 0 {
		t.Fatalf("failed batch kept buffered (buffered=%d), want dropped", st.BufferedRuns)
	}
	if st.LastFlushErr == "" {
		t.Fatal("last flush error not recorded")
	}
}

// TestWriterStopFlushesRemainder: Stop drains and persists whatever is still
// buffered, on both started and never-started writers.
func TestWriterStopFlushesRemainder(t *testing.T) {
	store := newFakeStore()
	w := NewWriter(store, testOptions())
	w.Start(context.Background())
	w.RecordRun(runStart("t6"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	w.Stop(ctx)
	if got := store.runs["t6"]; got.TraceID != "t6" {
		t.Fatal("started writer did not flush on stop")
	}

	store2 := newFakeStore()
	w2 := NewWriter(store2, testOptions())
	w2.RecordRun(runStart("t7"))
	w2.Stop(ctx)
	if got := store2.runs["t7"]; got.TraceID != "t7" {
		t.Fatal("never-started writer did not flush on stop")
	}
}

// TestOptionsNormalization: invalid options cannot produce a zero channel or
// zero cadence.
func TestOptionsNormalization(t *testing.T) {
	o := FromOptions(Options{Enabled: true, ChannelSize: 1, FlushInterval: time.Nanosecond, FlushBatchSize: 0, RetentionDays: -3, SampleRatio: 5})
	if o.ChannelSize < minChannelSize || o.FlushInterval < minFlushInterval || o.FlushBatchSize < minFlushBatch {
		t.Fatalf("unsafe options survived: %+v", o)
	}
	if o.SampleRatio != 1 || o.RetentionDays != 30 {
		t.Fatalf("bounds not clamped: %+v", o)
	}
}
