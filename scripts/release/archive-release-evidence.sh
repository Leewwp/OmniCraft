#!/usr/bin/env bash
# =============================================================================
# OmniCraft release-evidence archiver: proves the archive contract adapter
# locally with deterministic fixtures — copies the manifest, SBOMs, artifacts
# and migration manifest into the GitHub-Release asset destination and the
# operator off-site destination, and writes a machine-readable receipt with
# per-object digests and one-year (default) retention metadata.
#
# The real encrypted operator off-site destination requires external
# credentials and remains an Ops-08 release blocker; this script records the
# simulated destination and the blocker explicitly and never accepts or emits
# credentials.
#
# Usage:
#   bash scripts/release/archive-release-evidence.sh -Manifest <path> -TargetDir <dir>
#       [-ReportDir <dir>] [-RetentionDays <int>]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST=""
TARGET_DIR=""
REPORT_DIR=""
RETENTION_DAYS=365

while [ $# -gt 0 ]; do
  case "$1" in
    -Manifest)
      [ $# -ge 2 ] || { echo "missing value for -Manifest" >&2; exit 2; }
      MANIFEST="$2"; shift 2 ;;
    -TargetDir)
      [ $# -ge 2 ] || { echo "missing value for -TargetDir" >&2; exit 2; }
      TARGET_DIR="$2"; shift 2 ;;
    -ReportDir)
      [ $# -ge 2 ] || { echo "missing value for -ReportDir" >&2; exit 2; }
      REPORT_DIR="$2"; shift 2 ;;
    -RetentionDays)
      [ $# -ge 2 ] || { echo "missing value for -RetentionDays" >&2; exit 2; }
      RETENTION_DAYS="$2"; shift 2 ;;
    *)
      echo "unknown argument: $1" >&2
      echo "usage: archive-release-evidence.sh -Manifest <path> -TargetDir <dir> [-ReportDir <dir>] [-RetentionDays <int>]" >&2
      exit 2 ;;
  esac
done

if [ -z "$MANIFEST" ] || [ -z "$TARGET_DIR" ]; then
  echo "usage: archive-release-evidence.sh -Manifest <path> -TargetDir <dir> [-ReportDir <dir>] [-RetentionDays <int>]" >&2
  exit 2
fi

[ -f "$MANIFEST" ] || { echo "manifest not found: $MANIFEST" >&2; exit 1; }
case "$RETENTION_DAYS" in
  ''|*[!0-9]*) echo "retention days must be a positive integer" >&2; exit 2 ;;
esac

MANIFEST="$(cd "$(dirname "$MANIFEST")" && pwd)/$(basename "$MANIFEST")"
TARGET_DIR="$(mkdir -p "$TARGET_DIR" && cd "$TARGET_DIR" && pwd)"
if [ -z "$REPORT_DIR" ]; then
  REPORT_DIR="$(dirname "$MANIFEST")"
fi
mkdir -p "$REPORT_DIR"
REPORT_DIR="$(cd "$REPORT_DIR" && pwd)"

export OMNICRAFT_ARCHIVE_MANIFEST="$MANIFEST"
export OMNICRAFT_ARCHIVE_TARGET="$TARGET_DIR"
export OMNICRAFT_ARCHIVE_REPORT="$REPORT_DIR"
export OMNICRAFT_ARCHIVE_RETENTION="$RETENTION_DAYS"

python3 - <<'PY'
import datetime
import hashlib
import json
import os
import shutil
import sys

manifest_path = os.environ["OMNICRAFT_ARCHIVE_MANIFEST"]
target_dir = os.environ["OMNICRAFT_ARCHIVE_TARGET"]
report_dir = os.environ["OMNICRAFT_ARCHIVE_REPORT"]
retention_days = int(os.environ["OMNICRAFT_ARCHIVE_RETENTION"])

manifest_dir = os.path.dirname(manifest_path)

try:
    with open(manifest_path, encoding="utf-8") as f:
        manifest = json.load(f)
except (OSError, ValueError) as e:
    print(f"archive-release-evidence: manifest is not valid JSON: {e}", file=sys.stderr)
    sys.exit(1)

objects = ["release-manifest.json"]
for comp in manifest.get("components", []):
    objects.append(comp["sbom_path"])
for artifact in manifest.get("artifacts", []):
    objects.append(artifact["path"])
mm = manifest.get("migration_manifest", {})
if mm.get("path"):
    objects.append(mm["path"])
for extra in ("provenance-verification.json", "sbom-generation.json"):
    if os.path.isfile(os.path.join(manifest_dir, extra)):
        objects.append(extra)

missing = [p for p in objects if not os.path.isfile(os.path.join(manifest_dir, p))]
if missing:
    print(f"archive-release-evidence: missing source files: {', '.join(missing)}", file=sys.stderr)
    sys.exit(1)

def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()

destinations = [
    {"type": "github-release", "base": target_dir},
    {
        "type": "offsite-encrypted",
        "base": os.path.join(target_dir, "offsite"),
        "encryption": {
            "method": "age-envelope",
            "simulated": True,
            "note": "real encrypted operator off-site destination requires operator credentials (Ops-08)",
        },
    },
]

for dest in destinations:
    os.makedirs(dest["base"], exist_ok=True)
    dest["objects"] = []
    for name in sorted(objects):
        src = os.path.join(manifest_dir, name)
        dst = os.path.join(dest["base"], name)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        shutil.copy2(src, dst)
        dest["objects"].append({
            "name": name,
            "sha256": sha256_file(dst),
            "size": os.path.getsize(dst),
        })

created_at = datetime.datetime.now(datetime.timezone.utc)
expires_at = created_at + datetime.timedelta(days=retention_days)
receipt = {
    "schema_version": 1,
    "source_manifest": "release-manifest.json",
    "source_commit": manifest.get("commit"),
    "created_at": created_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
    "retention_days": retention_days,
    "expires_at": expires_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
    "destinations": destinations,
    "redaction_checked": False,
    "blockers": [
        "real encrypted operator off-site destination requires operator credentials (Ops-08)",
    ],
}
receipt_path = os.path.join(target_dir, "archive-receipt.json")
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
    "blockers": receipt["blockers"],
}
report_path = os.path.join(report_dir, "archive-release-evidence.json")
with open(report_path, "w", encoding="utf-8") as f:
    json.dump(report, f, indent=2)
    f.write("\n")

print(f"archive-release-evidence: {len(objects)} objects archived to {target_dir} "
      f"(retention {retention_days}d, expires {receipt['expires_at']})")
print("archive-release-evidence: BLOCKER: real encrypted operator off-site destination "
      "requires operator credentials (Ops-08)")
PY
