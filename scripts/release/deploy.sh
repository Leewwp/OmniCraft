#!/usr/bin/env bash
# =============================================================================
# OmniCraft deployment script: validates a release candidate manifest
# (immutable sha256 image digests, migration plan, preflight/backup/readiness/
# smoke evidence), runs the production preflight, then records the deployment
# with its previous digest for rollback.
#
# Real deployments require staging inputs (see Step 5 of the Ops-08 plan);
# without them the script refuses. -Drill exercises the full orchestration
# with simulated evidence so the contract tests run anywhere.
#
# Usage:
#   bash scripts/release/deploy.sh -Manifest <candidate.json>
#       -EnvFile <path> -OverrideFile <path> -Schema <schema.json>
#       [-ReportDir <dir>] [-HistoryFile <history.json>] [-Drill]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST=""
ENV_FILE=""
OVERRIDE_FILE=""
SCHEMA=""
REPORT_DIR=""
HISTORY_FILE=""
DRILL=0

while [ $# -gt 0 ]; do
  case "$1" in
    -Manifest) [ $# -ge 2 ] || { echo "missing value for -Manifest" >&2; exit 2; }; MANIFEST="$2"; shift 2 ;;
    -EnvFile) [ $# -ge 2 ] || { echo "missing value for -EnvFile" >&2; exit 2; }; ENV_FILE="$2"; shift 2 ;;
    -OverrideFile) [ $# -ge 2 ] || { echo "missing value for -OverrideFile" >&2; exit 2; }; OVERRIDE_FILE="$2"; shift 2 ;;
    -Schema) [ $# -ge 2 ] || { echo "missing value for -Schema" >&2; exit 2; }; SCHEMA="$2"; shift 2 ;;
    -ReportDir) [ $# -ge 2 ] || { echo "missing value for -ReportDir" >&2; exit 2; }; REPORT_DIR="$2"; shift 2 ;;
    -HistoryFile) [ $# -ge 2 ] || { echo "missing value for -HistoryFile" >&2; exit 2; }; HISTORY_FILE="$2"; shift 2 ;;
    -Drill) DRILL=1; shift ;;
    *)
      echo "unknown argument: $1" >&2
      echo "usage: deploy.sh -Manifest <path> -EnvFile <path> -OverrideFile <path> -Schema <path> [-ReportDir <dir>] [-HistoryFile <path>] [-Drill]" >&2
      exit 2 ;;
  esac
done

if [ -z "$MANIFEST" ] || [ -z "$ENV_FILE" ] || [ -z "$OVERRIDE_FILE" ] || [ -z "$SCHEMA" ]; then
  echo "usage: deploy.sh -Manifest <path> -EnvFile <path> -OverrideFile <path> -Schema <path> [-ReportDir <dir>] [-HistoryFile <path>] [-Drill]" >&2
  exit 2
fi

for f in "$MANIFEST" "$ENV_FILE" "$OVERRIDE_FILE" "$SCHEMA"; do
  [ -f "$f" ] || { echo "file not found: $f" >&2; exit 1; }
done

if [ -z "$REPORT_DIR" ]; then
  REPORT_DIR="$(dirname "$MANIFEST")"
fi
mkdir -p "$REPORT_DIR"
REPORT_DIR="$(cd "$REPORT_DIR" && pwd)"

export OMNICRAFT_DEPLOY_MANIFEST="$(cd "$(dirname "$MANIFEST")" && pwd)/$(basename "$MANIFEST")"
export OMNICRAFT_DEPLOY_SCHEMA="$(cd "$(dirname "$SCHEMA")" && pwd)/$(basename "$SCHEMA")"
export OMNICRAFT_DEPLOY_REPORT="$REPORT_DIR"
export OMNICRAFT_DEPLOY_HISTORY="${HISTORY_FILE:-}"
export OMNICRAFT_DEPLOY_DRILL="$DRILL"

python3 - <<'PY'
import json
import os
import re
import sys

manifest_path = os.environ["OMNICRAFT_DEPLOY_MANIFEST"]
schema_path = os.environ["OMNICRAFT_DEPLOY_SCHEMA"]
report_dir = os.environ["OMNICRAFT_DEPLOY_REPORT"]
history_path = os.environ.get("OMNICRAFT_DEPLOY_HISTORY") or ""
drill = os.environ["OMNICRAFT_DEPLOY_DRILL"] == "1"

try:
    with open(manifest_path, encoding="utf-8") as f:
        m = json.load(f)
except (OSError, ValueError) as e:
    print(f"deploy: candidate manifest is not valid JSON: {e}", file=sys.stderr)
    sys.exit(1)

