package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEffectiveConfigMatchesRuntimeOverridePrecedence(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend", "config.yaml"), []byte(`agent:
  llm_provider: openai_compat
  llm_model: base-model
`), 0o600); err != nil {
		t.Fatal(err)
	}
	override := filepath.Join(root, "override.yaml")
	if err := os.WriteFile(override, []byte(`agent:
  llm_provider: qwen
  llm_model: override-model
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_LLM_PROVIDER", "minimax")
	t.Setenv("AGENT_LLM_MODEL", "MiniMax-M1")

	var checks []check
	cfg := loadEffectiveConfig(root, override, &checks)
	if cfg == nil {
		t.Fatalf("loadEffectiveConfig returned nil, checks = %#v", checks)
	}
	if got, want := cfg.Agent.LLMProvider, "minimax"; got != want {
		t.Fatalf("provider = %q, want explicit environment override %q", got, want)
	}
	if got, want := cfg.Agent.LLMModel, "MiniMax-M1"; got != want {
		t.Fatalf("model = %q, want explicit environment override %q", got, want)
	}
}
