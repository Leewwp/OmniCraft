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
rc=0
bash "$ARCHIVE" -Manifest "$FIXTURE/release-manifest.json" -TargetDir "$TEMP_ROOT/dest" -RetentionDays 0 >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || { echo "FAIL: zero retention must exit 2" >&2; exit 1; }
echo "OK: non-positive retention rejected"
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

# -------------------------------------------------------- deployment evidence
# Ops-08 passes a deployment manifest, not the Ops-06 SBOM manifest. The
# archiver must retain the complete evidence directory in that mode.
DEPLOY_FIXTURE="$TEMP_ROOT/deployment-fixture"
mkdir -p "$DEPLOY_FIXTURE"
printf 'preflight evidence\n' > "$DEPLOY_FIXTURE/preflight-summary.json"
printf 'backup evidence\n' > "$DEPLOY_FIXTURE/backup-manifest.json"
printf 'readiness evidence\n' > "$DEPLOY_FIXTURE/readiness.log"
printf 'smoke evidence\n' > "$DEPLOY_FIXTURE/smoke-response.txt"
python3 - "$DEPLOY_FIXTURE/deployment-manifest.json" <<'PY'
import json, sys
with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump({
        "schema_version": "1.0",
        "commit": "3333333333333333333333333333333333333333",
        "images": {},
        "preflight": {"status": "ok"},
        "backup": {"status": "ok"},
        "readiness": {"status": "ok"},
        "smoke": {"status": "ok"},
    }, f)
PY
expect_archive 0 "deployment evidence archive" -Manifest "$DEPLOY_FIXTURE/deployment-manifest.json" -TargetDir "$TEMP_ROOT/deployment-dest" -ReportDir "$DEPLOY_FIXTURE"
python3 - "$TEMP_ROOT/deployment-dest/archive-receipt.json" <<'PY'
import json, sys
receipt = json.load(open(sys.argv[1], encoding="utf-8"))
names = {obj["name"] for dest in receipt["destinations"] for obj in dest["objects"]}
required = {"deployment-manifest.json", "preflight-summary.json", "backup-manifest.json", "readiness.log", "smoke-response.txt"}
if not required.issubset(names):
    raise SystemExit(f"deployment evidence archive missing {sorted(required - names)}")
print("deployment evidence archive assertions passed")
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

# ------------------------------------------------- real destination validation
FIXTURE5="$TEMP_ROOT/fixture5"
make_fixture "$FIXTURE5"
rc=0
bash "$ARCHIVE" -Manifest "$FIXTURE5/release-manifest.json" -TargetDir "$TEMP_ROOT/dest5" -GitHubRelease "CHANGE_ME" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 1 ] || { echo "FAIL: placeholder -GitHubRelease must be rejected" >&2; exit 1; }
echo "OK: placeholder -GitHubRelease rejected"

rc=0
bash "$ARCHIVE" -Manifest "$FIXTURE5/release-manifest.json" -GitHubRelease "v0.1.0-fixture" -OffsiteUri "not-an-oss-uri" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 1 ] || { echo "FAIL: malformed -OffsiteUri must be rejected" >&2; exit 1; }
echo "OK: malformed -OffsiteUri rejected"

rc=0
bash "$ARCHIVE" -Manifest "$FIXTURE5/release-manifest.json" -TargetDir "$TEMP_ROOT/dest5" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 0 ] || { echo "FAIL: -TargetDir only mode must still work" >&2; exit 1; }
echo "OK: -TargetDir only mode retained"

rc=0
bash "$ARCHIVE" -Manifest "$FIXTURE5/release-manifest.json" -GitHubRelease "v0.1.0-fixture" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 1 ] || { echo "FAIL: real mode must require both durable destinations" >&2; exit 1; }
echo "OK: real mode requires both durable destinations"

# ------------------------------- real destinations with fake gh/ossutil CLIs
FAKE_BIN="$TEMP_ROOT/fake-bin"
mkdir -p "$FAKE_BIN"
cat > "$FAKE_BIN/gh" <<'EOF'
#!/usr/bin/env bash
# fake gh: records invocation; release view returns one asset per upload
if [ "$1" = "release" ] && [ "$2" = "upload" ]; then
  echo "$@" >> "$GH_RECORD"
  exit 0
fi
if [ "$1" = "release" ] && [ "$2" = "view" ]; then
  python3 - "$GH_RECORD" <<'PY'
import json
import os
import sys

names = []
for line in open(sys.argv[1], encoding="utf-8"):
    fields = line.split()
    if len(fields) >= 4:
        upload = fields[3]
        names.append(upload.split("#", 1)[1] if "#" in upload else os.path.basename(upload))
print(json.dumps({"assets": [{"name": name, "url": "https://example.invalid/" + name,
                              "size": 10, "state": "uploaded"} for name in names]}))
PY
  exit 0
fi
echo "fake gh: unexpected args: $*" >&2
exit 1
EOF
cat > "$FAKE_BIN/ossutil" <<'EOF'
#!/usr/bin/env bash
# fake ossutil: records invocation; stat returns verified retention metadata
if [ "$1" = "cp" ]; then
  echo "$@" >> "$OSSUTIL_RECORD"
  exit 0
