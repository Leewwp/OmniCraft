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

func TestCheckPgbouncerImageDigest(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		want bool
	}{
		{"digest pinned", "edoburu/pgbouncer@sha256:4c1ca296ef525f108f5d3552cc337c0c09587cf8dae7f0067fd93349e47dc1cd", true},
		{"registry qualified digest pinned", "registry.example/edoburu/pgbouncer@sha256:4c1ca296ef525f108f5d3552cc337c0c09587cf8dae7f0067fd93349e47dc1cd", true},
		{"mutable tag rejected", "edoburu/pgbouncer:latest", false},
		{"empty reference rejected", "", false},
		{"short digest rejected", "edoburu/pgbouncer@sha256:4c1ca296ef525f108f5d3552cc337c0c09587cf8dae7f0067fd93349e47dc1c", false},
		{"uppercase hex digest rejected", "edoburu/pgbouncer@sha256:4C1CA296EF525F108F5D3552CC337C0C09587CF8DAE7F0067FD93349E47DC1CD", false},
		{"tag plus digest rejected", "edoburu/pgbouncer:1.2@sha256:4c1ca296ef525f108f5d3552cc337c0c09587cf8dae7f0067fd93349e47dc1cd", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PGBOUNCER_IMAGE", tc.ref)
			var checks []check
			add := func(name string, ok bool, detail string) {
				checks = append(checks, check{Name: name, OK: ok, Detail: detail})
			}
			checkPgbouncerImageDigest(add)
			if len(checks) != 1 {
				t.Fatalf("expected exactly one check, got %d", len(checks))
			}
			if checks[0].Name != "pgbouncer_image.digest_pinned" {
				t.Fatalf("check name = %q, want pgbouncer_image.digest_pinned", checks[0].Name)
			}
			if checks[0].OK != tc.want {
				t.Fatalf("PGBOUNCER_IMAGE=%q digest_pinned = %v, want %v (detail: %s)",
					tc.ref, checks[0].OK, tc.want, checks[0].Detail)
			}
		})
	}
}
