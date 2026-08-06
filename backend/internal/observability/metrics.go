package observability

import (
	"database/sql"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

var (
	allowedDependencies = map[string]bool{"oss": true, "green": true, "captcha": true, "smtp": true, "llm": true}
	allowedResults      = map[string]bool{"success": true, "failure": true}
	allowedStatusClass  = map[string]bool{"1xx": true, "2xx": true, "3xx": true, "4xx": true, "5xx": true}
	allowedHTTPMethods  = map[string]bool{"GET": true, "HEAD": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true, "CONNECT": true, "TRACE": true}
	// unresolvedRoute is the bounded fallback for requests that match no Gin
	// route template; raw paths must never become label values.
	unresolvedRoute  = "unmatched"
	unresolvedMethod = "other"
)

var defaultMetrics atomic.Pointer[Metrics]

// SetDefaultMetrics installs the process-wide recorder used by adapters and
// workers that are constructed below the HTTP composition root. A nil value
// disables recording, which keeps those packages safe in isolated tests and
// command-line tools.
func SetDefaultMetrics(metrics *Metrics) {
	defaultMetrics.Store(metrics)
}

// ObserveExternalCall records a bounded external dependency observation when
// the server has installed its metrics registry.
func ObserveExternalCall(dependency string, started time.Time, err error) {
	metrics := defaultMetrics.Load()
	if metrics == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "failure"
	}
	metrics.ObserveExternal(dependency, result, time.Since(started).Seconds())
}

// SetQueueBacklog records the current queue depth when a queue adapter has a
// bounded observation available.
func SetDefaultQueueBacklog(count float64) {
	if metrics := defaultMetrics.Load(); metrics != nil {
		metrics.SetQueueBacklog(count)
	}
}

// IncDefaultWorkerFailures records an exhausted worker retry through the
// installed process-wide registry.
func IncDefaultWorkerFailures() {
	if metrics := defaultMetrics.Load(); metrics != nil {
		metrics.IncWorkerFailures()
	}
}

// NormalizeRoute validates the Gin full-path template used as a logs and
// metrics label. Callers must pass Gin's FullPath rather than Request.URL.Path;
// the identifier checks are defense in depth, not a general raw-URL sanitizer.
func NormalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" || len(route) > 128 || !strings.HasPrefix(route, "/") || strings.ContainsAny(route, "?#%") {
		return unresolvedRoute
	}
	for _, segment := range strings.Split(route, "/") {
		if segment == "" || strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "*") {
			continue
		}
		if looksLikeIdentifier(segment) {
			return unresolvedRoute
		}
	}
	return route
}

func looksLikeIdentifier(segment string) bool {
	allDigits := true
	allHex := true
	for _, r := range segment {
		if r < '0' || r > '9' {
			allDigits = false
		}
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') || r == '-') {
			allHex = false
		}
	}
	return allDigits || (len(segment) >= 16 && allHex)
}

func normalizeHTTPMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if allowedHTTPMethods[method] {
		return method
	}
	return unresolvedMethod
}

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}

// Metrics owns the bounded, low-cardinality Prometheus metric set. Labels are
// restricted to allowlists; IDs, raw URLs and free-form text never appear as
// label values.
type Metrics struct {
	Registry *prometheus.Registry

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	panics       prometheus.Counter

	queueBacklog      prometheus.Gauge
	workerFailures    prometheus.Counter
	migrationStatus   prometheus.Gauge
	migrationLastSeen prometheus.Gauge

	externalRequests *prometheus.CounterVec
	externalDuration *prometheus.HistogramVec
}

