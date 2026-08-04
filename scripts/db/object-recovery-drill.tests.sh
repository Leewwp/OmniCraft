#!/usr/bin/env bash
# Contract tests for scripts/db/object-recovery-drill.sh. The real drill requires
# docker and is exercised separately; these tests cover argument validation,
# compose-file existence and the evidence writer.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DRILL="$SCRIPT_DIR/object-recovery-drill.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE="$REPO_ROOT/ops/recovery/docker-compose.recovery.yml"

if [ ! -f "$DRILL" ]; then
  echo "object-recovery-drill.sh does not exist" >&2
  exit 1
fi
if [ ! -f "$COMPOSE" ]; then
  echo "ops/recovery/docker-compose.recovery.yml does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-object-drill.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

expect_exit() {
  local want="$1"
  local message="$2"
  shift 2
  local got=0
  bash "$DRILL" "$@" >/dev/null 2>&1 || got=$?
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $message (exit $got, want $want)" >&2
    exit 1
  fi
}

expect_exit 2 "missing -ComposeFile/-ReportDir must be rejected"
expect_exit 2 "unknown arguments must be rejected" -ReportDir "$TEMP_ROOT" -Nope
expect_exit 1 "a missing compose file must be rejected" -ComposeFile "$TEMP_ROOT/nope.yml" -ReportDir "$TEMP_ROOT"
expect_exit 1 "a compose file without the recovery stack must be rejected" \
  -ComposeFile "$REPO_ROOT/docker-compose.yml" -ReportDir "$TEMP_ROOT"

if grep -Fq 'down -v --remove-orphans >/dev/null 2>&1 || true' "$DRILL"; then
  echo "FAIL: object recovery drill must propagate teardown failure" >&2
  exit 1
fi

echo "object-recovery-drill contract tests passed"
