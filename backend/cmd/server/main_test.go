package main

import (
	"os"
	"strings"
	"testing"

	"omnicraft/backend/config"
)

func TestMainCallsValidateReleaseAfterLoadAndBeforeExternalInit(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(src)
	loadIdx := strings.Index(text, "cfg := config.Load()")
	validateIdx := strings.Index(text, "cfg.ValidateRelease()")
	dbIdx := strings.Index(text, "database.Init")
	if loadIdx < 0 {
		t.Fatal("main must load config")
	}
	if validateIdx < 0 {
		t.Fatal("main must call cfg.ValidateRelease()")
	}
	if validateIdx < loadIdx {
		t.Fatal("ValidateRelease must run after config load")
	}
	if dbIdx < 0 {
		t.Fatal("main must initialize database")
	}
	if validateIdx > dbIdx {
		t.Fatal("ValidateRelease must run before database initialization")
	}
	redisIdx := strings.Index(text, "redisclient.Init")
	if redisIdx < 0 {
		t.Fatal("main must initialize redis")
	}
	if validateIdx > redisIdx {
		t.Fatal("ValidateRelease must run before redis initialization")
	}
}

func TestMainRegistersRoutesThroughRouterPackage(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, `"omnicraft/backend/internal/router"`) {
		t.Fatal("main must import the dedicated router package")
	}
	if strings.Contains(text, `"omnicraft/backend/internal/handler"`) {
		t.Fatal("main must not import handler as the route composition owner")
	}
	proxyIdx := strings.Index(text, "SetTrustedProxies")
	corsIdx := strings.Index(text, "r.Use(middleware.CORS(cfg))")
	routesIdx := strings.Index(text, "router.RegisterRoutes")
	if proxyIdx < 0 {
		t.Fatal("main must configure Gin trusted proxies")
	}
	if corsIdx < 0 {
		t.Fatal("main must retain CORS middleware")
	}
	if routesIdx < 0 {
		t.Fatal("main must register routes through router.RegisterRoutes")
	}
	if !(proxyIdx < corsIdx && corsIdx < routesIdx) {
		t.Fatal("trusted proxies must precede CORS, and CORS must precede route registration")
	}
}

func TestMainSetsUpObservability(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(src)

	if !strings.Contains(text, `observability.NewLogger`) {
		t.Fatal("main must build the JSON observability logger")
	}
	if !strings.Contains(text, `slog.SetDefault(logger)`) {
		t.Fatal("main must install the observability logger as the slog default")
	}
	if !strings.Contains(text, `observability.NewIPHasher`) {
		t.Fatal("main must build the client IP hasher")
	}
	if !strings.Contains(text, `observability.NewMetrics()`) {
		t.Fatal("main must create the metrics registry")
	}
	if !strings.Contains(text, `observability.NewDatabaseCollector`) {
		t.Fatal("main must register the database pool collector")
	}
	if !strings.Contains(text, `observability.NewRedisClientCollector`) {
		t.Fatal("main must register the Redis pool collector")
	}
	if !strings.Contains(text, `observability.NewServer`) {
		t.Fatal("main must start the internal observability server")
	}
	if !strings.Contains(text, `middleware.Metrics(metrics)`) {
		t.Fatal("main must install the metrics middleware")
	}
	if !strings.Contains(text, `middleware.PanicRecovery(logger, metrics)`) {
		t.Fatal("main must install the panic recovery middleware with the metrics registry")
	}
	if !strings.Contains(text, `"/healthz"`) {
		t.Fatal("main must keep the liveness /healthz endpoint")
	}
}

func TestMainReadinessProbeIsOpaqueAndBounded(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, `errors.New("dependency unavailable")`) {
		t.Fatal("readiness must return an opaque error without dependency details")
	}
	if !strings.Contains(text, "PingContext") {
		t.Fatal("readiness must ping the database")
	}
	if !strings.Contains(text, "rdb.Ping") {
		t.Fatal("readiness must ping redis")
	}
}

func TestResolveJSONBodyLimitDefaultsToTextUploadLimit(t *testing.T) {
	cfg := &config.Config{}
	cfg.Limits.TextMaxMB = 10

	got := resolveJSONBodyLimit(cfg)
	want := int64(10 * 1024 * 1024)
	if got != want {
		t.Fatalf("resolveJSONBodyLimit() = %d, want %d", got, want)
	}
}
