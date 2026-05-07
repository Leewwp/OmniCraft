#!/usr/bin/env bash
# =============================================================================
# OmniCraft PostgreSQL Backup Script
# =============================================================================
# Creates compressed backups. Designed for cron-based scheduling.
# Usage:
#   ./scripts/backup-db.sh                        # backup to default dir
#   BACKUP_DIR=/mnt/backups ./scripts/backup-db.sh
#   ./scripts/backup-db.sh --retain 7              # keep last 7 backups
#
# Cron example (daily at 2am):
#   0 2 * * * /path/to/OmniCraft/scripts/backup-db.sh >> /var/log/omnicraft-backup.log 2>&1
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="${BACKUP_DIR:-${SCRIPT_DIR}/../backups}"
RETAIN_COUNT="${RETAIN_COUNT:-30}"

# Parse --retain argument
for arg in "$@"; do
    case "${arg}" in
        --retain)
            shift
            RETAIN_COUNT="$1"
            shift
            ;;
        --retain=*)
            RETAIN_COUNT="${arg#*=}"
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

# Ensure backup directory exists
mkdir -p "${BACKUP_DIR}"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/omnicraft_${DB_NAME}_${TIMESTAMP}.sql.gz"

echo "==> Starting backup at $(date)"
echo "  Database: ${DB_NAME}@${DB_HOST}:${DB_PORT}"
echo "  Destination: ${BACKUP_FILE}"

# Check if pg_dump is available
if ! command -v pg_dump &> /dev/null; then
    echo "ERROR: pg_dump is not installed. Please install PostgreSQL client."
    exit 1
fi

# Perform backup
if pg_dump -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
    --no-owner --no-acl --compress=9 -f "${BACKUP_FILE}"; then
    BACKUP_SIZE=$(du -h "${BACKUP_FILE}" | cut -f1)
    echo "==> Backup completed: ${BACKUP_FILE} (${BACKUP_SIZE})"
else
    echo "ERROR: Backup failed!"
    exit 1
fi

# Rotate old backups
BACKUP_COUNT=$(ls -1 "${BACKUP_DIR}"/omnicraft_*.sql.gz 2>/dev/null | wc -l)
if [ "${BACKUP_COUNT}" -gt "${RETAIN_COUNT}" ]; then
    echo "==> Rotating old backups (keeping ${RETAIN_COUNT} of ${BACKUP_COUNT})"
    ls -1t "${BACKUP_DIR}"/omnicraft_*.sql.gz | tail -n +$((RETAIN_COUNT + 1)) | while read -r old_backup; do
        echo "  Removing: $(basename "${old_backup}")"
        rm -f "${old_backup}"
    done
fi

echo "==> Backup finished at $(date)"
