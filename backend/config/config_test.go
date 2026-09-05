package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// validateReleaseErrPrefix is the stable error prefix returned by
// Config.ValidateRelease(). ValidateRelease() returns a plain fmt.Errorf
// (not a structured AppError), so we cannot use errors.As to unwrap a code
// field. Instead, we assert on two things: (1) the stable prefix that
// identifies the error category, and (2) the field-path substring that
// identifies the specific validation rule. This is more robust than a
// single strings.Contains on an arbitrary human-readable substring because
// the prefix acts as a stable error-code-like identifier even though the
// production code does not yet use a structured error type.
const validateReleaseErrPrefix = "release mode configuration error"

func loadDefaultConfigForTest(t *testing.T) *Config {
	t.Helper()

	raw, err := os.ReadFile("../config.yaml")
	require.NoError(t, err)

	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(string(raw))))

	var cfg Config
	require.NoError(t, v.Unmarshal(&cfg))
	return &cfg
}

func TestDefaultRAGChunkingConfig(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)
	// A-04 裁决（2026-09-05）：hybrid 默认开（C1）；扩展/rerank 维持默认关。
	require.True(t, cfg.Features.RAGHybridEnabled)
	require.False(t, cfg.Features.RAGQueryExpansionEnabled)
	require.False(t, cfg.Features.RAGRerankEnabled)
	require.Equal(t, 512, cfg.RAG.Chunking.MaxTokens)
	require.Equal(t, 48, cfg.RAG.Chunking.OverlapTokens)
	require.Equal(t, 1, cfg.RAG.Chunking.ChunkingVersion)
	require.Equal(t, "cl100k_base", cfg.RAG.Chunking.TokenizerEncoding)
	require.Equal(t, "http://127.0.0.1:9200", cfg.RAG.Index.URL)
	require.Equal(t, 1, cfg.RAG.Index.GenerationStart)
	require.Equal(t, "text-embedding-v4", cfg.RAG.Index.EmbeddingModel)
	require.Equal(t, 1, cfg.RAG.Index.HealthPollIntervalSec)
	require.Equal(t, 10, cfg.RAG.Index.TimeoutSec)
	require.Equal(t, 2, cfg.RAG.Index.AuditTimeoutSec)
	require.Equal(t, 2, cfg.RAG.Index.LockCleanupTimeoutSec)
	require.Equal(t, 65536, cfg.RAG.Index.ErrorBodyMaxBytes)
	require.Equal(t, 1048576, cfg.RAG.Index.ResponseBodyMaxBytes)
	require.Equal(t, 200, cfg.RAG.Hybrid.BM25TopK)
	require.Equal(t, 200, cfg.RAG.Hybrid.VectorTopK)
	require.Equal(t, 60, cfg.RAG.Hybrid.RRFK)
	require.Equal(t, 10, cfg.RAG.Hybrid.FinalTopK)
	// Canonical profile: the pg_jieba Postgres retriever is the lexical
	// primary; OpenSearch is the optional fallback.
	require.Equal(t, "postgres", cfg.RAG.Hybrid.KeywordSource)
}