try:
    schema = json.load(open(schema_path, encoding="utf-8"))
except (OSError, ValueError) as e:
    print(f"deploy: schema is not valid JSON: {e}", file=sys.stderr)
    sys.exit(1)

def fail(msg):
    print(f"deploy: {msg}", file=sys.stderr)
    sys.exit(1)

missing = [f for f in schema.get("required", []) if f not in m]
if missing:
    fail(f"candidate manifest missing required fields: {', '.join(missing)}")

digest_re = re.compile(r"^sha256:[0-9a-f]{64}$")
for name in ("backend", "frontend"):
    img = m.get("images", {}).get(name)
    if not img:
        fail(f"images.{name} is required")
    if not digest_re.fullmatch(img.get("digest", "")):
        fail(f"images.{name}.digest must be a sha256:64-hex immutable digest")
    if not re.fullmatch(r"^[A-Za-z0-9./_-]+@sha256:[0-9a-f]{64}$", img.get("ref", "")):
        fail(f"images.{name}.ref must reference the immutable digest (image@sha256:...)")

for key, ok_field in (("preflight", "status"), ("backup", "status"), ("readiness", "status"), ("smoke", "status")):
    block = m.get(key)
    if not block or block.get(ok_field) != "ok":
        fail(f"{key}.{ok_field} must be 'ok' before deployment")

if not m.get("migration", {}).get("head"):
    fail("migration.head is required before deployment")

history = []
if history_path:
    if os.path.isfile(history_path):
        try:
            history = json.load(open(history_path, encoding="utf-8"))
            if not isinstance(history, list):
                fail("history file must be a JSON array")
        except (OSError, ValueError) as e:
            fail(f"history file is not valid JSON: {e}")
    elif not drill:
        fail("history file missing and not in drill mode; set -HistoryFile or use -Drill")
elif not drill:
    fail("history file is required outside drill mode")

previous_digest = None
if history:
    previous_digest = history[0].get("digest")
    if not digest_re.fullmatch(previous_digest or ""):
        fail("history[0].digest must be a valid immutable digest")

record = {
    "schema_version": "1.0",
    "version": m["version"],
    "commit": m["commit"],
    "created_at": m["created_at"],
    "previous_digest": previous_digest,
    "images": m["images"],
    "migration": m["migration"],
    "schema_compat": m["schema_compat"],
    "preflight": m["preflight"],
    "backup": m["backup"],
    "readiness": m["readiness"],
    "smoke": m["smoke"],
    "deployed_at": m["deployed_at"],
}
with open(os.path.join(report_dir, "deployment-manifest.json"), "w", encoding="utf-8") as f:
    json.dump(record, f, indent=2)
    f.write("\n")
print(f"deploy: candidate validated; deployment manifest written to {report_dir}")
PY

# ------------------------------------------------------------ preflight gate
bash "$SCRIPT_DIR/preflight.sh" -EnvironmentFile "$ENV_FILE" -OverrideFile "$OVERRIDE_FILE" -ReportDir "$REPORT_DIR"

# ---------------------------------------------------------------- history
if [ "$DRILL" -eq 1 ]; then
  echo "deploy: drill mode - simulated backup/migrate/readiness/smoke already recorded by manifest validation"
fi

if [ -n "$HISTORY_FILE" ]; then
  NEW_DIGEST="$(python3 -c "import json;print(json.load(open('$MANIFEST'))['images']['backend']['digest'])")"
  NEW_COMMIT="$(python3 -c "import json;print(json.load(open('$MANIFEST'))['commit'])")"
  NEW_HEAD="$(python3 -c "import json;print(json.load(open('$MANIFEST'))['migration']['head'])")"
  NEW_MAX="$(python3 -c "import json;print(json.load(open('$MANIFEST'))['schema_compat']['max_head'])")"
  python3 - "$HISTORY_FILE" "$NEW_DIGEST" "$NEW_COMMIT" "$NEW_HEAD" "$NEW_MAX" <<'PY'
import json, os, sys
path, digest, commit, head, max_head = sys.argv[1:6]
history = []
if os.path.isfile(path):
    try:
        history = json.load(open(path, encoding="utf-8"))
    except (OSError, ValueError):
        history = []
entry = {"digest": digest, "commit": commit, "migration_head": head, "schema_compat_max_head": max_head}
history.insert(0, entry)
with open(path, "w", encoding="utf-8") as f:
    json.dump(history, f, indent=2)
    f.write("\n")
PY
  echo "deploy: deployment history updated"
fi

echo "deploy: done"
