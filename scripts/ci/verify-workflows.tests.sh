#!/usr/bin/env bash
# Contract tests for scripts/ci/verify-workflows.sh against the real workflows
# and mutated copies. Verifies stable job names, triggers, concurrency, minimal
# permissions, SHA-pinned actions, lockfile cache keys, always-upload artifacts
# with retention policy, absence of production secrets, and the Windows Tauri
# path-filtered no-op job.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CHECKER="$SCRIPT_DIR/verify-workflows.sh"
if [ ! -f "$CHECKER" ]; then
  echo "verify-workflows.sh does not exist" >&2
  exit 1
fi

REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CI_SRC="$REPO_ROOT/.github/workflows/ci.yml"
TAURI_SRC="$REPO_ROOT/.github/workflows/tauri-ci.yml"
if [ ! -f "$CI_SRC" ] || [ ! -f "$TAURI_SRC" ]; then
  echo "workflow files do not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-verify-workflows.XXXXXX")"
WF="$TEMP_ROOT/.github/workflows"
mkdir -p "$WF"
trap 'rm -rf "$TEMP_ROOT"' EXIT

expect_ok() {
  local message="$1"
  shift
  if ! bash "$CHECKER" -WorkflowsDir "$WF" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

expect_fail() {
  local message="$1"
  if bash "$CHECKER" -WorkflowsDir "$WF" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

mutate() {
  python3 - "$WF/ci.yml" "$WF/tauri-ci.yml" "$1" <<'PY'
import sys

ci_path, tauri_path, spec = sys.argv[1], sys.argv[2], sys.argv[3]
parts = spec.split("||")
target, key = parts[0], parts[1]
needle = parts[2] if len(parts) > 2 else ""
replacement = parts[3] if len(parts) > 3 else ""
needle = needle.replace("\\n", "\n")
replacement = replacement.replace("\\n", "\n")


def strip_block(text, key):
    lines = text.splitlines()
    out = []
    i = 0
    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        indent = len(line) - len(line.lstrip(" "))
        if stripped == key + ":" or stripped.startswith(key + ": "):
            j = i + 1
            while j < len(lines):
                nxt = lines[j]
                if nxt.strip() and (len(nxt) - len(nxt.lstrip(" "))) <= indent:
                    break
                j += 1
            i = j
            continue
        out.append(line)
        i += 1
    return "\n".join(out) + "\n"


path = ci_path if target == "ci" else tauri_path
with open(path, encoding="utf-8") as f:
    text = f.read()
if key == "__strip__":
    text = strip_block(text, needle)
elif key == "__replace__":
    assert needle in text, f"needle not found: {needle}"
    text = text.replace(needle, replacement, 1)
else:
    raise SystemExit(f"unknown mutation: {key}")
with open(path, "w", encoding="utf-8") as f:
    f.write(text)
PY
}

reset_fixtures() {
  cp "$CI_SRC" "$WF/ci.yml"
  cp "$TAURI_SRC" "$WF/tauri-ci.yml"
}

expect_fail "missing workflows directory must be rejected" -WorkflowsDir "$TEMP_ROOT/nope"

reset_fixtures
rm "$WF/tauri-ci.yml"
expect_fail "missing tauri-ci.yml must be rejected"

reset_fixtures
rm "$WF/ci.yml"
expect_fail "missing ci.yml must be rejected"

reset_fixtures
expect_ok "real workflows must satisfy the contract"

reset_fixtures
mutate "ci||__strip__||pull_request"
expect_fail "ci.yml must declare the pull_request trigger"

reset_fixtures
mutate "ci||__strip__||push"
expect_fail "ci.yml must declare the push trigger"

reset_fixtures
mutate "ci||__strip__||failure_probe"
expect_fail "ci.yml must guard the failure_probe dispatch input"

reset_fixtures
mutate "ci||__replace__||  backend:||  beep:"
expect_fail "stable job name backend must not be renamed"

reset_fixtures
mutate "ci||__replace__||contents: read||contents: write"
expect_fail "permissions must stay minimal"

reset_fixtures
mutate "ci||__strip__||concurrency"
expect_fail "concurrency must be configured"

reset_fixtures
mutate "ci||__replace__||actions/checkout@||actions/checkout@v4"
expect_fail "actions must be SHA-pinned, not floating"

reset_fixtures
mutate "ci||__replace__||hashFiles(||hash("
expect_fail "cache keys must derive from lockfiles via hashFiles"

reset_fixtures
mutate "ci||__replace__||if: always()||if: success()"
expect_fail "evidence artifacts must upload under if: always()"

reset_fixtures
mutate "ci||__replace__||90 || 30||90"
expect_fail "artifact retention must express the 30/90-day policy"

reset_fixtures
mutate "ci||__replace__||run: bash scripts/verify-project.sh -Scope Backend -ReportDir artifacts/backend||run: echo \"\${{ secrets.PRODUCTION_KEY }}\""
expect_fail "production secrets must never be referenced"

reset_fixtures
mutate "tauri||__strip__||pull_request"
expect_fail "tauri-ci.yml must trigger on every pull request"

reset_fixtures
mutate "tauri||__replace__||runs-on: windows-latest||runs-on: ubuntu-latest"
expect_fail "tauri-windows must run on Windows"

reset_fixtures
mutate "tauri||__replace__||!= 'true'||== 'true'"
expect_fail "tauri-windows must keep the explicit no-op branch"

reset_fixtures
mutate "tauri||__replace__||  tauri-windows:||  desktop:"
expect_fail "stable job name tauri-windows must not be renamed"

reset_fixtures
mutate "ci||__replace__||\non:||\non:\n    pull_request:\n      paths: [frontend/**]"
expect_fail "workflow-level paths must not gate required checks"

echo "verify-workflows contract tests passed"
