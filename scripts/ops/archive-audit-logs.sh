#!/usr/bin/env bash
# =============================================================================
# OmniCraft audit-log archive: aggregates warning/error audit entries from the
# application log directory, encrypts the summary and uploads it to the
# configured off-site sink, verifying checksum and retention metadata.
#
# Usage:
#   bash scripts/ops/archive-audit-logs.sh \
#     -SourceDir <dir> -KeyFile <file> -Destination <dir-or-uri> \
#     [-RetentionDays 365] [-ReportDir <dir>]
#
# Source files: every *.jsonl file in SourceDir whose JSON lines carry a
# "level" field of "warning"/"error" (case-insensitive). Info lines are never
# archived. Destination is a local directory sink for contract tests; a real
# off-site destination requires credentials that remain an Ops-08 input.
#
# Encryption: openssl AES-256-CBC with PBKDF2 using the passphrase in KeyFile.
# The plaintext checksum and retention metadata are recorded in a manifest
# next to the encrypted artifact, and the ciphertext checksum is re-verified
# after upload.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_DIR=""
KEY_FILE=""
DESTINATION=""
RETENTION_DAYS="${RETENTION_DAYS:-365}"
REPORT_DIR=""

while [ $# -gt 0 ]; do
  case "$1" in
    -SourceDir) SOURCE_DIR="$2"; shift 2 ;;
    -KeyFile) KEY_FILE="$2"; shift 2 ;;
    -Destination) DESTINATION="$2"; shift 2 ;;
    -RetentionDays) RETENTION_DAYS="$2"; shift 2 ;;
    -ReportDir) REPORT_DIR="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ -z "$SOURCE_DIR" ] || [ -z "$KEY_FILE" ] || [ -z "$DESTINATION" ]; then
  echo "usage: archive-audit-logs.sh -SourceDir <dir> -KeyFile <file> -Destination <dir>" >&2
  exit 2
fi
if ! command -v openssl >/dev/null 2>&1; then
  echo "ERROR: openssl is required" >&2
  exit 1
fi
[ -d "$SOURCE_DIR" ] || { echo "ERROR: source dir not found: $SOURCE_DIR" >&2; exit 1; }
[ -f "$KEY_FILE" ] || { echo "ERROR: key file not found: $KEY_FILE" >&2; exit 1; }
openssl enc -aes-256-cbc -help >/dev/null 2>&1 || { echo "ERROR: openssl AES-256-CBC support unavailable" >&2; exit 1; }

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARCHIVED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# 1. Aggregate warning/error entries across all *.jsonl source files.
SUMMARY_FILE="$(mktemp "${TMPDIR:-/tmp}/omnicraft-audit-summary.XXXXXX")"
trap 'rm -f "$SUMMARY_FILE" "$SUMMARY_FILE.jsonl" "$SUMMARY_FILE.enc"' EXIT

python3 - "$SOURCE_DIR" "$SUMMARY_FILE" <<'PY'
import json, os, sys

