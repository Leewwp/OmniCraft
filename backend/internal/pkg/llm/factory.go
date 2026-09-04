package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"omnicraft/backend/config"
)

// NewProvider builds the agent's single LLM surface from cfg.Agent.
//
// Wiring modes:
//   - single provider (embedding_provider empty or equal to llm_provider):
//     chat and embedding share one adapter; the embedding credential may fall
//     back to the chat key (same vendor).
//   - split providers (canonical profile: MiniMax chat via the minimax
//     adapter + DashScope text-embedding-v4 via openai_compat): a
//     CompositeProvider routes chat/streaming to the chat adapter and
//     embeddings (including the v4 batch endpoint) to the embedding adapter.
//     The embedding credential must be its own — borrowing the chat key
//     across vendors always fails (a MiniMax key against DashScope 401s), so
//     a missing key fails closed instead of degrading.
func NewProvider(cfg *config.Config) LLMProvider {
	chatProviderType := cfg.Agent.LLMProvider
	embedProviderType := strings.ToLower(strings.TrimSpace(cfg.Agent.EmbeddingProvider))
	timeout := time.Duration(cfg.Agent.ProviderTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	retries := cfg.Agent.ProviderMaxRetries
	dimensions := cfg.Agent.EmbeddingDimensions

	if embedProviderType != "" && embedProviderType != strings.ToLower(strings.TrimSpace(chatProviderType)) {
		embedAPIKey := strings.TrimSpace(cfg.Agent.EmbeddingAPIKey)
		if embedAPIKey == "" {
			return &failingProvider{reason: fmt.Sprintf(
				"agent.embedding_provider %q differs from agent.llm_provider %q but agent.embedding_api_key is empty: cross-vendor embedding requires its own credential (AGENT_EMBEDDING_API_KEY)",
				cfg.Agent.EmbeddingProvider, chatProviderType)}
		}
		chat := NewProviderFromConfig(chatProviderType, cfg.Agent.LLMAPIKey, cfg.Agent.LLMAPIBase, cfg.Agent.LLMModel, cfg.Agent.EmbeddingModel,
			WithTimeout(timeout), WithMaxRetries(retries))
		embed := NewProviderFromConfig(embedProviderType, embedAPIKey, cfg.Agent.EmbeddingAPIBase, cfg.Agent.EmbeddingModel, cfg.Agent.EmbeddingModel,
			WithTimeout(timeout), WithMaxRetries(retries),
			WithEmbeddingAPIBase(cfg.Agent.EmbeddingAPIBase),
			WithEmbeddingGroupID(cfg.Agent.EmbeddingGroupID),
			WithEmbeddingAPIKey(embedAPIKey),
			WithEmbeddingDimensions(dimensions))
		return NewCompositeProvider(chat, embed)
	}

	// Single-provider wiring: embedding follows the chat provider, and the
	// embedding credential defaults to the chat key unless a dedicated
	// embedding key is configured. Cross-vendor setups (e.g. MiniMax chat +
	// DashScope embedding) must use the split wiring above instead.
	embedAPIKey := cfg.Agent.EmbeddingAPIKey
	if strings.TrimSpace(embedAPIKey) == "" {
		embedAPIKey = cfg.Agent.LLMAPIKey
	}
	return NewProviderFromConfig(
		chatProviderType, cfg.Agent.LLMAPIKey, cfg.Agent.LLMAPIBase, cfg.Agent.LLMModel, cfg.Agent.EmbeddingModel,
		WithTimeout(timeout),
		WithMaxRetries(retries),
		WithEmbeddingAPIBase(cfg.Agent.EmbeddingAPIBase),
		WithEmbeddingGroupID(cfg.Agent.EmbeddingGroupID),
		WithEmbeddingAPIKey(embedAPIKey),
		WithEmbeddingDimensions(dimensions),
	)
}

func NewProviderFromConfig(providerType, apiKey, apiBase, model, embedModel string, opts ...ProviderOption) LLMProvider {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "openai_compat", "":
		// The empty default routes to the OpenAI-compatible wire format: it is
		// the only adapter family that carries tools, token budgets and the
		// reasoning channel end to end.
		return NewOpenAICompatProvider(apiKey, apiBase, model, embedModel, opts...)
	case "minimax":
		return NewMiniMaxProvider(apiKey, apiBase, model, embedModel, opts...)
	case "qwen":
		// The native DashScope adapter is retired for agent use: it sends
		// neither tools nor max_tokens and parses neither tool_calls nor
		// reasoning, so the tool loop and think forwarding silently die.
		// DashScope belongs behind openai_compat with the compatible-mode base
		// (no trailing /v1 — the provider appends the path itself).
		return &failingProvider{reason: `llm provider "qwen" (native DashScope adapter) is retired for agent use: use openai_compat with the DashScope compatible-mode base instead`}
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
// provider to a working adapter with an incompatible model/base/embedding tuple.
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

// failingProvider fails closed with a pointed configuration reason (retired
// provider wiring, missing cross-vendor credential) instead of silently
// degrading to a half-functional path.
type failingProvider struct {
	reason string
}

func (p *failingProvider) err() error { return errors.New(p.reason) }

func (p *failingProvider) Chat(context.Context, ChatRequest) (*ChatResponse, error) {
	return nil, p.err()
}

func (p *failingProvider) ChatStream(context.Context, ChatRequest, func(ChatDelta) error) error {
	return p.err()
}

func (p *failingProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, p.err()
}
