// Package agenttrace implements the async batch writer for the self-built
// agent trace persistence (SP-21 T1, wayfinder map #549): agent turns and
// their phase nodes are recorded off the request path through an in-memory
// channel, merged in a bounded buffer (so a start and its terminal update
// collapse into one row when both land before a flush), and flushed to
// PostgreSQL in batches. A full channel drops records and counts them —
// trace persistence must never block or fail an agent turn (the polyu
// synchronous-double-write lesson).
package agenttrace

import (
	"context"
	"hash/fnv"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"omnicraft/backend/internal/model"
)

// Store is the persistence seam the writer depends on; it is satisfied by
// repository.AgentTraceRepository and by test fakes.
type Store interface {
	UpsertRuns(ctx context.Context, runs []model.AgentTraceRun) error
	UpsertNodes(ctx context.Context, nodes []model.AgentTraceNode) error
	PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// Options mirrors observability.agent_trace from config. Invalid values are
// never trusted here: FromOptions normalizes them so a stale config file
// cannot produce a zero-capacity channel or a zero flush interval.
type Options struct {
	Enabled         bool
	SampleRatio     float64
	ChannelSize     int
	FlushInterval   time.Duration
	FlushBatchSize  int
	DigestMaxRunes  int
	KeepFullPrompt  bool
	RetentionDays   int
}

const (
	minChannelSize    = 64
	minFlushInterval = 100 * time.Millisecond
	minFlushBatch    = 10
)

// FromOptions normalizes an Options into a safe working set. Callers map
// config values in; the writer never sees a zero-sized channel or a
// non-positive flush cadence.
func FromOptions(o Options) Options {
	if o.ChannelSize < minChannelSize {
		o.ChannelSize = minChannelSize
	}
	if o.FlushInterval < minFlushInterval {
		o.FlushInterval = minFlushInterval
	}
	if o.FlushBatchSize < minFlushBatch {
		o.FlushBatchSize = minFlushBatch
	}
	if o.DigestMaxRunes < 0 {
		o.DigestMaxRunes = 0
	}
	if o.RetentionDays <= 0 {
		o.RetentionDays = 30
	}
	if o.SampleRatio < 0 {
		o.SampleRatio = 0
	}
	if o.SampleRatio > 1 {
		o.SampleRatio = 1
	}
	return o
}

// Stats is a point-in-time snapshot for tests, logs and later admin/metrics
// exposure. Dropped counts are monotonic.
type Stats struct {
	DroppedRuns   uint64
	DroppedNodes  uint64
	BufferedRuns  int
	BufferedNodes int
	Flushes       uint64
	FlushErrors   uint64
	LastFlushErr  string
	PurgedRows    uint64
}

type record struct {
	run  *model.AgentTraceRun
	node *model.AgentTraceNode
}

type nodeKey struct {
	trace string
	key   string
}

// Writer batches run/node upserts asynchronously. A nil *Writer is valid and
// drops every record, so callers wire it unconditionally and gate with
// config (the DisplayURLSigner nil-safe pattern).
type Writer struct {
	store Store
	opts  Options

	ch       chan record
	stopCh   chan struct{}
	doneCh   chan struct{}
	started  atomic.Bool
	startOne sync.Once
	stopOne  sync.Once

	droppedRuns  atomic.Uint64
	droppedNodes atomic.Uint64
	flushes      atomic.Uint64
	flushErrors  atomic.Uint64
	purgedRows   atomic.Uint64

	mu    sync.Mutex
	runs  map[string]*model.AgentTraceRun
	nodes map[nodeKey]*model.AgentTraceNode
	// recentRuns / recentNodes hold copies of the last flushed rows (bounded
	// by recentCap) so a terminal fragment arriving after its RUNNING row
	// was flushed still merges onto the flushed state instead of upserting
	// a partial row that clobbers the start identity columns.
	recentRuns  map[string]model.AgentTraceRun
	recentNodes map[nodeKey]model.AgentTraceNode
	// lastFlushErr is guarded by mu alongside the buffers.
	lastFlushErr string
	closed       bool
}

func NewWriter(store Store, opts Options) *Writer {
	opts = FromOptions(opts)
	return &Writer{
		store:       store,
		opts:        opts,
		ch:          make(chan record, opts.ChannelSize),
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
		runs:        make(map[string]*model.AgentTraceRun),
		nodes:       make(map[nodeKey]*model.AgentTraceNode),
		recentRuns:  make(map[string]model.AgentTraceRun),
		recentNodes: make(map[nodeKey]model.AgentTraceNode),
	}
}

// Enabled reports whether the writer accepts records.
func (w *Writer) Enabled() bool {
	return w != nil && w.opts.Enabled
}

// Sampled is the deterministic per-trace sampling decision (FNV-1a over the
// trace id, stable across processes): the caller asks once per turn and
// gates every Record call of that trace with the same answer, so runs and
// their nodes are sampled consistently.
func (w *Writer) Sampled(traceID string) bool {
	if w == nil {
		return false
	}
	if w.opts.SampleRatio >= 1 {
		return true
	}
	if w.opts.SampleRatio <= 0 {
		return false
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(traceID))
	return float64(h.Sum32()%10_000) < w.opts.SampleRatio*10_000
}

// RecordRun enqueues one run upsert fragment (start, terminal or any field
// overlay). It returns false when the record was dropped (disabled, closed
// writer, full channel or full buffer); drops are counted, never blocking.
func (w *Writer) RecordRun(run model.AgentTraceRun) bool {
	if w == nil || !w.opts.Enabled {
		return false
	}
	return w.enqueue(record{run: &run}, &w.droppedRuns)
}

// RecordNode enqueues one node upsert fragment with the same drop semantics
// as RecordRun. Digests are truncated here unless KeepFullPrompt is set, so
// no caller can persist unbounded prompt bodies by accident.
func (w *Writer) RecordNode(node model.AgentTraceNode) bool {
	if w == nil || !w.opts.Enabled {
		return false
	}
	if !w.opts.KeepFullPrompt {
		node.PromptDigest = truncateRunes(node.PromptDigest, w.opts.DigestMaxRunes)
		node.CompletionDigest = truncateRunes(node.CompletionDigest, w.opts.DigestMaxRunes)
	}
	return w.enqueue(record{node: &node}, &w.droppedNodes)
}

func (w *Writer) enqueue(rec record, dropped *atomic.Uint64) bool {
	w.mu.Lock()
	closed := w.closed
	buffered := len(w.runs) + len(w.nodes)
	w.mu.Unlock()
	if closed || buffered >= w.opts.ChannelSize {
		dropped.Add(1)
		return false
	}
	select {
	case w.ch <- rec:
		return true
	default:
		dropped.Add(1)
		return false
	}
}

// Start launches the flusher goroutine (idempotent). The context bounds
// flush/purge calls; on ctx.Done or Stop the buffer is drained and flushed
// once, then the goroutine exits.
func (w *Writer) Start(ctx context.Context) {
	if w == nil {
		return
	}
	w.startOne.Do(func() {
		w.started.Store(true)
		go w.run(ctx)
	})
}

// finalFlush drains the channel and persists the buffer once, on a detached
// bounded context: the parent is typically already cancelled at shutdown.
func (w *Writer) finalFlush() {
	w.drain()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w.flush(ctx)
}

// Stop drains the channel into the buffer, flushes once and waits for the
// flusher to exit (bounded by ctx). Safe to call multiple times and on a
// never-started writer, which flushes synchronously instead.
func (w *Writer) Stop(ctx context.Context) {
	if w == nil {
		return
	}
	w.stopOne.Do(func() {
		w.mu.Lock()
		w.closed = true
		w.mu.Unlock()
		close(w.stopCh)
	})
	if !w.started.Load() {
		w.finalFlush()
		return
	}
	select {
	case <-w.doneCh:
	case <-ctx.Done():
		slog.Warn("agent trace writer stop timed out; buffered records may be lost")
	}
}

func (w *Writer) run(parent context.Context) {
	defer close(w.doneCh)
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	w.purge(ctx)
	ticker := time.NewTicker(w.opts.FlushInterval)
	defer ticker.Stop()
	lastPurge := time.Now()
	purgeEvery := 6 * time.Hour
	for {
		select {
		case <-ctx.Done():
			w.finalFlush()
			return
		case <-w.stopCh:
			w.finalFlush()
			return
		case <-ticker.C:
			w.drain()
			w.flush(ctx)
			if time.Since(lastPurge) >= purgeEvery {
				w.purge(ctx)
				lastPurge = time.Now()
			}
		}
	}
}

// drain moves every queued record into the merge buffer without blocking.
func (w *Writer) drain() {
	for {
		select {
		case rec := <-w.ch:
			w.apply(rec)
		default:
			return
		}
	}
}

func (w *Writer) apply(rec record) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// The buffer is the memory bound: past capacity the record is dropped
	// here too, not only at the channel, so a stalled flusher can never grow
	// the merge map without limit.
	if len(w.runs)+len(w.nodes) >= w.opts.ChannelSize {
		if rec.run != nil {
			w.droppedRuns.Add(1)
		} else {
			w.droppedNodes.Add(1)
		}
		return
	}
	if rec.run != nil {
		dst, ok := w.runs[rec.run.TraceID]
		if !ok {
			// Late fragment: resurrect the last flushed snapshot so the
			// merged row stays complete (start identity + terminal fields).
			if prior, seen := w.recentRuns[rec.run.TraceID]; seen {
				cp := prior
				w.runs[cp.TraceID] = &cp
				dst = &cp
			} else {
				cp := *rec.run
				if cp.StartedAt.IsZero() {
					cp.StartedAt = time.Now()
				}
				w.runs[cp.TraceID] = &cp
				return
			}
		}
		mergeRun(dst, rec.run)
	}
	if rec.node != nil {
		key := nodeKey{trace: rec.node.TraceID, key: rec.node.NodeKey}
		dst, ok := w.nodes[key]
		if !ok {
			if prior, seen := w.recentNodes[key]; seen {
				cp := prior
				w.nodes[key] = &cp
				dst = &cp
			} else {
				cp := *rec.node
				if cp.StartedAt.IsZero() {
					cp.StartedAt = time.Now()
				}
				w.nodes[key] = &cp
				return
			}
		}
		mergeNode(dst, rec.node)
	}
}

