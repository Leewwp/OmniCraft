#!/usr/bin/env bash
# Manual root Python E2E entry point (bash port of the former run-root-python-e2e.ps1).
# Usage: bash scripts/run-root-python-e2e.sh [all|search-download|admin-journey]
set -u

SUITE="${1:-all}"
PYTHON="${PYTHON:-python3}"

case "$SUITE" in
  all|search-download|admin-journey) ;;
  *)
    echo "unknown suite: $SUITE (expected all|search-download|admin-journey)" >&2
    exit 2
    ;;
esac

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(dirname "$SCRIPT_DIR")"

run_suite() {
  local label="$1"
  local script_path="$2"
  echo ""
  echo "=== Running $label ==="
  "$PYTHON" "$script_path" || {
    echo "$label failed with exit code $?" >&2
    exit 1
  }
}

if [ "$SUITE" = "all" ] || [ "$SUITE" = "search-download" ]; then
  run_suite "search/download manual root E2E" "$ROOT/e2e/test_search_download.py"
fi

if [ "$SUITE" = "all" ] || [ "$SUITE" = "admin-journey" ]; then
  run_suite "admin journey manual root E2E" "$ROOT/e2e/test_admin_journey.py"
fi