func TestValidateReleaseRejectsInvalidRAGChunkingConfig(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	cases := []struct {
		name   string
		mutate func(*RAGChunkingConfig)
		want   string
	}{
		{"non-positive max tokens", func(c *RAGChunkingConfig) { c.MaxTokens = 0 }, "rag.chunking.max_tokens"},
		{"negative overlap", func(c *RAGChunkingConfig) { c.OverlapTokens = -1 }, "rag.chunking.overlap_tokens"},
		{"overlap reaches max", func(c *RAGChunkingConfig) { c.OverlapTokens = c.MaxTokens }, "rag.chunking.overlap_tokens"},
		{"non-positive version", func(c *RAGChunkingConfig) { c.ChunkingVersion = 0 }, "rag.chunking.version"},
		{"unsupported tokenizer", func(c *RAGChunkingConfig) { c.TokenizerEncoding = "o200k_base" }, "rag.chunking.tokenizer_encoding"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validReleaseConfigForTest()
			cfg.Features.RAGHybridEnabled = true
			cfg.RAG.Chunking = RAGChunkingConfig{
				MaxTokens: 512, OverlapTokens: 48, ChunkingVersion: 1, TokenizerEncoding: "cl100k_base",
			}
			tc.mutate(&cfg.RAG.Chunking)

			err := cfg.ValidateRelease()
			require.Error(t, err)
			require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateReleaseRejectsInvalidRAGIndexConfig(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	cfg := validReleaseConfigForTest()
	cfg.Features.RAGHybridEnabled = true
	cfg.RAG.Chunking = RAGChunkingConfig{MaxTokens: 512, OverlapTokens: 48, ChunkingVersion: 1, TokenizerEncoding: "cl100k_base"}
	cfg.RAG.Index = RAGIndexConfig{
		URL:             "not-a-url",
		GenerationStart: 1, EmbeddingModel: "text-embedding-3-small", TimeoutSec: 10,
	}
	cfg.RAG.Hybrid = RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10}
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.Contains(t, err.Error(), "rag.index.url")
}

func TestValidateReleaseRejectsMissingRAGOperationalLimits(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	cfg := validReleaseConfigForTest()
	cfg.Features.RAGHybridEnabled = true
	cfg.RAG.Chunking = RAGChunkingConfig{MaxTokens: 512, OverlapTokens: 48, ChunkingVersion: 1, TokenizerEncoding: "cl100k_base"}
	cfg.RAG.Index = RAGIndexConfig{
		URL: "http://opensearch:9200", GenerationStart: 1, EmbeddingModel: "text-embedding-3-small",
		TimeoutSec: 10, AuditTimeoutSec: 0, LockCleanupTimeoutSec: 0, ErrorBodyMaxBytes: 0, ResponseBodyMaxBytes: 0,
	}
	cfg.RAG.Hybrid = RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10}
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.Contains(t, err.Error(), "rag.index.audit_timeout_sec")
	require.Contains(t, err.Error(), "rag.index.lock_cleanup_timeout_sec")
	require.Contains(t, err.Error(), "rag.index.error_body_max_bytes")
	require.Contains(t, err.Error(), "rag.index.response_body_max_bytes")
}

func TestValidateReleaseRejectsRAGEmbeddingIdentityDrift(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	cfg := validReleaseConfigForTest()
	cfg.Features.RAGHybridEnabled = true
	cfg.Agent.EmbeddingModel = "text-embedding-3-small"
	cfg.RAG.Chunking = RAGChunkingConfig{MaxTokens: 512, OverlapTokens: 48, ChunkingVersion: 1, TokenizerEncoding: "cl100k_base"}
	cfg.RAG.Index = RAGIndexConfig{URL: "http://opensearch:9200", GenerationStart: 1, EmbeddingModel: "other-model", TimeoutSec: 10}
	cfg.RAG.Hybrid = RAGHybridConfig{BM25TopK: 200, VectorTopK: 200, RRFK: 60, FinalTopK: 10}
	require.ErrorContains(t, cfg.ValidateRelease(), "must match agent.embedding_model")
}

func TestDefaultConfigDoesNotExposeFixedRAGIndexIdentity(t *testing.T) {
	raw, err := os.ReadFile("../config.yaml")
	require.NoError(t, err)
	require.NotContains(t, string(raw), "index_prefix:")
	require.NotContains(t, string(raw), "read_alias:")
}

func TestLoadAppliesRAGIndexURLEnvOverride(t *testing.T) {
	t.Setenv("RAG_INDEX_URL", "http://opensearch:9200")
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/config.yaml", []byte("rag:\n  index:\n    url: http://127.0.0.1:9200\n"), 0o600))
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWD)) })
	require.Equal(t, "http://opensearch:9200", Load().RAG.Index.URL)
}

func TestLoadAppliesRAGHybridFinalTopKEnvOverride(t *testing.T) {
	t.Setenv("RAG_HYBRID_FINAL_TOPK", "20")
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/config.yaml", []byte("rag:\n  hybrid:\n    final_topk: 10\n"), 0o600))
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWD)) })
	require.Equal(t, 20, Load().RAG.Hybrid.FinalTopK)
}

func TestLoadRAGRerankKeyFallsBackToDashScopeMasterKey(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "dash-master-key")
	t.Setenv("RAG_RERANK_API_KEY", "")
	t.Setenv("RAG_RERANK_API_BASE", "https://rerank.example.com")
	t.Setenv("RAG_RERANK_FALLBACK_API_BASE", "https://fallback.example.com")
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/config.yaml", []byte("rag:\n  rerank:\n    provider: dashscope\n"), 0o600))
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWD)) })
	cfg := Load()
	require.Equal(t, "dash-master-key", cfg.RAG.Rerank.APIKey)
	require.Equal(t, "https://rerank.example.com", cfg.RAG.Rerank.APIBase)
	require.Equal(t, "https://fallback.example.com", cfg.RAG.Rerank.FallbackAPIBase)
}

func TestLoadExplicitRAGRerankKeyWinsOverDashScopeFallback(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "dash-master-key")
	t.Setenv("RAG_RERANK_API_KEY", "dedicated-rerank-key")
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/config.yaml", []byte("rag:\n  rerank:\n    provider: dashscope\n"), 0o600))
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWD)) })
	require.Equal(t, "dedicated-rerank-key", Load().RAG.Rerank.APIKey)
}

func TestLoadRAGRerankKeyStaysEmptyWithoutAnyKeyEnv(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "")
	t.Setenv("RAG_RERANK_API_KEY", "")
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/config.yaml", []byte("rag:\n  rerank:\n    provider: dashscope\n"), 0o600))
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWD)) })
	require.Empty(t, Load().RAG.Rerank.APIKey)
}

