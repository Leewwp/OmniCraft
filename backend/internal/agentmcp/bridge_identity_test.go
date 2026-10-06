package agentmcp

// #816 守门测试：MCP 桥接必须按调用主体绑定身份。沿用审计探针形态——
// 桩 stdio MCP 服务器（测试二进制再执行自身）的 whoami 工具报告子进程
// pid + 启动 env 注入的身份 + 每调用 _meta 里的 viewer，断言两个不同
// viewer 的调用各自以正确身份执行、身份感知会话不共享、同 viewer 会话
// 复用不跨越 per-user 收窄。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"omnicraft/backend/config"
)

// TestMain routes a re-exec of the test binary into the stub stdio MCP
// server (OMNICRAFT_MCP_STUB_SERVER=1): the bridge spawns it as a real
// subprocess, exactly like the production docserver path.
func TestMain(m *testing.M) {
	if os.Getenv("OMNICRAFT_MCP_STUB_SERVER") == "1" {
		runStubIdentityServer()
		return
	}
	os.Exit(m.Run())
}

// runStubIdentityServer serves one whoami tool over stdio: it reports the
// subprocess pid, the env-bound identity (OMNICRAFT_DOC_USER_ID, empty =
// none) and the per-call viewer metadata, which is everything the
// isolation assertions need to observe from inside a real subprocess.
func runStubIdentityServer() {
	s := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "stub-identity", Version: "v1"}, nil)
	type in struct{}
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "whoami",
		Description: "Reports pid, env-bound identity and per-call viewer meta.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, _ in) (*sdkmcp.CallToolResult, any, error) {
		identity := os.Getenv("OMNICRAFT_DOC_USER_ID")
		if identity == "" {
			identity = "none"
		}
		viewer := "none"
		if v, ok := req.Params.Meta[ViewerMetaKey]; ok {
			viewer = fmt.Sprintf("%v", v)
		}
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{
				Text: fmt.Sprintf("pid=%d identity=%s meta_viewer=%s", os.Getpid(), identity, viewer),
			}},
		}, nil, nil
	})
	if err := s.Run(context.Background(), &sdkmcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, "stub mcp server:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// stubIdentityCfg builds a bridge against the stub server subprocess: the
// static env marker selects stub mode inside the test binary, the identity
// binding itself arrives per viewer via identity_env.
func stubIdentityCfg(sessionMax int) config.AgentMCPConfig {
	return config.AgentMCPConfig{
		Enabled: true, CallTimeoutSec: 20, ResultMaxBytes: 8192,
		IdentitySessionMax: sessionMax,
		Servers: []config.AgentMCPServerConfig{{
			ID:          "doc",
			Command:     os.Args[0],
			IdentityEnv: "OMNICRAFT_DOC_USER_ID",
			Env:         map[string]string{"OMNICRAFT_MCP_STUB_SERVER": "1"},
		}},
	}
}

func callWhoami(t *testing.T, b *Bridge, ctx context.Context, viewer int64) string {
	t.Helper()
	out, _, err := b.CallTool(ctx, viewer, "mcp_doc_whoami", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("viewer %d whoami: %v", viewer, err)
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("whoami payload decode: %v (%s)", err, out)
	}
	return payload.Content
}

func pidOfField(report string) int {
	for _, f := range strings.Fields(report) {
		if strings.HasPrefix(f, "pid=") {
			n, _ := strconv.Atoi(strings.TrimPrefix(f, "pid="))
			return n
		}
	}
	return 0
}

// TestIdentityServerBindsViewerIdentityPerCall is the #816 core probe:
// two viewers calling the same identity-aware server each execute under
// their own identity in their own subprocess, a repeat call reuses the
// same viewer's session, and the viewer id travels per call.
func TestIdentityServerBindsViewerIdentityPerCall(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns real stdio subprocesses")
	}
	b := New(stubIdentityCfg(8))
	ctx := context.Background()

	// Definitions ride the viewer's own session: the subprocess must boot
	// before the model can even see the tool.
	defs := b.ToolDefinitions(ctx, 7)
	if len(defs) != 1 || defs[0].Name != "mcp_doc_whoami" {
		t.Fatalf("defs = %+v", defs)
	}

	v7 := callWhoami(t, b, ctx, 7)
	v9 := callWhoami(t, b, ctx, 9)
	v7again := callWhoami(t, b, ctx, 7)

	if !strings.Contains(v7, "identity=7") || !strings.Contains(v7again, "identity=7") {
		t.Fatalf("viewer 7 must execute under its own identity: %q / %q", v7, v7again)
	}
	if !strings.Contains(v9, "identity=9") {
		t.Fatalf("viewer 9 must execute under its own identity: %q", v9)
	}
	if !strings.Contains(v7, "meta_viewer=7") || !strings.Contains(v9, "meta_viewer=9") {
		t.Fatalf("per-call viewer metadata missing: %q / %q", v7, v9)
	}
	pid7, pid7again, pid9 := pidOfField(v7), pidOfField(v7again), pidOfField(v9)
	if pid7 == 0 || pid7again == 0 || pid9 == 0 {
		t.Fatalf("whoami must report the subprocess pid: %q / %q / %q", v7, v7again, v9)
	}
	if pid7 == pid9 {
		t.Fatalf("identity-aware session shared across viewers: pid %d served viewers 7 and 9 — per-user sessions must not cross", pid7)
	}
	if pid7 != pid7again {
		t.Fatalf("same viewer must reuse its per-user session: pid %d vs %d", pid7, pid7again)
	}
}

