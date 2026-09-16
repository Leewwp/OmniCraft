// TurnRecorder is the per-turn instrumentation handle (SP-21 T2, map #549):
// it carries one chat turn's trace identity through the call graph (context
// propagation, so deep seams like the routing provider and the hybrid
// retriever can record without constructor changes), buffers routing events,
// and emits run/node records to the async Writer. A nil *TurnRecorder is
// inert — tracing never changes behavior when disabled or unsampled.
package agenttrace

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"omnicraft/backend/internal/model"
)

// mustMarshal renders v as JSON; the value types marshaled here (routing
// events) cannot fail, so a defensive "null" suffices.
func mustMarshal(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return raw
}

// Node status helpers reused by recorders.
const (
	NodeTypeLLMRound     = "llm_round"
	NodeTypeTool         = "tool"
	NodeTypeRetrieval    = "retrieval"
	NodeTypeRerank       = "rerank"
	NodeTypeTitle        = "title"
	NodeTypeUploadAssist = "upload_assist"
	NodeTypeCompliance   = "compliance"
	NodeTypeGuide        = "guide"
	NodeTypeFollowUps    = "follow_ups"
	NodeTypeModerate     = "moderation"
	NodeTypeClassify     = "classify"
	NodeTypeChitchat     = "chitchat"
	NodeTypeCitations    = "citation_revalidation"
)

// Routing reasons, mirroring the routing chain's deterministic failover
// signals (#545). Duplicated as plain strings so this package stays
// independent of the llm package (which imports it).
const (
	RetryReasonProviderError = "provider_error"
	RetryReasonBlank         = "blank_answer"
)

// RoutingEvent is one deterministic failover decision of the model routing
// chain (#545): which model failed (or went blank), which model took over,
// and why. Error strings are safe to persist (provider keys live in request
// headers, never in errors).
type RoutingEvent struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
	Err    string `json:"err,omitempty"`
}

type recorderKey struct{}

// NewTurnRecorder binds one chat turn to the writer. The sampling decision
// is made once here (Writer.Sampled) and gates every record the recorder
// emits, so runs and nodes stay consistent.
func NewTurnRecorder(w *Writer, traceID string) *TurnRecorder {
	if w == nil || !w.Enabled() || traceID == "" {
		return nil
	}
	if !w.Sampled(traceID) {
		return nil
	}
	return &TurnRecorder{writer: w, traceID: traceID}
}

type TurnRecorder struct {
	writer  *Writer
	traceID string

	routingMu sync.Mutex
	routing   []RoutingEvent

	nodeSeq atomic.Int64
}

// NextNodeKey mints a unique node key within the turn ("prefix_2"), so
// repeated channel calls across tool rounds never collide on the
// (trace_id, node_key) upsert key. Nil-safe (empty).
func (r *TurnRecorder) NextNodeKey(prefix string) string {
	if r == nil {
		return ""
	}
	return prefix + "_" + strconv.FormatInt(r.nodeSeq.Add(1), 10)
}

// TraceID is the OTel trace id shared by the SSE events and the run row.
func (r *TurnRecorder) TraceID() string {
	if r == nil {
		return ""
	}
	return r.traceID
}

// WithTurnRecorder propagates the recorder through the context so seams
// without explicit parameters (routing provider, retriever channels) can
// record nodes.
func WithTurnRecorder(ctx context.Context, r *TurnRecorder) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, recorderKey{}, r)
}

// TurnRecorderFrom extracts the turn recorder; nil when absent (tracing
// off, unsampled, or a call path outside a traced turn).
func TurnRecorderFrom(ctx context.Context) *TurnRecorder {
	if ctx == nil {
		return nil
	}
	r, _ := ctx.Value(recorderKey{}).(*TurnRecorder)
	return r
}

// RecordRouting appends one routing event (nil-safe). Called by the routing
// provider's failover path via TurnRecorderFrom.
func (r *TurnRecorder) RecordRouting(from, to, reason string, err error) {
	if r == nil {
		return
	}
	event := RoutingEvent{From: from, To: to, Reason: reason}
	if err != nil {
		event.Err = err.Error()
	}
	r.routingMu.Lock()
	r.routing = append(r.routing, event)
	r.routingMu.Unlock()
}

// ServingModel returns the model the last routing event handed off to
// ("" when no failover happened — the primary served the request).
func (r *TurnRecorder) ServingModel() string {
	if r == nil {
		return ""
	}
	r.routingMu.Lock()
	defer r.routingMu.Unlock()
	if len(r.routing) == 0 {
		return ""
	}
	return r.routing[len(r.routing)-1].To
}

// RoutingEventsJSON renders the buffered events for the run row's
// routing_events column ([] when none).
func (r *TurnRecorder) RoutingEventsJSON() model.JSONB {
	if r == nil {
		return model.JSONB("[]")
	}
	r.routingMu.Lock()
	defer r.routingMu.Unlock()
	if len(r.routing) == 0 {
		return model.JSONB("[]")
	}
	raw := mustMarshal(r.routing)
	return model.JSONB(raw)
}

