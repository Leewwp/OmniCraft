package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentRoutesUseAgentRateLimit(t *testing.T) {
	source := readRoutesSource(t)

	want := `agent := v1.Group("/agent", authReq, agentGuard, middleware.AgentRateLimit(rdb, cfg))`
	if !strings.Contains(source, want) {
		t.Fatalf("agent routes must include group-level AgentRateLimit and agentGuard")
	}
}

func TestAuthCredentialRoutesUseCredentialRateLimit(t *testing.T) {
	source := readRoutesSource(t)

	if strings.Contains(source, `auth.POST("/register", authHandler.Register)`) {
		t.Fatalf("register route is missing CredentialRateLimit")
	}
	if strings.Contains(source, `auth.POST("/login", authHandler.Login)`) {
		t.Fatalf("login route is missing CredentialRateLimit")
	}
	if !strings.Contains(source, `middleware.CredentialRateLimit(rdb, &cfg.RateLimit)`) {
		t.Fatalf("auth credential routes must include middleware.CredentialRateLimit")
	}
}

func readRoutesSource(t *testing.T) string {
	t.Helper()
	bytes, err := os.ReadFile(filepath.Join(".", "routes.go"))
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	return string(bytes)
}
