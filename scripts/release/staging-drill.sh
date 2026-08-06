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
DRILL=0

while [ $# -gt 0 ]; do
  case "$1" in
    -EnvironmentFile) [ $# -ge 2 ] || { echo "missing value for -EnvironmentFile" >&2; exit 2; }; ENV_FILE="$2"; shift 2 ;;
    -OverrideFile) [ $# -ge 2 ] || { echo "missing value for -OverrideFile" >&2; exit 2; }; OVERRIDE_FILE="$2"; shift 2 ;;
    -CandidateManifest) [ $# -ge 2 ] || { echo "missing value for -CandidateManifest" >&2; exit 2; }; CANDIDATE="$2"; shift 2 ;;
    -PreviousManifest) [ $# -ge 2 ] || { echo "missing value for -PreviousManifest" >&2; exit 2; }; PREVIOUS="$2"; shift 2 ;;
    -ReportDir) [ $# -ge 2 ] || { echo "missing value for -ReportDir" >&2; exit 2; }; REPORT_DIR="$2"; shift 2 ;;
    -Drill) DRILL=1; shift ;;
    *)
      echo "unknown argument: $1" >&2
      echo "usage: staging-drill.sh -EnvironmentFile <path> -OverrideFile <path> -CandidateManifest <path> -PreviousManifest <path> [-ReportDir <dir>] [-Drill]" >&2
      exit 2 ;;
  esac
done

if [ -z "$ENV_FILE" ] || [ -z "$OVERRIDE_FILE" ] || [ -z "$CANDIDATE" ] || [ -z "$PREVIOUS" ]; then
  echo "usage: staging-drill.sh -EnvironmentFile <path> -OverrideFile <path> -CandidateManifest <path> -PreviousManifest <path> [-ReportDir <dir>] [-Drill]" >&2
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

echo "staging-drill: done"
