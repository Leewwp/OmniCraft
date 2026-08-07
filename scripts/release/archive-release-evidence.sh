#!/usr/bin/env bash
# =============================================================================
# OmniCraft release-evidence archiver: proves the archive contract — copies
# either an Ops-06 release manifest (SBOMs, artifacts and migration manifest)
# or an Ops-08 deployment manifest plus its complete evidence directory into
# durable destinations, then writes a machine-readable receipt with
# per-object digests and one-year (default) retention metadata.
#
# Two operation modes:
#   - Local adapter mode (Ops-06): -TargetDir copies into a local destination
#     with a simulated encrypted off-site sink and records the Ops-08
#     blocker; runs anywhere with deterministic fixtures.
#   - Real destination mode (Ops-08 Step 5): -GitHubRelease <tag> uploads
#     the objects as GitHub Release assets via `gh`; -OffsiteUri oss://.../
#     uploads the objects to the operator off-site Aliyun OSS bucket with
#     SSE-OSS AES256 encryption and retention metadata via `ossutil`. Credentials
#     come from the operator environment (source the off-site archive env
#     file first; ARCHIVE_AK_ID/ARCHIVE_AK_SECRET + OFFSITE_ARCHIVE_* are
#     accepted) and are never written to any receipt.
#
# Usage:
#   bash scripts/release/archive-release-evidence.sh -Manifest <path>
#       -TargetDir <dir> [-GitHubRelease <tag>] [-OffsiteUri oss://bucket/prefix]
#       [-ReportDir <dir>] [-RetentionDays <int>]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST=""
TARGET_DIR=""
REPORT_DIR=""
RETENTION_DAYS=365
GITHUB_RELEASE=""
OFFSITE_URI=""

while [ $# -gt 0 ]; do
  case "$1" in
    -Manifest)
      [ $# -ge 2 ] || { echo "missing value for -Manifest" >&2; exit 2; }
      MANIFEST="$2"; shift 2 ;;
    -TargetDir)
      [ $# -ge 2 ] || { echo "missing value for -TargetDir" >&2; exit 2; }
      TARGET_DIR="$2"; shift 2 ;;
    -GitHubRelease)
      [ $# -ge 2 ] || { echo "missing value for -GitHubRelease" >&2; exit 2; }
      GITHUB_RELEASE="$2"; shift 2 ;;
    -OffsiteUri)
      [ $# -ge 2 ] || { echo "missing value for -OffsiteUri" >&2; exit 2; }
      OFFSITE_URI="$2"; shift 2 ;;
    -ReportDir)
      [ $# -ge 2 ] || { echo "missing value for -ReportDir" >&2; exit 2; }
      REPORT_DIR="$2"; shift 2 ;;
    -RetentionDays)
      [ $# -ge 2 ] || { echo "missing value for -RetentionDays" >&2; exit 2; }
      RETENTION_DAYS="$2"; shift 2 ;;
    *)
      echo "unknown argument: $1" >&2
      echo "usage: archive-release-evidence.sh -Manifest <path> [-TargetDir <dir>] [-GitHubRelease <tag>] [-OffsiteUri oss://bucket/prefix] [-ReportDir <dir>] [-RetentionDays <int>]" >&2
      exit 2 ;;
  esac
done

if [ -z "$MANIFEST" ] || { [ -z "$TARGET_DIR" ] && [ -z "$GITHUB_RELEASE" ] && [ -z "$OFFSITE_URI" ]; }; then
  echo "usage: archive-release-evidence.sh -Manifest <path> [-TargetDir <dir>] [-GitHubRelease <tag>] [-OffsiteUri oss://bucket/prefix] [-ReportDir <dir>] [-RetentionDays <int>]" >&2
  exit 2
fi

[ -f "$MANIFEST" ] || { echo "manifest not found: $MANIFEST" >&2; exit 1; }
case "$RETENTION_DAYS" in
  ''|*[!0-9]*) echo "retention days must be a positive integer" >&2; exit 2 ;;
esac
[ "$RETENTION_DAYS" -gt 0 ] || { echo "retention days must be a positive integer" >&2; exit 2; }

