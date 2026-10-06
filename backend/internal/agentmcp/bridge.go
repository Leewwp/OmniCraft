// Package agentmcp is the MCP client bridge (SP-23 M3, #568): configured
// stdio MCP servers surface to the agent as mcp_<server>_<tool> tools.
// Connections are lazy (first use), calls carry their own timeout, results
// are serialized and truncated to the configured cap before they re-enter
// the model conversation, and failures are surfaced with the underlying
// error string (the #547 lesson: silent registration is not availability).
// Every call carries its calling viewer (#816): identity-aware servers
// (identity_env) run in one subprocess session per viewer, and all calls
// carry the viewer id in their _meta — a bridged tool never executes under
// an identity other than its caller's.
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
	"strconv"
	"strings"
	"sync"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/llm"
)

// ToolNamePrefix namespaces every bridged tool: mcp_<server>_<tool>.
const ToolNamePrefix = "mcp_"

// ViewerMetaKey is the per-call _meta field carrying the calling viewer id
// to bridged servers (#816): every call travels with its subject so servers
// can honor (or reject) it even on shared sessions.
const ViewerMetaKey = "omnicraft.viewer_id"

// defaultIdentitySessionMax bounds live per-viewer subprocess sessions per
// identity-aware server when config leaves identity_session_max unset
// (non-positive values are clamped here as well).
const defaultIdentitySessionMax = 16

var unsafeToolChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// Bridge owns the client sessions for its configured servers. Non-identity
// servers share one session; identity-aware servers (identity_env declared)
// get one subprocess session PER VIEWER (#816) — a shared subprocess would
// execute every caller's tools under its launch identity. The zero value is
// inert (Enabled=false keeps every method a no-op), so wiring it
// unconditionally is safe.
type Bridge struct {
	cfg    config.AgentMCPConfig
	mu     sync.Mutex
	claims map[string]*serverSession // session key → live session (server id, or server id + viewer for identity-aware)
	// inflight dedupes concurrent connects per session key (singleflight):
	// the second caller waits for the first one's result instead of racing
	// a second subprocess.
	inflight map[string]*connectCall
	// identityOrder tracks per-viewer session keys per identity-aware
	// server in insertion order, so the cap (identity_session_max) evicts
	// the oldest viewer's subprocess instead of leaking one per user.
	identityOrder map[string][]string
	// connectFn is the subprocess-spawning connect step, a field so
	// concurrency tests can stub it without real processes.
	connectFn func(ctx context.Context, srv config.AgentMCPServerConfig, viewerID int64) (*serverSession, error)
}

type connectCall struct {
	done chan struct{}
	sess *serverSession
	err  error
}

func (c *connectCall) wait() (*serverSession, error) {
	<-c.done
	return c.sess, c.err
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
	if cfg.IdentitySessionMax <= 0 {
		cfg.IdentitySessionMax = defaultIdentitySessionMax
	}
	b := &Bridge{
		cfg:           cfg,
		claims:        map[string]*serverSession{},
		inflight:      map[string]*connectCall{},
		identityOrder: map[string][]string{},
	}
	b.connectFn = b.connect
	return b
}

// NamespacedTool mints the agent-facing tool name for one server tool.
func NamespacedTool(serverID, toolName string) string {
	s := unsafeToolChars.ReplaceAllString(serverID, "_")
	t := unsafeToolChars.ReplaceAllString(toolName, "_")
	return ToolNamePrefix + s + "_" + t
}