// mergeRun overlays the non-zero fields of src onto dst: a terminal record
// patches a buffered RUNNING row in place, so both orders collapse to one
// coherent row per trace before any flush.
func mergeRun(dst, src *model.AgentTraceRun) {
	if src.ConversationID != nil {
		dst.ConversationID = src.ConversationID
	}
	if src.MessageID != nil {
		dst.MessageID = src.MessageID
	}
	if src.UserID != nil {
		dst.UserID = src.UserID
	}
	if src.Surface != "" {
		dst.Surface = src.Surface
	}
	if src.Status != "" {
		dst.Status = src.Status
	}
	if src.ErrorCode != "" {
		dst.ErrorCode = src.ErrorCode
	}
	if src.ErrorMessage != "" {
		dst.ErrorMessage = src.ErrorMessage
	}
	if !src.StartedAt.IsZero() {
		dst.StartedAt = src.StartedAt
	}
	if src.EndedAt != nil {
		dst.EndedAt = src.EndedAt
	}
	if src.DurationMs != nil {
		dst.DurationMs = src.DurationMs
	}
	if src.TTFTMs != nil {
		dst.TTFTMs = src.TTFTMs
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.AnswerKind != "" {
		dst.AnswerKind = src.AnswerKind
	}
	if len(src.RoutingEvents) > 4 { // more than "[]" / "null"
		dst.RoutingEvents = src.RoutingEvents
	}
	if src.PromptName != "" {
		dst.PromptName = src.PromptName
	}
	if src.PromptVersion != nil {
		dst.PromptVersion = src.PromptVersion
	}
	if len(src.Extra) > 2 { // more than "{}" / "null"
		dst.Extra = src.Extra
	}
}

func mergeNode(dst, src *model.AgentTraceNode) {
	if src.ParentNodeKey != nil {
		dst.ParentNodeKey = src.ParentNodeKey
	}
	if src.Depth != 0 {
		dst.Depth = src.Depth
	}
	if src.NodeType != "" {
		dst.NodeType = src.NodeType
	}
	if src.NodeName != "" {
		dst.NodeName = src.NodeName
	}
	if src.Status != "" {
		dst.Status = src.Status
	}
	if src.ErrorCode != "" {
		dst.ErrorCode = src.ErrorCode
	}
	if src.ErrorMessage != "" {
		dst.ErrorMessage = src.ErrorMessage
	}
	if !src.StartedAt.IsZero() {
		dst.StartedAt = src.StartedAt
	}
	if src.EndedAt != nil {
		dst.EndedAt = src.EndedAt
	}
	if src.DurationMs != nil {
		dst.DurationMs = src.DurationMs
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.PromptDigest != "" {
		dst.PromptDigest = src.PromptDigest
	}
	if src.CompletionDigest != "" {
		dst.CompletionDigest = src.CompletionDigest
	}
	if src.TokensIn != nil {
		dst.TokensIn = src.TokensIn
	}
	if src.TokensOut != nil {
		dst.TokensOut = src.TokensOut
	}
	if src.CostEstimate != nil {
		dst.CostEstimate = src.CostEstimate
	}
	if len(src.Extra) > 2 {
		dst.Extra = src.Extra
	}
}

// flush snapshots the merge buffer and writes it through the store. A failed
// flush is logged and counted, then dropped: retrying would grow memory and
// observability data is not worth failing anything for. Flushed rows are
// remembered (bounded) so late terminal fragments merge onto them.
func (w *Writer) flush(ctx context.Context) {
	w.mu.Lock()
	runs := make([]model.AgentTraceRun, 0, len(w.runs))
	for _, r := range w.runs {
		runs = append(runs, *r)
	}
	nodes := make([]model.AgentTraceNode, 0, len(w.nodes))
	for _, n := range w.nodes {
		nodes = append(nodes, *n)
	}
	clear(w.runs)
	clear(w.nodes)
	recentCap := 2 * w.opts.ChannelSize
	if len(w.recentRuns)+len(runs) > recentCap {
		clear(w.recentRuns)
	}
	if len(w.recentNodes)+len(nodes) > recentCap {
		clear(w.recentNodes)
	}
	for _, r := range runs {
		w.recentRuns[r.TraceID] = r
	}
	for _, n := range nodes {
		w.recentNodes[nodeKey{trace: n.TraceID, key: n.NodeKey}] = n
	}
	w.mu.Unlock()
	if len(runs) == 0 && len(nodes) == 0 {
		return
	}

	if len(runs) > 0 {
		if err := w.store.UpsertRuns(ctx, runs); err != nil {
			w.flushFailed("runs", err)
		}
	}
	if len(nodes) > 0 {
		if err := w.store.UpsertNodes(ctx, nodes); err != nil {
			w.flushFailed("nodes", err)
		}
	}
	w.flushes.Add(1)
}

func (w *Writer) flushFailed(kind string, err error) {
	w.flushErrors.Add(1)
	w.mu.Lock()
	w.lastFlushErr = err.Error()
	w.mu.Unlock()
	slog.Warn("agent trace flush dropped a batch",
		"kind", kind, "policy", "drop", "error", err.Error())
}

func (w *Writer) purge(ctx context.Context) {
	if w.store == nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -w.opts.RetentionDays)
	removed, err := w.store.PurgeBefore(ctx, cutoff)
	if err != nil {
		slog.Warn("agent trace retention purge failed", "error", err.Error())
		return
	}
	if removed > 0 {
		w.purgedRows.Add(uint64(removed))
		slog.Info("agent trace retention purge", "removed_rows", removed)
	}
}

// Stats snapshots writer counters.
func (w *Writer) Stats() Stats {
	if w == nil {
		return Stats{}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return Stats{
		DroppedRuns:   w.droppedRuns.Load(),
		DroppedNodes:  w.droppedNodes.Load(),
		BufferedRuns:  len(w.runs),
		BufferedNodes: len(w.nodes),
		Flushes:       w.flushes.Load(),
		FlushErrors:   w.flushErrors.Load(),
		LastFlushErr:  w.lastFlushErr,
		PurgedRows:    w.purgedRows.Load(),
	}
}

// truncateRunes bounds a digest to max runes; 0 max means "no digest".
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
