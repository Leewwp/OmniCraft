package llm

import "omnicraft/backend/config"

func NewProvider(cfg *config.Config) LLMProvider {
	providerType := cfg.Agent.LLMProvider
	apiKey := cfg.Agent.LLMAPIKey
	apiBase := cfg.Agent.LLMAPIBase
	model := cfg.Agent.LLMModel
	embedModel := cfg.Agent.EmbeddingModel
	return NewProviderFromConfig(providerType, apiKey, apiBase, model, embedModel)
}

func NewProviderFromConfig(providerType, apiKey, apiBase, model, embedModel string) LLMProvider {
	switch providerType {
	case "openai_compat":
		return NewOpenAICompatProvider(apiKey, apiBase, model, embedModel)
	default:
		return NewQwenProvider(model, embedModel)
	}
}
