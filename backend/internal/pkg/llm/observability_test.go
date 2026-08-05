package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omnicraft/backend/internal/observability"
)

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
