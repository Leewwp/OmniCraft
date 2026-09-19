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

func (g *GuardedBridge) ToolDefinitions(ctx context.Context) []llm.ToolDefinition {
	return g.Inner.ToolDefinitions(ctx)
}

func (g *GuardedBridge) CallTool(ctx context.Context, name string, rawArgs json.RawMessage) (string, bool, error) {
	var resultJSON string
	var truncated bool
	err := g.Breaker.Do(func() error {
		var err error
		resultJSON, truncated, err = g.Inner.CallTool(ctx, name, rawArgs)
		return err
	})
	return resultJSON, truncated, err
}