MANIFEST="$(cd "$(dirname "$MANIFEST")" && pwd)/$(basename "$MANIFEST")"
if [ -n "$TARGET_DIR" ]; then
  TARGET_DIR="$(mkdir -p "$TARGET_DIR" && cd "$TARGET_DIR" && pwd)"
fi
if [ -z "$REPORT_DIR" ]; then
  REPORT_DIR="$(dirname "$MANIFEST")"
fi
mkdir -p "$REPORT_DIR"
REPORT_DIR="$(cd "$REPORT_DIR" && pwd)"

export OMNICRAFT_ARCHIVE_MANIFEST="$MANIFEST"
export OMNICRAFT_ARCHIVE_TARGET="${TARGET_DIR:-}"
export OMNICRAFT_ARCHIVE_REPORT="$REPORT_DIR"
export OMNICRAFT_ARCHIVE_RETENTION="$RETENTION_DAYS"
export OMNICRAFT_ARCHIVE_GITHUB_RELEASE="$GITHUB_RELEASE"
export OMNICRAFT_ARCHIVE_OFFSITE_URI="$OFFSITE_URI"
export OMNICRAFT_ARCHIVE_OFFSITE_AK_ID="${OFFSITE_ARCHIVE_AK_ID:-${ARCHIVE_AK_ID:-}}"
export OMNICRAFT_ARCHIVE_OFFSITE_AK_SECRET="${OFFSITE_ARCHIVE_AK_SECRET:-${ARCHIVE_AK_SECRET:-}}"
export OMNICRAFT_ARCHIVE_OFFSITE_ENDPOINT="${OFFSITE_ARCHIVE_ENDPOINT:-}"
export OMNICRAFT_ARCHIVE_OFFSITE_REGION="${OFFSITE_ARCHIVE_REGION:-}"
export OMNICRAFT_ARCHIVE_OFFSITE_BUCKET="${OFFSITE_ARCHIVE_BUCKET:-}"

python3 - <<'PY'
import datetime
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys

manifest_path = os.environ["OMNICRAFT_ARCHIVE_MANIFEST"]
target_dir = os.environ.get("OMNICRAFT_ARCHIVE_TARGET") or ""
report_dir = os.environ["OMNICRAFT_ARCHIVE_REPORT"]
retention_days = int(os.environ["OMNICRAFT_ARCHIVE_RETENTION"])
github_release = os.environ.get("OMNICRAFT_ARCHIVE_GITHUB_RELEASE") or ""
offsite_uri = os.environ.get("OMNICRAFT_ARCHIVE_OFFSITE_URI") or ""
offsite_ak_id = os.environ.get("OMNICRAFT_ARCHIVE_OFFSITE_AK_ID") or ""
offsite_ak_secret = os.environ.get("OMNICRAFT_ARCHIVE_OFFSITE_AK_SECRET") or ""
offsite_endpoint = os.environ.get("OMNICRAFT_ARCHIVE_OFFSITE_ENDPOINT") or ""
offsite_region = os.environ.get("OMNICRAFT_ARCHIVE_OFFSITE_REGION") or ""
offsite_bucket = os.environ.get("OMNICRAFT_ARCHIVE_OFFSITE_BUCKET") or ""

real_mode = bool(github_release or offsite_uri)
manifest_dir = os.path.dirname(manifest_path)
manifest_name = os.path.basename(manifest_path)

def fail(msg):
    print(f"archive-release-evidence: {msg}", file=sys.stderr)
    sys.exit(1)

def placeholder(value):
    lower = value.lower()
    return any(tok in lower for tok in ("<", ">", "change_me", "replace_me", "placeholder", "your-", "example.com"))

try:
    with open(manifest_path, encoding="utf-8") as f:
        manifest = json.load(f)
except (OSError, ValueError) as e:
    fail(f"manifest is not valid JSON: {e}")

if github_release and placeholder(github_release):
    fail("-GitHubRelease value contains placeholders")

