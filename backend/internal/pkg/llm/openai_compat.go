package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"omnicraft/backend/internal/observability"
)

type OpenAICompatProvider struct {
	apiKey       string
	apiBase      string
	embedAPIBase string
	embedGroupID string
	model        string
	embedModel   string
	client       *http.Client
	maxRetries   int
	system       string
}

func NewOpenAICompatProvider(apiKey, apiBase, model, embedModel string, opts ...ProviderOption) *OpenAICompatProvider {
	if apiBase == "" {
		apiBase = "https://api.openai.com"
	}
	apiBase = strings.TrimRight(apiBase, "/")
	cfg := providerConfig{timeout: 60 * time.Second, maxRetries: 2}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &OpenAICompatProvider{
		apiKey:       apiKey,
		apiBase:      apiBase,
		embedAPIBase: strings.TrimRight(defaultString(cfg.embeddingAPIBase, apiBase), "/"),
		embedGroupID: cfg.embeddingGroupID,
		model:        model,
		embedModel:   embedModel,
		client:       &http.Client{Timeout: cfg.timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
		maxRetries:   cfg.maxRetries,
		system:       "openai_compatible",
	}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

type openAIRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
	// StreamOptions is nil for non-streaming and for providers that do not
	// opt in; the omitempty keeps their request bodies byte-identical.
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

// streamOptions carries the OpenAI-compatible stream_options request field.
type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// openAIMessage is the shared message/delta shape of the OpenAI-compatible
// chat completion response.
type openAIMessage struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ReasoningContent is the OpenAI-compatible reasoning channel used by
	// DeepSeek-R1/Qwen3-thinking style providers.
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type openAIChoice struct {
	Message      openAIMessage `json:"message"`
	Delta        openAIMessage `json:"delta"`
	FinishReason string        `json:"finish_reason"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Model reports the configured chat model (used by the opt-in agent-smoke
// release-evidence command).
func (p *OpenAICompatProvider) Model() string { return p.model }

func (p *OpenAICompatProvider) doPost(ctx context.Context, path string, body interface{}) (resp *http.Response, started time.Time, err error) {
	return p.doPostAt(ctx, p.apiBase, path, body)
}

func (p *OpenAICompatProvider) doPostAt(ctx context.Context, base, path string, body interface{}) (resp *http.Response, started time.Time, err error) {
	started = time.Now()
	b, err := json.Marshal(body)
	if err != nil {
		return nil, started, err
	}
	resp, err = retryDo(ctx, p.client, strings.TrimRight(base, "/")+path, p.apiKey, b, p.maxRetries)
	return resp, started, err
}

func (p *OpenAICompatProvider) Chat(ctx context.Context, req ChatRequest) (response *ChatResponse, err error) {
	ctx, span := startLLMSpan(ctx, "chat", p.model, p.system, req.Temperature)
	defer func() {
		var usage *TokenUsage
		if response != nil {
			usage = response.Usage
		}
		finishLLMSpan(span, err, usage)
	}()
	payload := openAIRequest{
		Model:       p.model,
		Messages:    req.Messages,
		Tools:       req.Tools,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	resp, started, err := p.doPost(ctx, "/v1/chat/completions", payload)
	defer func() { observability.ObserveExternalCall("llm", started, err) }()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai api error %d", resp.StatusCode)
	}
	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	usage := usageFromRaw(result.Usage.PromptTokens, result.Usage.CompletionTokens)
	return &ChatResponse{
		Content:   result.Choices[0].Message.Content,
		ToolCalls: result.Choices[0].Message.ToolCalls,
		Usage:     usage,
	}, nil
}

func (p *OpenAICompatProvider) ChatStream(ctx context.Context, req ChatRequest, handler func(delta ChatDelta) error) (err error) {
	ctx, span := startLLMSpan(ctx, "chat.stream", p.model, p.system, req.Temperature)
	var lastUsage *TokenUsage
	defer func() { finishLLMSpan(span, err, lastUsage) }()
	payload := openAIRequest{
		Model:       p.model,
		Messages:    req.Messages,
		Tools:       req.Tools,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}
	resp, started, err := p.doPost(ctx, "/v1/chat/completions", payload)
	defer func() { observability.ObserveExternalCall("llm", started, err) }()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openai api error %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	finished := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			finished = true
			return handler(ChatDelta{Done: true})
		}
		var chunk openAIResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := ChatDelta{
			Content:   chunk.Choices[0].Delta.Content,
			Thinking:  chunk.Choices[0].Delta.ReasoningContent,
			ToolCalls: chunk.Choices[0].Delta.ToolCalls,
			Usage:     usageFromRaw(chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens),
			Done:      chunk.Choices[0].FinishReason == "stop",
		}
		if delta.Usage != nil {
			lastUsage = delta.Usage
		}
		if err := handler(delta); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !finished {
		return fmt.Errorf("openai stream ended before completion")
	}
	return nil
}

func usageFromRaw(prompt, completion int) *TokenUsage {
	if prompt == 0 && completion == 0 {
		return nil
	}
	return &TokenUsage{PromptTokens: prompt, CompletionTokens: completion}
}

type openAIEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (p *OpenAICompatProvider) GetEmbedding(ctx context.Context, text string) (embedding []float32, err error) {
	ctx, span := startLLMSpan(ctx, "embedding", p.embedModel, p.system, 0)
	defer func() { finishLLMSpan(span, err, nil) }()
	payload := openAIEmbeddingRequest{Model: p.embedModel, Input: text}
	path := "/v1/embeddings"
	if p.embedGroupID != "" {
		path += "?GroupId=" + url.QueryEscape(p.embedGroupID)
	}
	resp, started, err := p.doPostAt(ctx, p.embedAPIBase, path, payload)
	defer func() { observability.ObserveExternalCall("llm", started, err) }()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding api error %d", resp.StatusCode)
	}
	var result openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}
	return result.Data[0].Embedding, nil
}