fi
if [ "$1" = "stat" ] || [ "$1" = "ls" ]; then
  echo "$@" >> "$OSSUTIL_RECORD"
  if [ "$1" = "ls" ]; then
    echo "2026-08-07 00:00:00 +0800 CST  10  IA  ETAGMOCK  $2"
  else
    cat <<'OUT'
object size: 10
x-oss-meta-retention-days: 365
x-oss-meta-expires-at: 2027-08-07T00:00:00Z
x-oss-meta-archive-commit: 2222222222222222222222222222222222222222
x-oss-server-side-encryption: AES256
OUT
  fi
  exit 0
fi
echo "fake ossutil: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$FAKE_BIN/gh" "$FAKE_BIN/ossutil"

FIXTURE6="$TEMP_ROOT/fixture6"
make_fixture "$FIXTURE6"
GH_RECORD="$TEMP_ROOT/gh.calls"
OSSUTIL_RECORD="$TEMP_ROOT/ossutil.calls"
PATH="$FAKE_BIN:$PATH" OFFSITE_ARCHIVE_AK_ID="LTAI5tFIXTURE0000000000" \
  OFFSITE_ARCHIVE_AK_SECRET="fixture-secret-not-committed" \
  OFFSITE_ARCHIVE_ENDPOINT="oss-cn-hangzhou.aliyuncs.com" \
  GH_RECORD="$GH_RECORD" OSSUTIL_RECORD="$OSSUTIL_RECORD" \
  bash "$ARCHIVE" -Manifest "$FIXTURE6/release-manifest.json" \
  -GitHubRelease "v0.1.0-fixture" -OffsiteUri "oss://omnicraft-fixture/ops-evidence" \
  -ReportDir "$FIXTURE6" -RetentionDays 365
python3 - "$FIXTURE6/archive-receipt.json" "$GH_RECORD" "$OSSUTIL_RECORD" <<'PY'
import json, os, sys

receipt = json.load(open(sys.argv[1], encoding="utf-8"))
gh_calls = open(sys.argv[2], encoding="utf-8").read()
ossutil_calls = open(sys.argv[3], encoding="utf-8").read()

def fail(msg):
    print("FAIL: " + msg, file=sys.stderr)
    sys.exit(1)

if receipt["blockers"]:
    fail("real destination mode must not record simulated Ops-08 blockers: %s" % receipt["blockers"])
if receipt["retention_days"] != 365:
    fail("one-year retention not honored in real mode")
if receipt["redaction_checked"] is not True:
    fail("redaction_checked must confirm the receipt was scanned")
for dest in receipt["destinations"]:
    if dest["type"] == "github-release":
        if dest.get("tag") != "v0.1.0-fixture":
            fail("github-release destination must carry the tag")
        if not dest["objects"] or any("sha256" not in o for o in dest["objects"]):
            fail("github-release objects must carry digests")
    elif dest["type"] == "offsite-encrypted":
        if dest.get("uri") != "oss://omnicraft-fixture/ops-evidence":
            fail("offsite-encrypted destination must carry the uri")
        if dest.get("encryption", {}).get("simulated") is not False:
            fail("offsite encryption must be marked real (sse-oss)")
        if not dest["objects"] or any(not o.get("verified") for o in dest["objects"]):
            fail("offsite objects must be verified")
    elif dest["type"] != "local-mirror":
        fail("unexpected destination type " + dest["type"])
if "upload" not in gh_calls or "v0.1.0-fixture" not in gh_calls:
    fail("fake gh must have been invoked for release upload")
if "--meta" not in ossutil_calls or "AES256" not in ossutil_calls or "oss://omnicraft-fixture/ops-evidence" not in ossutil_calls:
    fail("fake ossutil must have been invoked with the off-site uri")
if "stat" not in ossutil_calls:
    fail("offsite objects must be metadata-verified via stat")
if "LTAI5tFIXTURE" in open(sys.argv[1], encoding="utf-8").read():
    fail("receipt must never contain credentials")
print("real destination mode assertions passed")
PY

# ------------------- real destinations refuse missing tooling on PATH
FIXTURE7="$TEMP_ROOT/fixture7"
make_fixture "$FIXTURE7"
EMPTY_BIN="$TEMP_ROOT/empty-bin"
mkdir -p "$EMPTY_BIN"
rc=0
PATH="$EMPTY_BIN:$PATH" OFFSITE_ARCHIVE_AK_ID="x" OFFSITE_ARCHIVE_AK_SECRET="y" \
  OFFSITE_ARCHIVE_ENDPOINT="oss-cn-hangzhou.aliyuncs.com" \
  bash "$ARCHIVE" -Manifest "$FIXTURE7/release-manifest.json" \
  -GitHubRelease "v0.1.0-fixture" -OffsiteUri "oss://omnicraft-fixture/ops" \
  -ReportDir "$FIXTURE7" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 1 ] || { echo "FAIL: missing gh/ossutil on PATH must fail" >&2; exit 1; }
echo "OK: missing tooling on PATH rejected"

echo "All archive-release-evidence contract tests passed"
