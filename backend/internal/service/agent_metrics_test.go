package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/observability"
)

// SP-24 R7: the tool-execution chokepoint feeds the SLA counter with every
// outcome (local registry and MCP bridge both exit through withToolError).
func TestToolExecutionFeedsSLACounter(t *testing.T) {
	metrics := observability.NewMetrics()
	observability.SetDefaultMetrics(metrics)
	defer observability.SetDefaultMetrics(nil)

	withToolError(nil, ToolSearchContent, ErrAgentToolInvalidArgs, time.Now())
	outcome := &AgentToolOutcome{}
	if err := withToolError(outcome, ToolSearchContent, nil, time.Now()); err != nil {
		t.Fatalf("withToolError success path: %v", err)
	}

	families, err := metrics.Registry.Gather()
	require.NoError(t, err)
	values := map[string]float64{}
	for _, mf := range families {
		if mf.GetName() != "omnicraft_agent_tool_calls_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			var tool, result string
			for _, l := range m.GetLabel() {
				if l.GetName() == "tool" {
					tool = l.GetValue()
				}
				if l.GetName() == "outcome" {
					result = l.GetValue()
				}
			}
			values[tool+"/"+result] = m.GetCounter().GetValue()
		}
	}
	require.Equal(t, float64(1), values["search_content/error"])
	require.Equal(t, float64(1), values["search_content/success"])
}
