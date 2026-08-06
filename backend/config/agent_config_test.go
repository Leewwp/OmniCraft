package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentConfigLimitsFromYAML(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	// Existing limits remain mapped.
	require.Positive(t, cfg.Agent.RateLimitPerDay)
	require.Positive(t, cfg.Agent.MaxUserMessageChars)
	require.Positive(t, cfg.Agent.ChatMaxContextMsgs)

	// New typed budget/limits block.
	require.Positive(t, cfg.Agent.RateLimitPerMinute)
	require.Positive(t, cfg.Agent.MaxToolCallsPerTurn)
	require.Positive(t, cfg.Agent.MaxOutputTokens)
	require.Positive(t, cfg.Agent.ProviderTimeoutSec)
	require.Positive(t, cfg.Agent.CitationMaxCount)
	require.GreaterOrEqual(t, cfg.Agent.ProviderMaxRetries, 0)

	// The per-minute burst limit must never exceed the daily budget.
	require.GreaterOrEqual(t, cfg.Agent.RateLimitPerDay, cfg.Agent.RateLimitPerMinute)
}

func TestAgentConfigReleaseAcceptsPositiveLimits(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", strings.Repeat("a", 32))
	cfg := validReleaseConfigForTest()
	cfg.Agent.WebAgentEnabled = true
	cfg.Agent.LLMAPIKey = "release-test-key"
	cfg.Agent.RateLimitPerDay = 50
	cfg.Agent.RateLimitPerMinute = 5
	cfg.Agent.MaxToolCallsPerTurn = 8
	cfg.Agent.MaxOutputTokens = 1200
	cfg.Agent.ProviderTimeoutSec = 60
	cfg.Agent.ProviderMaxRetries = 2
	cfg.Agent.CitationMaxCount = 5
	require.NoError(t, cfg.ValidateRelease())
}

func TestAgentConfigReleaseRequiresPositiveLimits(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", strings.Repeat("a", 32))

	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"zero minute limit", func(c *Config) { c.Agent.RateLimitPerMinute = 0 }, "agent.rate_limit_per_minute"},
		{"zero tool calls", func(c *Config) { c.Agent.MaxToolCallsPerTurn = 0 }, "agent.max_tool_calls_per_turn"},
		{"zero output tokens", func(c *Config) { c.Agent.MaxOutputTokens = 0 }, "agent.max_output_tokens"},
		{"zero provider timeout", func(c *Config) { c.Agent.ProviderTimeoutSec = 0 }, "agent.provider_timeout_sec"},
		{"negative provider retries", func(c *Config) { c.Agent.ProviderMaxRetries = -1 }, "agent.provider_max_retries"},
		{"zero citation max", func(c *Config) { c.Agent.CitationMaxCount = 0 }, "agent.citation_max_count"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validReleaseConfigForTest()
			cfg.Agent.WebAgentEnabled = true
			cfg.Agent.LLMAPIKey = "release-test-key"
			cfg.Agent.RateLimitPerDay = 50
			cfg.Agent.RateLimitPerMinute = 5
			cfg.Agent.MaxToolCallsPerTurn = 8
			cfg.Agent.MaxOutputTokens = 1200
			cfg.Agent.ProviderTimeoutSec = 60
			cfg.Agent.ProviderMaxRetries = 2
			cfg.Agent.CitationMaxCount = 5
			tc.mutate(cfg)

			err := cfg.ValidateRelease()
			require.Error(t, err)
			require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix),
				"error = %q, want prefix %q", err.Error(), validateReleaseErrPrefix)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
