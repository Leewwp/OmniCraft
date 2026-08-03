#!/usr/bin/env bash
# Validates an Ops evidence summary against release/ops-evidence.schema.json.
# Usage: bash scripts/ops/validate-evidence.sh -Schema <path> -Summary <path> [-ExpectedCommit <sha>]
set -u

SCHEMA=""
SUMMARY=""
EXPECTED_COMMIT=""

while [ $# -gt 0 ]; do
  case "$1" in
    -Schema)
      if [ $# -lt 2 ]; then
        echo "missing value for -Schema" >&2
        exit 2
      fi
      shift
      SCHEMA="$1"
      ;;
    -Summary)
      if [ $# -lt 2 ]; then
        echo "missing value for -Summary" >&2
        exit 2
      fi
      shift
      SUMMARY="$1"
      ;;
    -ExpectedCommit)
      if [ $# -lt 2 ]; then
        echo "missing value for -ExpectedCommit" >&2
        exit 2
      fi
      shift
      EXPECTED_COMMIT="$1"
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

if [ -z "$SCHEMA" ] || [ -z "$SUMMARY" ]; then
  echo "usage: validate-evidence.sh -Schema <path> -Summary <path> [-ExpectedCommit <sha>]" >&2
  exit 2
fi

if [ ! -f "$SCHEMA" ]; then
  echo "schema file not found: $SCHEMA" >&2
  exit 1
fi

if [ ! -f "$SUMMARY" ]; then
  echo "summary file not found: $SUMMARY" >&2
  exit 1
fi

export OMNICRAFT_EVIDENCE_SCHEMA="$SCHEMA"
export OMNICRAFT_EVIDENCE_SUMMARY="$SUMMARY"
export OMNICRAFT_EVIDENCE_EXPECTED_COMMIT="$EXPECTED_COMMIT"

python3 - <<'PY'
import json
import os
import re
import sys

schema_path = os.environ["OMNICRAFT_EVIDENCE_SCHEMA"]
summary_path = os.environ["OMNICRAFT_EVIDENCE_SUMMARY"]
expected_commit = os.environ.get("OMNICRAFT_EVIDENCE_EXPECTED_COMMIT", "")


def fail(message):
    print(f"invalid ops evidence: {message}", file=sys.stderr)
    sys.exit(1)


try:
    with open(schema_path, encoding="utf-8") as f:
        schema = json.load(f)
except (OSError, ValueError) as e:
    fail(f"schema is not valid JSON: {e}")

if not isinstance(schema, dict) or "required" not in schema or "properties" not in schema:
    fail("schema must be an object with 'required' and 'properties'")

try:
    with open(summary_path, encoding="utf-8") as f:
        summary = json.load(f)
except (OSError, ValueError) as e:
    fail(f"summary is not valid JSON: {e}")

if not isinstance(summary, dict):
    fail("summary must be a JSON object")

required = schema["required"]
missing = [field for field in required if field not in summary]
if missing:
    fail(f"missing required fields: {', '.join(missing)}")

properties = schema["properties"]


def type_ok(value, types):
    if isinstance(types, str):
        types = [types]
    for t in types:
        if t == "string" and isinstance(value, str):
            return True
        if t == "array" and isinstance(value, list):
            return True
        if t == "object" and isinstance(value, dict):
            return True
        if t == "boolean" and isinstance(value, bool):
            return True
        if t == "integer" and isinstance(value, int) and not isinstance(value, bool):
            return True
        if t == "null" and value is None:
            return True
    return False


for field, value in summary.items():
    prop = properties.get(field)
    if prop is None:
        continue
    if "type" in prop and not type_ok(value, prop["type"]):
        fail(f"field '{field}' has an unexpected type")
    if isinstance(value, list) and "items" in prop:
        for item in value:
            if not type_ok(item, prop["items"].get("type")):
                fail(f"field '{field}' contains an entry with an unexpected type")

if len(summary["commands"]) != len(summary["exit_codes"]):
    fail("commands and exit_codes must pair one-to-one")

if expected_commit:
    if not re.fullmatch(r"[0-9a-f]{40}", expected_commit):
        fail(f"-ExpectedCommit is not a 40-hex sha: {expected_commit}")
    if summary["commit"] != expected_commit:
        fail(f"commit mismatch: expected {expected_commit}, got {summary['commit']}")

print(f"ops evidence summary valid: {summary_path}")
PY
