package agentmcp

import (
	"context"
	"encoding/json"

	"omnicraft/backend/internal/pkg/breaker"
	"omnicraft/backend/internal/pkg/llm"
)

// GuardedBridge wraps the MCP client bridge behind a circuit breaker
// (SP-24 R5). An open circuit fails the tool call fast with breaker.ErrOpen
// instead of paying another subprocess/stdio round-trip against a dead
// server; the tool layer renders it like any MCP failure.
type GuardedBridge struct {
	Inner   *Bridge
	Breaker *breaker.Breaker
}

func NewGuardedBridge(inner *Bridge, br *breaker.Breaker) *GuardedBridge {
	return &GuardedBridge{Inner: inner, Breaker: br}
}

func (g *GuardedBridge) ToolDefinitions(ctx context.Context, viewerID int64) []llm.ToolDefinition {
	return g.Inner.ToolDefinitions(ctx, viewerID)
}

// CallTool executes one bridged tool on behalf of viewerID (#816); the
// breaker wraps the call, the inner bridge binds the identity.
func (g *GuardedBridge) CallTool(ctx context.Context, viewerID int64, name string, rawArgs json.RawMessage) (string, bool, error) {
	var resultJSON string
	var truncated bool
	err := g.Breaker.Do(func() error {
		var err error
		resultJSON, truncated, err = g.Inner.CallTool(ctx, viewerID, name, rawArgs)
		return err
	})
	return resultJSON, truncated, err
}
