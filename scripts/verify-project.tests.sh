#!/usr/bin/env bash
# Contract tests for scripts/verify-project.sh (bash port of verify-project.tests.ps1).
# Verifies exact command order, tier additions, fail-fast, caller-location restore,
# and standalone non-zero exit propagation using fake tool wrappers.
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

echo "verify-project contract tests passed"
