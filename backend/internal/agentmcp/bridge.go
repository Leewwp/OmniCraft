// Package agentmcp is the MCP client bridge (SP-23 M3, #568): configured
// stdio MCP servers surface to the agent as mcp_<server>_<tool> tools.
// Connections are lazy (first use), calls carry their own timeout, results
// are serialized and truncated to the configured cap before they re-enter
// the model conversation, and failures are surfaced with the underlying
// error string (the #547 lesson: silent registration is not availability).
package agentmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/llm"
)

// ToolNamePrefix namespaces every bridged tool: mcp_<server>_<tool>.
const ToolNamePrefix = "mcp_"

var unsafeToolChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// Bridge owns one client session per configured server. The zero value is
// inert (Enabled=false keeps every method a no-op), so wiring it
// unconditionally is safe.
type Bridge struct {
	cfg    config.AgentMCPConfig
	mu     sync.Mutex
	claims map[string]*serverSession // server id → live session
}

type serverSession struct {
	client  *sdkmcp.ClientSession
	defs    []llm.ToolDefinition // namespaced, server-local tool name kept for routing
	localOf map[string]string    // namespaced tool → server-local tool name
	cmd     *exec.Cmd
}

// New builds the bridge from config; nil cfg = inert.
func New(cfg config.AgentMCPConfig) *Bridge {
	if !cfg.Enabled {
		return &Bridge{}
	}
	return &Bridge{cfg: cfg, claims: map[string]*serverSession{}}
}

// NamespacedTool mints the agent-facing tool name for one server tool.
func NamespacedTool(serverID, toolName string) string {
	s := unsafeToolChars.ReplaceAllString(serverID, "_")
	t := unsafeToolChars.ReplaceAllString(toolName, "_")
	return ToolNamePrefix + s + "_" + t
}

// ToolDefinitions lists every bridged tool across all configured servers.
// Connection failures degrade to "that server contributes no tools" with an
// error log — the agent loop must never block on a dead subprocess.
func (b *Bridge) ToolDefinitions(ctx context.Context) []llm.ToolDefinition {
	if b == nil || !b.cfg.Enabled {
		return nil
	}
	var out []llm.ToolDefinition
	for _, srv := range b.cfg.Servers {
		sess, err := b.session(ctx, srv)
		if err != nil {
			slog.Warn("mcp server tools unavailable", "server", srv.ID, "error", err.Error())
			continue
		}
		out = append(out, sess.defs...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CallTool routes one namespaced mcp_<server>_<tool> invocation. Unknown
// names and dead sessions return errors (the tool loop records them);
// timeouts are bounded by call_timeout_sec.
func (b *Bridge) CallTool(ctx context.Context, name string, rawArgs json.RawMessage) (resultJSON string, truncated bool, err error) {
	if b == nil || !b.cfg.Enabled {
		return "", false, fmt.Errorf("mcp bridge disabled")
	}
	serverID, _, ok := splitNamespaced(name)
	if !ok {
		return "", false, fmt.Errorf("unknown mcp tool %q", name)
	}
	srv, found := b.findServer(serverID)
	if !found {
		return "", false, fmt.Errorf("mcp server %q not configured", serverID)
	}
	sess, err := b.session(ctx, srv)
	if err != nil {
		return "", false, fmt.Errorf("mcp server %q unavailable: %w", serverID, err)
	}
	local, ok := sess.localOf[name]
	if !ok {
		return "", false, fmt.Errorf("mcp tool %q not advertised", name)
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(b.cfg.CallTimeoutSec)*time.Second)
	defer cancel()
	args := map[string]any{}
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return "", false, fmt.Errorf("mcp tool %q args decode: %w", name, err)
		}
	}
	res, err := sess.client.CallTool(callCtx, &sdkmcp.CallToolParams{Name: local, Arguments: args})
	if err != nil {
		// A transport-level failure likely killed the session: drop it so
		// the next call reconnects instead of hanging on a dead pipe.
		b.drop(serverID)
		return "", false, fmt.Errorf("mcp tool %q call failed: %w", name, err)
	}
	return marshalTruncated(res, b.cfg.ResultMaxBytes)
}

// marshalTruncated flattens the result content blocks to text and truncates
// to the configured byte cap, flagging the cut.
func marshalTruncated(res *sdkmcp.CallToolResult, maxBytes int) (string, bool, error) {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			sb.WriteString(tc.Text)
			sb.WriteString("\n")
		}
	}
	text := strings.TrimSpace(sb.String())
	truncated := false
	if len(text) > maxBytes {
		// Cut on a rune boundary, keeping the cap honest in bytes.
		cut := maxBytes
		for cut > 0 && !utf8RuneStart(text[cut]) {
			cut--
		}
		text = text[:cut] + "\n…[mcp result truncated]"
		truncated = true
	}
	payload := map[string]any{"is_error": res.IsError, "content": text}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", false, err
	}
	return string(raw), truncated, nil
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

