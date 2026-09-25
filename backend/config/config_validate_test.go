package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Ticket #671: Config.Validate() runs in every mode (debug and release) and
// checks required non-credential structural fields, enums/ranges and the
// structural requirements of enabled features. Credential checks stay in
// ValidateRelease().

// validDebugConfig returns the minimal structurally valid configuration: the
// required non-credential fields set, every feature switch off, no
// credentials. This is the "local mode without real credentials stays legal"
// baseline (spec user story 2).
func validDebugConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Mode:         "debug",
			Port:         "8080",
			ReadTimeout:  30,
			WriteTimeout: 30,
			IdleTimeout:  60,
		},
		Database:      DatabaseConfig{DSN: "host=localhost user=omnicraft dbname=omnicraft"},
		Redis:         RedisConfig{Addr: "localhost:6379"},
		Web:           WebConfig{PublicBaseURL: "http://localhost:3000"},
		JWT:           JWTConfig{Secret: "dev-secret-change-in-production"},
		Security:      SecurityConfig{AllowedOrigins: []string{"http://localhost:3000"}},
		Observability: ObservabilityConfig{MetricsPort: "9091"},
		Relay:         RelayConfig{BatchSize: 100, PollIntervalSec: 5},
	}
}

func TestValidateAcceptsMinimalDebugConfigWithoutCredentials(t *testing.T) {
	require.NoError(t, validDebugConfig().Validate())
}

func TestValidateAcceptsReleaseModeStructurally(t *testing.T) {
	cfg := validDebugConfig()
	cfg.Server.Mode = "release"
	// Structural validation must not demand credentials; ValidateRelease
	// owns those. Mode enum accepts release.
	require.NoError(t, cfg.Validate())
}

func TestValidateRejectsInvalidServerMode(t *testing.T) {
	for _, mode := range []string{"", "staging", "production", "DEBUG"} {
		cfg := validDebugConfig()
		cfg.Server.Mode = mode
		err := cfg.Validate()
		require.Error(t, err, "mode %q must be rejected", mode)
		contains(t, err, "server.mode")
	}
}

func TestValidateRejectsMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		path   string
	}{
		{"server.port empty", func(c *Config) { c.Server.Port = "" }, "server.port"},
		{"server.port zero", func(c *Config) { c.Server.Port = "0" }, "server.port"},
		{"server.read_timeout zero", func(c *Config) { c.Server.ReadTimeout = 0 }, "server.read_timeout"},
		{"server.write_timeout zero", func(c *Config) { c.Server.WriteTimeout = 0 }, "server.write_timeout"},
		{"server.idle_timeout zero", func(c *Config) { c.Server.IdleTimeout = 0 }, "server.idle_timeout"},
		{"database.dsn empty", func(c *Config) { c.Database.DSN = "" }, "database.dsn"},
		{"redis.addr empty", func(c *Config) { c.Redis.Addr = "" }, "redis.addr"},
		{"web.public_base_url empty", func(c *Config) { c.Web.PublicBaseURL = "" }, "web.public_base_url"},
		{"jwt.secret empty", func(c *Config) { c.JWT.Secret = "" }, "jwt.secret"},
		{"security.allowed_origins empty", func(c *Config) { c.Security.AllowedOrigins = nil }, "security.allowed_origins"},
		{"observability.metrics_port empty", func(c *Config) { c.Observability.MetricsPort = "" }, "observability.metrics_port"},
		{"relay.batch_size zero", func(c *Config) { c.Relay.BatchSize = 0 }, "relay.batch_size"},
		{"relay.batch_size over max", func(c *Config) { c.Relay.BatchSize = 10001 }, "relay.batch_size"},
		{"relay.poll_interval_sec zero", func(c *Config) { c.Relay.PollIntervalSec = 0 }, "relay.poll_interval_sec"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validDebugConfig()
			tc.mutate(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			contains(t, err, tc.path)
		})
	}
}

func TestValidateListsAllMissingRequiredPathsAtOnce(t *testing.T) {
	cfg := validDebugConfig()
	cfg.Server.Port = ""
	cfg.Database.DSN = ""
	cfg.Redis.Addr = ""
	err := cfg.Validate()
	require.Error(t, err)
	for _, path := range []string{"server.port", "database.dsn", "redis.addr"} {
		contains(t, err, path)
	}
}

func TestValidateConditionalAgentBlockGatedByFeature(t *testing.T) {
	// Feature off: zero agent limits are legal (disabled feature, local mode).
	cfg := validDebugConfig()
	cfg.Agent.CitationMaxCount = 0
	cfg.Agent.RateLimitPerDay = 0
	require.NoError(t, cfg.Validate())

	// Feature on: the structural limits must be positive even in debug.
	cfg.Agent.WebAgentEnabled = true
	cfg.Agent.LLMProvider = "minimax"
	cfg.Agent.LLMModel = "MiniMax-M3"
	cfg.Agent.LLMAPIKey = "placeholder"
	cfg.Agent.RateLimitPerDay = 50
	cfg.Agent.RateLimitPerMinute = 5
	cfg.Agent.MaxToolCallsPerTurn = 6
	cfg.Agent.MaxOutputTokens = 8192
	cfg.Agent.ProviderTimeoutSec = 60
	cfg.Agent.MaxUserMessageChars = 2000
	cfg.Agent.ChatMaxContextMsgs = 20
	cfg.Agent.ConversationListLimit = 20
	cfg.Agent.ConversationPageSize = 10
	cfg.Agent.ChatContextTokenBudget = 24000
	cfg.RateLimit.AgentWindowSec = 86400
	cfg.RateLimit.AgentMinuteWindowSec = 60
	cfg.Agent.CitationMaxCount = 0 // still zero -> rejected
	err := cfg.Validate()
	require.Error(t, err)
	contains(t, err, "agent.citation_max_count")
}

