package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"omnicraft/backend/internal/observability"
)

const qwenBaseURL = "https://dashscope.aliyuncs.com/compatible-mode"

type QwenProvider struct {
	apiKey     string
	model      string
	embedModel string
	client     *http.Client
}

func NewQwenProvider(apiKey, model, embedModel string) *QwenProvider {
	if apiKey == "" {
		apiKey = os.Getenv("AGENT_LLM_API_KEY")
	}
	return &QwenProvider{
		apiKey:     apiKey,
		model:      model,
		embedModel: embedModel,
		client:     &http.Client{},
	}
}

func (p *QwenProvider) doPost(ctx context.Context, path string, body interface{}) (resp *http.Response, started time.Time, err error) {
	started = time.Now()
	b, err := json.Marshal(body)
	if err != nil {
		return nil, started, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", qwenBaseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, started, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err = p.client.Do(req)
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
	if len(result.Output.Choices) == 0 {
		return &ChatResponse{Content: result.Output.Text}, nil
	}
	return &ChatResponse{Content: result.Output.Choices[0].Message.Content}, nil
}

func (p *QwenProvider) ChatStream(ctx context.Context, req ChatRequest, handler func(delta ChatDelta) error) (err error) {
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
			return fmt.Errorf("qwen stream response is invalid")
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
