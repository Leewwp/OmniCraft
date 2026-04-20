package llm

import "omnicraft/backend/config"

func NewProvider(cfg *config.Config) LLMProvider {
	switch cfg.Agent.LLMProvider {
	case "openai_compat":
		return NewOpenAICompatProvider(
			cfg.Agent.LLMAPIKey,
			cfg.Agent.LLMAPIBase,
			cfg.Agent.LLMModel,
			cfg.Agent.EmbeddingModel,
		)
	default:
		return NewQwenProvider(cfg.Agent.LLMModel, cfg.Agent.EmbeddingModel)
	}
}
