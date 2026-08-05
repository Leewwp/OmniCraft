package observability

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// stubDriver lets tests build a *sql.DB without any real backend so pool
// configuration can be asserted through sql.DBStats.
type stubDriver struct{ driver.Driver }

func (stubDriver) Open(name string) (driver.Conn, error) { return nil, errors.New("stub") }

func gatherMetricNames(t *testing.T, reg *prometheus.Registry) []string {
	t.Helper()
	metrics, err := reg.Gather()
	require.NoError(t, err)
	names := make([]string, 0, len(metrics))
	for _, m := range metrics {
		names = append(names, m.GetName())
	}
	return names
}

func metricValue(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	metrics, err := reg.Gather()
	require.NoError(t, err)
	for _, m := range metrics {
		if m.GetName() == name {
			for _, mf := range m.GetMetric() {
				return mf.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func TestNewMetricsRegistersBaseMetricSet(t *testing.T) {
	m := NewMetrics()
	// Labeled vectors emit nothing until the first observation; record one so
	// every metric family is present in the gather output.
	m.ObserveHTTPRequest("GET", "/healthz", "2xx", 0.001)
	m.ObserveExternal("oss", "success", 0.001)
	names := gatherMetricNames(t, m.Registry)

	for _, want := range []string{
		"omnicraft_http_requests_total",
		"omnicraft_http_request_duration_seconds",
		"omnicraft_panics_total",
		"omnicraft_queue_backlog",
		"omnicraft_queue_worker_failures_total",
		"omnicraft_migration_status",
		"omnicraft_migration_last_success_timestamp_seconds",
		"omnicraft_external_dependency_requests_total",
		"omnicraft_external_dependency_duration_seconds",
	} {
		require.Contains(t, names, want, "metric %q must be registered", want)
	}
}

func TestExternalDependencyLabelAllowlist(t *testing.T) {
	m := NewMetrics()

	m.ObserveExternal("oss", "success", 1.5)
	m.ObserveExternal("green", "failure", 2.5)
	m.ObserveExternal("captcha", "success", 0.5)
	m.ObserveExternal("smtp", "failure", 3.5)
	m.ObserveExternal("llm", "success", 4.5)

	// Unknown dependency or result labels must be silently ignored so the
	// cardinality of the metric stays bounded.
	m.ObserveExternal("oss", "unknown", 1)
	m.ObserveExternal("unknown-dep", "success", 1)
	m.ObserveExternal("", "", 1)

	metrics, err := m.Registry.Gather()
	require.NoError(t, err)
	for _, mf := range metrics {
		if mf.GetName() != "omnicraft_external_dependency_requests_total" {
			continue
		}
		for _, mm := range mf.GetMetric() {
			labels := mm.GetLabel()
			var dependency, result string
			for _, l := range labels {
				switch l.GetName() {
				case "dependency":
					dependency = l.GetValue()
				case "result":
					result = l.GetValue()
				}
			}
			require.Contains(t, []string{"oss", "green", "captcha", "smtp", "llm"}, dependency)
			require.Contains(t, []string{"success", "failure"}, result)
		}
	}
}

func TestHTTPStatusClassLabelAllowlist(t *testing.T) {
	m := NewMetrics()

	m.ObserveHTTPRequest("GET", "/healthz", "2xx", 1.0)
	m.ObserveHTTPRequest("GET", "/missing", "4xx", 1.0)
	m.ObserveHTTPRequest("GET", "/boom", "5xx", 1.0)

	// Out-of-range status classes must not create new label values.
	m.ObserveHTTPRequest("GET", "/invalid", "9xx", 1.0)
	m.ObserveHTTPRequest("GET", "/invalid", "abc", 1.0)
	m.ObserveHTTPRequest("", "", "", 1.0)

	metrics, err := m.Registry.Gather()
	require.NoError(t, err)
	for _, mf := range metrics {
		if mf.GetName() != "omnicraft_http_requests_total" {
			continue
		}
		for _, mm := range mf.GetMetric() {
			for _, l := range mm.GetLabel() {
				if l.GetName() == "status_class" {
					require.Contains(t, []string{"1xx", "2xx", "3xx", "4xx", "5xx"}, l.GetValue())
				}
			}
		}
	}
}

func TestHTTPMethodLabelAllowlistCollapsesUnknownMethods(t *testing.T) {
	m := NewMetrics()
	m.ObserveHTTPRequest("GET", "/healthz", "2xx", 0.1)
	m.ObserveHTTPRequest("X-User-Supplied-Method", "/healthz", "2xx", 0.1)

	metrics, err := m.Registry.Gather()
	require.NoError(t, err)
	for _, mf := range metrics {
		if mf.GetName() != "omnicraft_http_requests_total" {
			continue
		}
		for _, mm := range mf.GetMetric() {
			for _, label := range mm.GetLabel() {
				if label.GetName() == "method" {
					require.Contains(t, []string{"GET", "other"}, label.GetValue())
					require.NotContains(t, label.GetValue(), "User-Supplied")
				}
			}
		}
	}
}

func TestDatabaseCollectorExposesPoolStats(t *testing.T) {
	sql.Register("observability-stub", stubDriver{})
	raw, err := sql.Open("observability-stub", "")
	require.NoError(t, err)
	defer raw.Close()
	raw.SetMaxOpenConns(7)
	raw.SetMaxIdleConns(3)

	reg := prometheus.NewRegistry()
	reg.MustRegister(NewDatabaseCollector(raw))

	metrics, err := reg.Gather()
	require.NoError(t, err)
	found := false
	for _, mf := range metrics {
		if mf.GetName() == "omnicraft_db_pool_max_open_connections" {
			for _, mm := range mf.GetMetric() {
				require.Equal(t, float64(7), mm.GetGauge().GetValue())
				found = true
			}
		}
	}
	require.True(t, found, "db pool max open connections gauge missing")
}

func TestRedisCollectorExposesPoolStats(t *testing.T) {
	stats := redisPoolStats{TotalConns: 4, IdleConns: 2, StaleConns: 1}

	reg := prometheus.NewRegistry()
	reg.MustRegister(NewRedisCollector(func() redisPoolStats { return stats }))

	metrics, err := reg.Gather()
	require.NoError(t, err)
	got := map[string]float64{}
	for _, mf := range metrics {
		for _, mm := range mf.GetMetric() {
			got[mf.GetName()] = mm.GetGauge().GetValue()
		}
	}
	require.Equal(t, float64(4), got["omnicraft_redis_pool_total_connections"])
	require.Equal(t, float64(2), got["omnicraft_redis_pool_idle_connections"])
	require.Equal(t, float64(1), got["omnicraft_redis_pool_stale_connections"])
}

func TestServerHealthzIsLivenessOnly(t *testing.T) {
	s := NewServer(NewMetrics().Registry, func() error { return nil }, time.Second)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "ok")
}

func TestServerReadyzDependsOnReadyFunc(t *testing.T) {
	s := NewServer(NewMetrics().Registry, func() error { return nil }, time.Second)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	failing := NewServer(NewMetrics().Registry, func() error {
		return errDependencyUnavailable("postgres")
	}, time.Second)
	rec = httptest.NewRecorder()
	failing.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	// The body must never leak connection details or dependency errors.
	require.NotContains(t, strings.ToLower(rec.Body.String()), "postgres")
	require.NotContains(t, strings.ToLower(rec.Body.String()), "connection refused")
	require.NotContains(t, rec.Body.String(), "dsn")
}

func TestServerMetricsEndpointServesPrometheus(t *testing.T) {
	m := NewMetrics()
	m.ObserveHTTPRequest("GET", "/healthz", "2xx", 1.0)

	s := NewServer(m.Registry, func() error { return nil }, time.Second)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "omnicraft_http_requests_total")
}
