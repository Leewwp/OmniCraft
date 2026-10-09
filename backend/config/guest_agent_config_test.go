package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// T2 #854 guest agent configuration surface: the factory config keeps the
// anonymous surface off, the structural gate refuses a half-enabled guest
// configuration (positive limits, real signing secret, effective per-IP cost
// bucket) so the total gate can never degrade into a no-op.

// loadRawConfigForTest parses the given YAML the same way Load() does,
// without touching the environment.
func loadRawConfigForTest(t *testing.T, raw string) *Config {
	t.Helper()
	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(raw)))
	var cfg Config
	require.NoError(t, v.Unmarshal(&cfg))
	return &cfg
}

func TestFactoryConfigKeepsGuestAgentDisabled(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)
	require.False(t, cfg.Features.GuestAgentEnabled,
		"features.guest_agent_enabled must ship disabled (fail-closed default)")
	// The turn budget parameters ship with the documented factory values so a
	// host flipping the flag inherits the reviewed limits, not zero values.
	require.Equal(t, 3, cfg.Agent.Guest.MaxTotalTurns)
	require.Equal(t, 7, cfg.Agent.Guest.ConversationTTLDays)
	require.Equal(t, 3, cfg.Agent.Guest.MaxConcurrentTurns)
	require.Equal(t, 8760, cfg.Agent.Guest.CookieMaxAgeHours)
	require.Empty(t, cfg.Agent.Guest.CookieSecret,
		"factory config must never carry a guest cookie signing secret")
}

func TestFactoryGuestAgentBucketTierIsRegistered(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)
	spec, ok := cfg.RateLimit.GuestBucketFor("agent_guest")
	require.True(t, ok, "agent_guest cost bucket tier must be a registered default")
	require.Positive(t, spec.Capacity)
	require.Positive(t, spec.RefillPerMinute)
}

func TestGuestAgentEnvOverrideAppliesWithoutYAMLKey(t *testing.T) {
	// The signing secret must be injectable per-instance via the environment;
	// the committed config.yaml never carries a real value.
	tmp := t.TempDir()
	// web_agent_enabled carries its full structural prerequisite block (the
	// all-mode gate demands positive agent limits once it is on).
	base := strings.Replace(minimalValidCoreYAML, "server:",
		"rate_limit:\n  agent_window_sec: 86400\n  agent_minute_window_sec: 60\nfeatures:\n  guest_agent_enabled: true\nagent:\n  web_agent_enabled: true\n  rate_limit_per_day: 50\n  rate_limit_per_minute: 5\n  max_tool_calls_per_turn: 8\n  max_output_tokens: 4096\n  provider_timeout_sec: 60\n  citation_max_count: 8\n  max_user_message_chars: 4000\n  chat_max_context_messages: 20\n  conversation_list_limit: 50\n  conversation_page_size: 40\n  chat_context_token_budget: 6000\n  guest:\n    max_total_turns: 3\n    conversation_ttl_days: 7\n    max_concurrent_turns: 3\n    cookie_max_age_hours: 8760\nserver:", 1)
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte(base), 0o600))
	t.Setenv("AGENT_GUEST_COOKIE_SECRET", "0123456789abcdef0123456789abcdef-unit-test-secret")
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousWD)) })

	cfg := Load()
	require.True(t, cfg.Features.GuestAgentEnabled)
	require.Equal(t, "0123456789abcdef0123456789abcdef-unit-test-secret", cfg.Agent.Guest.CookieSecret)
	require.Equal(t, 3, cfg.Agent.Guest.MaxTotalTurns)
}

func guestEnabledConfig() *Config {
	cfg := &Config{}
	cfg.Features.GuestAgentEnabled = true
	cfg.Agent.WebAgentEnabled = true
	// Structural prerequisites of agent.web_agent_enabled itself.
	cfg.Agent.RateLimitPerDay = 50
	cfg.Agent.RateLimitPerMinute = 5
	cfg.Agent.MaxToolCallsPerTurn = 8
	cfg.Agent.MaxOutputTokens = 4096
	cfg.Agent.ProviderTimeoutSec = 60
	cfg.Agent.CitationMaxCount = 8
	cfg.Agent.MaxUserMessageChars = 4000
	cfg.Agent.ChatMaxContextMsgs = 20
	cfg.Agent.ConversationListLimit = 50
	cfg.Agent.ConversationPageSize = 40
	cfg.Agent.ChatContextTokenBudget = 6000
	cfg.RateLimit.AgentWindowSec = 86400
	cfg.RateLimit.AgentMinuteWindowSec = 60
	cfg.Agent.Guest = AgentGuestConfig{
		MaxTotalTurns:       3,
		ConversationTTLDays: 7,
		MaxConcurrentTurns:  3,
		CookieSecret:        "unit-test-guest-cookie-secret-0123456789abcdef",
		CookieMaxAgeHours:   8760,
	}
	return cfg
}

