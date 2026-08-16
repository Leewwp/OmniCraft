package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaultConfigDeclaresRelayConfig pins the outbox relay tuning defaults
// (issue #200): the batch size bounds how many due outbox events one relay run
// claims and poll interval bounds delivery latency. Relay values are
// worker-side only (ADR 0005, cmd/worker); they are never part of the public
// config response.
func TestDefaultConfigDeclaresRelayConfig(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	require.Equal(t, 100, cfg.Relay.BatchSize)
	require.Equal(t, 1, cfg.Relay.PollIntervalSec)
}

// TestValidateReleaseRejectsInvalidRelayValues pins the release-mode guard on
// the outbox relay loop: zero, negative and out-of-range batch size / poll
// interval must be rejected with the field path in the message.
func TestValidateReleaseRejectsInvalidRelayValues(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cases := []struct {
		name string
		mut  func(*RelayConfig)
		want string
	}{
		{"zero batch size", func(r *RelayConfig) { r.BatchSize = 0 }, "relay.batch_size"},
		{"negative batch size", func(r *RelayConfig) { r.BatchSize = -1 }, "relay.batch_size"},
		{"batch size above max", func(r *RelayConfig) { r.BatchSize = RelayMaxBatchSize + 1 }, "relay.batch_size"},
		{"zero poll interval", func(r *RelayConfig) { r.PollIntervalSec = 0 }, "relay.poll_interval_sec"},
		{"negative poll interval", func(r *RelayConfig) { r.PollIntervalSec = -5 }, "relay.poll_interval_sec"},
		{"poll interval above max", func(r *RelayConfig) { r.PollIntervalSec = RelayMaxPollIntervalSec + 1 }, "relay.poll_interval_sec"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validReleaseConfigForTest()
			tc.mut(&cfg.Relay)

			err := cfg.ValidateRelease()
			require.Error(t, err)
			require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix),
				"error = %q, want prefix %q", err.Error(), validateReleaseErrPrefix)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestValidateReleaseAcceptsRelayBoundaryValues pins the inclusive boundary of
// the relay validation range: min and max batch size / poll interval are both
// valid.
func TestValidateReleaseAcceptsRelayBoundaryValues(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cases := []struct {
		name  string
		relay RelayConfig
	}{
		{"minimum valid", RelayConfig{BatchSize: RelayMinBatchSize, PollIntervalSec: RelayMinPollIntervalSec}},
		{"maximum valid", RelayConfig{BatchSize: RelayMaxBatchSize, PollIntervalSec: RelayMaxPollIntervalSec}},
		{"non-default valid", RelayConfig{BatchSize: 200, PollIntervalSec: 2}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validReleaseConfigForTest()
			cfg.Relay = tc.relay

			require.NoError(t, cfg.ValidateRelease())
		})
	}
}