// TestIdentityServerRejectsAnonymousViewer: identity-aware servers fail
// closed without an authenticated viewer — never boot a subprocess under a
// defaulted/zero identity.
func TestIdentityServerRejectsAnonymousViewer(t *testing.T) {
	b := New(stubIdentityCfg(8))
	if _, _, err := b.CallTool(context.Background(), 0, "mcp_doc_whoami", nil); err == nil {
		t.Fatal("identity-aware server must refuse calls without an authenticated viewer")
	}
	if defs := b.ToolDefinitions(context.Background(), 0); len(defs) != 0 {
		t.Fatalf("identity-aware server must contribute no tools for an anonymous viewer, got %d", len(defs))
	}
}

// TestSharedServerSendsViewerMetaPerCall: non-identity servers keep ONE
// shared session, but the viewer id still travels with every call so the
// server can honor or reject it.
func TestSharedServerSendsViewerMetaPerCall(t *testing.T) {
	var mu sync.Mutex
	var metas []string
	s := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "shared", Version: "v1"}, nil)
	type in struct{}
	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "probe", Description: "captures call meta"}, func(ctx context.Context, req *sdkmcp.CallToolRequest, _ in) (*sdkmcp.CallToolResult, any, error) {
		mu.Lock()
		viewer := "none"
		if v, ok := req.Params.Meta[ViewerMetaKey]; ok {
			viewer = fmt.Sprintf("%v", v)
		}
		metas = append(metas, viewer)
		mu.Unlock()
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "ok"}},
		}, nil, nil
	})
	cfg := config.AgentMCPConfig{
		Enabled: true, CallTimeoutSec: 5, ResultMaxBytes: 4096,
		Servers: []config.AgentMCPServerConfig{{ID: "shared", Command: "/nonexistent"}},
	}
	b := New(cfg)
	dialFake(t, b, "shared", s)

	for _, v := range []int64{7, 9} {
		if _, _, err := b.CallTool(context.Background(), v, "mcp_shared_probe", json.RawMessage(`{}`)); err != nil {
			t.Fatalf("viewer %d: %v", v, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(metas) != 2 || metas[0] != "7" || metas[1] != "9" {
		t.Fatalf("per-call viewer metadata on the shared session = %v, want [7 9]", metas)
	}
	b.mu.Lock()
	live := len(b.claims)
	b.mu.Unlock()
	if live != 1 {
		t.Fatalf("non-identity server must keep one shared session, got %d", live)
	}
}

// TestIdentitySessionEvictionCapsPerViewerSubprocesses: per-user sessions
// are bounded per server; the oldest viewer's session is closed when the
// cap overflows, so the per-user fix cannot become a subprocess leak.
func TestIdentitySessionEvictionCapsPerViewerSubprocesses(t *testing.T) {
	b := New(config.AgentMCPConfig{
		Enabled: true, CallTimeoutSec: 5, ResultMaxBytes: 1024, IdentitySessionMax: 2,
		Servers: []config.AgentMCPServerConfig{{ID: "doc", Command: "/bin/true", IdentityEnv: "OMNICRAFT_DOC_USER_ID"}},
	})
	b.connectFn = func(ctx context.Context, srv config.AgentMCPServerConfig, viewerID int64) (*serverSession, error) {
		return &serverSession{localOf: map[string]string{}}, nil
	}
	srv := config.AgentMCPServerConfig{ID: "doc", IdentityEnv: "OMNICRAFT_DOC_USER_ID"}
	for _, v := range []int64{1, 2, 3} {
		if _, err := b.session(context.Background(), srv, v); err != nil {
			t.Fatalf("viewer %d session: %v", v, err)
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.claims) != 2 {
		t.Fatalf("cap 2 must bound live per-viewer sessions, got %d (%v)", len(b.claims), b.claims)
	}
	if _, ok := b.claims[sessionKey(srv, 1)]; ok {
		t.Fatal("oldest viewer session must be evicted on overflow")
	}
	if _, ok := b.claims[sessionKey(srv, 3)]; !ok {
		t.Fatal("newest viewer session must stay live")
	}
}

// TestServerEnvInjectsIdentityPerViewer: the subprocess env carries the
// static entries plus the per-viewer identity binding; a non-identity
// server never receives an identity key.
func TestServerEnvInjectsIdentityPerViewer(t *testing.T) {
	srv := config.AgentMCPServerConfig{
		ID:          "doc",
		IdentityEnv: "omnicraft_doc_user_id", // viper lowercases; the bridge normalizes
		Env:         map[string]string{"other_key": "v"},
	}
	env := strings.Join(serverEnv(srv, 42), "\n")
	if !strings.Contains(env, "OMNICRAFT_DOC_USER_ID=42") {
		t.Fatalf("identity env not injected per viewer: %q", env)
	}
	if !strings.Contains(env, "OTHER_KEY=v") {
		t.Fatalf("static env entry lost: %q", env)
	}
	for _, kv := range strings.Split(env, "\n") {
		if strings.HasPrefix(kv, "OMNICRAFT_DOC_USER_ID=") && kv != "OMNICRAFT_DOC_USER_ID=42" {
			t.Fatalf("a second identity value must not survive alongside the per-viewer binding: %q", kv)
		}
	}

	shared := config.AgentMCPServerConfig{ID: "plain", Env: map[string]string{"a": "1"}}
	if env := serverEnv(shared, 42); strings.Contains(strings.Join(env, "\n"), "OMNICRAFT_DOC_USER_ID") {
		t.Fatalf("non-identity server must not receive identity env: %v", env)
	}
}