offsite_prefix = ""
if offsite_uri:
    if not re.fullmatch(r"oss://[a-z0-9][a-z0-9-]{2,62}(/.*)?", offsite_uri):
        fail(f"-OffsiteUri must be oss://bucket/prefix: {offsite_uri}")
    parsed = offsite_uri[len("oss://"):]
    bucket, _, prefix = parsed.partition("/")
    if offsite_bucket and offsite_bucket != bucket:
        fail(f"OFFSITE_ARCHIVE_BUCKET {offsite_bucket} does not match -OffsiteUri bucket {bucket}")
    offsite_bucket = bucket
    offsite_prefix = prefix.rstrip("/")
    if not offsite_endpoint:
        if not offsite_region:
            fail("OFFSITE_ARCHIVE_ENDPOINT (or OFFSITE_ARCHIVE_REGION) is required for the off-site destination")
        offsite_endpoint = f"oss-{offsite_region}.aliyuncs.com"
    if not offsite_ak_id or not offsite_ak_secret:
        fail("off-site archive credentials missing: set ARCHIVE_AK_ID/ARCHIVE_AK_SECRET or OFFSITE_ARCHIVE_AK_ID/OFFSITE_ARCHIVE_AK_SECRET")

if real_mode and (not github_release or not offsite_uri):
    fail("real archive mode requires both -GitHubRelease and -OffsiteUri")

if real_mode and not target_dir:
    target_dir = os.path.join(report_dir, "archive-local")

def safe_object_name(value):
    if not isinstance(value, str) or not value or "\\" in value:
        fail(f"archive object path must be a non-empty relative POSIX path: {value!r}")
    path = pathlib.PurePosixPath(value)
    if path.is_absolute() or ".." in path.parts:
        fail(f"archive object path escapes the manifest directory: {value}")
    normalized = path.as_posix()
    if normalized in ("", "."):
        fail(f"archive object path is empty: {value!r}")
    return normalized

objects = [manifest_name]
is_release_manifest = bool(manifest.get("components") or manifest.get("artifacts") or manifest.get("migration_manifest"))
if is_release_manifest:
    for comp in manifest.get("components", []):
        objects.append(safe_object_name(comp["sbom_path"]))
    for artifact in manifest.get("artifacts", []):
        objects.append(safe_object_name(artifact["path"]))
    mm = manifest.get("migration_manifest", {})
    if mm.get("path"):
        objects.append(safe_object_name(mm["path"]))
    for extra in ("provenance-verification.json", "sbom-generation.json"):
        if os.path.isfile(os.path.join(manifest_dir, extra)):
            objects.append(extra)
else:
    # Ops-08 deployment manifests are evidence indexes rather than SBOM
    # manifests. Archive the complete evidence directory so the receipt can
    # be independently checked (preflight, backup, migration, readiness,
    # smoke, rollback and recovery comparison), not just the top-level JSON.
    for root, dirs, files in os.walk(manifest_dir):
        dirs[:] = sorted(d for d in dirs if d not in {"archive-local", "offsite"})
        for filename in sorted(files):
            relative = os.path.relpath(os.path.join(root, filename), manifest_dir)
            if relative in {"archive-receipt.json", "archive-release-evidence.json"}:
                continue
            objects.append(safe_object_name(relative))

missing = [p for p in objects if not os.path.isfile(os.path.join(manifest_dir, p))]
if missing:
    fail(f"missing source files: {', '.join(missing)}")
# A path may be referenced both as an artifact and as the migration manifest;
# archive each unique object once.
objects = list(dict.fromkeys(objects))

def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()

created_at = datetime.datetime.now(datetime.timezone.utc)
expires_at = created_at + datetime.timedelta(days=retention_days)
expires_str = expires_at.strftime("%Y-%m-%dT%H:%M:%SZ")

destinations = []
blockers = []

def record_objects(base, name_prefix="", verified=False):
    recorded = []
    for name in sorted(objects):
        src = os.path.join(manifest_dir, name)
        dst = os.path.join(base, name)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        shutil.copy2(src, dst)
        entry = {
            "name": name,
            "sha256": sha256_file(dst),
            "size": os.path.getsize(dst),
        }
        if name_prefix:
            entry["remote_path"] = f"{name_prefix}/{name}"
        if verified:
            entry["verified"] = True
        recorded.append(entry)
    return recorded

