package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"omnicraft/backend/internal/agentmcp"
)

// AgentMCPResult is the bridged external-tool outcome (SP-23 M3): the
// serialized (and byte-capped) MCP tool payload. Tool descriptions and
// results are untrusted input to the model; the M4 guardrail fences them
// before they reach the provider message stream.
type AgentMCPResult struct {
	Server    string `json:"server"`
	Tool      string `json:"tool"`
	Result    string `json:"result"`
	Truncated bool   `json:"truncated,omitempty"`
}

// mcpToolOutcome routes one mcp_<server>_<tool> call through the bridge and
// wraps it as a tool outcome. Errors surface with the underlying error
// string (the #547 lesson: silent success hides dead subprocesses).
func (s *AgentService) mcpToolOutcome(ctx context.Context, name string, rawArgs json.RawMessage) (*AgentToolOutcome, error) {
	serverID, local, ok := splitMCPToolName(name)
	if !ok {
		return nil, ErrAgentToolUnknown
	}
	resultJSON, truncated, err := s.mcpBridge.CallTool(ctx, name, rawArgs)
	if err != nil {
		return nil, err
	}
	return &AgentToolOutcome{
		MCP: &AgentMCPResult{Server: serverID, Tool: local, Result: resultJSON, Truncated: truncated},
		Execution: AgentToolExecution{
			Name: name, Status: AgentToolStatusSuccess, Hits: 1,
			ArgsSummary: mcpArgsSummary(rawArgs),
		},
	}, nil
}

func splitMCPToolName(name string) (serverID, tool string, ok bool) {
	if !strings.HasPrefix(name, agentmcp.ToolNamePrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(name, agentmcp.ToolNamePrefix)
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// mcpArgsSummary derives a display-safe argument summary (bounded, no raw
// JSON echo).
func mcpArgsSummary(raw json.RawMessage) string {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil || len(args) == 0 {
		return fmt.Sprintf("%d bytes", len(raw))
	}
	parts := make([]string, 0, len(args))
	for k, v := range args {
		parts = append(parts, k+"="+summarizeValue(v))
	}
	out := strings.Join(parts, " ")
	if len([]rune(out)) > 80 {
		out = string([]rune(out)[:80]) + "…"
	}
	return out
}

func summarizeValue(v any) string {
	switch t := v.(type) {
	case string:
		if len([]rune(t)) > 24 {
			return string([]rune(t)[:24]) + "…"
		}
		return t
	case float64:
		return trimFloat(t)
	default:
		return "…"
	}
}

func trimFloat(f float64) string {
	s := fmt.Sprintf("%v", f)
	return s
}