func TestLoadDoesNotLetGenericAgentEnvShadowAgentSection(t *testing.T) {
	t.Setenv("AGENT", "1")
	t.Setenv("AGENT_WEB_AGENT_ENABLED", "true")

	tmp := t.TempDir()
	configYAML := []byte(`agent:
  web_agent_enabled: false
  llm_provider: minimax
  llm_model: MiniMax-M1
  llm_api_base: https://api.minimaxi.com
  embedding_model: embo-01
`)
	require.NoError(t, os.WriteFile(tmp+"/config.yaml", configYAML, 0o600))

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWD)) })

	cfg := Load()
	require.Equal(t, "minimax", cfg.Agent.LLMProvider)
	require.Equal(t, "MiniMax-M1", cfg.Agent.LLMModel)
	require.Equal(t, "embo-01", cfg.Agent.EmbeddingModel)
	require.True(t, cfg.Agent.WebAgentEnabled, "explicit namespaced override must still apply")
}

func TestLoadAppliesExplicitAgentEnvAfterConfigOverride(t *testing.T) {
	t.Setenv("AGENT_WEB_AGENT_ENABLED", "true")
	t.Setenv("AGENT_LLM_PROVIDER", "minimax")
	t.Setenv("AGENT_LLM_MODEL", "MiniMax-M1")
	t.Setenv("AGENT_LLM_API_BASE", "https://api.minimaxi.com")
	t.Setenv("AGENT_EMBEDDING_MODEL", "embo-01")
	t.Setenv("AGENT_EMBEDDING_API_BASE", "https://api.minimax.chat")
	t.Setenv("AGENT_EMBEDDING_GROUP_ID", "group-123")

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/config.yaml", []byte(`agent:
  web_agent_enabled: false
  llm_provider: openai_compat
  llm_model: stale-chat
  llm_api_base: https://stale.example.test
  embedding_model: stale-embedding
`), 0o600))
	require.NoError(t, os.WriteFile(tmp+"/override.yaml", []byte(`agent:
  web_agent_enabled: false
  llm_provider: qwen
  llm_model: stale-override-chat
  llm_api_base: https://stale-override.example.test
  embedding_model: stale-override-embedding
`), 0o600))
	t.Setenv("CONFIG_OVERRIDE_PATH", tmp+"/override.yaml")

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWD)) })

	cfg := Load()
	require.True(t, cfg.Agent.WebAgentEnabled)
	require.Equal(t, "minimax", cfg.Agent.LLMProvider)
	require.Equal(t, "MiniMax-M1", cfg.Agent.LLMModel)
	require.Equal(t, "https://api.minimaxi.com", cfg.Agent.LLMAPIBase)
	require.Equal(t, "embo-01", cfg.Agent.EmbeddingModel)
	require.Equal(t, "https://api.minimax.chat", cfg.Agent.EmbeddingAPIBase)
	require.Equal(t, "group-123", cfg.Agent.EmbeddingGroupID)
	require.Equal(t, "embo-01", cfg.RAG.Index.EmbeddingModel)
}

func TestLoadAllowsExplicitRAGIndexEmbeddingModelOverride(t *testing.T) {
	t.Setenv("AGENT_EMBEDDING_MODEL", "embo-01")
	t.Setenv("RAG_INDEX_EMBEDDING_MODEL", "index-model")

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/config.yaml", []byte(`agent:
  embedding_model: stale-agent
rag:
  index:
    embedding_model: stale-index
`), 0o600))
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWD)) })

	cfg := Load()
	require.Equal(t, "embo-01", cfg.Agent.EmbeddingModel)
	require.Equal(t, "index-model", cfg.RAG.Index.EmbeddingModel)
}

func TestLoadPrefersRepositoryRootEnvWhenStartedFromBackend(t *testing.T) {
	repo := t.TempDir()
	backendDir := filepath.Join(repo, "backend")
	require.NoError(t, os.Mkdir(backendDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(backendDir, "config.yaml"), []byte("agent:\n  llm_model: yaml-model\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".env"), []byte("AGENT_LLM_MODEL=root-model\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(backendDir, ".env"), []byte("AGENT_LLM_MODEL=stale-backend-model\n"), 0o600))

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(backendDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWD)) })

	// The loader's parent candidate represents the repository root when the
	// process is started from backend/; explicit environment remains absent.
	previousModel, hadModel := os.LookupEnv("AGENT_LLM_MODEL")
	require.NoError(t, os.Unsetenv("AGENT_LLM_MODEL"))
	t.Cleanup(func() {
		if hadModel {
			_ = os.Setenv("AGENT_LLM_MODEL", previousModel)
		} else {
			_ = os.Unsetenv("AGENT_LLM_MODEL")
		}
	})
	cfg := Load()
	require.Equal(t, "root-model", cfg.Agent.LLMModel)
}

func TestLoadOverridePartialSectionPreservesSiblingFields(t *testing.T) {
	// Regression guard for #127: an override file containing only part of a
	// nested section must merge field-by-field and never zero sibling fields
	// that are absent from the file.
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/override.yaml", []byte("queue:\n  enabled: true\n"), 0o600))

	cfg := loadDefaultConfigForTest(t)
	require.Equal(t, 3, cfg.Queue.MaxAttempts, "sanity: default max_attempts")
	require.Equal(t, []int{10, 60, 300}, cfg.Queue.RetryBackoffSec, "sanity: default retry backoff")

	LoadOverride(cfg, tmp+"/override.yaml")

	require.True(t, cfg.Queue.Enabled, "override must apply the present key")
	require.Equal(t, 3, cfg.Queue.MaxAttempts, "absent sibling key must not be zeroed")
	require.Equal(t, []int{10, 60, 300}, cfg.Queue.RetryBackoffSec, "absent sibling key must not be zeroed")
	require.Equal(t, 168, cfg.Queue.DLQTTLHours, "absent sibling key must not be zeroed")
	require.Equal(t, 2, cfg.Queue.WorkerReview, "absent sibling key must not be zeroed")
}

func TestLoadOverridePartialSectionsInSameFile(t *testing.T) {
	// Multiple partially-present sections must each merge independently
	// without zeroing siblings in any of them.
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/override.yaml", []byte(`queue:
  enabled: true
rate_limit:
  enabled: false
  search_per_minute: 0
`), 0o600))

	cfg := loadDefaultConfigForTest(t)
	LoadOverride(cfg, tmp+"/override.yaml")

	require.True(t, cfg.Queue.Enabled)
	require.Equal(t, 3, cfg.Queue.MaxAttempts)
	require.False(t, cfg.RateLimit.Enabled)
	require.Equal(t, 0, cfg.RateLimit.SearchPerMinute)
	require.Equal(t, 100, cfg.RateLimit.NormalPerMinute, "absent sibling key must not be zeroed")
	require.Equal(t, 200, cfg.RateLimit.UploadPerHour, "absent sibling key must not be zeroed")
}

func TestLoadOverrideMissingFileIsNoOp(t *testing.T) {
	tmp := t.TempDir()
	cfg := loadDefaultConfigForTest(t)
	LoadOverride(cfg, tmp+"/does-not-exist.yaml")
	require.Equal(t, 3, cfg.Queue.MaxAttempts)
	require.True(t, cfg.Queue.Enabled, "dev default enables the queue broker for the standalone worker (issue #138)")
}

func TestValidateReleaseRejectsBypassCaptchaAndLoggerSMTP(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Mode = "release"
	cfg.Captcha.Provider = "bypass"
	cfg.SMTP.Mode = "logger"

	err := cfg.ValidateRelease()
	require.Error(t, err)
	// Assert stable error-category prefix (acts as structured code substitute).
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix),
		"error = %q, want prefix %q", err.Error(), validateReleaseErrPrefix)
}

func TestValidateReleaseRejectsDefaultJWTSecret(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Mode = "release"
	cfg.Captcha.Provider = "aliyun_v2"
	cfg.SMTP.Mode = "smtp"
	cfg.JWT.Secret = "dev-secret-change-in-production"

	err := cfg.ValidateRelease()
	require.Error(t, err)
	// Assert stable error-category prefix (acts as structured code substitute).
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix),
		"error = %q, want prefix %q", err.Error(), validateReleaseErrPrefix)
	require.ErrorContains(t, err, "jwt.secret")
}

func validReleaseConfigForTest() *Config {
	return &Config{
		Server:   ServerConfig{Mode: "release", Port: "8080", ShutdownTimeout: 15, ReadTimeout: 30, WriteTimeout: 60, IdleTimeout: 120},
		Web:      WebConfig{PublicBaseURL: "https://app.omnicraft.prod"},
		Security: SecurityConfig{AllowedOrigins: []string{"https://app.omnicraft.prod"}},
		Database: DatabaseConfig{
			DSN: "host=db.omnicraft.prod port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=verify-full",
		},
		Redis: RedisConfig{Addr: "redis:6379", Password: "redis-secret", DB: 0},
		JWT: JWTConfig{
			Secret: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
		OSS: OSSConfig{
			Endpoint:        "https://oss-cn-hangzhou.aliyuncs.com",
			AccessKeyID:     "oss-key-id",
			AccessKeySecret: "oss-key-secret",
			BucketName:      "private-bucket",
			Domain:          "https://private-bucket.oss-cn-hangzhou.aliyuncs.com",
			DownloadURLTTL:  300,
		},
		Green: GreenConfig{
			AccessKeyID:     "green-key-id",
			AccessKeySecret: "green-key-secret",
			Region:          "cn-shanghai",
			CallbackURL:     "https://api.omnicraft.prod/api/v1/internal/ai-callback",
			Seed:            "test_seed_abc123",
			UID:             "1234567890123456",
		},
		Features: FeaturesConfig{
			PaymentEnabled:        false,
			CreatorSupportEnabled: false,
			DesktopDeployEnabled:  false,
		},
		Captcha: CaptchaConfig{
			Provider:        "aliyun_v2",
			Prefix:          "captcha-prefix",
			SceneID:         "captcha-scene",
			Region:          "cn",
			AccessKeyID:     "captcha-key-id",
			AccessKeySecret: "captcha-key-secret",
		},
		SMTP: SMTPConfig{
			Mode:        "smtp",
			Host:        "smtp.omnicraft.prod",
			Port:        587,
			User:        "mailer@omnicraft.prod",
			Password:    "smtp-secret",
			FromAddress: "noreply@omnicraft.prod",
		},
		Legal: LegalConfig{
			CurrentTermsVersion:   "2026-06-08",
			CurrentPrivacyVersion: "2026-06-08",
		},
		Client: ClientConfig{
			DownloadEnabled: false,
			DownloadURL:     "",
			LatestVersion:   "",
		},
		Agent: AgentConfig{
			WebAgentEnabled:       false,
			RateLimitPerDay:       50,
			UploadAssistMaxFileMB: 10,
			MaxUserMessageChars:   4000,
			ChatMaxContextMsgs:    10,
			ConversationListLimit: 50,
			ConversationPageSize:  20,
		},
		RateLimit: RateLimitConfig{
			Enabled:              true,
			NormalPerMinute:      100,
			UploadPerHour:        200,
			AgentWindowSec:       86400,
			AgentMinuteWindowSec: 60,
		},
		Relay: RelayConfig{
			BatchSize:       100,
			PollIntervalSec: 1,
		},
		Observability: ObservabilityConfig{
			MetricsPort:          "9091",
			LogLevel:             "info",
			LogIPHashSecret:      "observability-ip-hash-secret-value",
			LogIPKeyID:           "current",
			ReadHeaderTimeoutSec: 5,
			Readiness:            ReadinessConfig{DBTimeoutSec: 3, RedisTimeoutSec: 3},
		},
	}
}

func TestValidateReleaseAcceptsCompleteProductionConfig(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	cfg := validReleaseConfigForTest()
	if err := cfg.ValidateRelease(); err != nil {
		t.Fatalf("ValidateRelease() unexpected error: %v", err)
	}
}

func TestValidateReleaseRejectsIncompleteProductionConfig(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"localhost public base url", func(c *Config) { c.Web.PublicBaseURL = "http://localhost:3000" }, "web.public_base_url"},
		{"missing allowed origins", func(c *Config) { c.Security.AllowedOrigins = nil }, "security.allowed_origins"},
		{"wildcard allowed origin", func(c *Config) { c.Security.AllowedOrigins = []string{"*"} }, "wildcard"},
		{"localhost allowed origin", func(c *Config) { c.Security.AllowedOrigins = []string{"http://localhost:3000"} }, "localhost"},
		{"missing redis password", func(c *Config) { c.Redis.Password = "" }, "redis.password"},
		{"missing oss endpoint", func(c *Config) { c.OSS.Endpoint = "" }, "oss.endpoint"},
		{"missing oss secret", func(c *Config) { c.OSS.AccessKeySecret = "" }, "oss.access_key_secret"},
		{"missing green callback url", func(c *Config) { c.Green.CallbackURL = "" }, "green.callback_url"},
		{"missing green seed", func(c *Config) { c.Green.Seed = "" }, "green.seed"},
		{"missing green uid", func(c *Config) { c.Green.UID = "" }, "green.uid"},
		{"missing captcha public fields", func(c *Config) { c.Captcha.Prefix = "" }, "captcha.prefix"},
		{"missing captcha secret", func(c *Config) { c.Captcha.AccessKeySecret = "" }, "captcha.access_key_secret"},
		{"missing smtp host", func(c *Config) { c.SMTP.Host = "" }, "smtp.host"},
		{"missing smtp password", func(c *Config) { c.SMTP.Password = "" }, "smtp.password"},
		{"missing terms version", func(c *Config) { c.Legal.CurrentTermsVersion = "" }, "legal.current_terms_version"},
		{"missing privacy version", func(c *Config) { c.Legal.CurrentPrivacyVersion = "" }, "legal.current_privacy_version"},
		{"desktop deploy enabled", func(c *Config) { c.Features.DesktopDeployEnabled = true }, "desktop_deploy_enabled"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validReleaseConfigForTest()
			tc.mutate(cfg)
			err := cfg.ValidateRelease()
			if err == nil {
				t.Fatal("ValidateRelease() error = nil, want error")
			}
			// Assert the stable error-category prefix first; this acts as a
			// structured error-code substitute since ValidateRelease returns
			// plain fmt.Errorf, not a typed error with a Code field.
			if !strings.HasPrefix(err.Error(), validateReleaseErrPrefix) {
				t.Fatalf("ValidateRelease() error = %q, want prefix %q", err.Error(), validateReleaseErrPrefix)
			}
			// Then assert the specific field-path identifier is present.
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateRelease() error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestValidateReleaseRejectsMissingLLMKeyEncryptionSecret(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "")
	cfg := validReleaseConfigForTest()
	err := cfg.ValidateRelease()
	if err == nil {
		t.Fatal("ValidateRelease() error = nil, want error")
	}
	// Assert stable error-category prefix (acts as structured code substitute).
	if !strings.HasPrefix(err.Error(), validateReleaseErrPrefix) {
		t.Fatalf("ValidateRelease() error = %q, want prefix %q", err.Error(), validateReleaseErrPrefix)
	}
	if !strings.Contains(err.Error(), "LLM_KEY_ENCRYPTION_SECRET") {
		t.Fatalf("ValidateRelease() error = %q, want substring LLM_KEY_ENCRYPTION_SECRET", err.Error())
	}
}

func TestDefaultConfigDeclaresCreatorSupportDisabled(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)
	require.False(t, cfg.Features.CreatorSupportEnabled)
}

func TestDefaultConfigHasAbuseControlLimits(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	require.Positive(t, cfg.RateLimit.NormalPerMinute)
	require.Positive(t, cfg.RateLimit.UploadPerHour)
	require.Positive(t, cfg.RateLimit.CredentialPerMinute)
	require.Positive(t, cfg.RateLimit.SearchPerMinute)
	require.Positive(t, cfg.RateLimit.MaxJSONBodyBytes)
	require.Positive(t, cfg.RateLimit.MaxQueryChars)
	require.Positive(t, cfg.RateLimit.MaxSearchPage)
}

func TestDefaultConfigJSONBodyLimitAllowsTextUploads(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)
	minBodyBytes := int64(cfg.Limits.TextMaxMB) * 1024 * 1024
	require.GreaterOrEqual(t, cfg.RateLimit.MaxJSONBodyBytes, minBodyBytes)
}

func TestBrowseHistoryConfig(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	require.Equal(t, 7, cfg.BrowseHistory.RetentionDays)
	require.Equal(t, "03:00", cfg.BrowseHistory.CleanupTime)
}

func TestCollaborationConfig(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	require.Equal(t, 20, cfg.Collaboration.InviteDailyLimit)
	require.Equal(t, 7, cfg.Collaboration.InviteExpireDays)
	require.Equal(t, 5, cfg.Collaboration.MaxInviteesPerPublish)
	require.Equal(t, 10, cfg.Collaboration.MaxContributorsPerItem)
}

func TestApplyTestModeReplacesNormalDatabaseConfigurationAfterOverrides(t *testing.T) {
	t.Setenv("OMNICRAFT_TEST_MODE", "1")
	t.Setenv("OMNICRAFT_TEST_DB_DSN", "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=omnicraft_test_cross_stack sslmode=disable")
	t.Setenv("OMNICRAFT_TEST_REDIS_DB", "15")

	cfg := &Config{
		Database: DatabaseConfig{DSN: "host=db.omnicraft.prod dbname=production", ReadDSN: "host=replica.omnicraft.prod dbname=production"},
		Redis:    RedisConfig{Addr: "127.0.0.1:6379", DB: 0},
	}

	require.NoError(t, applyTestMode(cfg))
	require.Equal(t, "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=omnicraft_test_cross_stack sslmode=disable", cfg.Database.DSN)
	require.Empty(t, cfg.Database.ReadDSN)
	require.Equal(t, 15, cfg.Redis.DB)
}

func TestApplyTestModeRejectsUnsafeTestConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		redisDB  string
		wantPart string
	}{
		{"remote database", "host=db.omnicraft.prod dbname=omnicraft_test_cross_stack", "15", "loopback"},
		{"normal database name", "host=127.0.0.1 dbname=omnicraft", "15", "omnicraft_test_"},
		{"redis db zero", "host=127.0.0.1 dbname=omnicraft_test_cross_stack", "0", "non-zero"},
		{"invalid redis db", "host=127.0.0.1 dbname=omnicraft_test_cross_stack", "not-a-number", "valid integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OMNICRAFT_TEST_MODE", "1")
			t.Setenv("OMNICRAFT_TEST_DB_DSN", tt.dsn)
			t.Setenv("OMNICRAFT_TEST_REDIS_DB", tt.redisDB)

			err := applyTestMode(&Config{Redis: RedisConfig{Addr: "127.0.0.1:6379"}})
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantPart)
		})
	}
}

func TestApplyTestModeRequiresLoopbackRedisWithValidPort(t *testing.T) {
	tests := []struct {
		name      string
		redisAddr string
		wantErr   bool
	}{
		{"localhost", "localhost:6379", false},
		{"ipv4 loopback", "127.0.0.1:6379", false},
		{"ipv6 loopback", "[::1]:6379", false},
		{"remote host", "redis.omnicraft.prod:6379", true},
		{"empty address", "", true},
		{"missing port", "127.0.0.1", true},
		{"invalid port", "127.0.0.1:not-a-port", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OMNICRAFT_TEST_MODE", "1")
			t.Setenv("OMNICRAFT_TEST_DB_DSN", "host=127.0.0.1 dbname=omnicraft_test_cross_stack")
			t.Setenv("OMNICRAFT_TEST_REDIS_DB", "15")

			err := applyTestMode(&Config{Redis: RedisConfig{Addr: tt.redisAddr}})
			if (err != nil) != tt.wantErr {
				t.Fatalf("applyTestMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyTestModeRequiresExplicitTestInputs(t *testing.T) {
	t.Setenv("OMNICRAFT_TEST_MODE", "1")
	t.Setenv("OMNICRAFT_TEST_DB_DSN", "")
	t.Setenv("OMNICRAFT_TEST_REDIS_DB", "")

	err := applyTestMode(&Config{})
	require.ErrorContains(t, err, "OMNICRAFT_TEST_DB_DSN")
}

func TestDefaultConfigDeclaresObservabilitySection(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	require.Equal(t, "9091", cfg.Observability.MetricsPort)
	require.Equal(t, "info", cfg.Observability.LogLevel)
	require.NotEmpty(t, cfg.Observability.LogIPKeyID)
	require.Positive(t, cfg.Observability.Readiness.DBTimeoutSec)
	require.Positive(t, cfg.Observability.Readiness.RedisTimeoutSec)
}

func TestDefaultConfigDeclaresWorkerServiceName(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)
	require.Equal(t, "omnicraft-worker", cfg.Worker.ServiceName)
}

func TestOverrideFromEnvAppliesOTelServiceNameToServerAndWorker(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "omnicraft-test")
	cfg := &Config{}

	OverrideFromEnv(cfg)

	require.Equal(t, "omnicraft-test", cfg.Observability.Tracing.ServiceName)
	require.Equal(t, "omnicraft-test", cfg.Worker.ServiceName)
}

func TestValidateReleaseRequiresIPHashSecretAndKeyID(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Observability.LogIPHashSecret = ""
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
	require.ErrorContains(t, err, "observability.log_ip_hash_secret")

	cfg = validReleaseConfigForTest()
	cfg.Observability.LogIPKeyID = ""
	err = cfg.ValidateRelease()
	require.Error(t, err)
	require.ErrorContains(t, err, "observability.log_ip_key_id")
}

func TestValidateReleaseRejectsIncompleteIPKeyRotation(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Observability.LogIPHashSecret = "current-secret-value"
	cfg.Observability.LogIPKeyID = "current"
	cfg.Observability.IPKeyRotation.PreviousKeyID = "previous"

	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.ErrorContains(t, err, "observability.ip_key_rotation")
}

func TestValidateReleaseRejectsMalformedRotationWindow(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Observability.LogIPHashSecret = "current-secret-value"
	cfg.Observability.LogIPKeyID = "current"
	cfg.Observability.IPKeyRotation = IPKeyRotationConfig{
		PreviousSecret: "previous-secret-value",
		PreviousKeyID:  "previous",
		ActiveFrom:     "not-a-timestamp",
		ActiveUntil:    "2026-12-31T23:59:59Z",
	}

	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.ErrorContains(t, err, "ip_key_rotation")
}

func TestValidateReleaseRejectsReversedRotationWindow(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Observability.LogIPHashSecret = "current-secret-value"
	cfg.Observability.LogIPKeyID = "current"
	cfg.Observability.IPKeyRotation = IPKeyRotationConfig{
		PreviousSecret: "previous-secret-value",
		PreviousKeyID:  "previous",
		ActiveFrom:     "2026-12-31T23:59:59Z",
		ActiveUntil:    "2026-01-01T00:00:00Z",
	}

	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.ErrorContains(t, err, "ip_key_rotation")
}

func TestValidateReleaseAcceptsValidRotationWindow(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Observability.LogIPHashSecret = "current-secret-value"
	cfg.Observability.LogIPKeyID = "current"
	cfg.Observability.IPKeyRotation = IPKeyRotationConfig{
		PreviousSecret: "previous-secret-value",
		PreviousKeyID:  "previous",
		ActiveFrom:     "2026-01-01T00:00:00Z",
		ActiveUntil:    "2026-12-31T23:59:59Z",
	}

	require.NoError(t, cfg.ValidateRelease())
}

func TestValidateReleaseRejectsPlaceholderValues(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "")

	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"angle placeholder in jwt secret", func(c *Config) { c.JWT.Secret = "<openssl-rand-base64-64>" }, "jwt.secret"},
		{"change-me placeholder in redis password", func(c *Config) { c.Redis.Password = "CHANGE_ME" }, "redis.password"},
		{"example domain in public base url", func(c *Config) { c.Web.PublicBaseURL = "https://app.example.com" }, "web.public_base_url"},
		{"placeholder in smtp host", func(c *Config) { c.SMTP.Host = "<smtp-host>" }, "smtp.host"},
		{"placeholder in oss bucket", func(c *Config) { c.OSS.BucketName = "your-bucket-name" }, "oss.bucket_name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validReleaseConfigForTest()
			tc.mutate(cfg)
			err := cfg.ValidateRelease()
			require.Error(t, err)
			require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestValidateReleaseRequiresVerifyFullDatabaseTLS(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "")

	cfg := validReleaseConfigForTest()
	cfg.Database.DSN = "host=db.omnicraft.prod port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=disable"
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
	require.ErrorContains(t, err, "sslmode")

	cfg = validReleaseConfigForTest()
	cfg.Database.DSN = "host=db.omnicraft.prod port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=require"
	err = cfg.ValidateRelease()
	require.Error(t, err)
	require.ErrorContains(t, err, "verify-full")
}

func TestValidateReleaseAllowsPrivateDbHostTLSNegotiation(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "pgbouncer")

	cfg := validReleaseConfigForTest()
	cfg.Database.DSN = "host=pgbouncer port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=disable"
	require.NoError(t, cfg.ValidateRelease())
}

func TestValidateReleaseRejectsSeedPlaceholder(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "")

	cfg := validReleaseConfigForTest()
	cfg.Green.Seed = "<seed-placeholder>"
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
	require.ErrorContains(t, err, "green.seed")
}

func TestValidateReleaseRejectsUIDPlaceholder(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "")

	cfg := validReleaseConfigForTest()
	cfg.Green.UID = "<aliyun-main-account-uid>"
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
	require.ErrorContains(t, err, "green.uid")
}

func TestValidateReleaseRejectsNonNumericUID(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "")

	cfg := validReleaseConfigForTest()
	cfg.Green.UID = "not-a-number"
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
	require.ErrorContains(t, err, "green.uid")
}

func TestValidateReleaseRejectsOverlongSeed(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "")

	cfg := validReleaseConfigForTest()
	cfg.Green.Seed = strings.Repeat("a", 65)
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
	require.ErrorContains(t, err, "green.seed")
}

func TestValidateReleaseRejectsIllegalSeedChars(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "")

	cfg := validReleaseConfigForTest()
	cfg.Green.Seed = "seed-with-dashes-123"
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
	require.ErrorContains(t, err, "green.seed")
}

func TestValidateReleaseAllowsEmptySeedAndUIDInLocalMode(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Server.Mode = "local"
	cfg.Green.Seed = ""
	cfg.Green.UID = ""
	require.NoError(t, cfg.ValidateRelease())
}

func TestApplyTestModeLeavesNormalConfigurationUnchanged(t *testing.T) {
	t.Setenv("OMNICRAFT_TEST_MODE", "")
	t.Setenv("OMNICRAFT_TEST_DB_DSN", "host=127.0.0.1 dbname=omnicraft_test_ignored")
	t.Setenv("OMNICRAFT_TEST_REDIS_DB", "15")

	cfg := &Config{Database: DatabaseConfig{DSN: "host=db.omnicraft.prod dbname=production", ReadDSN: "host=replica.omnicraft.prod dbname=production"}, Redis: RedisConfig{DB: 0}}
	require.NoError(t, applyTestMode(cfg))
	require.Equal(t, "host=db.omnicraft.prod dbname=production", cfg.Database.DSN)
	require.Equal(t, "host=replica.omnicraft.prod dbname=production", cfg.Database.ReadDSN)
	require.Equal(t, 0, cfg.Redis.DB)
}

func TestValidGreenSeed(t *testing.T) {
	cases := []struct {
		name string
		seed string
		want bool
	}{
		{"alphanumeric underscores", "seed_abc_123", true},
		{"single char", "a", true},
		{"exactly 64 chars", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"empty rejected", "", false},
		{"whitespace rejected", "  a  ", false},
		{"dash rejected", "seed-abc", false},
		{"dot rejected", "seed.abc", false},
		{"overlong 65 chars rejected", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidGreenSeed(tc.seed); got != tc.want {
				t.Fatalf("ValidGreenSeed(%q) = %v, want %v", tc.seed, got, tc.want)
			}
		})
	}
}

func TestUploadConfigNormalizesGalleryDefaultsAndRejectsInvalidBounds(t *testing.T) {
	defaults := (UploadConfig{}).NormalizedGalleryLimits()
	require.Equal(t, 2, defaults.ImageGalleryMinItems)
	require.Equal(t, 9, defaults.ImageGalleryMaxItems)
	require.Equal(t, 1, defaults.VideoGalleryMinItems)
	require.Equal(t, 3, defaults.VideoGalleryMaxItems)
	require.NoError(t, (UploadConfig{}).ValidateGalleryLimits())

	invalid := UploadConfig{ImageGalleryMinItems: 10, ImageGalleryMaxItems: 2}
	require.ErrorContains(t, invalid.ValidateGalleryLimits(), "image gallery min_items")
}
