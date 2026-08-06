#!/usr/bin/env bash
# Contract tests for scripts/release/archive-release-evidence.sh: the local
# adapter proving the archive contract (GitHub Release asset destination plus
# encrypted operator off-site destination with one-year retention metadata).
# Real off-site credentials remain an Ops-08 release blocker; the adapter is
# proven with deterministic local fixtures.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ARCHIVE="$SCRIPT_DIR/archive-release-evidence.sh"

if [ ! -f "$ARCHIVE" ]; then
  echo "archive-release-evidence.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-archive.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

make_fixture() {
  local dir="$1"
  mkdir -p "$dir/sboms" "$dir/artifacts"
  printf 'fixture sbom content\n' > "$dir/sboms/backend-go.cdx.json"
  printf 'fixture container payload\n' > "$dir/artifacts/backend-image.tar"
  printf 'fixture migration ledger\n' > "$dir/migration-manifest.json"
  python3 - "$dir" <<'PY'
import hashlib, json, os, sys

out = sys.argv[1]
sha = {}
for name in ("sboms/backend-go.cdx.json", "artifacts/backend-image.tar", "migration-manifest.json"):
    with open(os.path.join(out, name), "rb") as f:
        sha[name] = hashlib.sha256(f.read()).hexdigest()
manifest = {
    "schema_version": "1.0",
    "commit": "2222222222222222222222222222222222222222",
    "version": "0.1.0-fixture",
    "components": [{"id": "backend-go", "ecosystem": "go", "sbom_path": "sboms/backend-go.cdx.json", "sbom_sha256": sha["sboms/backend-go.cdx.json"]}],
    "artifacts": [{"name": "backend-image", "type": "container-image", "path": "artifacts/backend-image.tar", "sha256": sha["artifacts/backend-image.tar"], "digest": "sha256:" + sha["artifacts/backend-image.tar"]}],
    "migration_manifest": {"path": "migration-manifest.json", "sha256": sha["migration-manifest.json"], "count": 1},
}
with open(os.path.join(out, "release-manifest.json"), "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")
PY
}

expect_archive() {
  local expected="$1" label="$2"
  shift 2
  local actual=0
  bash "$ARCHIVE" "$@" >"$TEMP_ROOT/$label.out" 2>"$TEMP_ROOT/$label.err" || actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL: $label: expected exit $expected, got $actual" >&2
    cat "$TEMP_ROOT/$label.err" >&2
    exit 1
  fi
  echo "OK: $label"
}

# ------------------------------------------------------------ usage errors
rc=0
bash "$ARCHIVE" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || { echo "FAIL: missing args must exit 2" >&2; exit 1; }
rc=0
bash "$ARCHIVE" -Manifest "$TEMP_ROOT/does-not-exist.json" -TargetDir "$TEMP_ROOT/dest" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 1 ] || { echo "FAIL: missing manifest must exit 1" >&2; exit 1; }
echo "OK: usage errors"

# ---------------------------------------------------------------- happy path
FIXTURE="$TEMP_ROOT/fixture"
make_fixture "$FIXTURE"
DEST="$TEMP_ROOT/dest"
expect_archive 0 "archive happy path" -Manifest "$FIXTURE/release-manifest.json" -TargetDir "$DEST" -ReportDir "$FIXTURE"
python3 - "$DEST/archive-receipt.json" <<'PY'
import datetime, hashlib, json, os, sys

receipt = json.load(open(sys.argv[1], encoding="utf-8"))

def fail(msg):
    print("FAIL: " + msg, file=sys.stderr)
    sys.exit(1)

for field in ("schema_version", "source_commit", "created_at", "retention_days",
              "expires_at", "destinations", "redaction_checked", "blockers"):
    if field not in receipt:
        fail("receipt missing field " + field)
created = datetime.datetime.fromisoformat(receipt["created_at"].replace("Z", "+00:00"))
expires = datetime.datetime.fromisoformat(receipt["expires_at"].replace("Z", "+00:00"))
delta = (expires - created).days
if receipt["retention_days"] != 365 or delta != 365:
    fail("one-year retention not honored (retention %s, delta %d)" % (receipt["retention_days"], delta))
types = sorted(d["type"] for d in receipt["destinations"])
if types != ["github-release", "offsite-encrypted"]:
    fail("destinations must be github-release and offsite-encrypted, got %s" % types)
if not any("Ops-08" in b for b in receipt["blockers"]):
    fail("real off-site destination must be recorded as an Ops-08 blocker")
objects = {}
for dest in receipt["destinations"]:
    for obj in dest["objects"]:
        objects[obj["name"]] = (obj["sha256"], obj["size"])
for name in ("sboms/backend-go.cdx.json", "artifacts/backend-image.tar", "release-manifest.json", "migration-manifest.json"):
    if name not in objects:
        fail("archive missing object " + name)
    # recompute from the destination copy itself (simulated download)
    dest_path = None
    for dest in receipt["destinations"]:
        for obj in dest["objects"]:
            if obj["name"] == name:
                dest_path = os.path.join(dest["base"], name)
    if dest_path is None or not os.path.exists(dest_path):
        fail("archived file not present at destination: " + name)
    sha = hashlib.sha256(open(dest_path, "rb").read()).hexdigest()
    if sha != objects[name][0]:
        fail("sha256 mismatch for " + name)
    if os.path.getsize(dest_path) != objects[name][1]:
        fail("size mismatch for " + name)
# the fixture manifest has no migration-manifest field: archive must still copy
# the manifest itself; migration object only exists when referenced
print("archive receipt assertions passed")
PY

# ---------------------------------------------------- missing source file
FIXTURE2="$TEMP_ROOT/fixture2"
make_fixture "$FIXTURE2"
rm "$FIXTURE2/sboms/backend-go.cdx.json"
expect_archive 1 "missing source file fails archive" -Manifest "$FIXTURE2/release-manifest.json" -TargetDir "$TEMP_ROOT/dest2" -ReportDir "$FIXTURE2"

# -------------------------------------------------------- retention override
FIXTURE3="$TEMP_ROOT/fixture3"
make_fixture "$FIXTURE3"
expect_archive 0 "retention override" -Manifest "$FIXTURE3/release-manifest.json" -TargetDir "$TEMP_ROOT/dest3" -ReportDir "$FIXTURE3" -RetentionDays 30
python3 - "$TEMP_ROOT/dest3/archive-receipt.json" <<'PY'
import datetime, json, sys
receipt = json.load(open(sys.argv[1], encoding="utf-8"))
created = datetime.datetime.fromisoformat(receipt["created_at"].replace("Z", "+00:00"))
expires = datetime.datetime.fromisoformat(receipt["expires_at"].replace("Z", "+00:00"))
if (expires - created).days != 30:
    print("FAIL: -RetentionDays 30 not honored", file=sys.stderr)
    sys.exit(1)
print("retention override honored")
PY

# -------------------------------------------------------------- redaction
FIXTURE4="$TEMP_ROOT/fixture4"
make_fixture "$FIXTURE4"
expect_archive 0 "redaction fixture" -Manifest "$FIXTURE4/release-manifest.json" -TargetDir "$TEMP_ROOT/dest4" -ReportDir "$FIXTURE4"
if grep -E "AKIA[0-9A-Z]{16}|password|secret|ACCESS_KEY|access_key" "$TEMP_ROOT/dest4/archive-receipt.json" >/dev/null; then
  echo "FAIL: receipt must not contain secret-like content" >&2
  exit 1
fi
echo "OK: receipt is free of secret-like content"

echo "All archive-release-evidence contract tests passed"
