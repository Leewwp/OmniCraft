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
	"log/slog"
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
	// ErrLLMKeyEncryptionSecretMissing: the static-encryption key chain
	// (LLM_KEY_ENCRYPTION_SECRET → JWT_SECRET) resolves to nothing. Refuse to
	// derive a key from sha256("") — that public constant would fake-encrypt
	// the stored api_key_enc rows (SP-25 中-4).
	ErrLLMKeyEncryptionSecretMissing = errors.New("llm api key encryption secret is not configured (set LLM_KEY_ENCRYPTION_SECRET or JWT_SECRET)")
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

func (s *LLMConfigService) ListConfigs() ([]LLMConfigResponse, error) {
	configs, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	result := make([]LLMConfigResponse, 0, len(configs))
	for _, c := range configs {
		// 中-4 审计：无 v1: 前缀 = 存量明文落库（历史形态），逐行告警
		// 提示管理员轮换（重新录入即走加密链）。
		if strings.TrimSpace(c.APIKeyEnc) != "" && !strings.HasPrefix(c.APIKeyEnc, "v1:") {
			slog.Warn("llm config stores api_key_enc without v1: prefix (plaintext legacy row); re-enter the key to encrypt",
				"config_id", c.ID, "config_name", c.ConfigName)
		}
		result = append(result, toResponse(&c))
	}
	return result, nil
}

func (s *LLMConfigService) CreateConfig(name, providerType, apiBase, modelName, apiKey string) (*LLMConfigResponse, error) {
	return s.createConfig(name, providerType, apiBase, modelName, apiKey)
}

func (s *LLMConfigService) CreateConfigTx(tx *gorm.DB, name, providerType, apiBase, modelName, apiKey string) (*LLMConfigResponse, error) {
	txSvc := *s
	txSvc.repo = s.repo.WithTx(tx)
	return txSvc.createConfig(name, providerType, apiBase, modelName, apiKey)
}

func (s *LLMConfigService) createConfig(name, providerType, apiBase, modelName, apiKey string) (*LLMConfigResponse, error) {
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
	if err := s.repo.Create(c); err != nil {
		return nil, err
	}
	r := toResponse(c)
	return &r, nil
}

func (s *LLMConfigService) UpdateConfig(id int64, updates map[string]interface{}) error {
	return s.updateConfig(id, updates)
}

func (s *LLMConfigService) UpdateConfigTx(tx *gorm.DB, id int64, updates map[string]interface{}) error {
	txSvc := *s
	txSvc.repo = s.repo.WithTx(tx)
	return txSvc.updateConfig(id, updates)
}

func (s *LLMConfigService) updateConfig(id int64, updates map[string]interface{}) error {
	if _, err := s.repo.GetByID(id); err != nil {
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
	return s.repo.Update(id, updates)
}

func (s *LLMConfigService) DeleteConfig(id int64) error {
	return s.deleteConfig(id)
}

func (s *LLMConfigService) DeleteConfigTx(tx *gorm.DB, id int64) error {
	txSvc := *s
	txSvc.repo = s.repo.WithTx(tx)
	return txSvc.deleteConfig(id)
}

func (s *LLMConfigService) deleteConfig(id int64) error {
	if _, err := s.repo.GetByID(id); err != nil {
		return ErrConfigNotFound
	}
	return s.repo.Delete(id)
}

func (s *LLMConfigService) ActivateConfig(id int64) error {
	return s.activateConfig(id)
}

func (s *LLMConfigService) ActivateConfigTx(tx *gorm.DB, id int64) error {
	txSvc := *s
	txSvc.repo = s.repo.WithTx(tx)
	if _, err := txSvc.repo.GetByID(id); err != nil {
		return ErrConfigNotFound
	}
	return txSvc.repo.ActivateTx(id)
}

func (s *LLMConfigService) activateConfig(id int64) error {
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

	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("connection test failed: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

func (s *LLMConfigService) GetActiveConfig() (*model.LLMConfig, error) {
	return s.repo.GetActive()
}

func encryptLLMAPIKey(apiKey string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", nil
	}
	key, err := llmEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
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
		// 存量明文（该列历史上直接存过明文）：透传保持兼容；调用侧负责审计。
		return apiKeyEnc, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(apiKeyEnc, "v1:"))
	if err != nil {
		return "", err
	}
	key, err := llmEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
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

// llmEncryptionKey resolves the AES-GCM key for api_key_enc static
// encryption. Both env vars missing is a hard error, never sha256("") —
// that constant is publicly computable and would fake-encrypt stored
// provider keys (SP-25 中-4). Release mode fails startup on this via
// config.ValidateRelease. Note: the JWT_SECRET fallback couples key
// rotation — rotating the JWT secret breaks decryption of historical
// rows; rotate stored keys by re-entering them.
func llmEncryptionKey() ([]byte, error) {
	secret := os.Getenv("LLM_KEY_ENCRYPTION_SECRET")
	if secret == "" {
		secret = os.Getenv("JWT_SECRET")
	}
	if secret == "" {
		return nil, ErrLLMKeyEncryptionSecretMissing
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:], nil
}