// RecordRunStart emits the RUNNING run row (turn admitted, conversation
// resolved).
func (r *TurnRecorder) RecordRunStart(run model.AgentTraceRun) {
	if r == nil {
		return
	}
	run.TraceID = r.traceID
	if run.Status == "" {
		run.Status = model.AgentTraceStatusRunning
	}
	r.writer.RecordRun(run)
}

// RunEnd carries the terminal run fragment fields.
type RunEnd struct {
	Status       string
	ErrorCode    string
	ErrorMessage string
	StartedAt    time.Time
	EndedAt      time.Time
	// FirstDisplayDelta is the user-perceived first forwarded delta moment
	// (polyu USER_TTFT); zero means nothing was displayed.
	FirstDisplayDelta time.Time
	AnswerKind        string
	Model             string
	MessageID         *int64
}

// RecordRunEnd emits the terminal run fragment: status, timing, TTFT,
// answer kind, model, routing events and prompt attribution.
func (r *TurnRecorder) RecordRunEnd(end RunEnd) {
	if r == nil {
		return
	}
	endedAt := end.EndedAt
	if endedAt.IsZero() {
		endedAt = time.Now()
	}
	duration := endedAt.Sub(end.StartedAt).Milliseconds()
	run := model.AgentTraceRun{
		TraceID:       r.traceID,
		Status:        end.Status,
		ErrorCode:     end.ErrorCode,
		ErrorMessage:  end.ErrorMessage,
		StartedAt:     end.StartedAt,
		EndedAt:       &endedAt,
		DurationMs:    &duration,
		AnswerKind:    end.AnswerKind,
		Model:         end.Model,
		RoutingEvents: r.RoutingEventsJSON(),
		MessageID:     end.MessageID,
	}
	if !end.FirstDisplayDelta.IsZero() {
		ttft := end.FirstDisplayDelta.Sub(end.StartedAt).Milliseconds()
		if ttft < 0 {
			ttft = 0
		}
		run.TTFTMs = &ttft
	}
	r.writer.RecordRun(run)
}

// NodeSpan is one in-flight phase node; End completes it.
type NodeSpan struct {
	recorder *TurnRecorder
	node     model.AgentTraceNode
}

// StartNode emits the RUNNING node row first (a crash keeps the
// half-written scene), keyed by nodeKey within the trace. parent nil = a
// top-level phase node (depth 1); passing a parent span nests one level
// deeper (e.g. retrieval channels under their tool node).
func (r *TurnRecorder) StartNode(nodeType, nodeKey string, parent *NodeSpan, modelName string) *NodeSpan {
	if r == nil {
		return nil
	}
	node := model.AgentTraceNode{
		TraceID:   r.traceID,
		NodeKey:   nodeKey,
		NodeType:  nodeType,
		Status:    model.AgentTraceStatusRunning,
		StartedAt: time.Now(),
		Model:     modelName,
		Depth:     1,
	}
	if parent != nil && parent.node.NodeKey != "" {
		key := parent.node.NodeKey
		node.ParentNodeKey = &key
		node.Depth = parent.node.Depth + 1
	}
	r.writer.RecordNode(node)
	return &NodeSpan{recorder: r, node: node}
}

// NodeEndOptions carries terminal node attributes; zero values are skipped
// by the writer's merge semantics.
type NodeEndOptions struct {
	NodeName         string
	Status           string
	ErrorCode        string
	ErrorMessage     string
	PromptDigest     string
	CompletionDigest string
	TokensIn         *int64
	TokensOut        *int64
	Extra            model.JSONB
}

// End completes the span: SUCCESS by default, ERROR/ CANCELLED on failure,
// with duration measured from the span start.
func (ns *NodeSpan) End(opts NodeEndOptions) {
	if ns == nil {
		return
	}
	if opts.Status == "" {
		opts.Status = model.AgentTraceStatusSuccess
	}
	end := time.Now()
	duration := end.Sub(ns.node.StartedAt).Milliseconds()
	node := model.AgentTraceNode{
		TraceID:          ns.node.TraceID,
		NodeKey:          ns.node.NodeKey,
		NodeType:         ns.node.NodeType,
		ParentNodeKey:    ns.node.ParentNodeKey,
		Depth:            ns.node.Depth,
		Status:           opts.Status,
		ErrorCode:        opts.ErrorCode,
		ErrorMessage:     opts.ErrorMessage,
		StartedAt:        ns.node.StartedAt,
		EndedAt:          &end,
		DurationMs:       &duration,
		Model:            ns.node.Model,
		PromptDigest:     opts.PromptDigest,
		CompletionDigest: opts.CompletionDigest,
		TokensIn:         opts.TokensIn,
		TokensOut:        opts.TokensOut,
		Extra:            opts.Extra,
	}
	if opts.NodeName != "" {
		node.NodeName = opts.NodeName
	}
	ns.recorder.writer.RecordNode(node)
}

// EndWithError completes the span as ERROR with a bounded message.
func (ns *NodeSpan) EndWithError(errCode, errMsg string) {
	if ns == nil {
		return
	}
	ns.End(NodeEndOptions{Status: model.AgentTraceStatusError, ErrorCode: errCode, ErrorMessage: errMsg})
}
