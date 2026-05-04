package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"
)

var (
	ErrConfigNotFound = errors.New("llm config not found")
)

type LLMConfigService struct {
	repo *repository.LLMConfigRepository
	cfg  *config.Config
}

func NewLLMConfigService(repo *repository.LLMConfigRepository, cfg *config.Config) *LLMConfigService {
	return &LLMConfigService{repo: repo, cfg: cfg}
}

type LLMConfigResponse struct {
	ID           int64                  `json:"id"`
	ConfigName   string                 `json:"config_name"`
	ProviderType string                 `json:"provider_type"`
	APIBase      string                 `json:"api_base"`
	Model        string                 `json:"model"`
	APIKeyMasked string                 `json:"api_key_masked"`
	IsActive     bool                   `json:"is_active"`
	ExtraParams  map[string]interface{} `json:"extra_params"`
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func toResponse(c *model.LLMConfig) LLMConfigResponse {
	return LLMConfigResponse{
		ID:           c.ID,
		ConfigName:   c.ConfigName,
		ProviderType: c.ProviderType,
		APIBase:      c.APIBase,
		Model:        c.Model,
		APIKeyMasked: maskAPIKey(c.APIKeyEnc),
		IsActive:     c.IsActive,
		ExtraParams:  c.ExtraParams,
	}
}

func (s *LLMConfigService) ListConfigs() ([]LLMConfigResponse, error) {
	configs, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	result := make([]LLMConfigResponse, 0, len(configs))
	for _, c := range configs {
		result = append(result, toResponse(&c))
	}
	return result, nil
}

func (s *LLMConfigService) CreateConfig(name, providerType, apiBase, modelName, apiKey string) (*LLMConfigResponse, error) {
	c := &model.LLMConfig{
		ConfigName:   name,
		ProviderType: providerType,
		APIBase:      apiBase,
		Model:        modelName,
		APIKeyEnc:    apiKey,
		ExtraParams:  model.JSONMap{},
	}
	if err := s.repo.Create(c); err != nil {
		return nil, err
	}
	r := toResponse(c)
	return &r, nil
}

func (s *LLMConfigService) UpdateConfig(id int64, updates map[string]interface{}) error {
	if _, err := s.repo.GetByID(id); err != nil {
		return ErrConfigNotFound
	}
	return s.repo.Update(id, updates)
}

func (s *LLMConfigService) DeleteConfig(id int64) error {
	if _, err := s.repo.GetByID(id); err != nil {
		return ErrConfigNotFound
	}
	return s.repo.Delete(id)
}

func (s *LLMConfigService) ActivateConfig(id int64) error {
	if _, err := s.repo.GetByID(id); err != nil {
		return ErrConfigNotFound
	}
	return s.repo.Activate(id)
}

func (s *LLMConfigService) TestConnection(id int64) (string, error) {
	c, err := s.repo.GetByID(id)
	if err != nil {
		return "", ErrConfigNotFound
	}

	provider := llm.NewProviderFromConfig(c.ProviderType, c.APIKeyEnc, c.APIBase, c.Model, "")
	req := llm.ChatRequest{
		Messages:    []llm.ChatMessage{{Role: "user", Content: "Reply with just 'OK'"}},
		MaxTokens:   10,
		Temperature: 0,
	}

	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("connection test failed: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

func (s *LLMConfigService) GetActiveConfig() (*model.LLMConfig, error) {
	return s.repo.GetActive()
}
