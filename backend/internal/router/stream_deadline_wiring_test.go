package router

import (
	"os"
	"strings"
	"testing"
)

// #800 接线守门：三条流式路由（agent chat stream、usage-guide 流式形态、
// MCP Streamable HTTP）必须挂 StreamWriteDeadline，且窗口取自
// server.stream_write_window（不硬编码）。源码级断言沿用本包 routes 守门
// 测试的既有形态。
func TestStreamRoutesUseRollingWriteDeadline(t *testing.T) {
	raw, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	src := string(raw)
	for _, want := range []string{
		`agent.POST("/chat/stream", streamWriteDeadline`,
		`contents.GET("/:id/guide", optAuth, streamWriteDeadline`,
		`v1.POST("/mcp", optAuth, mcpIdentity, mcpLimiter, streamWriteDeadline`,
		`v1.GET("/mcp", optAuth, mcpIdentity, mcpLimiter, streamWriteDeadline`,
		`v1.DELETE("/mcp", optAuth, mcpIdentity, mcpLimiter, streamWriteDeadline`,
		`StreamWriteDeadline(time.Duration(cfg.Server.StreamWriteWindow) * time.Second)`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("routes.go must wire stream write deadline: missing %q", want)
		}
	}
}
