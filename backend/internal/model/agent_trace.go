package model

import "time"

// Agent trace persistence (SP-21 T1, map #549): rows are written by the
// async batch writer in internal/observability/agenttrace, never on the
// request path. Runs key on the OTel trace id (the same id the SSE events
// carry); nodes key on (trace_id, node_key) so a RUNNING insert followed by a
// terminal update is one idempotent upsert across flushes.

// Terminal-state constants shared by runs and nodes. RUNNING marks an
// in-flight (or crashed-before-report) unit.
const (
	AgentTraceStatusRunning  = "RUNNING"
	AgentTraceStatusSuccess  = "SUCCESS"
	AgentTraceStatusError    = "ERROR"
	AgentTraceStatusCanceled = "CANCELLED"
)

type AgentTraceRun struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	// TraceID is the OTel trace id of the HTTP request that drove the turn;
	// the unique upsert key for run start/terminal updates.
	TraceID string `gorm:"column:trace_id;type:varchar(64);not null;uniqueIndex:uq_agent_trace_runs_trace_id" json:"trace_id"`
	// ConversationID / MessageID / UserID are nullable links for the admin
	// list filters and the conversation -> trace drill-down. Nullable by
	// design: telemetry must not fail when a link is not yet known.
	ConversationID *int64 `json:"conversation_id,omitempty"`
	MessageID      *int64 `json:"message_id,omitempty"`
	UserID         *int64 `json:"user_id,omitempty"`
	Surface        string `gorm:"size:32;not null;default:''" json:"surface"`
	Status         string `gorm:"size:16;not null;default:'RUNNING'" json:"status"`
	ErrorCode      string `gorm:"size:64;not null;default:''" json:"error_code"`
	ErrorMessage   string `gorm:"type:text;not null;default:''" json:"error_message"`
	StartedAt      time.Time `gorm:"not null" json:"started_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	DurationMs     *int64     `json:"duration_ms,omitempty"`
	// TTFTMs is the user-perceived time to first answer delta (polyu
	// USER_TTFT promoted to a first-class column, not an extra blob key).
	TTFTMs        *int64 `gorm:"column:ttft_ms" json:"ttft_ms,omitempty"`
	Model         string `gorm:"size:100;not null;default:''" json:"model"`
	AnswerKind    string `gorm:"size:32;not null;default:''" json:"answer_kind"`
	RoutingEvents JSONB  `gorm:"type:jsonb;not null;default:'[]'" json:"routing_events"`
	PromptName    string `gorm:"size:100;not null;default:''" json:"prompt_name"`
	PromptVersion *int   `json:"prompt_version,omitempty"`
	Extra         JSONB  `gorm:"type:jsonb;not null;default:'{}'" json:"extra"`
}

func (AgentTraceRun) TableName() string { return "agent_trace_runs" }

type AgentTraceNode struct {
	ID      int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	TraceID   string `gorm:"column:trace_id;type:varchar(64);not null;uniqueIndex:uq_agent_trace_nodes_key,priority:1" json:"trace_id"`
	// NodeKey is the writer-side stable identity within one trace (e.g.
	// "llm_round_2", "tool_search_0"); with TraceID it forms the upsert key.
	NodeKey string `gorm:"type:varchar(96);not null;uniqueIndex:uq_agent_trace_nodes_key,priority:2" json:"node_key"`
	// ParentNodeKey references another node_key of the same trace (empty for
	// roots); depth is the tree level for the waterfall view.
	ParentNodeKey *string `gorm:"type:varchar(96)" json:"parent_node_key,omitempty"`
	Depth         int     `gorm:"not null;default:0" json:"depth"`
	NodeType      string  `gorm:"size:32;not null" json:"node_type"`
	NodeName      string  `gorm:"size:100;not null;default:''" json:"node_name"`
	Status        string  `gorm:"size:16;not null;default:'RUNNING'" json:"status"`
	ErrorCode     string  `gorm:"size:64;not null;default:''" json:"error_code"`
	ErrorMessage  string  `gorm:"type:text;not null;default:''" json:"error_message"`
	StartedAt     time.Time `gorm:"not null" json:"started_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	DurationMs    *int64     `json:"duration_ms,omitempty"`
	Model         string     `gorm:"size:100;not null;default:''" json:"model"`
	// PromptDigest / CompletionDigest are server-truncated summaries (or
	// full bodies when keep_full_prompt is enabled); raw prompt text is
	// never stored by default.
	PromptDigest     string   `gorm:"type:text;not null;default:''" json:"prompt_digest"`
	CompletionDigest string   `gorm:"type:text;not null;default:''" json:"completion_digest"`
	TokensIn         *int64   `json:"tokens_in,omitempty"`
	TokensOut        *int64   `json:"tokens_out,omitempty"`
	CostEstimate     *float64 `json:"cost_estimate,omitempty"`
	Extra            JSONB    `gorm:"type:jsonb;not null;default:'{}'" json:"extra"`
}

func (AgentTraceNode) TableName() string { return "agent_trace_nodes" }
