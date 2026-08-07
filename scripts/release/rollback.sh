#!/usr/bin/env bash
# =============================================================================
# OmniCraft rollback script: restores a previous application digest. Refuses
# unknown digests and schema-incompatible targets (the target's verified
# schema head must be >= the currently deployed migration head). Never runs
# destructive down SQL: forward-only migrations are left in place.
#
# Usage:
#   bash scripts/release/rollback.sh -Manifest <previous.json>
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
      echo "usage: rollback.sh -Manifest <path> -EnvFile <path> -OverrideFile <path> -Schema <path> [-ReportDir <dir>] [-HistoryFile <path>] [-Drill]" >&2
      exit 2 ;;
  esac
done

if [ -z "$MANIFEST" ] || [ -z "$ENV_FILE" ] || [ -z "$OVERRIDE_FILE" ] || [ -z "$SCHEMA" ]; then
  echo "usage: rollback.sh -Manifest <path> -EnvFile <path> -OverrideFile <path> -Schema <path> [-ReportDir <dir>] [-HistoryFile <path>] [-Drill]" >&2
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

export OMNICRAFT_ROLLBACK_MANIFEST="$(cd "$(dirname "$MANIFEST")" && pwd)/$(basename "$MANIFEST")"
export OMNICRAFT_ROLLBACK_SCHEMA="$(cd "$(dirname "$SCHEMA")" && pwd)/$(basename "$SCHEMA")"
export OMNICRAFT_ROLLBACK_REPORT="$REPORT_DIR"
export OMNICRAFT_ROLLBACK_HISTORY="${HISTORY_FILE:-}"
export OMNICRAFT_ROLLBACK_DRILL="$DRILL"

python3 - <<'PY'
import json
import os
import re
import sys

manifest_path = os.environ["OMNICRAFT_ROLLBACK_MANIFEST"]
schema_path = os.environ["OMNICRAFT_ROLLBACK_SCHEMA"]
report_dir = os.environ["OMNICRAFT_ROLLBACK_REPORT"]
history_path = os.environ.get("OMNICRAFT_ROLLBACK_HISTORY") or ""
drill = os.environ["OMNICRAFT_ROLLBACK_DRILL"] == "1"

try:
    with open(manifest_path, encoding="utf-8") as f:
        m = json.load(f)
except (OSError, ValueError) as e:
    print(f"rollback: target manifest is not valid JSON: {e}", file=sys.stderr)
    sys.exit(1)

try:
    schema = json.load(open(schema_path, encoding="utf-8"))
except (OSError, ValueError) as e:
    print(f"rollback: schema is not valid JSON: {e}", file=sys.stderr)
    sys.exit(1)

def fail(msg):
    print(f"rollback: {msg}", file=sys.stderr)
    sys.exit(1)

digest_re = re.compile(r"^sha256:[0-9a-f]{64}$")
target_digest = m.get("images", {}).get("backend", {}).get("digest", "")
if not digest_re.fullmatch(target_digest):
    fail("target manifest backend digest must be a sha256:64-hex immutable digest")

if not os.path.isfile(history_path):
    fail("history file is required for rollback (no recorded digests)")

try:
    history = json.load(open(history_path, encoding="utf-8"))
except (OSError, ValueError) as e:
    fail(f"history file is not valid JSON: {e}")
if not isinstance(history, list):
    fail("history file must be a JSON array")

known = {entry.get("digest") for entry in history}
if target_digest not in known:
    fail(f"rollback refused: digest {target_digest} is not recorded in deployment history")

if not history:
    fail("rollback refused: empty deployment history")

current = history[0]
db_head = current.get("migration_head", "")
target_max = m.get("schema_compat", {}).get("max_head", "")
if not db_head or not target_max:
    fail("history and target manifest must declare migration heads for schema compatibility")
if db_head > target_max:
    fail(f"rollback refused: target schema_compat.max_head {target_max} is older than the deployed migration head {db_head}")

record = {
    "schema_version": "1.0",
    "type": "rollback",
    "target_digest": target_digest,
    "target_commit": m.get("commit", ""),
    "from_digest": current.get("digest", ""),
    "migration_head": db_head,
    "schema_compat_max_head": target_max,
    "rolled_back_at": m.get("deployed_at", ""),
    "drill": drill,
}
with open(os.path.join(report_dir, "rollback-manifest.json"), "w", encoding="utf-8") as f:
    json.dump(record, f, indent=2)
    f.write("\n")
print(f"rollback: target digest {target_digest} validated; rollback manifest written to {report_dir}")
PY

# Forward-only migrations are never rolled back; only the application digest
# changes. There is intentionally no down-migration path in this script.
record_rollback_history() {
  [ -n "$HISTORY_FILE" ] || return 0
  python3 - "$HISTORY_FILE" "$MANIFEST" <<'PY'
import json
import sys

history_path, manifest_path = sys.argv[1:3]
with open(history_path, encoding="utf-8") as f:
    history = json.load(f)
with open(manifest_path, encoding="utf-8") as f:
    manifest = json.load(f)
target = manifest["images"]["backend"]["digest"]
entry = {
    "digest": target,
    "commit": manifest.get("commit", ""),
    "migration_head": manifest["migration"]["head"],
    "schema_compat_max_head": manifest["schema_compat"]["max_head"],
}
history = [item for item in history if item.get("digest") != target]
history.insert(0, entry)
with open(history_path, "w", encoding="utf-8") as f:
    json.dump(history, f, indent=2)
    f.write("\n")
PY
}

if [ "$DRILL" -eq 1 ]; then
  record_rollback_history
  echo "rollback: drill mode - compose side effects skipped"
else
  RELEASE_ENV_FILE="$ENV_FILE"
  RELEASE_COMPOSE_FILE="$COMPOSE_FILE"
  RELEASE_OVERRIDE_FILE="$REPORT_DIR/compose-images.yml"
  RELEASE_REPORT_DIR="$REPORT_DIR"
  RELEASE_DOCKER_BIN="${OMNICRAFT_DOCKER_BIN:-docker}"
  command -v "$RELEASE_DOCKER_BIN" >/dev/null 2>&1 || { echo "rollback: Docker is required outside -Drill" >&2; exit 1; }
  command -v curl >/dev/null 2>&1 || { echo "rollback: curl is required outside -Drill" >&2; exit 1; }
  source "$SCRIPT_DIR/compose-release.sh"
  # Do not run the target's migration image during rollback. Schema safety was
  # checked above; only application containers are switched back.
  render_release_override "$MANIFEST" "$RELEASE_OVERRIDE_FILE" 0
  release_compose config --quiet
  rollback_release_images
  : > "$REPORT_DIR/readiness.log"
  wait_for_release_health
  run_release_smoke
  python3 - "$REPORT_DIR/rollback-manifest.json" <<'PY'
import datetime
import json
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as f:
    record = json.load(f)
record["rolled_back_at"] = datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z")
record["readiness"] = {"status": "ok", "log": "readiness.log"}
record["smoke"] = {"status": "ok", "response": "smoke-response.txt"}
with open(path, "w", encoding="utf-8") as f:
    json.dump(record, f, indent=2)
    f.write("\n")
PY
  record_rollback_history
  echo "rollback: real compose image switch, readiness and smoke completed"
fi

echo "rollback: done"
