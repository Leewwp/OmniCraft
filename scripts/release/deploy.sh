#!/usr/bin/env bash
# =============================================================================
# OmniCraft deployment script: validates a release candidate manifest
# (immutable sha256 image digests, migration plan, preflight/backup/readiness/
# smoke evidence), runs the production preflight, then records the deployment
# with its previous digest for rollback.
#
# Real deployments require staging inputs (see Step 5 of the Ops-08 plan);
# without them the script refuses. -Drill only exercises manifest and history
# contracts; it never claims that a container was deployed.
#
# Usage:
#   bash scripts/release/deploy.sh -Manifest <candidate.json>
#       -EnvFile <path> -OverrideFile <path> -Schema <schema.json>
#       [-ComposeFile <compose.yml>] [-ReportDir <dir>] [-HistoryFile <history.json>] [-Drill]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MANIFEST=""
ENV_FILE=""
OVERRIDE_FILE=""
SCHEMA=""
COMPOSE_FILE=""
REPORT_DIR=""
HISTORY_FILE=""
DRILL=0

while [ $# -gt 0 ]; do
  case "$1" in
    -Manifest) [ $# -ge 2 ] || { echo "missing value for -Manifest" >&2; exit 2; }; MANIFEST="$2"; shift 2 ;;
    -EnvFile) [ $# -ge 2 ] || { echo "missing value for -EnvFile" >&2; exit 2; }; ENV_FILE="$2"; shift 2 ;;
    -OverrideFile) [ $# -ge 2 ] || { echo "missing value for -OverrideFile" >&2; exit 2; }; OVERRIDE_FILE="$2"; shift 2 ;;
    -Schema) [ $# -ge 2 ] || { echo "missing value for -Schema" >&2; exit 2; }; SCHEMA="$2"; shift 2 ;;
    -ComposeFile) [ $# -ge 2 ] || { echo "missing value for -ComposeFile" >&2; exit 2; }; COMPOSE_FILE="$2"; shift 2 ;;
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

if [ -z "$COMPOSE_FILE" ]; then
  COMPOSE_FILE="$REPO_ROOT/docs/deploy/docker-compose.single-server.yml"
fi

for f in "$MANIFEST" "$ENV_FILE" "$OVERRIDE_FILE" "$SCHEMA" "$COMPOSE_FILE"; do
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

RELEASE_ENV_FILE="$ENV_FILE"
RELEASE_COMPOSE_FILE="$COMPOSE_FILE"
RELEASE_OVERRIDE_FILE="$REPORT_DIR/compose-images.yml"
RELEASE_REPORT_DIR="$REPORT_DIR"
RELEASE_DOCKER_BIN="${OMNICRAFT_DOCKER_BIN:-docker}"
# The real path below delegates to docker compose through compose-release.sh.
source "$SCRIPT_DIR/compose-release.sh"

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
    if not block:
        fail(f"{key} evidence block is required")
    allowed = {"ok"} if drill else {"ok", "pending"}
    if block.get(ok_field) not in allowed:
        fail(f"{key}.{ok_field} must be one of {sorted(allowed)}")

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

# ---------------------------------------------------------------- real deployment
if [ "$DRILL" -eq 1 ]; then
  echo "deploy: drill mode - compose side effects skipped; contract evidence is fixture-only"
else
  command -v "$RELEASE_DOCKER_BIN" >/dev/null 2>&1 || { echo "deploy: Docker is required outside -Drill" >&2; exit 1; }
  command -v curl >/dev/null 2>&1 || { echo "deploy: curl is required outside -Drill" >&2; exit 1; }
  render_release_override "$MANIFEST" "$RELEASE_OVERRIDE_FILE"
  release_compose config --quiet
  deploy_release_images
  backup_id="$(run_release_backup)"
  run_release_migration
  activate_release_images
  : > "$REPORT_DIR/readiness.log"
  wait_for_release_health
  run_release_smoke
  python3 - "$REPORT_DIR/deployment-manifest.json" "$MANIFEST" "$HISTORY_FILE" "$backup_id" "$REPORT_DIR" <<'PY'
import datetime
import json
import os
import sys

output, candidate_path, history_path, backup_id, report_dir = sys.argv[1:]
with open(candidate_path, encoding="utf-8") as f:
    record = json.load(f)
history = []
if os.path.isfile(history_path):
    with open(history_path, encoding="utf-8") as f:
        history = json.load(f)
record["previous_digest"] = history[0]["digest"] if history else None
record["preflight"] = {"status": "ok", "summary": os.path.join(report_dir, "preflight-summary.json")}
record["backup"] = {"id": backup_id, "status": "ok"}
record["readiness"] = {"status": "ok"}
record["smoke"] = {"status": "ok"}
record["deployed_at"] = datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z")
with open(output, "w", encoding="utf-8") as f:
    json.dump(record, f, indent=2)
    f.write("\n")
PY
  echo "deploy: real compose deployment, migration, readiness and smoke completed"
fi

# ---------------------------------------------------------------- history
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
history = [item for item in history if item.get("digest") != digest]
history.insert(0, entry)
with open(path, "w", encoding="utf-8") as f:
    json.dump(history, f, indent=2)
    f.write("\n")
PY
  echo "deploy: deployment history updated"
fi

echo "deploy: done"
