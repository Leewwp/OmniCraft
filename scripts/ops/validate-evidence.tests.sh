#!/usr/bin/env bash
# Contract tests for scripts/ops/validate-evidence.sh.
# Verifies schema conformance: every required field, command/exit-code pairing,
# redaction boolean, blocker list, and optional -ExpectedCommit identity binding.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VALIDATOR="$SCRIPT_DIR/validate-evidence.sh"
if [ ! -f "$VALIDATOR" ]; then
  echo "validate-evidence.sh does not exist" >&2
  exit 1
fi

SCHEMA_SRC="$SCRIPT_DIR/../../release/ops-evidence.schema.json"
if [ ! -f "$SCHEMA_SRC" ]; then
  echo "release/ops-evidence.schema.json does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-evidence-validate.XXXXXX")"
SCHEMA="$TEMP_ROOT/ops-evidence.schema.json"
SUMMARY="$TEMP_ROOT/summary.json"
cp "$SCHEMA_SRC" "$SCHEMA"
trap 'rm -rf "$TEMP_ROOT"' EXIT

write_summary() {
  python3 - "$SUMMARY" <<'PY'
import json, sys
summary = {
    "task": "ops-01",
    "commit": None,
    "started_at": "2026-08-03T00:00:00Z",
    "finished_at": "2026-08-03T00:01:00Z",
    "commands": ["backend|go test ./...", "backend|go vet ./..."],
    "exit_codes": [0, 0],
    "tool_versions": {"go": "go1.25.12 darwin/arm64"},
    "evidence": ["artifacts/ops-01/backend-go-test.log"],
    "redaction_checked": False,
    "blockers": [],
}
with open(sys.argv[1], "w", encoding="utf-8") as out:
    json.dump(summary, out, indent=2)
PY
}

set_summary() {
  python3 - "$SUMMARY" "$1" <<'PY'
import json, sys
path, expr = sys.argv[1], sys.argv[2]
s = json.load(open(path, encoding="utf-8"))
exec(expr, {"s": s, "json": json})
with open(path, "w", encoding="utf-8") as out:
    json.dump(s, out, indent=2)
PY
}

expect_ok() {
  local message="$1"
  shift
  if ! bash "$VALIDATOR" "$@"; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

expect_fail() {
  local message="$1"
  shift
  if bash "$VALIDATOR" "$@"; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

expect_fail "missing -Schema argument must be rejected" -Summary "$SUMMARY"
expect_fail "missing -Summary argument must be rejected" -Schema "$SCHEMA"
expect_fail "missing schema file must be rejected" -Schema "$TEMP_ROOT/nope.json" -Summary "$SUMMARY"
expect_fail "missing summary file must be rejected" -Schema "$SCHEMA" -Summary "$TEMP_ROOT/nope.json"

echo "garbage schema" > "$TEMP_ROOT/bad-schema.json"
expect_fail "malformed schema must be rejected" -Schema "$TEMP_ROOT/bad-schema.json" -Summary "$SUMMARY"
rm -f "$TEMP_ROOT/bad-schema.json"

write_summary
expect_ok "valid pre-finalization summary must pass" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
set_summary 's["commit"] = "a" * 40'
expect_ok "committed summary must pass without -ExpectedCommit" -Schema "$SCHEMA" -Summary "$SUMMARY"
expect_fail "different commit must be rejected with -ExpectedCommit" -Schema "$SCHEMA" -Summary "$SUMMARY" -ExpectedCommit "$(printf 'b%.0s' {1..40})"
expect_fail "short commit must be rejected with -ExpectedCommit" -Schema "$SCHEMA" -Summary "$SUMMARY" -ExpectedCommit deadbeef
expect_ok "matching commit must pass with -ExpectedCommit" -Schema "$SCHEMA" -Summary "$SUMMARY" -ExpectedCommit "$(printf 'a%.0s' {1..40})"

write_summary
set_summary 'del s["task"]'
expect_fail "missing task field must be rejected" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
set_summary 'del s["commit"]'
expect_fail "missing commit field must be rejected" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
set_summary 'del s["commands"]'
expect_fail "missing commands field must be rejected" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
set_summary 'del s["exit_codes"]'
expect_fail "missing exit_codes field must be rejected" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
set_summary 's["exit_codes"] = [0]'
expect_fail "command/exit-code pairing must be enforced" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
set_summary 's["exit_codes"] = ["0", 0]'
expect_fail "non-integer exit codes must be rejected" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
set_summary 's["redaction_checked"] = "true"'
expect_fail "redaction_checked must be a boolean" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
set_summary 's["blockers"] = "none"'
expect_fail "blockers must be an array" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
set_summary 's["tool_versions"] = "go1.25.12"'
expect_fail "tool_versions must be an object" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
set_summary 's["commands"] = ["backend|go test ./...", 7]'
expect_fail "non-string command entries must be rejected" -Schema "$SCHEMA" -Summary "$SUMMARY"

write_summary
expect_fail "unknown argument must be rejected" -Schema "$SCHEMA" -Summary "$SUMMARY" -Bogus flag

echo "validate-evidence contract tests passed"
