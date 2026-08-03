#!/usr/bin/env bash
# Contract tests for scripts/verify-project.sh (bash port of verify-project.tests.ps1).
# Verifies exact command order, tier additions, scope selection, report writing,
# fail-fast, caller-location restore, and standalone non-zero exit propagation
# using fake tool wrappers.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VERIFIER="$SCRIPT_DIR/verify-project.sh"
if [ ! -f "$VERIFIER" ]; then
  echo "verify-project.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-verify-tests.XXXXXX")"
REPO_ROOT="$TEMP_ROOT/repo"
TOOLS_ROOT="$TEMP_ROOT/tools"
LOG_PATH="$TEMP_ROOT/commands.log"
CURRENT_DIR="$(pwd)"
ORIGINAL_PATH="$PATH"
ORIGINAL_LOG="${OMNICRAFT_VERIFY_TEST_LOG:-}"
ORIGINAL_FAIL="${OMNICRAFT_VERIFY_TEST_FAIL:-}"

cleanup() {
  PATH="$ORIGINAL_PATH"
  if [ -n "$ORIGINAL_LOG" ]; then
    export OMNICRAFT_VERIFY_TEST_LOG="$ORIGINAL_LOG"
  else
    unset OMNICRAFT_VERIFY_TEST_LOG
  fi
  if [ -n "$ORIGINAL_FAIL" ]; then
    export OMNICRAFT_VERIFY_TEST_FAIL="$ORIGINAL_FAIL"
  else
    unset OMNICRAFT_VERIFY_TEST_FAIL
  fi
  rm -rf "$TEMP_ROOT"
}
trap cleanup EXIT

mkdir -p "$TOOLS_ROOT" "$REPO_ROOT/backend" "$REPO_ROOT/frontend" "$REPO_ROOT/scripts" \
  "$REPO_ROOT/tools/doc-validator" "$REPO_ROOT/tauri-client/src-tauri"

