#!/usr/bin/env bash
# =============================================================================
# OmniCraft PostgreSQL Backup Script
# =============================================================================
# Creates a custom-format PostgreSQL dump together with a checksum manifest
# and the applied migration ledger. Designed for cron-based scheduling.
#
# Usage:
#   ./scripts/backup-db.sh                        # backup to default dir
#   BACKUP_DIR=/mnt/backups ./scripts/backup-db.sh
#   ./scripts/backup-db.sh --retain 7             # keep last 7 backups
#
# Cron example (daily at 2am):
#   0 2 * * * /path/to/OmniCraft/scripts/backup-db.sh >> /var/log/omnicraft-backup.log 2>&1
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="${BACKUP_DIR:-${SCRIPT_DIR}/../backups}"
# Parse --retain argument
RETAIN_COUNT="${RETAIN_COUNT:-7}"
while [ $# -gt 0 ]; do
    case "$1" in
        --retain)
            if [ $# -lt 2 ]; then
                echo "ERROR: --retain requires a count" >&2
                exit 2
            fi
            RETAIN_COUNT="$2"
            shift 2
            ;;
        --retain=*)
            RETAIN_COUNT="${1#*=}"
            shift
            ;;
        *)
            shift
            ;;
    esac
done

# Database connection string
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-omnicraft}"
DB_PASSWORD="${DB_PASSWORD:-omnicraft}"
DB_NAME="${DB_NAME:-omnicraft}"

export PGPASSWORD="${DB_PASSWORD}"

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

# Ensure backup directory exists
mkdir -p "${BACKUP_DIR}"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/omnicraft_${DB_NAME}_${TIMESTAMP}.custom"
MANIFEST_FILE="${BACKUP_FILE}.manifest.json"

echo "==> Starting backup at $(date)"
echo "  Database: ${DB_NAME}@${DB_HOST}:${DB_PORT}"
echo "  Destination: ${BACKUP_FILE}"

# Check if pg_dump is available
if ! command -v pg_dump &> /dev/null; then
    echo "ERROR: pg_dump is not installed. Please install PostgreSQL client."
    exit 1
fi

# The migration ledger is mandatory so a backup always carries a verified
# migration set. Databases initialized before the ledger runner was adopted
# must be rebuilt via scripts/init-db.sh before they can be backed up.
ledger_present="$(psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
    -tAc "SELECT to_regclass('public.schema_migrations') IS NOT NULL" 2>/dev/null || true)"
if [ "${ledger_present}" != "t" ]; then
    echo "ERROR: ${DB_NAME} has no schema_migrations ledger; rebuild the database with scripts/init-db.sh first"
    exit 1
fi

read_ledger_json() {
    psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -tAc \
        "SELECT json_agg(json_build_object('version', version, 'filename', filename, 'checksum', checksum) ORDER BY version) FROM schema_migrations"
}

# Read the migration set on both sides of pg_dump. The migration runner may
# commit concurrently, and a post-dump-only read can describe a newer schema
# than the snapshot stored in the dump. Reject that backup instead.
LEDGER_JSON_BEFORE="$(read_ledger_json)"

# Capture the start before pg_dump so the manifest describes the real backup
# window rather than only the post-dump manifest-writing window.
BACKUP_STARTED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Perform a custom-format dump (policy: postgres_full.format == custom).
# The dump streams to stdout and is redirected to the host file so the
# script works equally well with host client tools and container wrappers
# (for example docker exec pg_dump ... > file). Custom format is streamed
# to stdout by omitting -f; pg_dump refuses -f - for custom format.
if ! pg_dump -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
    --format=custom --no-owner --no-acl > "${BACKUP_FILE}"; then
    echo "ERROR: Backup failed!"
    exit 1
fi

if ! LEDGER_JSON_AFTER="$(read_ledger_json)"; then
    rm -f "${BACKUP_FILE}"
    echo "ERROR: could not verify the migration ledger after pg_dump; inconsistent dump removed" >&2
    exit 1
fi
if [ "${LEDGER_JSON_BEFORE}" != "${LEDGER_JSON_AFTER}" ]; then
    rm -f "${BACKUP_FILE}"
    echo "ERROR: migration ledger changed during pg_dump; inconsistent dump removed" >&2
    exit 1
fi

BACKUP_SIZE=$(du -h "${BACKUP_FILE}" | cut -f1)

# Checksum manifest + migration manifest (policy: checksum_manifest and
# migration_manifest are both required), including start/finish times.
DUMP_CHECKSUM="$(sha256_of "${BACKUP_FILE}")"
PG_VERSION="$(psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -tAc "SHOW server_version")"
LEDGER_JSON="${LEDGER_JSON_BEFORE}"

python3 - "$MANIFEST_FILE" "$(basename "${BACKUP_FILE}")" "$DB_NAME" "$DUMP_CHECKSUM" "$PG_VERSION" "$LEDGER_JSON" "$BACKUP_STARTED" <<'PY'
import datetime
import json
import sys

path, backup_file, source_db, checksum, pg_version, ledger_json, started = sys.argv[1:8]
manifest = {
    "schema_version": 1,
    "backup_file": backup_file,
    "source_db": source_db,
    "started_at": started,
    "finished_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "dump_checksum_sha256": checksum,
    "pg_version": pg_version,
    "migrations": json.loads(ledger_json),
}
with open(path, "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")
PY

echo "==> Backup completed: ${BACKUP_FILE} (${BACKUP_SIZE}, sha256 ${DUMP_CHECKSUM:0:12}...)"
echo "  Manifest: ${MANIFEST_FILE}"

# Rotate old backups (policy: local_retention.copies)
BACKUP_COUNT=$(ls -1 "${BACKUP_DIR}"/omnicraft_*.custom 2>/dev/null | wc -l)
if [ "${BACKUP_COUNT}" -gt "${RETAIN_COUNT}" ]; then
    echo "==> Rotating old backups (keeping ${RETAIN_COUNT} of ${BACKUP_COUNT})"
    ls -1t "${BACKUP_DIR}"/omnicraft_*.custom | tail -n +$((RETAIN_COUNT + 1)) | while read -r old_backup; do
        echo "  Removing: $(basename "${old_backup}")"
        rm -f "${old_backup}" "${old_backup}.manifest.json"
    done
fi

echo "==> Backup finished at $(date)"
