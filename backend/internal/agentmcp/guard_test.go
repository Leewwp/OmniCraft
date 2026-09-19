package agentmcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"omnicraft/backend/internal/pkg/breaker"
)

// SP-24 R5 MCP mount: an open circuit fails CallTool fast with
// breaker.ErrOpen — no subprocess/stdio round-trip against a dead server
// (a nil Inner proves the provider is never touched in that state).
func TestGuardedBridgeOpenSkipsServer(t *testing.T) {
	br := breaker.New("mcp", breaker.Config{FailureThreshold: 1, OpenTimeout: 30 * 1e9}, nil)
	if err := br.Do(func() error { return errors.New("server handshake dead") }); err == nil {
		t.Fatal("trip failure must propagate")
	}
	g := NewGuardedBridge(nil, br) // inner intentionally nil
	if _, _, err := g.CallTool(context.Background(), "mcp_doc_draft_read", json.RawMessage(`{}`)); !errors.Is(err, breaker.ErrOpen) {
		t.Fatalf("open circuit = %v, want breaker.ErrOpen", err)
	}
}
