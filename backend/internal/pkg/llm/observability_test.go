package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"omnicraft/backend/internal/observability"
)

func TestOpenAIChatEmitsSanitizedGenAISpan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`)); err != nil {
			t.Errorf("write test response: %v", err)
		}
	}))
	defer server.Close()

	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(previous)

	client := NewOpenAICompatProvider("test-key", server.URL, "test-model", "embedding-model")
	_, err := client.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "secret prompt"}}})
	require.NoError(t, err)

	spans := exporter.GetSpans()
	var llmSpan *tracetest.SpanStub
	for i := range spans {
		for _, attr := range spans[i].Attributes {
			require.NotContains(t, attr.Value.AsString(), "secret prompt")
		}
		if spans[i].Name == "llm.chat" {
			candidate := spans[i]
			llmSpan = &candidate
		}
	}
	require.NotNil(t, llmSpan)
}

func TestOpenAIExternalMetricRecordsHTTPFailureAfterResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("provider-secret-response"))
	}))
	defer server.Close()

	metrics := observability.NewMetrics()
	observability.SetDefaultMetrics(metrics)
	defer observability.SetDefaultMetrics(nil)

	provider := NewOpenAICompatProvider("test-key", server.URL, "test-model", "embedding-model")
	_, err := provider.Chat(context.Background(), ChatRequest{})
	if err == nil || strings.Contains(err.Error(), "provider-secret-response") {
		t.Fatalf("Chat() error = %v, want sanitized HTTP failure", err)
	}

	families, err := metrics.Registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	foundFailure := false
	for _, family := range families {
		if family.GetName() != "omnicraft_external_dependency_requests_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["dependency"] == "llm" && labels["result"] == "failure" && metric.GetCounter().GetValue() == 1 {
				foundFailure = true
			}
		}
	}
	if !foundFailure {
		t.Fatal("LLM HTTP failure was not recorded as an external dependency failure")
	}
}
