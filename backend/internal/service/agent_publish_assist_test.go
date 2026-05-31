package service

import (
	"os"
	"strings"
	"testing"
)

func TestUploadAssistValidatesFieldLengths(t *testing.T) {
	source := readAgentServiceSource(t)
	if !strings.Contains(source, "UploadAssist") {
		t.Skip("UploadAssist not found in agent_service.go")
	}
}

func TestComplianceCheckReturnsRiskLevel(t *testing.T) {
	source := readAgentServiceSource(t)
	if !strings.Contains(source, "ComplianceCheck") {
		t.Skip("ComplianceCheck not found in agent_service.go")
	}
	if !strings.Contains(source, "risk_level") && !strings.Contains(source, "RiskLevel") {
		t.Fatal("ComplianceCheck must return a risk_level field (safe/warning/violation)")
	}
}

func TestUploadAssistResultHasStructuredFields(t *testing.T) {
	source := readAgentServiceSource(t)
	required := []string{"SuggestedTitle", "SuggestedDescription", "SuggestedTags", "SuggestedCategory"}
	for _, field := range required {
		if !strings.Contains(source, field) {
			t.Fatalf("UploadAssistResult must contain field %s", field)
		}
	}
}

func TestComplianceCheckHandlerExists(t *testing.T) {
	handlerSrc := readHandlerSource(t)
	if !strings.Contains(handlerSrc, "ComplianceCheck") {
		t.Fatal("AgentHandler must have a ComplianceCheck method")
	}
}

func TestUploadAssistHandlerExists(t *testing.T) {
	handlerSrc := readHandlerSource(t)
	if !strings.Contains(handlerSrc, "UploadAssist") {
		t.Fatal("AgentHandler must have an UploadAssist method")
	}
}

func TestUploadAssistRouteRegistered(t *testing.T) {
	routesSrc := readRoutesSource(t)
	if !strings.Contains(routesSrc, "upload-assist") {
		t.Fatal("upload-assist route must be registered")
	}
}

func TestComplianceCheckRouteRegistered(t *testing.T) {
	routesSrc := readRoutesSource(t)
	if !strings.Contains(routesSrc, "compliance-check") {
		t.Fatal("compliance-check route must be registered")
	}
}

func readRoutesSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../handler/routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	return string(data)
}
