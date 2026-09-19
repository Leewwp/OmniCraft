package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// labeledCounterValue sums the counter samples of one metric family whose
// labels match want exactly.
func labeledCounterValue(t *testing.T, reg *prometheus.Registry, name string, want map[string]string) float64 {
	t.Helper()
	metrics, err := reg.Gather()
	require.NoError(t, err)
	total := 0.0
	for _, mf := range metrics {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			matches := true
			for k, v := range want {
				found := false
				for _, l := range m.GetLabel() {
					if l.GetName() == k {
						found = l.GetValue() == v
					}
				}
				if !found {
					matches = false
				}
			}
			if matches {
				total += m.GetCounter().GetValue()
			}
		}
	}
	return total
}

func TestAgentMetricFamiliesRegister(t *testing.T) {
	m := NewMetrics()
	m.ObserveAgentRun("SUCCESS", "grounded_content", true, 0.5, 4)
	m.IncAgentRoutingFallback("provider_error")
	m.IncAgentToolCall("search_content", false)

	names := gatherMetricNames(t, m.Registry)
	for _, want := range []string{
		"omnicraft_agent_runs_total",
		"omnicraft_agent_answers_total",
		"omnicraft_agent_routing_fallbacks_total",
		"omnicraft_agent_tool_calls_total",
		"omnicraft_agent_ttft_seconds",
		"omnicraft_agent_run_duration_seconds",
	} {
		require.Contains(t, names, want, "metric %q must be registered", want)
	}
}

func TestObserveAgentRunLabelVocabulary(t *testing.T) {
	m := NewMetrics()

	// Trace casing lowercases; kinds pass through when known.
	m.ObserveAgentRun("SUCCESS", "grounded_content", true, 0.4, 3)
	m.ObserveAgentRun("ERROR", "", false, 0, 0.01)
	m.ObserveAgentRun("CANCELLED", "conversational", false, 0, 1)

	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_runs_total", map[string]string{"status": "success"}))
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_runs_total", map[string]string{"status": "error"}))
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_runs_total", map[string]string{"status": "cancelled"}))
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_answers_total", map[string]string{"kind": "grounded_content"}))
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_answers_total", map[string]string{"kind": "conversational"}))

	// Unknown status and kind fold to "other" instead of vanishing, so
	// ratio alert denominators stay honest when vocabularies drift.
	m.ObserveAgentRun("WEIRD", "future_kind", false, 0, 1)
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_runs_total", map[string]string{"status": "other"}))
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_answers_total", map[string]string{"kind": "other"}))

	// An empty kind (error paths) must not touch the answers counter.
	require.Equal(t, 0.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_answers_total", map[string]string{"kind": "no_evidence"}))
}

func TestObserveAgentRunHistogramGating(t *testing.T) {
	m := NewMetrics()

	m.ObserveAgentRun("SUCCESS", "conversational", false, 0, 2)
	require.Equal(t, uint64(0), histogramCount(t, m.Registry, "omnicraft_agent_ttft_seconds"))
	require.Equal(t, uint64(1), histogramCount(t, m.Registry, "omnicraft_agent_run_duration_seconds"))

	m.ObserveAgentRun("SUCCESS", "grounded_content", true, 0.75, 5)
	require.Equal(t, uint64(1), histogramCount(t, m.Registry, "omnicraft_agent_ttft_seconds"))
	require.Equal(t, uint64(2), histogramCount(t, m.Registry, "omnicraft_agent_run_duration_seconds"))
}

func histogramCount(t *testing.T, reg *prometheus.Registry, name string) uint64 {
	t.Helper()
	metrics, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range metrics {
		if mf.GetName() == name {
			var total uint64
			for _, m := range mf.GetMetric() {
				total += m.GetHistogram().GetSampleCount()
			}
			return total
		}
	}
	return 0
}

func TestIncAgentRoutingFallbackReasons(t *testing.T) {
	m := NewMetrics()

	m.IncAgentRoutingFallback("provider_error")
	m.IncAgentRoutingFallback("blank_answer")
	m.IncAgentRoutingFallback("provider_error")
	m.IncAgentRoutingFallback("not_a_reason")

	require.Equal(t, 2.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_routing_fallbacks_total", map[string]string{"reason": "provider_error"}))
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_routing_fallbacks_total", map[string]string{"reason": "blank_answer"}))
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_routing_fallbacks_total", map[string]string{"reason": "other"}))
}

func TestIncAgentToolCallFolding(t *testing.T) {
	m := NewMetrics()

	m.IncAgentToolCall("search_content", false)
	m.IncAgentToolCall("generate_image", true)
	m.IncAgentToolCall("mcp_docserver_draft_read", false)
	m.IncAgentToolCall("mcp_docserver_apply_edit", true)
	m.IncAgentToolCall("totally_new_tool", false)

	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_tool_calls_total",
		map[string]string{"tool": "search_content", "outcome": "success"}))
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_tool_calls_total",
		map[string]string{"tool": "generate_image", "outcome": "error"}))
	// Every mcp_<server>_<tool> folds onto the single "mcp" label so
	// externally configured servers cannot grow the label set.
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_tool_calls_total",
		map[string]string{"tool": "mcp", "outcome": "success"}))
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_tool_calls_total",
		map[string]string{"tool": "mcp", "outcome": "error"}))
	require.Equal(t, 1.0, labeledCounterValue(t, m.Registry, "omnicraft_agent_tool_calls_total",
		map[string]string{"tool": "other", "outcome": "success"}))
}

func TestAgentDefaultHelpersNilSafe(t *testing.T) {
	require.NotPanics(t, func() {
		ObserveDefaultAgentRun("SUCCESS", "grounded_content", true, 0.5, 4)
		IncDefaultAgentRoutingFallback("provider_error")
		IncDefaultAgentToolCall("search_content", false)
	})
}