func TestValidateConditionalRAGHybridGatedByFeature(t *testing.T) {
	// Feature off: chunking zeros are legal.
	cfg := validDebugConfig()
	cfg.RAG.Chunking.MaxTokens = 0
	require.NoError(t, cfg.Validate())

	// Feature on: chunking must be positive in every mode.
	cfg.Features.RAGHybridEnabled = true
	err := cfg.Validate()
	require.Error(t, err)
	contains(t, err, "rag.chunking.max_tokens")
}

func TestValidateRejectsUnknownCaptchaProviderInAllModes(t *testing.T) {
	cfg := validDebugConfig()
	cfg.Captcha.Provider = "aliyun"
	err := cfg.Validate()
	require.Error(t, err)
	contains(t, err, "captcha.provider")

	// "bypass" stays legal in debug (release rejects it via ValidateRelease).
	cfg.Captcha.Provider = "bypass"
	require.NoError(t, cfg.Validate())

	// Empty means unconfigured: legal in debug.
	cfg.Captcha.Provider = ""
	require.NoError(t, cfg.Validate())
}

func TestValidateRejectsNegativeBreakerAndBadAuxCacheInAllModes(t *testing.T) {
	cfg := validDebugConfig()
	cfg.Resilience.Breaker.FailureThreshold = -1
	err := cfg.Validate()
	require.Error(t, err)
	contains(t, err, "resilience.breaker")

	cfg = validDebugConfig()
	cfg.Resilience.AuxCache.Title.Enabled = true
	cfg.Resilience.AuxCache.Title.TTLSec = 0
	err = cfg.Validate()
	require.Error(t, err)
	contains(t, err, "resilience.aux_cache.title.ttl_sec")
}

func TestValidateRejectsConditionalQueueMCPAndImageBlocks(t *testing.T) {
	// queue enabled with zero attempts
	cfg := validDebugConfig()
	cfg.Queue.Enabled = true
	err := cfg.Validate()
	require.Error(t, err)
	contains(t, err, "queue.max_attempts")

	// mcp enabled with zero timeout
	cfg = validDebugConfig()
	cfg.Agent.MCP.Enabled = true
	cfg.Agent.MCP.Servers = []AgentMCPServerConfig{{ID: "docs", Command: "/bin/docs"}}
	err = cfg.Validate()
	require.Error(t, err)
	contains(t, err, "agent.mcp.call_timeout_sec")

	// image configured with zero session limit (image checks ride the
	// web-agent block: the tool only mounts when the agent is enabled)
	cfg = validDebugConfig()
	cfg.Agent.WebAgentEnabled = true
	cfg.Agent.RateLimitPerDay = 50
	cfg.Agent.RateLimitPerMinute = 5
	cfg.Agent.MaxToolCallsPerTurn = 6
	cfg.Agent.MaxOutputTokens = 8192
	cfg.Agent.ProviderTimeoutSec = 60
	cfg.Agent.CitationMaxCount = 12
	cfg.Agent.MaxUserMessageChars = 2000
	cfg.Agent.ChatMaxContextMsgs = 20
	cfg.Agent.ConversationListLimit = 20
	cfg.Agent.ConversationPageSize = 10
	cfg.Agent.ChatContextTokenBudget = 24000
	cfg.RateLimit.AgentWindowSec = 86400
	cfg.RateLimit.AgentMinuteWindowSec = 60
	cfg.Agent.Image.Enabled = true
	cfg.Agent.Image.APIKey = "k"
	err = cfg.Validate()
	require.Error(t, err)
	contains(t, err, "agent.image.session_image_limit")
}

func TestValidateReleaseStillAggregatesStructuralFindings(t *testing.T) {
	// Regression guard for the shared structure helper: release mode must
	// keep surfacing structural errors through ValidateRelease (existing
	// 1267-line assertions rely on that surface).
	cfg := validDebugConfig()
	cfg.Server.Mode = "release"
	cfg.Features.RAGHybridEnabled = true
	cfg.RAG.Chunking.MaxTokens = 0
	err := cfg.ValidateRelease()
	require.Error(t, err)
	contains(t, err, validateReleaseErrPrefix)
	contains(t, err, "rag.chunking.max_tokens")
}

func TestValidateFactoryYAMLPasses(t *testing.T) {
	// The factory config.yaml (debug mode, RAG hybrid + rerank enabled) must
	// pass all-mode structural validation — the demo environment relies on it.
	cfg := loadDefaultConfigForTest(t)
	require.Equal(t, "debug", cfg.Server.Mode)
	require.NoError(t, cfg.Validate())
}

func TestValidateDisabledFeatureYAMLVariantPasses(t *testing.T) {
	// Turning the optional features off on the factory YAML keeps a legal
	// local configuration (zero-value conditional fields must be accepted).
	cfg := loadDefaultConfigForTest(t)
	cfg.Features.RAGHybridEnabled = false
	cfg.Features.RAGRerankEnabled = false
	cfg.Agent.WebAgentEnabled = false
	cfg.Features.ArchiveMalwareScanEnabled = false
	cfg.RAG.Contextual.Enabled = false
	require.NoError(t, cfg.Validate())
}

// contains asserts the validation error mentions the config path.
func contains(t *testing.T, err error, path string) {
	t.Helper()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), path),
		"error %q must mention %q", err.Error(), path)
}
