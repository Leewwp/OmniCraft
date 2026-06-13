package service

import (
	"context"
	"errors"
	"testing"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/llm"
)

type fakeAgentErrorProvider struct {
	err error
}

func (p *fakeAgentErrorProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, p.err
}

func (p *fakeAgentErrorProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	return handler(llm.ChatDelta{Done: true})
}

func (p *fakeAgentErrorProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}

func TestUploadAssistInvalidJSONFallsBackToOriginalMetadata(t *testing.T) {
	svc := NewAgentService(
		&fakeAgentLLMProvider{chatContent: "not-json"},
		nil,
		nil,
		nil,
		nil,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}},
	)

	result, err := svc.UploadAssist(context.Background(), 7, "Original Title", "Original Description", "cover.png", "gaming")
	if err != nil {
		t.Fatalf("UploadAssist: %v", err)
	}

	if result.SuggestedTitle != "Original Title" {
		t.Fatalf("SuggestedTitle = %q, want original title", result.SuggestedTitle)
	}
	if result.SuggestedDescription != "Original Description" {
		t.Fatalf("SuggestedDescription = %q, want original description", result.SuggestedDescription)
	}
	if result.SuggestedCategory != "gaming" {
		t.Fatalf("SuggestedCategory = %q, want gaming fallback", result.SuggestedCategory)
	}
	if len(result.SuggestedTags) != 0 {
		t.Fatalf("SuggestedTags = %#v, want empty fallback", result.SuggestedTags)
	}
}

func TestUploadAssistReturnsProviderError(t *testing.T) {
	svc := NewAgentService(
		&fakeAgentErrorProvider{err: errors.New("llm unavailable")},
		nil,
		nil,
		nil,
		nil,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}},
	)

	_, err := svc.UploadAssist(context.Background(), 7, "Original Title", "Original Description", "cover.png", "gaming")
	if err == nil || err.Error() != "llm unavailable" {
		t.Fatalf("UploadAssist error = %v, want llm unavailable", err)
	}
}

func TestComplianceCheckParsesStructuredRiskLevel(t *testing.T) {
	svc := NewAgentService(
		&fakeAgentLLMProvider{chatContent: `{"risk_level":"warning","reason":"possible copyright issue","suggestions":["add attribution"]}`},
		nil,
		nil,
		nil,
		nil,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}},
	)

	result, err := svc.ComplianceCheck(context.Background(), "Title", "Description", "article")
	if err != nil {
		t.Fatalf("ComplianceCheck: %v", err)
	}

	if result.RiskLevel != "warning" {
		t.Fatalf("RiskLevel = %q, want warning", result.RiskLevel)
	}
	if result.Reason != "possible copyright issue" {
		t.Fatalf("Reason = %q, want parsed reason", result.Reason)
	}
	if len(result.Suggestions) != 1 || result.Suggestions[0] != "add attribution" {
		t.Fatalf("Suggestions = %#v, want parsed suggestions", result.Suggestions)
	}
}

func TestComplianceCheckInvalidJSONFallsBackToSafeReason(t *testing.T) {
	svc := NewAgentService(
		&fakeAgentLLMProvider{chatContent: "manual review recommended"},
		nil,
		nil,
		nil,
		nil,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}},
	)

	result, err := svc.ComplianceCheck(context.Background(), "Title", "Description", "article")
	if err != nil {
		t.Fatalf("ComplianceCheck: %v", err)
	}

	if result.RiskLevel != "safe" {
		t.Fatalf("RiskLevel = %q, want safe fallback", result.RiskLevel)
	}
	if result.Reason != "manual review recommended" {
		t.Fatalf("Reason = %q, want raw fallback", result.Reason)
	}
}