# ------------------------------------------------------- local adapter mode
if not real_mode:
    local_base = target_dir
    offsite_base = os.path.join(target_dir, "offsite")
    os.makedirs(offsite_base, exist_ok=True)
    destinations = [
        {"type": "github-release", "base": local_base, "objects": record_objects(local_base)},
        {
            "type": "offsite-encrypted",
            "base": offsite_base,
            "encryption": {
                "method": "age-envelope",
                "simulated": True,
                "note": "real encrypted operator off-site destination requires operator credentials (Ops-08)",
            },
            "objects": record_objects(offsite_base),
        },
    ]
    blockers = [
        "real encrypted operator off-site destination requires operator credentials (Ops-08)",
    ]

# ------------------------------------------------------ real destination mode
if real_mode:
    os.makedirs(target_dir, exist_ok=True)
    all_names = sorted(objects)
    if github_release:
        if shutil.which("gh") is None:
            fail("gh CLI is required for -GitHubRelease but is not on PATH")
        assets = {}
        for name in all_names:
            src = os.path.join(manifest_dir, name)
            run = subprocess.run(
                # `file#label` preserves relative paths in the release asset
                # name; uploading only `src` would flatten SBOM directories
                # and make same-named evidence files collide.
                ["gh", "release", "upload", github_release, f"{src}#{name}", "--clobber"],
                capture_output=True, text=True)
            if run.returncode != 0:
                fail(f"github release upload failed for {name}: {run.stderr.strip()}")
        view = subprocess.run(
            ["gh", "release", "view", github_release, "--json", "assets"],
            capture_output=True, text=True)
        if view.returncode != 0:
            fail(f"github release asset verification failed: {view.stderr.strip()}")
        try:
            for asset in json.loads(view.stdout).get("assets", []):
                assets[asset["name"]] = {
                    "url": asset.get("url", ""),
                    "size": asset.get("size", 0),
                    "state": asset.get("state", ""),
                }
        except (KeyError, OSError, TypeError, ValueError) as e:
            fail(f"github release asset response is not valid JSON: {e}")
        github_objects = []
        for name in all_names:
            remote = assets.get(name, {})
            if remote.get("state") != "uploaded":
                fail(f"github release asset verification missing uploaded asset: {name}")
            sha = sha256_file(os.path.join(manifest_dir, name))
            github_objects.append({
                "name": name,
                "sha256": sha,
                "size": os.path.getsize(os.path.join(manifest_dir, name)),
                "remote_state": remote.get("state", "uploaded"),
            })
            if remote.get("url"):
                github_objects[-1]["asset_url"] = remote["url"]
        destinations.append({
            "type": "github-release",
            "tag": github_release,
            "objects": github_objects,
        })
    if offsite_uri:
        if shutil.which("ossutil") is None:
            fail("ossutil CLI is required for -OffsiteUri but is not on PATH")
        archive_commit = manifest.get('commit') or 'unknown'
        retention_meta = (
            f"x-oss-meta-retention-days:{retention_days}#"
            f"x-oss-meta-expires-at:{expires_str}#"
            f"x-oss-meta-archive-commit:{archive_commit}#"
            "x-oss-server-side-encryption:AES256"
        )
        ossutil_cmd = ["ossutil", "cp", "-e", offsite_endpoint,
                       "-i", offsite_ak_id, "-k", offsite_ak_secret,
                       "--meta", retention_meta]
        remote = f"oss://{offsite_bucket}"
        if offsite_prefix:
            remote += f"/{offsite_prefix}"
        for name in all_names:
            src = os.path.join(manifest_dir, name)
            run = subprocess.run(ossutil_cmd + [src, f"{remote}/{name}"], capture_output=True, text=True)
            if run.returncode != 0:
                fail(f"ossutil upload failed for {name}: {run.stderr.strip() or run.stdout.strip()}")
        oss_objects = []
        for name in all_names:
            # ListObjects only proves that an object name exists. stat is
            # required here because the release gate must verify the remote
            # encryption and retention metadata, not just record the upload
            # command's intended headers.
            verify = subprocess.run(
                ["ossutil", "stat", f"{remote}/{name}",
                 "-e", offsite_endpoint, "-i", offsite_ak_id, "-k", offsite_ak_secret],
                capture_output=True, text=True)
            if verify.returncode != 0:
                fail(f"off-site archive metadata verification failed for {name}: {verify.stderr.strip() or verify.stdout.strip()}")
            metadata = verify.stdout
            expected_metadata = {
                "x-oss-meta-retention-days": str(retention_days),
                "x-oss-meta-archive-commit": archive_commit,
                "x-oss-server-side-encryption": "AES256",
            }
            for key, expected in expected_metadata.items():
                match = re.search(rf"(?im)^\s*{re.escape(key)}\s*:\s*(.*?)\s*$", metadata)
                if not match or match.group(1) != expected:
                    fail(f"off-site archive metadata mismatch for {name}: {key}")
            expiry_match = re.search(r"(?im)^\s*x-oss-meta-expires-at\s*:\s*(\S+)\s*$", metadata)
            if not expiry_match:
                fail(f"off-site archive metadata missing for {name}: x-oss-meta-expires-at")
            sha = sha256_file(os.path.join(manifest_dir, name))
            entry = {
                "name": name,
                "sha256": sha,
                "size": os.path.getsize(os.path.join(manifest_dir, name)),
                "remote_path": f"{remote}/{name}",
                "verified": True,
                "retention_meta": {"retention_days": retention_days, "expires_at": expires_str},
            }
            oss_objects.append(entry)
        destinations.append({
            "type": "offsite-encrypted",
            "uri": offsite_uri,
            "endpoint": offsite_endpoint,
            "encryption": {"method": "sse-oss", "simulated": False},
            "objects": oss_objects,
        })
    # local mirror retained for deterministic digest evidence
    local_mirror = os.path.join(target_dir, "local-mirror")
    os.makedirs(local_mirror, exist_ok=True)
    destinations.insert(0, {"type": "local-mirror", "base": local_mirror, "objects": record_objects(local_mirror)})

