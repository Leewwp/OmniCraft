#!/usr/bin/env bash
# Contract tests for scripts/db/verify-backup-policy.sh: the committed policy
# files must pass, and each mutated copy must fail with its intended violation.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VERIFIER="$SCRIPT_DIR/verify-backup-policy.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
POLICY_SRC="$REPO_ROOT/release/backup-policy.json"
OBJECTIVES_SRC="$REPO_ROOT/release/recovery-objectives.json"

if [ ! -f "$VERIFIER" ]; then
  echo "verify-backup-policy.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-backup-policy.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

expect_ok() {
  local message="$1"
  shift
  if ! bash "$VERIFIER" "$@" -ReportDir "$TEMP_ROOT/ok" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

expect_fail() {
  local message="$1"
  shift
  if bash "$VERIFIER" "$@" -ReportDir "$TEMP_ROOT/fail" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

mutate_policy() {
  python3 - "$TEMP_ROOT/policy.json" "$POLICY_SRC" "$1" <<'PY'
import json
import sys

path, src, spec = sys.argv[1], sys.argv[2], sys.argv[3]
key, subkey, value = spec.split("||", 2)
with open(src, encoding="utf-8") as f:
    policy = json.load(f)
if subkey == "":
    policy[key] = value
else:
    policy[key][subkey] = value
with open(path, "w", encoding="utf-8") as f:
    json.dump(policy, f)
PY
}

mutate_objectives() {
  python3 - "$TEMP_ROOT/objectives.json" "$OBJECTIVES_SRC" "$1" <<'PY'
import json
import sys

path, src, spec = sys.argv[1], sys.argv[2], sys.argv[3]
key, subkey, value = spec.split("||", 2)
with open(src, encoding="utf-8") as f:
    objectives = json.load(f)
if key == "state":
    objectives["state"] = value
elif key == "delete-measured":
    del objectives["measured"]
elif key == "approved":
    objectives["state"] = "approved"
    objectives["approved_targets"] = {"postgres_rpo": 60, "postgres_rto": 120}
    objectives["approval"] = {}
elif subkey == "":
    objectives[key] = value
else:
    objectives[key][subkey] = value
with open(path, "w", encoding="utf-8") as f:
    json.dump(objectives, f)
PY
}

expect_ok "committed backup policy and objectives must validate" \
  -Policy "$POLICY_SRC" -Objectives "$OBJECTIVES_SRC"

mutate_policy "postgres_full||frequency||weekly"
expect_fail "non-daily backups must be rejected" -Policy "$TEMP_ROOT/policy.json" -Objectives "$OBJECTIVES_SRC"

mutate_policy "postgres_full||pre_migration||false"
expect_fail "missing pre-migration backups must be rejected" -Policy "$TEMP_ROOT/policy.json" -Objectives "$OBJECTIVES_SRC"

mutate_policy "postgres_full||format||plain"
expect_fail "plain-format dumps must be rejected" -Policy "$TEMP_ROOT/policy.json" -Objectives "$OBJECTIVES_SRC"

mutate_policy "local_retention||copies||6"
expect_fail "local retention other than 7 copies must be rejected" -Policy "$TEMP_ROOT/policy.json" -Objectives "$OBJECTIVES_SRC"

mutate_policy "off_host||immutable_or_versioned||false"
expect_fail "off-host without immutability/versioning must be rejected" -Policy "$TEMP_ROOT/policy.json" -Objectives "$OBJECTIVES_SRC"

mutate_policy "off_host||retention_days||14"
expect_fail "off-host retention shorter than 30 days must be rejected" -Policy "$TEMP_ROOT/policy.json" -Objectives "$OBJECTIVES_SRC"

mutate_policy "restore_drill||cadence||quarterly"
expect_fail "non-monthly restore cadence must be rejected" -Policy "$TEMP_ROOT/policy.json" -Objectives "$OBJECTIVES_SRC"

mutate_policy "restore_order||""||""redis"
python3 - "$TEMP_ROOT/policy.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    policy = json.load(f)
policy["restore_order"] = ["redis", "postgresql", "oss"]
with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump(policy, f)
PY
expect_fail "restore order that does not start with postgresql must be rejected" -Policy "$TEMP_ROOT/policy.json" -Objectives "$OBJECTIVES_SRC"

mutate_objectives "delete-measured||""||"
expect_fail "recovery objectives without measurements must be rejected" -Policy "$POLICY_SRC" -Objectives "$TEMP_ROOT/objectives.json"

mutate_objectives "approved||""||"
expect_fail "approved objectives without an approval reference must be rejected" -Policy "$POLICY_SRC" -Objectives "$TEMP_ROOT/objectives.json"

mutate_objectives "state||""||baseline_only"
expect_ok "baseline_only objectives remain valid" -Policy "$POLICY_SRC" -Objectives "$TEMP_ROOT/objectives.json"

echo "verify-backup-policy contract tests passed"
