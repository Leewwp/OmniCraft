package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncRouteListReplacesLegacyOwnerMarkerWithoutDuplication(t *testing.T) {
	root := t.TempDir()
	routerDir := filepath.Join(root, "backend", "internal", "router")
	if err := os.MkdirAll(routerDir, 0o755); err != nil {
		t.Fatalf("create router fixture: %v", err)
	}
	routeSource := `package router

import "github.com/gin-gonic/gin"

func RegisterRoutes(v1 *gin.RouterGroup) {
	v1.GET("/health", healthHandler)
}
`
	if err := os.WriteFile(filepath.Join(routerDir, "routes.go"), []byte(routeSource), 0o644); err != nil {
		t.Fatalf("write route fixture: %v", err)
	}

	legacyMarker := "<!-- AUTO-GENERATED: §3.2 API 路由清单 | source: backend/internal/handler/routes.go | DO NOT EDIT MANUALLY -->"
	newMarker := "<!-- AUTO-GENERATED: §3.2 API 路由清单 | source: backend/internal/router/routes.go | DO NOT EDIT MANUALLY -->"
	architecture := "# Architecture\n\n#### 完整 API 路由清单\n\n" + legacyMarker + "\n\n| stale |\n\n<!-- END AUTO-GENERATED: §3.2 -->\n"
	archPath := filepath.Join(root, "architecture.md")
	if err := os.WriteFile(archPath, []byte(architecture), 0o644); err != nil {
		t.Fatalf("write architecture fixture: %v", err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("enter fixture root: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	if err := SyncRouteList(); err != nil {
		t.Fatalf("SyncRouteList: %v", err)
	}
	updated, err := os.ReadFile(archPath)
	if err != nil {
		t.Fatalf("read updated architecture: %v", err)
	}
	text := string(updated)
	if strings.Count(text, "<!-- AUTO-GENERATED: §3.2 API 路由清单") != 1 {
		t.Fatalf("generated route section count = %d, want 1\n%s", strings.Count(text, "<!-- AUTO-GENERATED: §3.2 API 路由清单"), text)
	}
	if strings.Contains(text, legacyMarker) {
		t.Fatal("legacy handler route-owner marker must be replaced")
	}
	if !strings.Contains(text, newMarker) {
		t.Fatal("new router route-owner marker is missing")
	}
	if !strings.Contains(text, "`/api/v1/health`") {
		t.Fatal("generated route table is missing the router-owned route")
	}
}
