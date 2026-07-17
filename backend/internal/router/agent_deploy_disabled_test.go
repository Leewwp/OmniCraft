package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployScriptRouteRemoved(t *testing.T) {
	source := readRoutesSource(t)

	if strings.Contains(source, "agent/script") {
		t.Fatal("unsafe /agent/script/:id route must be removed; deploy grants use /deploy-grants instead")
	}
	if strings.Contains(source, "GenerateDeployScript") {
		t.Fatal("GenerateDeployScript handler must be removed from routes.go")
	}
}

func TestDeployScriptHandlerRemoved(t *testing.T) {
	bytes, err := os.ReadFile(filepath.Join("..", "handler", "agent.go"))
	if err != nil {
		t.Fatalf("read agent.go: %v", err)
	}
	source := string(bytes)

	if strings.Contains(source, "func (h *AgentHandler) GenerateDeployScript") {
		t.Fatal("GenerateDeployScript handler method must be removed from agent.go")
	}
}
