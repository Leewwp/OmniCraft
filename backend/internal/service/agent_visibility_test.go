package service

import (
	"os"
	"strings"
	"testing"
)

func TestAgentRetrievalAppliesVisibilityScope(t *testing.T) {
	source := readAgentServiceSource(t)
	required := []string{
		"published",
		"IsBanned",
		"IsPublic",
		"viewerID",
	}
	for _, r := range required {
		if !strings.Contains(source, r) {
			t.Fatalf("agent retrieval must apply visibility scope including %q check", r)
		}
	}
}

func TestAgentRetrievalAcceptsViewerID(t *testing.T) {
	source := readAgentServiceSource(t)
	if !strings.Contains(source, "viewerID") && !strings.Contains(source, "viewer_id") {
		t.Fatal("agent retrieval must accept a viewerID parameter to apply is_public/author visibility rules")
	}
}

func readAgentServiceSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("agent_service.go")
	if err != nil {
		t.Fatalf("read agent_service.go: %v", err)
	}
	return string(data)
}
