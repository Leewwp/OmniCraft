#!/usr/bin/env bash
# Contract tests for scripts/ops/observability-drill.sh: argument validation,
# compose prerequisites, environment allowlist and target safety. The real
# drill (containers, scraping, Loki queries) is exercised separately.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DRILL="$SCRIPT_DIR/observability-drill.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
if [ ! -f "$DRILL" ]; then
  echo "observability-drill.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-obs-drill.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

expect_fail() {
  local message="$1"
  shift
  if bash "$DRILL" "$@" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

expect_usage_fail() {
  local message="$1"
  shift
  if bash "$DRILL" "$@" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

# Missing required arguments -> usage error (exit 2).
bash "$DRILL" >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: no-args must exit 2" >&2; exit 1; }
bash "$DRILL" -Environment Local >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: missing ReportDir must exit 2" >&2; exit 1; }
expect_fail "missing Environment"
expect_fail "missing ReportDir" -Environment Local

# Invalid environment -> usage error.
bash "$DRILL" -Environment Production -ReportDir "$TEMP_ROOT" >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: Production environment must be rejected" >&2; exit 1; }

# Unknown argument -> usage error.
bash "$DRILL" -Bogus x -Environment Local -ReportDir "$TEMP_ROOT" >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: unknown argument must exit 2" >&2; exit 1; }

# Missing compose file -> failure (exit 1).
expect_fail "missing compose file" -Environment Local -ComposeFile "$TEMP_ROOT/nope.yml" -ReportDir "$TEMP_ROOT"

# Compose without the backend service -> failure.
cat > "$TEMP_ROOT/no-backend.yml" <<'YAML'
services:
  redis:
    image: redis:7-alpine
YAML
expect_fail "compose without backend" -Environment Local -ComposeFile "$TEMP_ROOT/no-backend.yml" -ReportDir "$TEMP_ROOT"

# A real compose file with backend service passes the static checks (the
# drill itself is not run here; argument/prerequisite handling is the target).
cat > "$TEMP_ROOT/with-backend.yml" <<'YAML'
services:
  backend:
    image: alpine:3.19
YAML
bash "$DRILL" -Environment Local -ComposeFile "$TEMP_ROOT/with-backend.yml" -ReportDir "$TEMP_ROOT" >/dev/null 2>&1
status=$?
# The static checks pass; the drill must then fail fast on the observability
# compose check if missing, or proceed into docker work otherwise. Either way
# it must not exit 2 (usage) at this stage.
[ "$status" -ne 2 ] || { echo "FAIL: valid static inputs must not be a usage error" >&2; exit 1; }

echo "OK: observability-drill contract tests passed"