// session lazily connects (or reconnects) one server and refreshes its
// tool list. Sessions are cached under the mutex; concurrent callers share
// one connection.
func (b *Bridge) session(ctx context.Context, srv config.AgentMCPServerConfig) (*serverSession, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sess, ok := b.claims[srv.ID]; ok {
		return sess, nil
	}
	cmd := exec.Command(srv.Command, srv.Args...)
	cmd.Env = append(cmd.Environ(), envSlice(srv.Env)...)
	// Subprocess diagnostics surface in the server log: the transport
	// discards stderr otherwise, and a dying MCP server would only ever
	// show as an opaque EOF.
	cmd.Stderr = os.Stderr
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "omnicraft-agent", Version: "v1"}, nil)
	// The handshake runs on a detached bounded context: the shared session
	// outlives any single request, and a canceled request (StrictMode
	// double-fire, client disconnect) must never poison the connect.
	handshakeCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	slog.Info("mcp bridge launching subprocess", "server", srv.ID, "command", srv.Command, "cwd", mustWd())
	sess, err := client.Connect(handshakeCtx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("connect %s: %w", srv.Command, err)
	}
	entry := &serverSession{client: sess, cmd: cmd, localOf: map[string]string{}}
	listCtx, listCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer listCancel()
	list, err := sess.ListTools(listCtx, nil)
	if err != nil {
		_ = sess.Close()
		return nil, fmt.Errorf("list tools: %w", err)
	}
	allow := map[string]bool{}
	for _, t := range srv.Tools {
		allow[t] = true
	}
	for _, tool := range list.Tools {
		if len(allow) > 0 && !allow[tool.Name] {
			continue
		}
		ns := NamespacedTool(srv.ID, tool.Name)
		entry.localOf[ns] = tool.Name
		entry.defs = append(entry.defs, llm.ToolDefinition{
			Name:        ns,
			Description: fmt.Sprintf("[mcp:%s] %s", srv.ID, tool.Description),
			Parameters:  schemaToParameters(tool.InputSchema),
		})
	}
	b.claims[srv.ID] = entry
	return entry, nil
}

func (b *Bridge) drop(serverID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sess, ok := b.claims[serverID]; ok {
		_ = sess.client.Close()
		delete(b.claims, serverID)
	}
}

func (b *Bridge) findServer(id string) (config.AgentMCPServerConfig, bool) {
	for _, srv := range b.cfg.Servers {
		if srv.ID == id {
			return srv, true
		}
	}
	return config.AgentMCPServerConfig{}, false
}

// splitNamespaced reverses NamespacedTool: mcp_<server>_<tool>. Server ids
// cannot contain '_' (sanitized at config use sites by convention), so the
// second segment is the id.
func splitNamespaced(name string) (serverID, tool string, ok bool) {
	if !strings.HasPrefix(name, ToolNamePrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(name, ToolNamePrefix)
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func mustWd() string {
	wd, _ := os.Getwd()
	return wd
}

// envSlice renders the server env map. Viper lowercases every config key,
// so the consumed key is uppercased back: yaml `omnicraft_doc_user_id` and
// `OMNICRAFT_DOC_USER_ID` both reach the subprocess as the latter (env var
// names are uppercase by convention; a genuinely lowercase variable cannot
// be expressed through this config path — documented in config.yaml).
func envSlice(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, strings.ToUpper(k)+"="+v)
	}
	sort.Strings(out)
	return out
}

// schemaToParameters converts an MCP input schema (a decoded JSON schema
// object) into the provider tool-parameters map. A nil/unexpected shape
// degrades to an empty object — the provider still sees a valid definition.
func schemaToParameters(schema any) map[string]any {
	obj, _ := schema.(map[string]any)
	if obj == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	params := map[string]any{}
	if t, ok := obj["type"].(string); ok && t != "" {
		params["type"] = t
	} else {
		params["type"] = "object"
	}
	if props, ok := obj["properties"].(map[string]any); ok {
		params["properties"] = props
	} else {
		params["properties"] = map[string]any{}
	}
	if req, ok := obj["required"].([]any); ok && len(req) > 0 {
		params["required"] = req
	}
	return params
}
