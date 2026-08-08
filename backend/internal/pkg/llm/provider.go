package llm

import (
	"context"
	"encoding/json"
	"time"
)

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// MarshalJSON renders tools in the OpenAI-compatible wire format
// {"type":"function","function":{...}}. OpenAI, DeepSeek and other
// openai_compat providers reject the legacy flat shape.
func (t ToolDefinition) MarshalJSON() ([]byte, error) {
	type functionShape struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  map[string]interface{} `json:"parameters"`
	}
	return json.Marshal(struct {
		Type     string        `json:"type"`
		Function functionShape `json:"function"`
	}{
		Type:     "function",
		Function: functionShape{Name: t.Name, Description: t.Description, Parameters: t.Parameters},
	})
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
	// Index marks streamed tool-call fragments. OpenAI-style providers split
	// one call across deltas by index; a nil Index means a complete call.
	Index *int `json:"index,omitempty"`
}

type ChatRequest struct {
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

type ChatResponse struct {
	Content   string      `json:"content"`
	ToolCalls []ToolCall  `json:"tool_calls,omitempty"`
	Usage     *TokenUsage `json:"usage,omitempty"`
}

// TokenUsage carries observed token counts. It is an observability value used
// for alerts, not a monetary ledger.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type ChatDelta struct {
	Content   string      `json:"content,omitempty"`
	ToolCalls []ToolCall  `json:"tool_calls,omitempty"`
	Usage     *TokenUsage `json:"usage,omitempty"`
	Done      bool        `json:"done"`
}

type LLMProvider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest, handler func(delta ChatDelta) error) error
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

// providerConfig carries bounded timeout/retry tuning read from
// cfg.Agent.ProviderTimeoutSec / ProviderMaxRetries.
type providerConfig struct {
	timeout    time.Duration
	maxRetries int
}

// ProviderOption configures timeout and retry behavior of a concrete provider.
type ProviderOption func(*providerConfig)

// WithTimeout sets the per-attempt HTTP timeout (cfg.Agent.ProviderTimeoutSec).
func WithTimeout(timeout time.Duration) ProviderOption {
	return func(c *providerConfig) { c.timeout = timeout }
}

// WithMaxRetries sets the number of retries for retryable network/429/5xx
// conditions (cfg.Agent.ProviderMaxRetries).
func WithMaxRetries(n int) ProviderOption {
	return func(c *providerConfig) { c.maxRetries = n }
}
