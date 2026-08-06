package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/observability"
)

var hex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)

func testLoggerAndHasher(t *testing.T) (*slog.Logger, *observability.IPHasher) {
	t.Helper()
	cfg := config.ObservabilityConfig{
		LogIPHashSecret: "middleware-test-secret-value",
		LogIPKeyID:      "test-key",
	}
	hasher, err := observability.NewIPHasher(cfg)
	require.NoError(t, err)
	handler := slog.NewJSONHandler(&bytes.Buffer{}, nil)
	return slog.New(handler), hasher
}

func captureLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var line map[string]any
		require.NoError(t, json.Unmarshal([]byte(raw), &line))
		lines = append(lines, line)
	}
	return lines
}

func TestLoggerHashesClientIPAndOmitsQueryString(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	hasher, err := observability.NewIPHasher(config.ObservabilityConfig{
		LogIPHashSecret: "middleware-test-secret-value",
		LogIPKeyID:      "test-key",
	})
	require.NoError(t, err)
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	r := gin.New()
	r.Use(RequestID())
	r.Use(Logger(logger, hasher))
	r.GET("/api/v1/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health?token=super-secret-token&sig=abc", nil)
	req.RemoteAddr = "203.0.113.9:12345"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	lines := captureLogLines(t, &buf)
	require.Len(t, lines, 1)
	line := lines[0]

	require.Equal(t, "request", line["msg"])
	require.Equal(t, "/api/v1/health", line["route"])
	require.Equal(t, "/api/v1/health", line["path"])
	require.Equal(t, "GET", line["method"])
	require.Equal(t, float64(200), line["status"])
	require.Equal(t, "none", line["error_class"])

	clientIP, ok := line["client_ip"].(string)
	require.True(t, ok)
	require.True(t, hex32.MatchString(clientIP), "client_ip = %q, want 32 lowercase hex chars", clientIP)
	require.NotEqual(t, "203.0.113.9", clientIP)

	require.Equal(t, "test-key", line["client_ip_key_id"])

	raw, err := json.Marshal(line)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "super-secret-token")
	require.NotContains(t, string(raw), "sig=abc")
	require.NotContains(t, string(raw), "203.0.113.9")
}

func TestLoggerUsesRouteTemplateNotRawPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	logger, hasher := testLoggerAndHasher(t)
	logger = slog.New(slog.NewJSONHandler(&buf, nil))

	r := gin.New()
	r.Use(Logger(logger, hasher))
	r.GET("/api/v1/content/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/content/42", nil))

	lines := captureLogLines(t, &buf)
	require.Len(t, lines, 1)
	require.Equal(t, "/api/v1/content/:id", lines[0]["route"])
}

func TestLoggerBoundedFallbackForUnmatchedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	logger, hasher := testLoggerAndHasher(t)
	logger = slog.New(slog.NewJSONHandler(&buf, nil))

	r := gin.New()
	r.Use(Logger(logger, hasher))

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/this/path/does/not/exist", nil))

	lines := captureLogLines(t, &buf)
	require.Len(t, lines, 1)
	require.Equal(t, "unmatched", lines[0]["route"])
}

func TestLoggerErrorClassForFailedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	logger, hasher := testLoggerAndHasher(t)
	logger = slog.New(slog.NewJSONHandler(&buf, nil))

	r := gin.New()
	r.Use(Logger(logger, hasher))
	r.GET("/missing", func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{}) })
	r.GET("/boom", func(c *gin.Context) { c.JSON(http.StatusInternalServerError, gin.H{}) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	lines := captureLogLines(t, &buf)
	require.Len(t, lines, 2)
	require.Equal(t, "client", lines[0]["error_class"])
	require.Equal(t, "internal", lines[1]["error_class"])
}

func metricValue(t *testing.T, m *observability.Metrics, name string) float64 {
	t.Helper()
	metrics, err := m.Registry.Gather()
	require.NoError(t, err)
	var total float64
	for _, mf := range metrics {
		if mf.GetName() == name {
			for _, mm := range mf.GetMetric() {
				total += mm.GetCounter().GetValue()
			}
		}
	}
	return total
}

func TestMetricsMiddlewareRecordsBoundedLabels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := observability.NewMetrics()
	r := gin.New()
	r.Use(Metrics(m))
	r.GET("/api/v1/content/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/boom", func(c *gin.Context) { c.JSON(http.StatusInternalServerError, gin.H{}) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/content/99", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/content/99", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, float64(3), metricValue(t, m, "omnicraft_http_requests_total"))

	metrics, err := m.Registry.Gather()
	require.NoError(t, err)
	for _, mf := range metrics {
		if mf.GetName() != "omnicraft_http_requests_total" {
			continue
		}
		for _, mm := range mf.GetMetric() {
			for _, l := range mm.GetLabel() {
				if l.GetName() == "route" {
					require.NotContains(t, l.GetValue(), "99")
					require.NotContains(t, l.GetValue(), "?")
				}
			}
		}
	}
}

func TestPanicRecoveryIncrementsPanicCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	logger, hasher := testLoggerAndHasher(t)
	logger = slog.New(slog.NewJSONHandler(&buf, nil))
	m := observability.NewMetrics()

	r := gin.New()
	r.Use(RequestID())
	r.Use(Logger(logger, hasher))
	r.Use(PanicRecovery(logger, m))
	r.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	require.Equal(t, float64(1), metricValue(t, m, "omnicraft_panics_total"))

	lines := captureLogLines(t, &buf)
	require.Len(t, lines, 2)
	panicLine := lines[0]
	require.Equal(t, "panic", panicLine["error_class"])
	raw, err := json.Marshal(panicLine)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "boom")
	require.Contains(t, string(raw), "error_class")
}