// NewMetrics creates and registers the production metric set.
func NewMetrics() *Metrics {
	m := &Metrics{
		Registry: prometheus.NewRegistry(),

		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omnicraft_http_requests_total",
			Help: "HTTP requests by route template, method and status class.",
		}, []string{"route", "method", "status_class"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "omnicraft_http_request_duration_seconds",
			Help:    "HTTP request latency by route template and method.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),
		panics: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omnicraft_panics_total",
			Help: "Recovered HTTP handler panics.",
		}),

		queueBacklog: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omnicraft_queue_backlog",
			Help: "Pending queue messages not yet consumed.",
		}),
		workerFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omnicraft_queue_worker_failures_total",
			Help: "Queue worker failures that exhausted retries.",
		}),
		migrationStatus: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omnicraft_migration_status",
			Help: "Latest migration run outcome (1 success, 0 failure).",
		}),
		migrationLastSeen: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omnicraft_migration_last_success_timestamp_seconds",
			Help: "Unix time of the last successful migration run.",
		}),

		externalRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omnicraft_external_dependency_requests_total",
			Help: "External dependency calls aggregated by dependency and result only.",
		}, []string{"dependency", "result"}),
		externalDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "omnicraft_external_dependency_duration_seconds",
			Help:    "External dependency latency aggregated by dependency only.",
			Buckets: prometheus.DefBuckets,
		}, []string{"dependency"}),
	}

	m.Registry.MustRegister(
		m.httpRequests,
		m.httpDuration,
		m.panics,
		m.queueBacklog,
		m.workerFailures,
		m.migrationStatus,
		m.migrationLastSeen,
		m.externalRequests,
		m.externalDuration,
	)
	return m
}

// ObserveHTTPRequest records a request. route must be a Gin full-path
// template or the fixed "unmatched" marker; non-allowed status classes are
// ignored so the label set stays bounded. It must not receive raw URL paths.
func (m *Metrics) ObserveHTTPRequest(method, route, statusClass string, durationSec float64) {
	if !allowedStatusClass[statusClass] {
		return
	}
	route = NormalizeRoute(route)
	method = normalizeHTTPMethod(method)
	m.httpRequests.WithLabelValues(route, method, statusClass).Inc()
	m.httpDuration.WithLabelValues(route, method).Observe(durationSec)
}

// ObserveExternal records an external dependency call aggregated only by
// dependency and result; unknown dependency/result labels are ignored.
func (m *Metrics) ObserveExternal(dependency, result string, durationSec float64) {
	if !allowedDependencies[dependency] || !allowedResults[result] {
		return
	}
	m.externalRequests.WithLabelValues(dependency, result).Inc()
	m.externalDuration.WithLabelValues(dependency).Observe(durationSec)
}

// IncPanics records a recovered handler panic.
func (m *Metrics) IncPanics() { m.panics.Inc() }

// SetQueueBacklog records the current pending queue length.
func (m *Metrics) SetQueueBacklog(count float64) { m.queueBacklog.Set(count) }

// IncWorkerFailures records a worker failure that exhausted retries.
func (m *Metrics) IncWorkerFailures() { m.workerFailures.Inc() }

// SetMigrationOutcome records the latest migration run result. lastSuccess
// is the Unix timestamp of the last successful run (0 when unknown).
func (m *Metrics) SetMigrationOutcome(ok bool, lastSuccess time.Time) {
	if ok {
		m.migrationStatus.Set(1)
		if !lastSuccess.IsZero() {
			m.migrationLastSeen.Set(float64(lastSuccess.Unix()))
		}
		return
	}
	m.migrationStatus.Set(0)
}

// DatabaseCollector exposes bounded sql.DB pool gauges.
type DatabaseCollector struct {
	db *sql.DB
}

func NewDatabaseCollector(db *sql.DB) *DatabaseCollector { return &DatabaseCollector{db: db} }

