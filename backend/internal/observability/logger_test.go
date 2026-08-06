package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
)

func readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	return string(content), err
}

func newJSONHandler(buf *bytes.Buffer, level string) slog.Handler {
	return slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})
}

var hex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)

func testHasherConfig() config.ObservabilityConfig {
	return config.ObservabilityConfig{
		LogIPHashSecret: "test-current-secret-value",
		LogIPKeyID:      "current-key",
		IPKeyRotation: config.IPKeyRotationConfig{
			PreviousSecret: "test-previous-secret-value",
			PreviousKeyID:  "previous-key",
			ActiveFrom:     "2026-01-01T00:00:00Z",
			ActiveUntil:    "2026-12-31T23:59:59Z",
		},
	}
}

func TestIPHasherHashIsLowercaseHex128(t *testing.T) {
	h, err := NewIPHasher(testHasherConfig())
	require.NoError(t, err)

	got := h.Hash("203.0.113.7")
	require.True(t, hex32.MatchString(got), "hash = %q, want 32 lowercase hex chars", got)
	require.Equal(t, got, strings.ToLower(got))
	require.NotEqual(t, "203.0.113.7", got)
}

func TestIPHasherHashIsDeterministicAndKeyed(t *testing.T) {
	h1, err := NewIPHasher(testHasherConfig())
	require.NoError(t, err)
	require.Equal(t, h1.Hash("203.0.113.7"), h1.Hash("203.0.113.7"))

	other, err := NewIPHasher(config.ObservabilityConfig{
		LogIPHashSecret: "a-completely-different-secret",
		LogIPKeyID:      "other",
	})
	require.NoError(t, err)
	require.NotEqual(t, h1.Hash("203.0.113.7"), other.Hash("203.0.113.7"))
}

func TestIPHasherNeverFallsBackToRawIP(t *testing.T) {
	h, err := NewIPHasher(config.ObservabilityConfig{LogIPHashSecret: "", LogIPKeyID: ""})
	require.NoError(t, err)

	for _, ip := range []string{"203.0.113.7", "2001:db8::1", "unknown"} {
		got := h.Hash(ip)
		require.NotEqual(t, ip, got, "hasher must never fall back to the raw IP")
		require.NotContains(t, got, ip)
	}
}

func TestIPHasherKeyIDReflectsCurrentKey(t *testing.T) {
	h, err := NewIPHasher(testHasherConfig())
	require.NoError(t, err)
	require.Equal(t, "current-key", h.KeyID())
}

func TestIPHasherPreviousKeyOnlyInsideRotationWindow(t *testing.T) {
	h, err := NewIPHasher(testHasherConfig())
	require.NoError(t, err)

	before, err := h.HashPrevious("203.0.113.7", time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC))
	require.Error(t, err, "previous key must be rejected before the rotation window")
	require.Empty(t, before)

	within, err := h.HashPrevious("203.0.113.7", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.True(t, hex32.MatchString(within))

	after, err := h.HashPrevious("203.0.113.7", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	require.Error(t, err, "previous key must be rejected after the rotation window")
	require.Empty(t, after)
}

func TestIPHasherHashPreviousUnavailableWithoutWindow(t *testing.T) {
	h, err := NewIPHasher(config.ObservabilityConfig{LogIPHashSecret: "secret", LogIPKeyID: "k"})
	require.NoError(t, err)
	got, err := h.HashPrevious("203.0.113.7", time.Now().UTC())
	require.Error(t, err)
	require.Empty(t, got)
}

func TestNewLoggerStableFields(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{Mode: "release"},
		Observability: config.ObservabilityConfig{
			LogLevel:        "info",
			LogIPHashSecret: "secret-value",
			LogIPKeyID:      "k",
		},
	}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Emit a line carrying every middleware-provided field and prove the
	// stable identity attributes are attached and field names are stable
	// (never timestamp/latency aliases).
	var buf bytes.Buffer
	handler := newJSONHandler(&buf, "info")
	l := newLoggerWithAttrs(handler, cfg)
	l.Info("request",
		"trace_id", "abc123",
		"request_id", "abc123",
		"route", "/healthz",
		"method", "GET",
		"status", 200,
		"duration_ms", 12,
		"client_ip", "abc",
		"client_ip_key_id", "k",
		"error_class", "none",
	)

	var line map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line))
	for _, field := range []string{"time", "level", "msg", "service", "environment", "version", "trace_id", "request_id", "route", "method", "status", "duration_ms", "client_ip", "error_class"} {
		_, ok := line[field]
		require.True(t, ok, "log line must declare stable field %q", field)
	}
	require.Equal(t, "omnicraft-backend", line["service"])
	require.Equal(t, "release", line["environment"])
	require.NotEmpty(t, line["version"])
	require.Equal(t, "abc123", line["trace_id"])
	require.Equal(t, "/healthz", line["route"])
	require.NotContains(t, line, "timestamp")
	require.NotContains(t, line, "latency")
	require.NotContains(t, buf.String(), "secret-value")
}

func TestErrorClassMapping(t *testing.T) {
	require.Equal(t, "client", ErrorClass(404))
	require.Equal(t, "client", ErrorClass(429))
	require.Equal(t, "internal", ErrorClass(500))
	require.Equal(t, "internal", ErrorClass(503))
	require.Equal(t, "none", ErrorClass(200))
	require.Equal(t, "none", ErrorClass(302))
}

func TestWriteTextfileWritesAtomicMetrics(t *testing.T) {
	dir := t.TempDir()
	err := WriteTextfile(dir, "omnicraft_migration.prom", []string{
		"# HELP omnicraft_migration_status Latest migration run outcome (1 success, 0 failure).",
		"# TYPE omnicraft_migration_status gauge",
		`omnicraft_migration_status 1`,
		`omnicraft_migration_last_success_timestamp_seconds 1750000000.123`,
	})
	require.NoError(t, err)

	content, err := readFile(dir + "/omnicraft_migration.prom")
	require.NoError(t, err)
	require.Contains(t, content, "# TYPE omnicraft_migration_status gauge")
	require.Contains(t, content, "omnicraft_migration_status 1")
	require.Contains(t, content, "omnicraft_migration_last_success_timestamp_seconds 1750000000.123")
	require.NotContains(t, content, ".tmp")
}

func TestWriteTextfileFailsWhenDirIsAFile(t *testing.T) {
	dir := t.TempDir()
	blocker := dir + "/blocker"
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	err := WriteTextfile(blocker, "x.prom", []string{"x 1"})
	require.Error(t, err)
}
