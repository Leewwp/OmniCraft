#!/usr/bin/env bash
# =============================================================================
# OmniCraft staging deploy/rollback drill: exercises the release path against
# the real staging environment — preflight, deploy candidate, verify, rollback
# to the previous digest against a compatible schema, verify again, redeploy
# candidate. All external inputs are validated up front; any missing or
# placeholder value stops the drill (Ops-08 Step 5 blocker discipline).
#
# Usage:
#   bash scripts/release/staging-drill.sh -EnvironmentFile <path>
#       -OverrideFile <path> -CandidateManifest <path> -PreviousManifest <path>
#       -ReportDir <dir>
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCHEMA="$REPO_ROOT/release/deployment-manifest.schema.json"

ENV_FILE=""
OVERRIDE_FILE=""
CANDIDATE=""
PREVIOUS=""
REPORT_DIR=""
RECOVERY_OBJECTIVES=""
MEASURED=""
DRILL=0

while [ $# -gt 0 ]; do
  case "$1" in
    -EnvironmentFile) [ $# -ge 2 ] || { echo "missing value for -EnvironmentFile" >&2; exit 2; }; ENV_FILE="$2"; shift 2 ;;
    -OverrideFile) [ $# -ge 2 ] || { echo "missing value for -OverrideFile" >&2; exit 2; }; OVERRIDE_FILE="$2"; shift 2 ;;
    -CandidateManifest) [ $# -ge 2 ] || { echo "missing value for -CandidateManifest" >&2; exit 2; }; CANDIDATE="$2"; shift 2 ;;
    -PreviousManifest) [ $# -ge 2 ] || { echo "missing value for -PreviousManifest" >&2; exit 2; }; PREVIOUS="$2"; shift 2 ;;
    -ReportDir) [ $# -ge 2 ] || { echo "missing value for -ReportDir" >&2; exit 2; }; REPORT_DIR="$2"; shift 2 ;;
    -RecoveryObjectives) [ $# -ge 2 ] || { echo "missing value for -RecoveryObjectives" >&2; exit 2; }; RECOVERY_OBJECTIVES="$2"; shift 2 ;;
    -Measured) [ $# -ge 2 ] || { echo "missing value for -Measured" >&2; exit 2; }; MEASURED="$2"; shift 2 ;;
    -Drill) DRILL=1; shift ;;
    *)
      echo "unknown argument: $1" >&2
      echo "usage: staging-drill.sh -EnvironmentFile <path> -OverrideFile <path> -CandidateManifest <path> -PreviousManifest <path> [-ReportDir <dir>] [-RecoveryObjectives <path>] [-Measured <path>] [-Drill]" >&2
      exit 2 ;;
  esac
done

if [ -z "$ENV_FILE" ] || [ -z "$OVERRIDE_FILE" ] || [ -z "$CANDIDATE" ] || [ -z "$PREVIOUS" ]; then
  echo "usage: staging-drill.sh -EnvironmentFile <path> -OverrideFile <path> -CandidateManifest <path> -PreviousManifest <path> [-ReportDir <dir>] [-RecoveryObjectives <path>] [-Measured <path>] [-Drill]" >&2
  exit 2
fi

for f in "$ENV_FILE" "$OVERRIDE_FILE" "$CANDIDATE" "$PREVIOUS"; do
  [ -f "$f" ] || { echo "file not found: $f" >&2; exit 1; }
done

if [ -z "$REPORT_DIR" ]; then
  REPORT_DIR="$(dirname "$CANDIDATE")"
fi
mkdir -p "$REPORT_DIR"
REPORT_DIR="$(cd "$REPORT_DIR" && pwd)"

