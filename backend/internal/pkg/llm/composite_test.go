package llm

import (
	"context"
	"strings"
	"testing"

	"omnicraft/backend/config"
)

type recordingEmbedProvider struct {
	LLMProvider
	embedCalls  []string
	batchCalled bool
}

func (p *recordingEmbedProvider) GetEmbedding(_ context.Context, text string) ([]float32, error) {
	p.embedCalls = append(p.embedCalls, text)
	return []float32{1}, nil
}

func (p *recordingEmbedProvider) GetEmbeddings(_ context.Context, texts []string) ([][]float32, error) {
	p.batchCalled = true
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{float32(i)}
	}
	return out, nil
}

func (p *recordingEmbedProvider) Model() string { return "embed-side-model" }

func TestCompositeProvider_ForwardsBatchEmbedding(t *testing.T) {
	embed := &recordingEmbedProvider{}
	chat := NewOpenAICompatProvider("k", "https://chat.example.com", "chat-model", "embed-model")
	p := NewCompositeProvider(chat, embed)

	vectors, err := p.GetEmbeddings(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("GetEmbeddings: %v", err)
	}
	if !embed.batchCalled {
		t.Fatal("batch endpoint not forwarded: full re-embedding would silently degrade to one request per text")
	}
	if len(vectors) != 2 {
		t.Fatalf("vectors = %d, want 2", len(vectors))
	}
	if len(embed.embedCalls) != 0 {
		t.Fatalf("single-text path used %d calls, want 0", len(embed.embedCalls))
	}
	if p.Model() != "chat-model" {
		t.Errorf("Model() = %q, want chat-side model", p.Model())
	}
	if p.EmbeddingModel() != "embed-side-model" {
		t.Errorf("EmbeddingModel() = %q, want embed-side model", p.EmbeddingModel())
	}
}

func TestCompositeProvider_SingleEmbeddingHitsEmbedDelegate(t *testing.T) {
	embed := &recordingEmbedProvider{}
	chat := NewOpenAICompatProvider("k", "https://chat.example.com", "chat-model", "embed-model")
	p := NewCompositeProvider(chat, embed)

	if _, err := p.GetEmbedding(context.Background(), "query"); err != nil {
		t.Fatalf("GetEmbedding: %v", err)
	}
	if len(embed.embedCalls) != 1 || embed.embedCalls[0] != "query" {
		t.Fatalf("embed calls = %v, want [query]", embed.embedCalls)
	}
}

func TestCompositeProvider_NoBatchDelegateFailsClosed(t *testing.T) {
	embed := struct{ LLMProvider }{}
	p := NewCompositeProvider(NewOpenAICompatProvider("k", "https://chat.example.com", "m", "e"), embed)
	if _, err := p.GetEmbeddings(context.Background(), []string{"a"}); err == nil {
		t.Fatal("want an explicit error when the embedding delegate has no batch endpoint")
	}
}

func TestNewProvider_SplitWiring(t *testing.T) {
	base := config.AgentConfig{
		LLMProvider:         "minimax",
		LLMModel:            "MiniMax-M3",
		LLMAPIBase:          "https://api.minimaxi.com",
		LLMAPIKey:           "chat-key",
		EmbeddingProvider:   "openai_compat",
		EmbeddingModel:      "text-embedding-v4",
		EmbeddingAPIBase:    "https://dashscope.aliyuncs.com/compatible-mode",
		EmbeddingDimensions: config.RAGEmbeddingDimensions,
	}

	t.Run("cross_vendor_without_embed_key_fails_closed", func(t *testing.T) {
		cfg := &config.Config{Agent: base}
		p := NewProvider(cfg)
		_, err := p.GetEmbedding(context.Background(), "q")
		if err == nil || !strings.Contains(err.Error(), "embedding_api_key") {
			t.Fatalf("cross-vendor embedding without its own key must fail closed with a pointed reason, got %v", err)
		}
	})

	t.Run("cross_vendor_with_embed_key_builds_composite", func(t *testing.T) {
		cfg := &config.Config{Agent: base}
		cfg.Agent.EmbeddingAPIKey = "dashscope-key"
		p := NewProvider(cfg)
		if _, ok := p.(*CompositeProvider); !ok {
			t.Fatalf("expected *CompositeProvider for split wiring, got %T", p)
		}
	})

	t.Run("same_provider_stays_single", func(t *testing.T) {
		cfg := &config.Config{Agent: base}
		cfg.Agent.EmbeddingProvider = "minimax"
		if _, ok := NewProvider(cfg).(*CompositeProvider); ok {
			t.Fatal("embedding_provider equal to llm_provider must keep the single-provider wiring")
		}
	})

	t.Run("empty_embedding_provider_stays_single", func(t *testing.T) {
		cfg := &config.Config{Agent: base}
		cfg.Agent.EmbeddingProvider = ""
		if _, ok := NewProvider(cfg).(*CompositeProvider); ok {
			t.Fatal("empty embedding_provider must keep the single-provider wiring")
		}
	})
}

func TestNewProvider_QwenNativeRetired(t *testing.T) {
	cfg := &config.Config{Agent: config.AgentConfig{
		LLMProvider: "qwen",
		LLMModel:    "qwen-plus",
		LLMAPIKey:   "k",
	}}
	_, err := NewProvider(cfg).Chat(context.Background(), ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("native qwen wiring must fail closed with the retirement reason, got %v", err)
	}
}
