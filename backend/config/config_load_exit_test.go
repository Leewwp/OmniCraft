package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Ticket #671: Load() runs the all-mode structural validation and refuses to
// boot (exit 1) on required-field misses and an invalid server.mode. Load is
// a process-level entry (it exits), so the exit path is proven by a subprocess
// re-exec, not by a direct call. The helper child case below runs first in
// this binary when LOAD_EXIT_CHILD=1.

func TestMain(m *testing.M) {
	if os.Getenv("LOAD_EXIT_CHILD") == "1" {
		// Child: Load() must os.Exit(1) before returning a broken config.
		// If it wrongly succeeds, exit 0 so the parent fails the test.
		Load()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func writeOverride(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "override.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func runLoadChild(t *testing.T, overridePath string) (int, string) {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)

	cmd := exec.Command(exe, "-test.run", "TestMain", "-test.timeout", "30s")
	cmd.Dir = t.TempDir() // no config.yaml here: the override must carry the damage
	cmd.Env = append(os.Environ(),
		"LOAD_EXIT_CHILD=1",
		"CONFIG_OVERRIDE_PATH="+overridePath,
		"OMNICRAFT_TEST_MODE=",
	)
	out, _ := cmd.CombinedOutput()
	return cmd.ProcessState.ExitCode(), string(out)
}

func TestLoadExitsOnInvalidServerMode(t *testing.T) {
	if os.Getenv("LOAD_EXIT_CHILD") == "1" {
		return // child role handled in TestMain
	}
	override := writeOverride(t, `
server:
  mode: staging
`)
	code, out := runLoadChild(t, override)
	require.NotEqual(t, 0, code, "Load must refuse to boot on invalid server.mode; output: %s", out)
}

func TestLoadExitsOnMissingRequiredFields(t *testing.T) {
	if os.Getenv("LOAD_EXIT_CHILD") == "1" {
		return
	}
	// Empty override: no config.yaml in the child cwd either, so every
	// required field is a zero value — Load must list them and exit 1.
	override := writeOverride(t, "")
	code, out := runLoadChild(t, override)
	require.Equal(t, 1, code, "expected exit status 1, got %d; output: %s", code, out)
	for _, path := range []string{"server.port", "database.dsn", "redis.addr", "web.public_base_url", "jwt.secret"} {
		require.True(t, strings.Contains(out, path), "output must mention %s; got: %s", path, out)
	}
}
