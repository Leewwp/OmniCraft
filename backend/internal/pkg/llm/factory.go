package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"omnicraft/backend/config"
)

func NewProvider(cfg *config.Config) LLMProvider {
	providerType := cfg.Agent.LLMProvider
	apiKey := cfg.Agent.LLMAPIKey
	apiBase := cfg.Agent.LLMAPIBase
	model := cfg.Agent.LLMModel
	embedModel := cfg.Agent.EmbeddingModel
	// Embedding credential defaults to the chat key unless a dedicated
	// embedding key is configured (DashScope embedding next to a different
	// chat vendor).
	embedAPIKey := cfg.Agent.EmbeddingAPIKey
	if strings.TrimSpace(embedAPIKey) == "" {
		embedAPIKey = apiKey
	}
	timeout := time.Duration(cfg.Agent.ProviderTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return NewProviderFromConfig(
		providerType, apiKey, apiBase, model, embedModel,
		WithTimeout(timeout),
		WithMaxRetries(cfg.Agent.ProviderMaxRetries),
		WithEmbeddingAPIBase(cfg.Agent.EmbeddingAPIBase),
		WithEmbeddingGroupID(cfg.Agent.EmbeddingGroupID),
		WithEmbeddingAPIKey(embedAPIKey),
		WithEmbeddingDimensions(cfg.Agent.EmbeddingDimensions),
	)
}

func NewProviderFromConfig(providerType, apiKey, apiBase, model, embedModel string, opts ...ProviderOption) LLMProvider {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "openai_compat":
		return NewOpenAICompatProvider(apiKey, apiBase, model, embedModel, opts...)
	case "minimax":
		return NewMiniMaxProvider(apiKey, apiBase, model, embedModel, opts...)
	case "", "qwen":
		return NewQwenProvider(apiKey, model, embedModel, opts...)
	default:
		return &unsupportedProvider{providerType: providerType}
	}
}

// NewRerankerFromConfig builds the A-03 rerank chain from rag.rerank config:
// a primary reranker plus an optional fallback. A side without an API key is
// skipped; when neither side is usable the returned reranker is nil and the
// caller keeps the RRF order (degradation chain: primary → fallback → RRF).
func NewRerankerFromConfig(cfg config.RAGRerankConfig) (Reranker, int) {
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	primary := buildReranker(cfg.Provider, cfg.APIKey, cfg.APIBase, cfg.Model, timeout)
	fallback := buildReranker(cfg.FallbackProvider, cfg.FallbackAPIKey, cfg.FallbackAPIBase, cfg.FallbackModel, timeout)
	switch {
	case primary != nil && fallback != nil:
		return NewFallbackReranker(primary, fallback), cfg.InputTopK
	case primary != nil:
		return primary, cfg.InputTopK
	case fallback != nil:
		return fallback, cfg.InputTopK
	default:
		return nil, cfg.InputTopK
	}
}

func buildReranker(provider, apiKey, apiBase, model string, timeout time.Duration) Reranker {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(model) == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "dashscope", "":
		return NewDashScopeReranker(apiKey, apiBase, model, timeout)
	case "siliconflow":
		return NewSiliconFlowReranker(apiKey, apiBase, model, timeout)
	default:
		return nil
	}
}

// unsupportedProvider fails closed instead of silently routing a misspelled
// provider to Qwen with an incompatible model/base/embedding tuple.
type unsupportedProvider struct {
	providerType string
}

func (p *unsupportedProvider) err() error {
	return fmt.Errorf("unsupported llm provider %q", strings.TrimSpace(p.providerType))
}

func (p *unsupportedProvider) Chat(context.Context, ChatRequest) (*ChatResponse, error) {
	return nil, p.err()
}

func (p *unsupportedProvider) ChatStream(context.Context, ChatRequest, func(ChatDelta) error) error {
	return p.err()
}

func (p *unsupportedProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, p.err()
}