var dbPoolDescriptions = map[string]*prometheus.Desc{
	"omnicraft_db_pool_open_connections":          prometheus.NewDesc("omnicraft_db_pool_open_connections", "Current open database connections.", nil, nil),
	"omnicraft_db_pool_in_use_connections":        prometheus.NewDesc("omnicraft_db_pool_in_use_connections", "Database connections in use.", nil, nil),
	"omnicraft_db_pool_idle_connections":          prometheus.NewDesc("omnicraft_db_pool_idle_connections", "Idle database connections.", nil, nil),
	"omnicraft_db_pool_wait_count_total":          prometheus.NewDesc("omnicraft_db_pool_wait_count_total", "Total waits for a database connection.", nil, nil),
	"omnicraft_db_pool_max_open_connections":      prometheus.NewDesc("omnicraft_db_pool_max_open_connections", "Maximum open database connections.", nil, nil),
	"omnicraft_db_pool_max_idle_closed_total":     prometheus.NewDesc("omnicraft_db_pool_max_idle_closed_total", "Connections closed due to max idle.", nil, nil),
	"omnicraft_db_pool_max_lifetime_closed_total": prometheus.NewDesc("omnicraft_db_pool_max_lifetime_closed_total", "Connections closed due to max lifetime.", nil, nil),
}

func (c *DatabaseCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range dbPoolDescriptions {
		ch <- desc
	}
}

func (c *DatabaseCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.db.Stats()
	emit := func(desc *prometheus.Desc, value float64) {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value)
	}
	emit(dbPoolDescriptions["omnicraft_db_pool_open_connections"], float64(stats.OpenConnections))
	emit(dbPoolDescriptions["omnicraft_db_pool_in_use_connections"], float64(stats.InUse))
	emit(dbPoolDescriptions["omnicraft_db_pool_idle_connections"], float64(stats.Idle))
	emit(dbPoolDescriptions["omnicraft_db_pool_wait_count_total"], float64(stats.WaitCount))
	emit(dbPoolDescriptions["omnicraft_db_pool_max_open_connections"], float64(stats.MaxOpenConnections))
	emit(dbPoolDescriptions["omnicraft_db_pool_max_idle_closed_total"], float64(stats.MaxIdleClosed))
	emit(dbPoolDescriptions["omnicraft_db_pool_max_lifetime_closed_total"], float64(stats.MaxLifetimeClosed))
}

type redisPoolStats struct {
	TotalConns int
	IdleConns  int
	StaleConns int
}

// RedisCollector exposes bounded Redis pool gauges from a stats provider.
type RedisCollector struct {
	stats func() redisPoolStats
}

func NewRedisCollector(stats func() redisPoolStats) *RedisCollector {
	return &RedisCollector{stats: stats}
}

// NewRedisClientCollector builds a Redis pool collector from a live client.
func NewRedisClientCollector(client *redis.Client) *RedisCollector {
	return &RedisCollector{stats: func() redisPoolStats {
		pool := client.PoolStats()
		return redisPoolStats{
			TotalConns: int(pool.TotalConns),
			IdleConns:  int(pool.IdleConns),
			StaleConns: int(pool.StaleConns),
		}
	}}
}

var redisPoolDescriptions = map[string]*prometheus.Desc{
	"omnicraft_redis_pool_total_connections": prometheus.NewDesc("omnicraft_redis_pool_total_connections", "Total Redis pool connections.", nil, nil),
	"omnicraft_redis_pool_idle_connections":  prometheus.NewDesc("omnicraft_redis_pool_idle_connections", "Idle Redis pool connections.", nil, nil),
	"omnicraft_redis_pool_stale_connections": prometheus.NewDesc("omnicraft_redis_pool_stale_connections", "Stale Redis pool connections.", nil, nil),
}

func (c *RedisCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range redisPoolDescriptions {
		ch <- desc
	}
}

func (c *RedisCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.stats()
	ch <- prometheus.MustNewConstMetric(redisPoolDescriptions["omnicraft_redis_pool_total_connections"], prometheus.GaugeValue, float64(stats.TotalConns))
	ch <- prometheus.MustNewConstMetric(redisPoolDescriptions["omnicraft_redis_pool_idle_connections"], prometheus.GaugeValue, float64(stats.IdleConns))
	ch <- prometheus.MustNewConstMetric(redisPoolDescriptions["omnicraft_redis_pool_stale_connections"], prometheus.GaugeValue, float64(stats.StaleConns))
}

// errDependencyUnavailable is the internal readiness failure marker; its
// message is never surfaced to clients.
func errDependencyUnavailable(dependency string) error {
	return errors.New(strings.ToLower(dependency) + " unavailable")
}
