package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewOpenAICompatProvider_SetsFields(t *testing.T) {
	t.Run("default_api_base", func(t *testing.T) {
		p := NewOpenAICompatProvider("key", "", "model", "embed")
		if p.apiBase != "https://api.openai.com" {
			t.Errorf("expected apiBase %q, got %q", "https://api.openai.com", p.apiBase)
		}
	})

	t.Run("custom_api_base", func(t *testing.T) {
		p := NewOpenAICompatProvider("key", "https://custom.api.com", "model", "embed")
		if p.apiBase != "https://custom.api.com" {
			t.Errorf("expected apiBase %q, got %q", "https://custom.api.com", p.apiBase)
		}
	})

	t.Run("trailing_slash_stripped", func(t *testing.T) {
		p := NewOpenAICompatProvider("key", "https://custom.api.com/", "model", "embed")
		if p.apiBase != "https://custom.api.com" {
			t.Errorf("expected apiBase %q, got %q", "https://custom.api.com", p.apiBase)
		}
	})

	t.Run("fields_set", func(t *testing.T) {
		p := NewOpenAICompatProvider("mykey", "https://example.com", "gpt-4", "text-embedding-3")
		if p.apiKey != "mykey" {
			t.Errorf("expected apiKey %q, got %q", "mykey", p.apiKey)
		}
		if p.model != "gpt-4" {
			t.Errorf("expected model %q, got %q", "gpt-4", p.model)
		}
		if p.embedModel != "text-embedding-3" {
			t.Errorf("expected embedModel %q, got %q", "text-embedding-3", p.embedModel)
		}
	})
}

func TestNewMiniMaxProvider_DefaultAPIBase(t *testing.T) {
	p := NewMiniMaxProvider("key", "", "model", "embed")
	if p.openAI.apiBase != "https://api.minimaxi.com" {
		t.Errorf("expected apiBase %q, got %q", "https://api.minimaxi.com", p.openAI.apiBase)
	}
}

func TestNewProviderFromConfig_RoutesCorrectly(t *testing.T) {
	t.Run("openai_compat", func(t *testing.T) {
		p := NewProviderFromConfig("openai_compat", "key", "https://api.example.com", "gpt-4", "embed")
		if _, ok := p.(*OpenAICompatProvider); !ok {
			t.Errorf("expected *OpenAICompatProvider, got %T", p)
		}
	})

	t.Run("minimax", func(t *testing.T) {
		p := NewProviderFromConfig("minimax", "key", "https://api.minimaxi.com", "MiniMax-M1", "embo-01")
		if _, ok := p.(*MiniMaxProvider); !ok {
			t.Errorf("expected *MiniMaxProvider, got %T", p)
		}
	})

	t.Run("unknown_fails_closed", func(t *testing.T) {
		p := NewProviderFromConfig("unknown_type", "key", "", "model", "embed")
		if _, ok := p.(*unsupportedProvider); !ok {
			t.Errorf("expected *unsupportedProvider for unknown type, got %T", p)
		}
		if _, err := p.Chat(context.Background(), ChatRequest{}); err == nil || !strings.Contains(err.Error(), "unknown_type") {
			t.Fatalf("unknown provider error = %v, want safe configuration error", err)
		}
	})

	t.Run("empty_type_defaults_to_qwen", func(t *testing.T) {
		p := NewProviderFromConfig("", "key", "", "model", "embed")
		if _, ok := p.(*QwenProvider); !ok {
			t.Errorf("expected *QwenProvider for empty type, got %T", p)
		}
	})
}

func TestOpenAICompatProvider_Chat_Serialization(t *testing.T) {
	var receivedReq *http.Request
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedReq = r
		receivedBody, _ = io.ReadAll(r.Body)

		resp := openAIResponse{
			Choices: []openAIChoice{{
				Message: openAIMessage{Content: "test response"},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAICompatProvider("test-key", server.URL, "gpt-4", "embed")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify Authorization header
	auth := receivedReq.Header.Get("Authorization")
	if auth != "Bearer test-key" {
		t.Errorf("expected Authorization %q, got %q", "Bearer test-key", auth)
	}

	// Verify request path
	if receivedReq.URL.Path != "/v1/chat/completions" {
		t.Errorf("expected path /v1/chat/completions, got %s", receivedReq.URL.Path)
	}

	// Verify request body
	var reqBody openAIRequest
	if err := json.Unmarshal(receivedBody, &reqBody); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if reqBody.Model != "gpt-4" {
		t.Errorf("expected model %q, got %q", "gpt-4", reqBody.Model)
	}
	if len(reqBody.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(reqBody.Messages))
	}
	if reqBody.Messages[0].Role != "user" || reqBody.Messages[0].Content != "hello" {
		t.Errorf("expected message {role:user, content:hello}, got {role:%s, content:%s}", reqBody.Messages[0].Role, reqBody.Messages[0].Content)
	}

	// Verify response parsing
	if resp.Content != "test response" {
		t.Errorf("expected response content %q, got %q", "test response", resp.Content)
	}
}

func TestOpenAICompatProvider_GetEmbedding_Serialization(t *testing.T) {
	var receivedReq *http.Request
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedReq = r
		receivedBody, _ = io.ReadAll(r.Body)

		resp := openAIEmbeddingResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
			}{
				{Embedding: []float32{0.1, 0.2, 0.3}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAICompatProvider("test-key", server.URL, "gpt-4", "text-embedding-3")
	embedding, err := p.GetEmbedding(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify Authorization header
	auth := receivedReq.Header.Get("Authorization")
	if auth != "Bearer test-key" {
		t.Errorf("expected Authorization %q, got %q", "Bearer test-key", auth)
	}

	// Verify request path
	if receivedReq.URL.Path != "/v1/embeddings" {
		t.Errorf("expected path /v1/embeddings, got %s", receivedReq.URL.Path)
	}

	// Verify request body
	var reqBody openAIEmbeddingRequest
	if err := json.Unmarshal(receivedBody, &reqBody); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if reqBody.Model != "text-embedding-3" {
		t.Errorf("expected model %q, got %q", "text-embedding-3", reqBody.Model)
	}
	if reqBody.Input != "hello world" {
		t.Errorf("expected input %q, got %q", "hello world", reqBody.Input)
	}

	// Verify response parsing
	expected := []float32{0.1, 0.2, 0.3}
	if len(embedding) != len(expected) {
		t.Fatalf("expected %d dims, got %d", len(expected), len(embedding))
	}
	for i, v := range expected {
		if embedding[i] != v {
			t.Errorf("dim[%d]: expected %v, got %v", i, v, embedding[i])
		}
	}
}
