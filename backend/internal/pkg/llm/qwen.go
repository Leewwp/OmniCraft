package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"omnicraft/backend/internal/observability"
)

const qwenBaseURL = "https://dashscope.aliyuncs.com/compatible-mode"

type QwenProvider struct {
	apiKey     string
	model      string
	embedModel string
	client     *http.Client
	maxRetries int
}

func NewQwenProvider(apiKey, model, embedModel string, opts ...ProviderOption) *QwenProvider {
	if apiKey == "" {
		apiKey = os.Getenv("AGENT_LLM_API_KEY")
	}
	cfg := providerConfig{timeout: 60 * time.Second, maxRetries: 2}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &QwenProvider{
		apiKey:     apiKey,
		model:      model,
		embedModel: embedModel,
		client:     &http.Client{Timeout: cfg.timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
		maxRetries: cfg.maxRetries,
	}
}

// Model reports the configured chat model (used by the opt-in agent-smoke
// release-evidence command).
func (p *QwenProvider) Model() string { return p.model }

func (p *QwenProvider) doPost(ctx context.Context, path string, body interface{}) (resp *http.Response, started time.Time, err error) {
	started = time.Now()
	b, err := json.Marshal(body)
	if err != nil {
		return nil, started, err
	}
	resp, err = retryDo(ctx, p.client, qwenBaseURL+path, p.apiKey, b, p.maxRetries)
	return resp, started, err
}

type qwenRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

type qwenResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Text string `json:"text"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (p *QwenProvider) Chat(ctx context.Context, req ChatRequest) (response *ChatResponse, err error) {
	ctx, span := startLLMSpan(ctx, "chat", p.model, "qwen", req.Temperature)
	totalTokens := -1
	defer func() {
		var usage *TokenUsage
		if response != nil {
			usage = response.Usage
		}
		finishLLMSpanWithTotal(span, err, usage, totalTokens)
	}()
	payload := qwenRequest{Model: p.model, Messages: req.Messages}
	resp, started, err := p.doPost(ctx, "/v1/chat/completions", payload)
	defer func() { observability.ObserveExternalCall("llm", started, err) }()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qwen api error %d", resp.StatusCode)
	}
	var result qwenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	totalTokens = result.Usage.TotalTokens
	if len(result.Output.Choices) == 0 {
		return &ChatResponse{Content: result.Output.Text}, nil
	}
	return &ChatResponse{Content: result.Output.Choices[0].Message.Content}, nil
}

func (p *QwenProvider) ChatStream(ctx context.Context, req ChatRequest, handler func(delta ChatDelta) error) (err error) {
	ctx, span := startLLMSpan(ctx, "chat.stream", p.model, "qwen", req.Temperature)
	totalTokens := -1
	defer func() { finishLLMSpanWithTotal(span, err, nil, totalTokens) }()
	payload := qwenRequest{Model: p.model, Messages: req.Messages, Stream: true}
	resp, started, err := p.doPost(ctx, "/v1/chat/completions", payload)
	defer func() { observability.ObserveExternalCall("llm", started, err) }()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qwen api error %d", resp.StatusCode)
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
		var chunk qwenResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage.TotalTokens > 0 {
			totalTokens = chunk.Usage.TotalTokens
		}
		var content string
		if len(chunk.Output.Choices) > 0 {
			content = chunk.Output.Choices[0].Message.Content
		} else {
			content = chunk.Output.Text
		}
		if err := handler(ChatDelta{Content: content}); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !finished {
		return fmt.Errorf("qwen stream ended before completion")
	}
	return nil
}

type qwenEmbeddingRequest struct {
	Model string `json:"model"`
	Input struct {
		Texts []string `json:"texts"`
	} `json:"input"`
	Parameters struct {
		TextType string `json:"text_type"`
	} `json:"parameters"`
}

type qwenEmbeddingResponse struct {
	Output struct {
		Embeddings []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"embeddings"`
	} `json:"output"`
}

func (p *QwenProvider) GetEmbedding(ctx context.Context, text string) (embedding []float32, err error) {
	ctx, span := startLLMSpan(ctx, "embedding", p.embedModel, "qwen", 0)
	defer func() { finishLLMSpan(span, err, nil) }()
	payload := qwenEmbeddingRequest{Model: p.embedModel}
	payload.Input.Texts = []string{text}
	payload.Parameters.TextType = "document"

	resp, started, err := p.doPost(ctx, "/v1/services/embeddings/text-embedding/text-embedding", payload)
	defer func() { observability.ObserveExternalCall("llm", started, err) }()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qwen embedding error %d", resp.StatusCode)
	}
	var result qwenEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Output.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}
	return result.Output.Embeddings[0].Embedding, nil
}
