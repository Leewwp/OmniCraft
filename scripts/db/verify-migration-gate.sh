#!/usr/bin/env bash
# Canonical CI/local gate for Ops-02 migration integration and DB contracts.
# Usage: bash scripts/db/verify-migration-gate.sh -ReportDir <dir>
set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
REPORT_DIR=""

while [ $# -gt 0 ]; do
  case "$1" in
    -ReportDir)
      [ $# -ge 2 ] || { echo "missing value for -ReportDir" >&2; exit 2; }
      REPORT_DIR="$2"
      shift 2
      ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

[ -n "$REPORT_DIR" ] || { echo "usage: verify-migration-gate.sh -ReportDir <dir>" >&2; exit 2; }
mkdir -p "$REPORT_DIR"
REPORT_DIR="$(cd "$REPORT_DIR" && pwd)"

STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
OVERALL=0
COMMANDS=()
EXIT_CODES=()
EVIDENCE=()

run_step() {
  local label="$1"
  local workdir="$2"
  shift 2
  local log="$REPORT_DIR/$label.log"
  local status=0
  set +e
  (cd "$workdir" && "$@") 2>&1 | tee "$log"
  status=${PIPESTATUS[0]}
  set -e
  COMMANDS+=("$*")
  EXIT_CODES+=("$status")
  EVIDENCE+=("$log")
  if [ "$status" -ne 0 ]; then
    OVERALL=1
  fi
}

run_step migration-integration "$REPO_ROOT/backend" go test ./internal/migration ./cmd/migrate -count=1
run_step recovery-contracts "$REPO_ROOT" bash scripts/db/recovery-drill.tests.sh
run_step policy-contracts "$REPO_ROOT" bash scripts/db/verify-backup-policy.tests.sh
run_step fixture-contracts "$REPO_ROOT" bash scripts/db/build-historical-fixture.tests.sh
run_step object-contracts "$REPO_ROOT" bash scripts/db/object-recovery-drill.tests.sh
run_step redis-contracts "$REPO_ROOT" bash scripts/db/redis-reconciliation-drill.tests.sh

FINISHED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
python3 - "$REPORT_DIR/summary.json" "$STARTED_AT" "$FINISHED_AT" "$OVERALL" \
  "${EXIT_CODES[@]}" <<'PY'
import json
import os
import sys

path, started, finished, overall, *codes = sys.argv[1:]
labels = [
    "go test ./internal/migration ./cmd/migrate -count=1",
    "bash scripts/db/recovery-drill.tests.sh",
    "bash scripts/db/verify-backup-policy.tests.sh",
    "bash scripts/db/build-historical-fixture.tests.sh",
    "bash scripts/db/object-recovery-drill.tests.sh",
    "bash scripts/db/redis-reconciliation-drill.tests.sh",
]
evidence = [
    "migration-integration.log",
    "recovery-contracts.log",
    "policy-contracts.log",
    "fixture-contracts.log",
    "object-contracts.log",
    "redis-contracts.log",
]
summary = {
    "gate": "ops-02-migration-contracts",
    "started_at": started,
    "finished_at": finished,
    "status": "passed" if overall == "0" else "failed",
    "commands": labels,
    "exit_codes": [int(code) for code in codes],
    "evidence": evidence,
}
with open(path, "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2)
    f.write("\n")
PY

exit "$OVERALL"
