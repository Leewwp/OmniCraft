#!/usr/bin/env bash
# Contract tests for scripts/ops/finalize-evidence.sh.
# Verifies commit binding, evidence inventory hashing, immutability of run fields,
# refusal on dirty/non-HEAD state (unless fixture mode), and idempotency.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FINALIZER="$SCRIPT_DIR/finalize-evidence.sh"
if [ ! -f "$FINALIZER" ]; then
  echo "finalize-evidence.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-evidence-finalize.XXXXXX")"
REPO="$TEMP_ROOT/repo"
SUMMARY="$REPO/summary.json"
trap 'rm -rf "$TEMP_ROOT"' EXIT

mkdir -p "$REPO/artifacts/ops-01"
cd "$REPO" || exit 1
git init -q .
git config user.email "test@example.com"
git config user.name "Test Agent"
echo "base" > tracked.txt
git add tracked.txt
git commit -qm "base"
FIRST_COMMIT="$(git rev-parse HEAD)"
git commit -qm "second" --allow-empty
SECOND_COMMIT="$(git rev-parse HEAD)"
git reset -q --hard "$FIRST_COMMIT"

echo "go test log content" > artifacts/ops-01/backend-go-test.log
echo "go vet log content" > artifacts/ops-01/backend-go-vet.log

write_summary() {
  python3 - "$SUMMARY" <<'PY'
import json, sys
summary = {
    "task": "ops-01",
    "commit": None,
    "started_at": "2026-08-03T00:00:00Z",
    "finished_at": "2026-08-03T00:01:00Z",
    "commands": ["backend|go test ./...", "backend|go vet ./..."],
    "exit_codes": [0, 0],
    "tool_versions": {"go": "go1.25.12 darwin/arm64"},
    "evidence": ["artifacts/ops-01/backend-go-test.log", "artifacts/ops-01/backend-go-vet.log"],
    "redaction_checked": False,
    "blockers": [],
}
with open(sys.argv[1], "w", encoding="utf-8") as out:
    json.dump(summary, out, indent=2)
PY
}

expect_fail() {
  local message="$1"
  shift
  if bash "$FINALIZER" "$@"; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

expect_fail "missing -Summary must be rejected" -Commit "$FIRST_COMMIT"
expect_fail "missing -Commit must be rejected" -Summary "$SUMMARY"
expect_fail "missing summary file must be rejected" -Summary "$TEMP_ROOT/nope.json" -Commit "$FIRST_COMMIT"
expect_fail "malformed commit must be rejected" -Summary "$SUMMARY" -Commit deadbeef

write_summary
bash "$FINALIZER" -Summary "$SUMMARY" -Commit "$FIRST_COMMIT" || {
  echo "FAIL: clean worktree finalization must succeed" >&2
  exit 1
}
python3 - "$SUMMARY" "$FIRST_COMMIT" <<'PY'
import hashlib, json, sys
path, expected_commit = sys.argv[1], sys.argv[2]
s = json.load(open(path, encoding="utf-8"))
assert s["commit"] == expected_commit, "commit must be bound to the given sha"
assert isinstance(s["evidence_sha256"], dict), "evidence hashes must be recorded"
for rel in s["evidence"]:
    want = hashlib.sha256(open(rel, "rb").read()).hexdigest()
    assert s["evidence_sha256"].get(rel) == want, f"hash mismatch for {rel}"
for field in ("task", "started_at", "finished_at", "commands", "exit_codes", "tool_versions", "evidence", "redaction_checked", "blockers"):
    assert field in s, f"finalization must preserve {field}"
PY
if [ $? -ne 0 ]; then
  echo "FAIL: finalize must bind commit and hash evidence" >&2
  exit 1
fi

cp "$SUMMARY" "$TEMP_ROOT/finalized-1.json"
bash "$FINALIZER" -Summary "$SUMMARY" -Commit "$FIRST_COMMIT" || {
  echo "FAIL: finalization must be idempotent and rerunnable" >&2
  exit 1
}
if ! diff -q "$TEMP_ROOT/finalized-1.json" "$SUMMARY" >/dev/null; then
  echo "FAIL: second finalization must produce identical output" >&2
  exit 1
fi

bash "$FINALIZER" -Summary "$SUMMARY" -Commit "$SECOND_COMMIT" && {
  echo "FAIL: non-HEAD commit must be refused" >&2
  exit 1
}

echo "tracked mutation" > tracked.txt
write_summary
expect_fail "dirty worktree must be refused" -Summary "$SUMMARY" -Commit "$FIRST_COMMIT"
bash "$FINALIZER" -Fixture -Summary "$SUMMARY" -Commit "$FIRST_COMMIT" || {
  echo "FAIL: fixture mode must bypass git state checks" >&2
  exit 1
}
git checkout -q -- tracked.txt

write_summary
rm -f artifacts/ops-01/backend-go-vet.log
expect_fail "missing evidence file must be refused" -Summary "$SUMMARY" -Commit "$FIRST_COMMIT"
git checkout -q -- tracked.txt

write_summary
python3 - "$SUMMARY" <<'PY'
import json, sys
path = sys.argv[1]
s = json.load(open(path, encoding="utf-8"))
s["commands"] = ["backend|go test ./..."]
s["exit_codes"] = [0, 0]
with open(path, "w", encoding="utf-8") as out:
    json.dump(s, out, indent=2)
PY
expect_fail "malformed summary must be refused" -Summary "$SUMMARY" -Commit "$FIRST_COMMIT"

echo "go test log content" > artifacts/ops-01/backend-go-test.log
echo "go vet log content" > artifacts/ops-01/backend-go-vet.log
write_summary
bash "$FINALIZER" -Fixture -RedactionChecked true -Blocker "missing real OSS" -Blocker "pending sign-off" -Summary "$SUMMARY" -Commit "$FIRST_COMMIT" || {
  echo "FAIL: finalize must accept attestation flags in fixture mode" >&2
  exit 1
}
python3 - "$SUMMARY" <<'PY'
import json, sys
s = json.load(open(sys.argv[1], encoding="utf-8"))
assert s["redaction_checked"] is True
assert s["blockers"] == ["missing real OSS", "pending sign-off"]
PY
if [ $? -ne 0 ]; then
  echo "FAIL: redaction/blocker attestations must be written" >&2
  exit 1
fi

echo "finalize-evidence contract tests passed"
