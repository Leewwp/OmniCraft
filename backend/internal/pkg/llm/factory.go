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
