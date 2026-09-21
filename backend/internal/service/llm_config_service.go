package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"

	"gorm.io/gorm"
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
	if decrypted, err := decryptLLMAPIKey(key); err == nil {
		key = decrypted
	}
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

func (s *LLMConfigService) ListConfigs(ctx context.Context) ([]LLMConfigResponse, error) {
	configs, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]LLMConfigResponse, 0, len(configs))
	for _, c := range configs {
		result = append(result, toResponse(&c))
	}
	return result, nil
}

func (s *LLMConfigService) CreateConfig(ctx context.Context, name, providerType, apiBase, modelName, apiKey string) (*LLMConfigResponse, error) {
	return s.createConfig(ctx, name, providerType, apiBase, modelName, apiKey)
}

func (s *LLMConfigService) CreateConfigTx(ctx context.Context, tx *gorm.DB, name, providerType, apiBase, modelName, apiKey string) (*LLMConfigResponse, error) {
	txSvc := *s
	txSvc.repo = s.repo.WithTx(tx)
	return txSvc.createConfig(ctx, name, providerType, apiBase, modelName, apiKey)
}

func (s *LLMConfigService) createConfig(ctx context.Context, name, providerType, apiBase, modelName, apiKey string) (*LLMConfigResponse, error) {
	apiKeyEnc, err := encryptLLMAPIKey(apiKey)
	if err != nil {
		return nil, err
	}
	c := &model.LLMConfig{
		ConfigName:   name,
		ProviderType: providerType,
		APIBase:      apiBase,
		Model:        modelName,
		APIKeyEnc:    apiKeyEnc,
		ExtraParams:  model.JSONMap{},
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	r := toResponse(c)
	return &r, nil
}

func (s *LLMConfigService) UpdateConfig(ctx context.Context, id int64, updates map[string]interface{}) error {
	return s.updateConfig(ctx, id, updates)
}

func (s *LLMConfigService) UpdateConfigTx(ctx context.Context, tx *gorm.DB, id int64, updates map[string]interface{}) error {
	txSvc := *s
	txSvc.repo = s.repo.WithTx(tx)
	return txSvc.updateConfig(ctx, id, updates)
}

func (s *LLMConfigService) updateConfig(ctx context.Context, id int64, updates map[string]interface{}) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return ErrConfigNotFound
	}
	delete(updates, "api_key_enc")
	if raw, ok := updates["api_key"]; ok {
		delete(updates, "api_key")
		apiKey, _ := raw.(string)
		if strings.TrimSpace(apiKey) != "" {
			apiKeyEnc, err := encryptLLMAPIKey(apiKey)
			if err != nil {
				return err
			}
			updates["api_key_enc"] = apiKeyEnc
		}
	}
	return s.repo.Update(ctx, id, updates)
}

func (s *LLMConfigService) DeleteConfig(ctx context.Context, id int64) error {
	return s.deleteConfig(ctx, id)
}

func (s *LLMConfigService) DeleteConfigTx(ctx context.Context, tx *gorm.DB, id int64) error {
	txSvc := *s
	txSvc.repo = s.repo.WithTx(tx)
	return txSvc.deleteConfig(ctx, id)
}

func (s *LLMConfigService) deleteConfig(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return ErrConfigNotFound
	}
	return s.repo.Delete(ctx, id)
}

func (s *LLMConfigService) ActivateConfig(ctx context.Context, id int64) error {
	return s.activateConfig(ctx, id)
}

func (s *LLMConfigService) ActivateConfigTx(ctx context.Context, tx *gorm.DB, id int64) error {
	txSvc := *s
	txSvc.repo = s.repo.WithTx(tx)
	if _, err := txSvc.repo.GetByID(ctx, id); err != nil {
		return ErrConfigNotFound
	}
	return txSvc.repo.ActivateTx(ctx, id)
}

func (s *LLMConfigService) activateConfig(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return ErrConfigNotFound
	}
	return s.repo.Activate(ctx, id)
}

func (s *LLMConfigService) TestConnection(ctx context.Context, id int64) (string, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", ErrConfigNotFound
	}

	apiKey, err := decryptLLMAPIKey(c.APIKeyEnc)
	if err != nil {
		return "", err
	}
	provider := llm.NewProviderFromConfig(c.ProviderType, apiKey, c.APIBase, c.Model, "")
	req := llm.ChatRequest{
		Messages:    []llm.ChatMessage{{Role: "user", Content: "Reply with just 'OK'"}},
		MaxTokens:   10,
		Temperature: 0,
	}

	resp, err := provider.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("connection test failed: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

func (s *LLMConfigService) GetActiveConfig(ctx context.Context) (*model.LLMConfig, error) {
	return s.repo.GetActive(ctx)
}

func encryptLLMAPIKey(apiKey string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", nil
	}
	block, err := aes.NewCipher(llmEncryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(apiKey), nil)
	return "v1:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptLLMAPIKey(apiKeyEnc string) (string, error) {
	if strings.TrimSpace(apiKeyEnc) == "" {
		return "", nil
	}
	if !strings.HasPrefix(apiKeyEnc, "v1:") {
		return apiKeyEnc, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(apiKeyEnc, "v1:"))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(llmEncryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted api key")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func llmEncryptionKey() []byte {
	secret := os.Getenv("LLM_KEY_ENCRYPTION_SECRET")
	if secret == "" {
		secret = os.Getenv("JWT_SECRET")
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
