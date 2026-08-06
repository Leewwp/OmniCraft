package llm

import (
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
	)
}

func NewProviderFromConfig(providerType, apiKey, apiBase, model, embedModel string, opts ...ProviderOption) LLMProvider {
	switch providerType {
	case "openai_compat":
		return NewOpenAICompatProvider(apiKey, apiBase, model, embedModel, opts...)
	default:
		return NewQwenProvider(apiKey, model, embedModel, opts...)
	}
}