cat > "$TOOLS_ROOT/fake-command.sh" <<'FAKE'
#!/usr/bin/env bash
set -u
TOOL="$1"
shift
line="$(basename "$(pwd)")|$TOOL"
if [ $# -gt 0 ]; then
  line="$line $*"
fi
if [ -n "${OMNICRAFT_VERIFY_TEST_LOG:-}" ]; then
  printf '%s\n' "$line" >> "$OMNICRAFT_VERIFY_TEST_LOG"
fi
if [ -n "${OMNICRAFT_VERIFY_TEST_FAIL:-}" ] && [ "$line" = "$OMNICRAFT_VERIFY_TEST_FAIL" ]; then
  exit 23
fi
exit 0
FAKE
chmod +x "$TOOLS_ROOT/fake-command.sh"

for tool in go npm cargo; do
  wrapper="$TOOLS_ROOT/$tool"
  printf '#!/usr/bin/env bash\nexec "$(dirname "$0")/fake-command.sh" %s "$@"\n' "$tool" > "$wrapper"
  chmod +x "$wrapper"
done

export PATH="$TOOLS_ROOT:$PATH"
export OMNICRAFT_VERIFY_TEST_LOG="$LOG_PATH"

cp "$VERIFIER" "$REPO_ROOT/scripts/verify-project.sh"

DEFAULT_COMMANDS="backend|go test ./...
backend|go vet ./...
backend|go build ./...
frontend|npm run test:unit
frontend|npm run lint
frontend|npm run build
doc-validator|go test ./...
doc-validator|go run . --check --profile release"

assert_log() {
  local expected="$1"
  local message="$2"
  local actual=""
  if [ -f "$LOG_PATH" ]; then
    actual="$(cat "$LOG_PATH")"
  fi
  if [ "$actual" != "$expected" ]; then
    echo "FAIL: $message" >&2
    echo "--- expected ---" >&2
    printf '%s\n' "$expected" >&2
    echo "--- actual ---" >&2
    printf '%s\n' "$actual" >&2
    exit 1
  fi
}

run_verifier() {
  ( cd "$CURRENT_DIR" && bash "$VERIFIER" "$@" >/dev/null 2>&1 )
  return $?
}

rm -f "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT"
assert_log "$DEFAULT_COMMANDS" "default tier command contract changed"

rm -f "$LOG_PATH"
( cd "$CURRENT_DIR" && bash "$REPO_ROOT/scripts/verify-project.sh" >/dev/null 2>&1 )
if [ $? -ne 0 ]; then
  echo "FAIL: standalone verifier must resolve RepoRoot from its own scripts directory when omitted" >&2
  exit 1
fi
assert_log "$DEFAULT_COMMANDS" "default RepoRoot command contract changed"

rm -f "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT" --full
assert_log "$DEFAULT_COMMANDS
frontend|npm run test:contracts" "full tier must add mocked browser contracts"

rm -f "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT" --release
assert_log "$DEFAULT_COMMANDS
frontend|npm run test:e2e" "release tier must add the complete Playwright suite"

rm -f "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT" --tauri
assert_log "$DEFAULT_COMMANDS
tauri-client|npm run build
tauri-client|cargo test --manifest-path src-tauri/Cargo.toml" "Tauri option must add frontend and Rust gates"

rm -f "$LOG_PATH"
export OMNICRAFT_VERIFY_TEST_FAIL="backend|go vet ./..."
( cd "$TEMP_ROOT" && bash "$VERIFIER" --repo-root "$REPO_ROOT" >/dev/null 2>&1 )
if [ $? -eq 0 ]; then
  echo "FAIL: verifier must fail when an external command fails" >&2
  exit 1
fi
if [ "$(pwd)" != "$CURRENT_DIR" ]; then
  echo "FAIL: verifier must restore the caller location after failure" >&2
  exit 1
fi
assert_log "backend|go test ./...
backend|go vet ./..." "verifier must stop immediately after the first failed command"

rm -f "$LOG_PATH"
( cd "$CURRENT_DIR" && bash "$REPO_ROOT/scripts/verify-project.sh" >/dev/null 2>&1 )
if [ $? -eq 0 ]; then
  echo "FAIL: standalone verifier process must return a non-zero exit code on child-command failure" >&2
  exit 1
fi
unset OMNICRAFT_VERIFY_TEST_FAIL

# --- scope selection contract ---
BACKEND_COMMANDS="backend|go test ./...
backend|go vet ./...
backend|go build ./..."
FRONTEND_COMMANDS="frontend|npm run test:unit
frontend|npm run lint
frontend|npm run build"
DOCS_COMMANDS="doc-validator|go test ./...
doc-validator|go run . --check --profile release"

rm -f "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT" -Scope Backend
assert_log "$BACKEND_COMMANDS" "backend scope command contract changed"

rm -f "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT" -Scope Frontend
assert_log "$FRONTEND_COMMANDS" "frontend scope command contract changed"

rm -f "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT" -Scope Docs
assert_log "$DOCS_COMMANDS" "docs scope command contract changed"

rm -f "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT" -Scope All
assert_log "$DEFAULT_COMMANDS" "all scope must equal the default command contract"

rm -f "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT" -Scope Frontend --full
assert_log "$FRONTEND_COMMANDS
frontend|npm run test:contracts" "full tier must add browser contracts only inside the frontend scope"

rm -f "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT" -Scope Backend --tauri
assert_log "$BACKEND_COMMANDS
tauri-client|npm run build
tauri-client|cargo test --manifest-path src-tauri/Cargo.toml" "tauri tier must stay orthogonal to scope"

rm -f "$LOG_PATH"
( cd "$TEMP_ROOT" && bash "$VERIFIER" --repo-root "$REPO_ROOT" -Scope Bogus >/dev/null 2>&1 )
if [ $? -eq 0 ]; then
  echo "FAIL: unknown scope must be rejected" >&2
  exit 1
fi
if [ -s "$LOG_PATH" ]; then
  echo "FAIL: rejected scope must not start any command" >&2
  exit 1
fi

rm -f "$LOG_PATH"
( cd "$TEMP_ROOT" && bash "$VERIFIER" --repo-root "$REPO_ROOT" -ReportDir >/dev/null 2>&1 )
if [ $? -eq 0 ]; then
  echo "FAIL: missing report-dir value must be rejected" >&2
  exit 1
fi
if [ -s "$LOG_PATH" ]; then
  echo "FAIL: rejected report parameter must not start any command" >&2
  exit 1
fi

# --- report contract ---
REPORT_DIR="$TEMP_ROOT/report/ops-01"
DEFAULT_LOG_FILES="backend-go-test
backend-go-vet
backend-go-build
frontend-npm-test-unit
frontend-npm-lint
frontend-npm-build
doc-validator-go-test
doc-validator-go-check"

rm -rf "$REPORT_DIR" "$LOG_PATH"
run_verifier --repo-root "$REPO_ROOT" -ReportDir "$REPORT_DIR"
if [ $? -ne 0 ]; then
  echo "FAIL: report run must succeed on green commands" >&2
  exit 1
fi
if [ ! -f "$REPORT_DIR/summary.json" ]; then
  echo "FAIL: report run must write summary.json" >&2
  exit 1
fi
for slug in $DEFAULT_LOG_FILES; do
  if [ ! -f "$REPORT_DIR/$slug.log" ]; then
    echo "FAIL: missing per-command log $slug.log" >&2
    exit 1
  fi
done
python3 - "$REPORT_DIR/summary.json" <<'PY'
import json, sys
s = json.load(open(sys.argv[1]))
required = ["task", "commit", "started_at", "finished_at", "commands",
            "exit_codes", "tool_versions", "evidence", "redaction_checked", "blockers"]
missing = [k for k in required if k not in s]
if missing:
    print("FAIL: summary missing fields", missing)
    sys.exit(1)
if s["task"] != "ops-01":
    print("FAIL: task must derive from report dir basename", s["task"])
    sys.exit(1)
if s["commit"] is not None:
    print("FAIL: commit must be null before finalization")
    sys.exit(1)
if s["redaction_checked"] is not False:
    print("FAIL: redaction_checked must default to false")
    sys.exit(1)
if s["blockers"] != []:
    print("FAIL: blockers must default to an empty list")
    sys.exit(1)
if len(s["commands"]) != len(s["exit_codes"]):
    print("FAIL: commands and exit_codes must be paired")
    sys.exit(1)
if s["commands"] != ["backend|go test ./...", "backend|go vet ./...", "backend|go build ./...",
                     "frontend|npm run test:unit", "frontend|npm run lint", "frontend|npm run build",
                     "doc-validator|go test ./...", "doc-validator|go run . --check --profile release"]:
    print("FAIL: summary command list must match the default contract")
    sys.exit(1)
if len(s["evidence"]) != 8 or not all(e.endswith(".log") for e in s["evidence"]):
    print("FAIL: evidence must list the eight per-command logs")
    sys.exit(1)
for tool in ("go", "node", "npm"):
    if tool not in s["tool_versions"]:
        print("FAIL: tool_versions must record", tool)
        sys.exit(1)
PY
if [ $? -ne 0 ]; then
  exit 1
fi

rm -rf "$REPORT_DIR" "$LOG_PATH"
export OMNICRAFT_VERIFY_TEST_FAIL="backend|go vet ./..."
run_verifier --repo-root "$REPO_ROOT" -ReportDir "$REPORT_DIR"
if [ $? -eq 0 ]; then
  echo "FAIL: failing command must fail the report run" >&2
  exit 1
fi
python3 - "$REPORT_DIR/summary.json" <<'PY'
import json, sys
s = json.load(open(sys.argv[1]))
if s["exit_codes"] != [0, 23]:
    print("FAIL: failing exit code must be recorded in summary", s["exit_codes"])
    sys.exit(1)
if len(s["commands"]) != 2:
    print("FAIL: fail-fast must truncate the recorded command list")
    sys.exit(1)
if len(s["evidence"]) != 2:
    print("FAIL: evidence must only cover executed commands")
    sys.exit(1)
PY
if [ $? -ne 0 ]; then
  exit 1
fi
unset OMNICRAFT_VERIFY_TEST_FAIL

rm -rf "$TEMP_ROOT/relative-report" "$LOG_PATH"
( cd "$TEMP_ROOT" && bash "$VERIFIER" --repo-root "$REPO_ROOT" -ReportDir relative-report >/dev/null 2>&1 )
if [ $? -ne 0 ]; then
  echo "FAIL: relative report dir must resolve against the caller location" >&2
  exit 1
fi
if [ ! -f "$TEMP_ROOT/relative-report/summary.json" ]; then
  echo "FAIL: relative report dir must write summary.json at the caller location" >&2
  exit 1
fi

echo "verify-project contract tests passed"
