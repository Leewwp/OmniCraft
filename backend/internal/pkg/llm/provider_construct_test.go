package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omnicraft/backend/config"
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

	t.Run("empty_type_defaults_to_openai_compat", func(t *testing.T) {
		p := NewProviderFromConfig("", "key", "", "model", "embed")
		if _, ok := p.(*OpenAICompatProvider); !ok {
			t.Errorf("expected *OpenAICompatProvider for empty type, got %T", p)
		}
	})

	t.Run("qwen_native_retired_fails_closed", func(t *testing.T) {
		p := NewProviderFromConfig("qwen", "key", "", "model", "embed")
		if _, ok := p.(*failingProvider); !ok {
			t.Errorf("expected *failingProvider for retired native qwen, got %T", p)
		}
		if _, err := p.Chat(context.Background(), ChatRequest{}); err == nil || !strings.Contains(err.Error(), "retired") {
			t.Fatalf("native qwen error = %v, want retirement reason", err)
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

// #545: model registry entries without a credential keep the legacy
// single-provider surface; a credentialed entry upgrades the surface to a
// RoutingProvider whose chain still starts at the configured primary.
func TestNewProviderRoutingWiring(t *testing.T) {
	legacy := &config.Config{Agent: config.AgentConfig{
		LLMProvider: "minimax", LLMModel: "MiniMax-M3", LLMAPIKey: "k",
	}}
	if _, ok := NewProvider(legacy).(*RoutingProvider); ok {
		t.Fatal("no registry entries must keep the single-provider surface")
	}

	withKey := &config.Config{Agent: config.AgentConfig{
		LLMProvider: "minimax", LLMModel: "MiniMax-M3", LLMAPIKey: "k",
		Models: []config.AgentModelConfig{
			{ID: "DeepSeek", Provider: "deepseek", Model: "deepseek-chat", APIBase: "https://api.deepseek.com", APIKey: "ds-key", DisplayName: "DeepSeek"},
			{ID: "nokey", Provider: "deepseek", Model: "x", APIBase: "https://api.deepseek.com"},
		},
		Routing: config.AgentRoutingConfig{Fallbacks: []string{"deepseek", "nokey"}, RetryOn: []string{"blank_answer"}},
	}}
	router, ok := NewProvider(withKey).(*RoutingProvider)
	if !ok {
		t.Fatal("credentialed registry entries must build a RoutingProvider")
	}
	options := router.ModelOptions()
	if len(options) != 2 || options[0].ID != "minimax" || options[1].ID != "deepseek" {
		t.Fatalf("options = %#v, want minimax primary + deepseek", options)
	}
	if options[1].DisplayName != "DeepSeek" {
		t.Fatalf("display name = %q", options[1].DisplayName)
	}
	if !router.ModelRegistered("deepseek") || router.ModelRegistered("nokey") {
		t.Fatal("registry membership must follow the credential rule")
	}
	if !router.ModelRegistered("minimax") {
		t.Fatal("the synthesized primary must be registered")
	}
}

// #545 hotfix regression: the registry provider id "deepseek" must map to a
// working OpenAI-compatible adapter with the DeepSeek thinking style. Before
// the factory case existed the entry registered an unsupportedProvider that
// failed every call instantly (live demo logs: 6/6 instant provider_error).
func TestNewProviderFromConfigDeepSeekServesChat(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		gotBody = buf
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	p := NewProviderFromConfig("deepseek", "k", server.URL, "deepseek-chat", "")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		Thinking: ThinkingAdaptive,
	})
	if err != nil {
		t.Fatalf("deepseek provider must serve chat, got %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q", resp.Content)
	}
	var payload struct {
		Model    string `json:"model"`
		Thinking *struct {
			Type string `json:"type"`
		} `json:"thinking"`
	}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("decode captured body: %v", err)
	}
	if payload.Model != "deepseek-chat" {
		t.Errorf("wire model = %q, want deepseek-chat", payload.Model)
	}
	if payload.Thinking == nil || payload.Thinking.Type != "enabled" {
		t.Errorf("wire thinking = %+v, want {type:enabled} (deepseek adaptive naming)", payload.Thinking)
	}
}

// #545 hotfix regression through the config seam: a credentialed deepseek
// registry entry pinned via ModelPref must actually receive the request —
// registration alone (the pre-fix state) is not enough.
func TestNewProviderDeepSeekEntryServesChat(t *testing.T) {
	var gotAuth string
	deepseekHit := false
	ds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deepseekHit = true
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"from-deepseek"}}]}`))
	}))
	defer ds.Close()

	cfg := &config.Config{Agent: config.AgentConfig{
		LLMProvider: "openai_compat", LLMModel: "primary-model", LLMAPIKey: "k", LLMAPIBase: ds.URL,
		Models: []config.AgentModelConfig{
			{ID: "deepseek", Provider: "deepseek", Model: "deepseek-chat", APIBase: ds.URL, APIKey: "ds-key"},
		},
		Routing: config.AgentRoutingConfig{Fallbacks: []string{"deepseek"}},
	}}
	router, ok := NewProvider(cfg).(*RoutingProvider)
	if !ok {
		t.Fatal("credentialed registry entries must build a RoutingProvider")
	}
	resp, err := router.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		ModelPref: "deepseek",
	})
	if err != nil {
		t.Fatalf("deepseek-pinned chat must serve through the registry, got %v", err)
	}
	if !deepseekHit {
		t.Fatal("request never reached the deepseek entry's endpoint")
	}
	if gotAuth != "Bearer ds-key" {
		t.Errorf("Authorization = %q, want the registry entry key", gotAuth)
	}
	if resp.Content != "from-deepseek" {
		t.Fatalf("content = %q, want the deepseek endpoint reply", resp.Content)
	}
}