// ToolDefinitions lists every bridged tool of the configured servers as
// they present to one viewer (#816): identity-aware servers ride the
// viewer's own per-user session, so even the tool surface is discovered
// under the caller's identity — never a borrowed one. Connection failures
// degrade to "that server contributes no tools" with an error log — the
// agent loop must never block on a dead subprocess.
func (b *Bridge) ToolDefinitions(ctx context.Context, viewerID int64) []llm.ToolDefinition {
	if b == nil || !b.cfg.Enabled {
		return nil
	}
	var out []llm.ToolDefinition
	for _, srv := range b.cfg.Servers {
		if srv.IdentityEnv != "" && viewerID <= 0 {
			// Fail closed: an identity-aware server never boots without a
			// caller, not even for listing.
			slog.Warn("mcp server tools skipped: identity-aware server requires an authenticated viewer", "server", srv.ID)
			continue
		}
		sess, err := b.session(ctx, srv, viewerID)
		if err != nil {
			slog.Warn("mcp server tools unavailable", "server", srv.ID, "error", err.Error())
			continue
		}
		out = append(out, sess.defs...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CallTool routes one namespaced mcp_<server>_<tool> invocation executed
// on behalf of viewerID (#816): identity-aware servers run the call in the
// viewer's own per-user subprocess session, and every call carries the
// viewer id in its _meta. Unknown names and dead sessions return errors
// (the tool loop records them); timeouts are bounded by call_timeout_sec.
func (b *Bridge) CallTool(ctx context.Context, viewerID int64, name string, rawArgs json.RawMessage) (resultJSON string, truncated bool, err error) {
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
	if srv.IdentityEnv != "" && viewerID <= 0 {
		return "", false, fmt.Errorf("mcp tool %q requires an authenticated viewer (identity-aware server %q)", name, serverID)
	}
	sess, err := b.session(ctx, srv, viewerID)
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
	res, err := sess.client.CallTool(callCtx, &sdkmcp.CallToolParams{
		Name:      local,
		Arguments: args,
		Meta:      sdkmcp.Meta{ViewerMetaKey: viewerID},
	})
	if err != nil {
		// A transport-level failure likely killed the session: drop it so
		// the next call reconnects instead of hanging on a dead pipe.
		b.drop(sessionKey(srv, viewerID))
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

// sessionKey scopes a live session (#816): identity-aware servers get one
// session PER VIEWER (the NUL separator cannot appear in a charset-validated
// server id), non-identity servers share one. Session reuse must never
// cross this per-user boundary.
func sessionKey(srv config.AgentMCPServerConfig, viewerID int64) string {
	if srv.IdentityEnv == "" {
		return srv.ID
	}
	return srv.ID + "\x00" + strconv.FormatInt(viewerID, 10)
}

// session lazily connects (or reconnects) one server on behalf of a viewer
// and refreshes its tool list. Established sessions are returned from the
// cache under a short mutex window; the connect itself (handshake +
// ListTools, up to ~30s for a slow server) runs OUTSIDE the mutex
// (SP-25 低-10) so one dead server can no longer stall every other server's
// session establishment. Concurrent callers for the same session key share
// one connect via inflight.
func (b *Bridge) session(ctx context.Context, srv config.AgentMCPServerConfig, viewerID int64) (*serverSession, error) {
	key := sessionKey(srv, viewerID)
	b.mu.Lock()
	if sess, ok := b.claims[key]; ok {
		b.mu.Unlock()
		return sess, nil
	}
	if call, ok := b.inflight[key]; ok {
		b.mu.Unlock()
		return call.wait()
	}
	call := &connectCall{done: make(chan struct{})}
	b.inflight[key] = call
	b.mu.Unlock()

	sess, err := b.connectFn(ctx, srv, viewerID)

	b.mu.Lock()
	delete(b.inflight, key)
	if err == nil {
		b.claims[key] = sess
		if srv.IdentityEnv != "" {
			b.identityOrder[srv.ID] = append(b.identityOrder[srv.ID], key)
			b.evictIdentitySessionsLocked(srv.ID)
		}
	}
	b.mu.Unlock()
	call.sess, call.err = sess, err
	close(call.done)
	return sess, err
}

// evictIdentitySessionsLocked enforces identity_session_max for one
// identity-aware server: the oldest per-viewer sessions are closed and
// forgotten on overflow. b.mu must be held.
func (b *Bridge) evictIdentitySessionsLocked(serverID string) {
	order := b.identityOrder[serverID]
	for len(order) > b.cfg.IdentitySessionMax && len(order) > 0 {
		oldest := order[0]
		order = order[1:]
		if sess, ok := b.claims[oldest]; ok {
			if sess.client != nil {
				_ = sess.client.Close()
			}
			delete(b.claims, oldest)
		}
	}
	b.identityOrder[serverID] = order
}

// connect spawns the server subprocess, runs the handshake and lists its
// tools. Identity-aware servers receive the calling viewer's id through
// their declared identity env (#816). It must be called without b.mu held.
func (b *Bridge) connect(ctx context.Context, srv config.AgentMCPServerConfig, viewerID int64) (*serverSession, error) {
	cmd := exec.Command(srv.Command, srv.Args...)
	cmd.Env = append(cmd.Environ(), serverEnv(srv, viewerID)...)
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
	logAttrs := []any{"server", srv.ID, "command", srv.Command, "cwd", mustWd()}
	if srv.IdentityEnv != "" {
		logAttrs = append(logAttrs, "identity_env", strings.ToUpper(srv.IdentityEnv), "viewer_id", viewerID)
	}
	slog.Info("mcp bridge launching subprocess", logAttrs...)
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
	return entry, nil
}

func (b *Bridge) drop(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sess, ok := b.claims[key]; ok {
		if sess.client != nil {
			_ = sess.client.Close()
		}
		delete(b.claims, key)
	}
	// Keep the per-server identity accounting honest so the eviction cap
	// measures live sessions, not history: identity session keys are
	// "<server id>\x00<viewer>" (shared keys carry no separator).
	if i := strings.IndexByte(key, '\x00'); i >= 0 {
		serverID := key[:i]
		if order, ok := b.identityOrder[serverID]; ok {
			for j, k := range order {
				if k == key {
					b.identityOrder[serverID] = append(order[:j], order[j+1:]...)
					break
				}
			}
		}
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
// are charset-validated at config load (sanitizeAgentMCPServerIDs: no '_'),
// so the second segment is the id.
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

// serverEnv renders the subprocess environment for one server session
// (#816): the static env map plus, for identity-aware servers, the calling
// viewer's id under the declared identity env key. Viper lowercases every
// config key, so consumed keys are uppercased back: yaml
// `omnicraft_doc_user_id` and `OMNICRAFT_DOC_USER_ID` both reach the
// subprocess as the latter (env var names are uppercase by convention; a
// genuinely lowercase variable cannot be expressed through this config
// path — documented in config.yaml). The map merge keeps one value per
// key: the per-viewer binding is the sole identity value in the child env
// (config validation rejects a static identity key outright).
func serverEnv(srv config.AgentMCPServerConfig, viewerID int64) []string {
	merged := make(map[string]string, len(srv.Env)+1)
	for k, v := range srv.Env {
		merged[strings.ToUpper(k)] = v
	}
	if srv.IdentityEnv != "" {
		merged[strings.ToUpper(srv.IdentityEnv)] = strconv.FormatInt(viewerID, 10)
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
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
