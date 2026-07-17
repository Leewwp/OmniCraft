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

func TestResolveJSONBodyLimitDefaultsToTextUploadLimit(t *testing.T) {
	cfg := &config.Config{}
	cfg.Limits.TextMaxMB = 10

	got := resolveJSONBodyLimit(cfg)
	want := int64(10 * 1024 * 1024)
	if got != want {
		t.Fatalf("resolveJSONBodyLimit() = %d, want %d", got, want)
	}
}