func TestValidateAcceptsCompleteGuestAgentConfig(t *testing.T) {
	cfg := guestEnabledConfig()
	errs := []string{}
	cfg.validateStructure(&errs)
	require.Empty(t, errs)
}

func TestValidateRejectsGuestAgentWithoutWebAgent(t *testing.T) {
	cfg := guestEnabledConfig()
	cfg.Agent.WebAgentEnabled = false
	errs := []string{}
	cfg.validateStructure(&errs)
	require.NotEmpty(t, errs)
	require.Contains(t, strings.Join(errs, "; "), "agent.web_agent_enabled")
}

func TestValidateRejectsNonPositiveGuestAgentLimits(t *testing.T) {
	for name, mutate := range map[string]func(*Config){
		"max_total_turns":       func(c *Config) { c.Agent.Guest.MaxTotalTurns = 0 },
		"conversation_ttl_days": func(c *Config) { c.Agent.Guest.ConversationTTLDays = -1 },
		"max_concurrent_turns":  func(c *Config) { c.Agent.Guest.MaxConcurrentTurns = 0 },
		"cookie_max_age_hours":  func(c *Config) { c.Agent.Guest.CookieMaxAgeHours = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := guestEnabledConfig()
			mutate(cfg)
			errs := []string{}
			cfg.validateStructure(&errs)
			require.NotEmpty(t, errs)
			require.Contains(t, strings.Join(errs, "; "), "agent.guest."+name)
		})
	}
}

func TestValidateRejectsWeakOrMissingGuestCookieSecret(t *testing.T) {
	for name, secret := range map[string]string{
		"empty":       "",
		"too-short":   "short-secret",
		"placeholder": "your-guest-cookie-secret-change-before-use-0001",
	} {
		t.Run(name, func(t *testing.T) {
			cfg := guestEnabledConfig()
			cfg.Agent.Guest.CookieSecret = secret
			errs := []string{}
			cfg.validateStructure(&errs)
			require.NotEmpty(t, errs)
			require.Contains(t, strings.Join(errs, "; "), "agent.guest.cookie_secret")
		})
	}
}

func TestValidateRejectsZeroedAgentGuestCostBucketOverride(t *testing.T) {
	// A zeroed explicit override must fail startup, not silently fall back to
	// the code default: the cost bucket is the guest surface's spend control
	// and turning the generic browsing limiter off must not no-op it.
	cfg := guestEnabledConfig()
	cfg.RateLimit.GuestBuckets = map[string]GuestBucketConfig{
		"agent_guest": {Capacity: 0, RefillPerMinute: 0},
	}
	errs := []string{}
	cfg.validateStructure(&errs)
	require.NotEmpty(t, errs)
	require.Contains(t, strings.Join(errs, "; "), "agent_guest")
}

func TestValidateAllowsValidAgentGuestBucketOverride(t *testing.T) {
	cfg := guestEnabledConfig()
	cfg.RateLimit.GuestBuckets = map[string]GuestBucketConfig{
		"agent_guest": {Capacity: 10, RefillPerMinute: 2},
	}
	errs := []string{}
	cfg.validateStructure(&errs)
	require.Empty(t, errs)
	spec, ok := cfg.RateLimit.GuestBucketFor("agent_guest")
	require.True(t, ok)
	require.Equal(t, 10, spec.Capacity)
	require.Equal(t, float64(2), spec.RefillPerMinute)
}

func TestGuestAgentDisabledSkipsGuestStructuralGate(t *testing.T) {
	// With the gate off, an empty guest block must not fail validation: the
	// factory config ships that way.
	cfg := loadRawConfigForTest(t, minimalValidCoreYAML)
	cfg.Features.GuestAgentEnabled = false
	errs := []string{}
	cfg.validateStructure(&errs)
	require.Empty(t, errs)
}
