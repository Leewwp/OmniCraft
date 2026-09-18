package agentmcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/llm"
)

// echoServer is a minimal in-memory MCP server advertising one echo tool.
func echoServer(t *testing.T) *sdkmcp.Server {
	t.Helper()
	s := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "echo", Version: "v1"}, nil)
	type echoIn struct {
		Text string `json:"text" jsonschema:"text to echo"`
	}
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "echo",
		Description: "Echo the text back.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in echoIn) (*sdkmcp.CallToolResult, any, error) {
		return textResult(t, in.Text)
	})
	return s
}

func textResult(t *testing.T, text string) (*sdkmcp.CallToolResult, any, error) {
	t.Helper()
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}, nil, nil
}

// dialFake connects a Bridge against an in-memory server without a
// subprocess: the transport pair is spliced in through a test-only hook.
func dialFake(t *testing.T, b *Bridge, serverID string, srv *sdkmcp.Server) {
	t.Helper()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	ss, err := srv.Connect(context.Background(), t1, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "omnicraft-agent", Version: "v1"}, nil).
		Connect(context.Background(), t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	entry := &serverSession{client: cs, localOf: map[string]string{}}
	for _, tool := range list.Tools {
		ns := NamespacedTool(serverID, tool.Name)
		entry.localOf[ns] = tool.Name
		entry.defs = append(entry.defs, defFor(serverID, tool, ns))
	}
	b.mu.Lock()
	b.claims[serverID] = entry
	b.mu.Unlock()
}

func defFor(serverID string, tool *sdkmcp.Tool, ns string) llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        ns,
		Description: serverID + " " + tool.Description,
		Parameters:  schemaToParameters(tool.InputSchema),
	}
}

func TestBridgeDisabledIsInert(t *testing.T) {
	b := New(config.AgentMCPConfig{})
	if defs := b.ToolDefinitions(context.Background()); defs != nil {
		t.Fatalf("disabled bridge must expose no tools, got %d", len(defs))
	}
	if _, _, err := b.CallTool(context.Background(), "mcp_doc_draft_read", nil); err == nil {
		t.Fatal("disabled bridge must refuse calls")
	}
}

func TestBridgeListAndCallRoundTrip(t *testing.T) {
	cfg := config.AgentMCPConfig{
		Enabled: true, CallTimeoutSec: 5, ResultMaxBytes: 4096,
		Servers: []config.AgentMCPServerConfig{{ID: "echo", Command: "/nonexistent"}},
	}
	b := New(cfg)
	dialFake(t, b, "echo", echoServer(t))

	defs := b.ToolDefinitions(context.Background())
	if len(defs) != 1 || defs[0].Name != "mcp_echo_echo" {
		t.Fatalf("defs = %+v", defs)
	}
	params := defs[0].Parameters
	if params["type"] != "object" {
		t.Fatalf("parameters shape = %#v", defs[0].Parameters)
	}
	if _, ok := params["properties"]; !ok {
		t.Fatalf("input schema properties missing: %#v", params)
	}

	out, truncated, err := b.CallTool(context.Background(), "mcp_echo_echo", json.RawMessage(`{"text":"你好 MCP"}`))
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("short result must not truncate")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	if content, _ := payload["content"].(string); content != "你好 MCP" {
		t.Fatalf("content = %v", payload["content"])
	}

	// unknown tool / unknown server
	if _, _, err := b.CallTool(context.Background(), "mcp_echo_nothing", nil); err == nil {
		t.Fatal("unadvertised tool must error")
	}
	if _, _, err := b.CallTool(context.Background(), "mcp_ghost_x", nil); err == nil {
		t.Fatal("unconfigured server must error")
	}
}

func TestBridgeTruncation(t *testing.T) {
	cfg := config.AgentMCPConfig{
		Enabled: true, CallTimeoutSec: 5, ResultMaxBytes: 100,
		Servers: []config.AgentMCPServerConfig{{ID: "echo", Command: "/nonexistent"}},
	}
	b := New(cfg)
	dialFake(t, b, "echo", echoServer(t))
	out, truncated, err := b.CallTool(context.Background(), "mcp_echo_echo",
		json.RawMessage(`{"text":"`+strings.Repeat("长", 200)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !truncated || len(out) > 4096 {
		t.Fatalf("truncated=%v len=%d", truncated, len(out))
	}
	if !strings.Contains(out, "mcp result truncated") {
		t.Fatal("truncation marker missing")
	}
}

func TestSplitNamespaced(t *testing.T) {
	if s, tl, ok := splitNamespaced("mcp_doc_draft_read"); !ok || s != "doc" || tl != "draft_read" {
		t.Fatalf("split = %q %q %v", s, tl, ok)
	}
	for _, bad := range []string{"search_content", "mcp_", "mcp_x_"} {
		if _, _, ok := splitNamespaced(bad); ok {
			t.Fatalf("%q must not split", bad)
		}
	}
}

func TestBridgeTimeoutDoesNotHang(t *testing.T) {
	// A server tool that sleeps past the call timeout must return an error
	// at the deadline, not hang the agent loop.
	s := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "slow", Version: "v1"}, nil)
	type in struct{}
	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "slow", Description: "sleeps"}, func(ctx context.Context, req *sdkmcp.CallToolRequest, _ in) (*sdkmcp.CallToolResult, any, error) {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return textResult(t, "done")
		}
	})
	cfg := config.AgentMCPConfig{
		Enabled: true, CallTimeoutSec: 1, ResultMaxBytes: 1024,
		Servers: []config.AgentMCPServerConfig{{ID: "slow", Command: "/nonexistent"}},
	}
	b := New(cfg)
	dialFake(t, b, "slow", s)

	start := time.Now()
	_, _, err := b.CallTool(context.Background(), "mcp_slow_slow", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("timeout must surface as an error")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("call took %v, timeout not enforced", time.Since(start))
	}
}