receipt = {
    "schema_version": 1,
    "source_manifest": manifest_name,
    "source_commit": manifest.get("commit"),
    "created_at": created_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
    "retention_days": retention_days,
    "expires_at": expires_str,
    "destinations": destinations,
    "redaction_checked": False,
    "blockers": blockers,
}
receipt_text = json.dumps(receipt, sort_keys=True)
for secret in (offsite_ak_id, offsite_ak_secret):
    if len(secret) >= 8 and secret in receipt_text:
        fail("archive receipt contains off-site credential material")
receipt["redaction_checked"] = True
receipt_dir = report_dir if real_mode else target_dir
receipt_path = os.path.join(receipt_dir, "archive-receipt.json")
with open(receipt_path, "w", encoding="utf-8") as f:
    json.dump(receipt, f, indent=2)
    f.write("\n")

report = {
    "source_commit": manifest.get("commit"),
    "created_at": receipt["created_at"],
    "retention_days": retention_days,
    "expires_at": receipt["expires_at"],
    "object_count": len(objects),
    "destinations": [d["type"] for d in destinations],
    "receipt_path": receipt_path,
    "receipt_sha256": sha256_file(receipt_path),
    "blockers": blockers,
}
report_path = os.path.join(report_dir, "archive-release-evidence.json")
with open(report_path, "w", encoding="utf-8") as f:
    json.dump(report, f, indent=2)
    f.write("\n")

print(f"archive-release-evidence: {len(objects)} objects archived "
      f"(retention {retention_days}d, expires {receipt['expires_at']})")
for dest in destinations:
    print(f"archive-release-evidence: destination {dest['type']} - {len(dest['objects'])} objects")
if blockers:
    print("archive-release-evidence: BLOCKER: " + " | ".join(blockers))
PY
