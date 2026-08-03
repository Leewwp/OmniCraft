#!/usr/bin/env bash
# Binds an Ops evidence summary to its reviewed commit and hashes its evidence
# inventory. Refuses a dirty or non-HEAD worktree unless -Fixture is given.
# Usage: bash scripts/ops/finalize-evidence.sh -Summary <path> -Commit <sha> [-Fixture] [-RedactionChecked true|false] [-Blocker <text>]...
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SUMMARY=""
COMMIT=""
FIXTURE=0
REDACTION_CHECKED=""
BLOCKERS=()

while [ $# -gt 0 ]; do
  case "$1" in
    -Summary)
      if [ $# -lt 2 ]; then
        echo "missing value for -Summary" >&2
        exit 2
      fi
      shift
      SUMMARY="$1"
      ;;
    -Commit)
      if [ $# -lt 2 ]; then
        echo "missing value for -Commit" >&2
        exit 2
      fi
      shift
      COMMIT="$1"
      ;;
    -Fixture)
      FIXTURE=1
      ;;
    -RedactionChecked)
      if [ $# -lt 2 ]; then
        echo "missing value for -RedactionChecked" >&2
        exit 2
      fi
      shift
      REDACTION_CHECKED="$1"
      ;;
    -Blocker)
      if [ $# -lt 2 ]; then
        echo "missing value for -Blocker" >&2
        exit 2
      fi
      shift
      BLOCKERS+=("$1")
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

if [ -z "$SUMMARY" ] || [ -z "$COMMIT" ]; then
  echo "usage: finalize-evidence.sh -Summary <path> -Commit <sha> [-Fixture] [-RedactionChecked true|false] [-Blocker <text>]..." >&2
  exit 2
fi

if ! printf '%s' "$COMMIT" | grep -Eq '^[0-9a-f]{40}$'; then
  echo "invalid commit sha: $COMMIT (expected 40 hex characters)" >&2
  exit 2
fi

if [ "$FIXTURE" -eq 0 ]; then
  HEAD_COMMIT="$(git rev-parse HEAD 2>/dev/null)"
  if [ -z "$HEAD_COMMIT" ]; then
    echo "not inside a git repository; refusing to finalize without -Fixture" >&2
    exit 1
  fi
  if [ "$HEAD_COMMIT" != "$COMMIT" ]; then
    echo "commit $COMMIT is not HEAD ($HEAD_COMMIT); refusing to finalize" >&2
    exit 1
  fi
  if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    echo "working tree has uncommitted tracked changes; refusing to finalize (use -Fixture to bypass)" >&2
    exit 1
  fi
fi

if [ -n "$REDACTION_CHECKED" ] && [ "$REDACTION_CHECKED" != "true" ] && [ "$REDACTION_CHECKED" != "false" ]; then
  echo "invalid -RedactionChecked value: $REDACTION_CHECKED (expected true|false)" >&2
  exit 2
fi

if [ ! -f "$SUMMARY" ]; then
  echo "summary file not found: $SUMMARY" >&2
  exit 1
fi

export OMNICRAFT_EVIDENCE_SUMMARY="$SUMMARY"
export OMNICRAFT_EVIDENCE_COMMIT="$COMMIT"
export OMNICRAFT_EVIDENCE_REDACTION_CHECKED="$REDACTION_CHECKED"
export OMNICRAFT_EVIDENCE_BLOCKERS="$(IFS=$'\n'; printf '%s' "${BLOCKERS[*]:-}")"

python3 - <<'PY'
import hashlib
import json
import os
import sys

summary_path = os.environ["OMNICRAFT_EVIDENCE_SUMMARY"]
commit = os.environ["OMNICRAFT_EVIDENCE_COMMIT"]
redaction_checked = os.environ.get("OMNICRAFT_EVIDENCE_REDACTION_CHECKED", "")
blockers_env = os.environ.get("OMNICRAFT_EVIDENCE_BLOCKERS", "")

with open(summary_path, encoding="utf-8") as f:
    summary = json.load(f)

if not isinstance(summary, dict):
    print("invalid ops evidence: summary must be a JSON object", file=sys.stderr)
    sys.exit(1)

required = ["task", "commit", "started_at", "finished_at", "commands",
            "exit_codes", "tool_versions", "evidence", "redaction_checked", "blockers"]
missing = [field for field in required if field not in summary]
if missing:
    print(f"invalid ops evidence: missing required fields: {', '.join(missing)}", file=sys.stderr)
    sys.exit(1)

if not isinstance(summary["commands"], list) or not isinstance(summary["exit_codes"], list):
    print("invalid ops evidence: commands and exit_codes must be arrays", file=sys.stderr)
    sys.exit(1)
if len(summary["commands"]) != len(summary["exit_codes"]):
    print("invalid ops evidence: commands and exit_codes must pair one-to-one", file=sys.stderr)
    sys.exit(1)
if not isinstance(summary["evidence"], list):
    print("invalid ops evidence: evidence must be an array", file=sys.stderr)
    sys.exit(1)

hashes = {}
for entry in summary["evidence"]:
    if not os.path.exists(entry):
        print(f"invalid ops evidence: referenced evidence file not found: {entry}", file=sys.stderr)
        sys.exit(1)
    with open(entry, "rb") as f:
        hashes[entry] = hashlib.sha256(f.read()).hexdigest()

summary["commit"] = commit
summary["evidence_sha256"] = hashes
if redaction_checked == "true":
    summary["redaction_checked"] = True
elif redaction_checked == "false":
    summary["redaction_checked"] = False
if blockers_env:
    summary["blockers"] = blockers_env.split("\n")

with open(summary_path, "w", encoding="utf-8") as out:
    json.dump(summary, out, ensure_ascii=False, indent=2)
    out.write("\n")

print(f"finalized ops evidence: {summary_path}")
PY