# ---------------------------------------------------------------------------
# Step 5 real staging inputs. The drill script validates each input and
# refuses placeholders; missing values are a release blocker, not a skip.
# ---------------------------------------------------------------------------
if [ "$DRILL" -ne 1 ]; then
  REQUIRED_INPUTS=(
    OMNICRAFT_STAGING_ENV_FILE
    OMNICRAFT_STAGING_OVERRIDE_FILE
    OMNICRAFT_CANDIDATE_MANIFEST
    OMNICRAFT_PREVIOUS_MANIFEST
    OMNICRAFT_STAGING_OSS_BUCKET
    OMNICRAFT_OFFSITE_ARCHIVE_URI
    GITHUB_RELEASE_TAG
  )
  MISSING=()
  for v in "${REQUIRED_INPUTS[@]}"; do
    if [ -z "${!v:-}" ]; then
      MISSING+=("$v")
    fi
  done
  if [ ${#MISSING[@]} -gt 0 ]; then
    echo "staging-drill: BLOCKED - missing real staging inputs: ${MISSING[*]}" >&2
    echo "staging-drill: real staging environment/OSS/off-site credentials are required (Ops-08 Step 5)" >&2
    exit 3
  fi
  for v in "${REQUIRED_INPUTS[@]}"; do
    case "${!v}" in
      *"<"* | *">"* | *CHANGE_ME* | *REPLACE_ME* | *PLACEHOLDER* | *example.com* | *your-*)
        echo "staging-drill: input $v contains placeholder value" >&2
        exit 3 ;;
    esac
  done
  echo "staging-drill: all real staging inputs present"
fi

# ---------------------------------------------------------------------------
# Orchestration: preflight -> deploy candidate -> verify -> rollback ->
# verify -> redeploy candidate. Each step must succeed or the drill stops.
# ---------------------------------------------------------------------------
HISTORY="$REPORT_DIR/history.json"
if [ -f "$HISTORY" ]; then
  cp "$HISTORY" "$HISTORY.bak"
fi
python3 - "$HISTORY" "$PREVIOUS" <<'PY'
import json, os, re, sys
path, previous, = sys.argv[1:3]
m = json.load(open(previous, encoding="utf-8"))
digest = m["images"]["backend"]["digest"]
if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
    raise SystemExit(f"previous manifest digest invalid: {digest}")
history = []
if os.path.isfile(path):
    try:
        history = json.load(open(path, encoding="utf-8"))
    except (OSError, ValueError):
        history = []
if not any(e.get("digest") == digest for e in history):
    history.insert(0, {
        "digest": digest,
        "commit": m.get("commit", ""),
        "migration_head": m.get("migration", {}).get("head", ""),
        "schema_compat_max_head": m.get("schema_compat", {}).get("max_head", ""),
    })
with open(path, "w", encoding="utf-8") as f:
    json.dump(history, f, indent=2)
    f.write("\n")
PY

echo "staging-drill: preflight"
bash "$SCRIPT_DIR/preflight.sh" -EnvironmentFile "$ENV_FILE" -OverrideFile "$OVERRIDE_FILE" -ReportDir "$REPORT_DIR"

echo "staging-drill: deploy candidate"
bash "$SCRIPT_DIR/deploy.sh" -Manifest "$CANDIDATE" -EnvFile "$ENV_FILE" -OverrideFile "$OVERRIDE_FILE" \
  -Schema "$SCHEMA" -ReportDir "$REPORT_DIR" -HistoryFile "$HISTORY" ${DRILL:+-Drill}

echo "staging-drill: verify candidate (readiness + smoke contract)"
python3 - "$CANDIDATE" <<'PY'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
for key in ("preflight", "readiness", "smoke"):
    if m.get(key, {}).get("status") != "ok":
        raise SystemExit(f"candidate {key} not ok after deploy")
print("candidate verification ok")
PY

echo "staging-drill: rollback to previous"
bash "$SCRIPT_DIR/rollback.sh" -Manifest "$PREVIOUS" -EnvFile "$ENV_FILE" -OverrideFile "$OVERRIDE_FILE" \
  -Schema "$SCHEMA" -ReportDir "$REPORT_DIR" -HistoryFile "$HISTORY" ${DRILL:+-Drill}

echo "staging-drill: verify rollback target"
python3 - "$PREVIOUS" <<'PY'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
if m.get("migration", {}).get("head", "") > m.get("schema_compat", {}).get("max_head", ""):
    raise SystemExit("rollback target incompatible with deployed schema")
print("rollback verification ok")
PY

echo "staging-drill: redeploy candidate"
bash "$SCRIPT_DIR/deploy.sh" -Manifest "$CANDIDATE" -EnvFile "$ENV_FILE" -OverrideFile "$OVERRIDE_FILE" \
  -Schema "$SCHEMA" -ReportDir "$REPORT_DIR" -HistoryFile "$HISTORY" ${DRILL:+-Drill}

# ---------------------------------------------------------------------------
# Ops-08 Step 5: machine-compare measured PostgreSQL + Aliyun OSS
# restore/reconciliation results against the user-approved RPO/RTO targets.
# ---------------------------------------------------------------------------
if [ -n "$RECOVERY_OBJECTIVES" ] || [ -n "$MEASURED" ]; then
  if [ -z "$RECOVERY_OBJECTIVES" ] || [ -z "$MEASURED" ]; then
    echo "staging-drill: -RecoveryObjectives and -Measured must be given together" >&2
    exit 2
  fi
  [ -f "$RECOVERY_OBJECTIVES" ] || { echo "recovery objectives file not found: $RECOVERY_OBJECTIVES" >&2; exit 1; }
  [ -f "$MEASURED" ] || { echo "measured file not found: $MEASURED" >&2; exit 1; }
  echo "staging-drill: compare measured recovery values against approved RPO/RTO targets"
  OBJECTIVES="$RECOVERY_OBJECTIVES" MEASURED_FILE="$MEASURED" REPORT_DIR="$REPORT_DIR" python3 - <<'PY'
import json, os, sys

objectives_path = os.environ["OBJECTIVES"]
measured_path = os.environ["MEASURED_FILE"]
report_dir = os.environ["REPORT_DIR"]

def fail(msg):
    print(f"staging-drill: {msg}", file=sys.stderr)
    sys.exit(1)

try:
    objectives = json.load(open(objectives_path, encoding="utf-8"))
except (OSError, ValueError) as e:
    fail(f"recovery objectives not valid JSON: {e}")
try:
    measured = json.load(open(measured_path, encoding="utf-8"))
except (OSError, ValueError) as e:
    fail(f"measured values not valid JSON: {e}")

if objectives.get("state") != "approved":
    fail("recovery objectives must be in approved state before comparing RPO/RTO")
targets = objectives.get("approved_targets") or {}
approval = objectives.get("approval") or {}
for key in ("postgres_rpo", "postgres_rto", "object_restore_rto", "service_rpo", "service_rto"):
    if not isinstance(targets.get(key), (int, float)):
        fail(f"approved target {key} is missing or not numeric")
    if not isinstance(measured.get(key), (int, float)):
        fail(f"measured value {key} is missing or not numeric")

comparisons = []
unmet = []
for key in ("postgres_rpo", "postgres_rto", "object_restore_rto", "service_rpo", "service_rto"):
    met = measured[key] <= targets[key]
    comparisons.append({
        "metric": key,
        "measured_minutes": measured[key],
        "approved_target_minutes": targets[key],
        "met": met,
    })
    if not met:
        unmet.append(key)

comparison = {
    "schema_version": 1,
    "state": "approved",
    "checked_at": measured.get("measured_at") or measured.get("last_drill") or "",
    "approval_ref": approval.get("ref", ""),
    "approver": approval.get("approver", ""),
    "comparisons": comparisons,
    "all_met": not unmet,
    "unmet": unmet,
}
with open(os.path.join(report_dir, "recovery-objective-comparison.json"), "w", encoding="utf-8") as f:
    json.dump(comparison, f, indent=2)
    f.write("\n")
for c in comparisons:
    mark = "ok " if c["met"] else "FAIL"
    print(f"  [{mark}] {c['metric']}: measured {c['measured_minutes']}min <= target {c['approved_target_minutes']}min")
if unmet:
    fail(f"measured RPO/RTO exceed approved targets: {', '.join(unmet)}")
print("staging-drill: all measured recovery values meet approved targets")
PY
fi

echo "staging-drill: done"
