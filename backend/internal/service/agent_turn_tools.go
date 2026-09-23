package service

// ToolRuntime is the true seam between the answer-turn orchestrator and
// tool execution (#661). The interface carries exactly two capabilities: the
// tool definitions the model sees, and single-tool execution. Two adapters
// implement it — the local fixed registry and the MCP bridge — and the
// production dispatch composes them (mcp_-prefixed names route to the
// bridge; the namespace cannot collide with the local registry). Turn
// policy stays in the orchestrator on purpose: budget gates, external
// attribution, citation candidate collection and the error→safe-code
// mapping the model consumes are answer-turn policy, not tool
// responsibility.
//
// If B5 (Eino orchestration) revives, this interface is the orchestrator
// replacement mount point — swap the runtime, keep the turn.

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"omnicraft/backend/internal/agentmcp"
	"omnicraft/backend/internal/pkg/llm"
)

// ToolRuntime enumerates the tool surface and executes one tool call.
type ToolRuntime interface {
	// ToolDefinitions enumerates the tool schema surface for the model.
	ToolDefinitions(ctx context.Context) []llm.ToolDefinition
	// ExecuteTool runs one tool call. Raw arguments arrive as model-authored
	// JSON; argument validation and visibility scoping happen inside the
	// adapter, never in the orchestrator.
	ExecuteTool(ctx context.Context, name string, rawArgs json.RawMessage, scope ToolScope) (*AgentToolOutcome, error)
}

// ToolScope carries the execution scoping the answer turn establishes once:
// the viewer behind the request, the conversation (per-conversation
// budgets), the live-turn image count, and the optional publish snapshot.
type ToolScope struct {
	ViewerID       int64
	ConversationID int64
	TurnImages     int
	Snapshot       *AgentPublishSnapshot
}

// localToolRuntime adapts the fixed local registry (toolRegistry map +
// ToolDefinitions schema surface).
type localToolRuntime struct{ svc *AgentService }

func (r *localToolRuntime) ToolDefinitions(ctx context.Context) []llm.ToolDefinition {
	return r.svc.ToolDefinitions()
}

func (r *localToolRuntime) ExecuteTool(ctx context.Context, name string, rawArgs json.RawMessage, scope ToolScope) (*AgentToolOutcome, error) {
	start := time.Now()
	handler, ok := r.svc.toolRegistry()[name]
	if !ok {
		return nil, ErrAgentToolUnknown
	}
	outcome, err := handler(ctx, rawArgs, agentToolScope{
		ViewerID:       scope.ViewerID,
		ConversationID: scope.ConversationID,
		TurnImages:     scope.TurnImages,
		Snapshot:       scope.Snapshot,
	})
	return outcome, withToolError(outcome, name, err, start)
}

// mcpToolRuntime adapts the guarded MCP bridge. The bridge is optional and
// late-wired (SetMCPBridge), so the adapter consults the service's current
// wiring on every call: an unwired bridge contributes no definitions and
// answers mcp_-prefixed calls with ErrAgentToolUnknown, never a panic.
type mcpToolRuntime struct{ svc *AgentService }

func (r *mcpToolRuntime) ToolDefinitions(ctx context.Context) []llm.ToolDefinition {
	if r.svc.mcpBridge == nil {
		return nil
	}
	return r.svc.mcpBridge.ToolDefinitions(ctx)
}

func (r *mcpToolRuntime) ExecuteTool(ctx context.Context, name string, rawArgs json.RawMessage, scope ToolScope) (*AgentToolOutcome, error) {
	start := time.Now()
	if r.svc.mcpBridge == nil {
		return nil, withToolError(nil, name, ErrAgentToolUnknown, start)
	}
	outcome, err := r.svc.mcpToolOutcome(ctx, name, rawArgs)
	return outcome, withToolError(outcome, name, err, start)
}

// dispatchToolRuntime composes the two adapters for production: definitions
// concatenate (bridge tools append to the local registry, mirroring the
// pre-#661 assembly) and mcp_-prefixed names dispatch to the bridge adapter.
type dispatchToolRuntime struct {
	local *localToolRuntime
	mcp   *mcpToolRuntime
}

func (r *dispatchToolRuntime) ToolDefinitions(ctx context.Context) []llm.ToolDefinition {
	tools := r.local.ToolDefinitions(ctx)
	return append(tools, r.mcp.ToolDefinitions(ctx)...)
}

func (r *dispatchToolRuntime) ExecuteTool(ctx context.Context, name string, rawArgs json.RawMessage, scope ToolScope) (*AgentToolOutcome, error) {
	if strings.HasPrefix(name, agentmcp.ToolNamePrefix) {
		return r.mcp.ExecuteTool(ctx, name, rawArgs, scope)
	}
	return r.local.ExecuteTool(ctx, name, rawArgs, scope)
}

// newDispatchToolRuntime is the production wiring: the service's local
// registry plus its (possibly late-wired) MCP bridge.
func newDispatchToolRuntime(svc *AgentService) *dispatchToolRuntime {
	return &dispatchToolRuntime{local: &localToolRuntime{svc: svc}, mcp: &mcpToolRuntime{svc: svc}}
}

// SetToolRuntime swaps the tool seam (tests inject a fake; production keeps
// the dispatch runtime built in the constructor).
func (s *AgentService) SetToolRuntime(rt ToolRuntime) {
	s.toolRuntime = rt
}
