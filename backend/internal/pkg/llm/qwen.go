package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
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
		client:     &http.Client{Timeout: cfg.timeout},
		maxRetries: cfg.maxRetries,
	}
}

// Model reports the configured chat model (used by the opt-in agent-smoke
// release-evidence command).
func (p *QwenProvider) Model() string { return p.model }

func (p *QwenProvider) doPost(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return retryDo(ctx, p.client, qwenBaseURL+path, p.apiKey, b, p.maxRetries)
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

func (p *QwenProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	payload := qwenRequest{Model: p.model, Messages: req.Messages}
	resp, err := p.doPost(ctx, "/v1/chat/completions", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qwen api error %d: %s", resp.StatusCode, string(body))
	}
	var result qwenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Output.Choices) == 0 {
		return &ChatResponse{Content: result.Output.Text}, nil
	}
	return &ChatResponse{Content: result.Output.Choices[0].Message.Content}, nil
}

func (p *QwenProvider) ChatStream(ctx context.Context, req ChatRequest, handler func(delta ChatDelta) error) error {
	payload := qwenRequest{Model: p.model, Messages: req.Messages, Stream: true}
	resp, err := p.doPost(ctx, "/v1/chat/completions", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qwen api error %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return handler(ChatDelta{Done: true})
		}
		var chunk qwenResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
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
	return scanner.Err()
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

func (p *QwenProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	payload := qwenEmbeddingRequest{Model: p.embedModel}
	payload.Input.Texts = []string{text}
	payload.Parameters.TextType = "document"

	resp, err := p.doPost(ctx, "/v1/services/embeddings/text-embedding/text-embedding", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qwen embedding error %d: %s", resp.StatusCode, string(body))
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