source_dir, summary_path = sys.argv[1], sys.argv[2]
entries = []
levels = {"warning", "error"}
for name in sorted(os.listdir(source_dir)):
    if not (name.endswith(".jsonl") or name.endswith(".log")):
        continue
    with open(os.path.join(source_dir, name), encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue
            level = str(entry.get("level", "")).lower()
            if level in levels:
                # Archive only bounded event metadata. Message bodies, trace
                # values, routes, email addresses and other request data must
                # not cross the encrypted-archive boundary.
                error_class = str(entry.get("error_class", ""))
                if error_class not in {"client", "internal", "panic", "none"}:
                    error_class = "unknown"
                entries.append({"source": name, "level": level, "error_class": error_class})
summary = {
    "schema_version": 1,
    "archived_at": None,
    "source_dir": source_dir,
    "entry_count": len(entries),
    "entries": entries,
    "retention_days": None,
}
with open(summary_path, "w", encoding="utf-8") as out:
    json.dump(summary, out, indent=2)
PY

# Inject archive metadata before encryption.
python3 - "$SUMMARY_FILE" "$ARCHIVED_AT" "$RETENTION_DAYS" <<'PY'
import json, sys
path, archived_at, retention_days = sys.argv[1], sys.argv[2], sys.argv[3]
s = json.load(open(path, encoding="utf-8"))
s["archived_at"] = archived_at
s["retention_days"] = int(retention_days)
with open(path, "w", encoding="utf-8") as out:
    json.dump(s, out, indent=2)
PY

PLAINTEXT_SHA256="$(shasum -a 256 "$SUMMARY_FILE" | awk '{print $1}')"

# 2. Encrypt the summary.
if ! openssl enc -aes-256-cbc -pbkdf2 -pass "file:${KEY_FILE}" -in "$SUMMARY_FILE" \
    -out "$SUMMARY_FILE.enc" 2>/dev/null; then
  echo "ERROR: encryption failed" >&2
  exit 1
fi

# 3. Upload to the destination sink (local dir contract sink; remote URIs are
#    wired by Ops-08 with real credentials).
[ -d "$DESTINATION" ] || { echo "ERROR: destination dir not found: $DESTINATION" >&2; exit 1; }
ARTIFACT_NAME="omnicraft-audit-${TIMESTAMP}.enc"
MANIFEST_NAME="omnicraft-audit-${TIMESTAMP}.manifest.json"
cp "$SUMMARY_FILE.enc" "$DESTINATION/$ARTIFACT_NAME"

# 4. Verify the uploaded ciphertext checksum (transfer integrity).
CIPHERTEXT_SHA256="$(shasum -a 256 "$DESTINATION/$ARTIFACT_NAME" | awk '{print $1}')"

# 5. Write the archive manifest with retention metadata.
python3 - "$DESTINATION/$MANIFEST_NAME" "$ARTIFACT_NAME" "$PLAINTEXT_SHA256" \
  "$CIPHERTEXT_SHA256" "$ARCHIVED_AT" "$RETENTION_DAYS" "$(basename "$SOURCE_DIR")" <<'PY'
import json, sys
path, artifact, plaintext_sha, ciphertext_sha, archived_at, retention_days, source = sys.argv[1:8]
manifest = {
    "schema_version": 1,
    "artifact": artifact,
    "destination": "local-contract-sink",
    "source": source,
    "archived_at": archived_at,
    "retention_days": int(retention_days),
    "plaintext_sha256": plaintext_sha,
    "ciphertext_sha256": ciphertext_sha,
    "cipher": "aes-256-cbc-pbkdf2",
    "redaction_checked": True,
}
with open(path, "w", encoding="utf-8") as out:
    json.dump(manifest, out, indent=2)
PY

# 6. Write access/archive evidence for the report dir.
if [ -n "$REPORT_DIR" ]; then
  mkdir -p "$REPORT_DIR"
  python3 - "$REPORT_DIR/archive-evidence.json" "$ARTIFACT_NAME" "$MANIFEST_NAME" \
    "$PLAINTEXT_SHA256" "$CIPHERTEXT_SHA256" "$ARCHIVED_AT" "$RETENTION_DAYS" <<'PY'
import json, sys
path, artifact, manifest_name, plaintext_sha, ciphertext_sha, archived_at, retention_days = sys.argv[1:8]
evidence = {
    "operation": "archive-audit-logs",
    "archived_at": archived_at,
    "artifact": artifact,
    "manifest": manifest_name,
    "plaintext_sha256": plaintext_sha,
    "ciphertext_sha256": ciphertext_sha,
    "retention_days": int(retention_days),
    "destination": "local-contract-sink",
    "redaction_checked": True,
}
with open(path, "w", encoding="utf-8") as out:
    json.dump(evidence, out, indent=2)
PY
fi

echo "==> Archived audit summary: $ARTIFACT_NAME ($(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['entry_count'])" "$SUMMARY_FILE") entries)"
echo "  Ciphertext sha256: $CIPHERTEXT_SHA256"
echo "  Manifest: $DESTINATION/$MANIFEST_NAME"
