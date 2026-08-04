#!/usr/bin/env bash
# =============================================================================
# OmniCraft PostgreSQL Restore Script
# =============================================================================
# Restores a custom-format backup into a NEW database (policy: restores never
# overwrite the source database), verifies the dump checksum and the applied
# migration ledger, and runs a smoke check.
#
# Usage:
#   bash scripts/restore-db.sh -Backup backups/omnicraft_<ts>.custom -TargetDB omnicraft_restore
#   ADMIN_DB_DSN="host=... user=... password=... dbname=postgres" \
#     bash scripts/restore-db.sh -Backup <file> -TargetDB <newdb>
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="${REPO_ROOT}/backend"
MIGRATIONS_DIR="${BACKEND_DIR}/migrations"

BACKUP_FILE=""
TARGET_DB=""
ADMIN_DSN="${ADMIN_DB_DSN:-}"
VERIFY_DSN=""

while [ $# -gt 0 ]; do
    case "$1" in
        -Backup)
            BACKUP_FILE="$2"
            shift 2
            ;;
        -TargetDB)
            TARGET_DB="$2"
            shift 2
            ;;
        -AdminDSN)
            ADMIN_DSN="$2"
            shift 2
            ;;
        -VerifyDSN)
            VERIFY_DSN="$2"
            shift 2
            ;;
        *)
            echo "unknown argument: $1" >&2
            exit 2
            ;;
    esac
done

if [ -z "${BACKUP_FILE}" ] || [ -z "${TARGET_DB}" ]; then
    echo "usage: restore-db.sh -Backup <custom-dump> -TargetDB <newdb> [-AdminDSN <dsn>]" >&2
    exit 2
fi
if ! echo "${TARGET_DB}" | grep -qE '^[a-z_][a-z0-9_]*$'; then
    echo "ERROR: -TargetDB must be a lowercase SQL identifier: ${TARGET_DB}" >&2
    exit 2
fi
if [ -z "${ADMIN_DSN}" ]; then
    ADMIN_DSN="host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=postgres sslmode=disable"
fi

MANIFEST_FILE="${BACKUP_FILE}.manifest.json"
if [ ! -f "${BACKUP_FILE}" ]; then
    echo "ERROR: backup file not found: ${BACKUP_FILE}" >&2
    exit 1
fi
if [ ! -f "${MANIFEST_FILE}" ]; then
    echo "ERROR: backup manifest not found: ${MANIFEST_FILE}" >&2
    exit 1
fi

STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}
echo "==> Verifying backup checksum"
ACTUAL_CHECKSUM="$(sha256_of "${BACKUP_FILE}")"
MANIFEST_CHECKSUM="$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['dump_checksum_sha256'])" "${MANIFEST_FILE}")"
if [ "${ACTUAL_CHECKSUM}" != "${MANIFEST_CHECKSUM}" ]; then
    echo "ERROR: dump checksum mismatch: file ${ACTUAL_CHECKSUM}, manifest ${MANIFEST_CHECKSUM}" >&2
    exit 1
fi
echo "  checksum OK (${ACTUAL_CHECKSUM:0:16}...)"

SOURCE_DB="$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['source_db'])" "${MANIFEST_FILE}")"
if [ "${TARGET_DB}" = "${SOURCE_DB}" ]; then
    echo "ERROR: refusing to restore into the source database ${SOURCE_DB}" >&2
    exit 1
fi
if [ -n "${VERIFY_DSN}" ]; then
    VERIFY_DB="$(python3 - "${VERIFY_DSN}" <<'PY'
import shlex
import sys

values = {}
for part in shlex.split(sys.argv[1]):
    key, separator, value = part.partition("=")
    if separator:
        values[key] = value
print(values.get("dbname", ""))
PY
)"
    if [ "${VERIFY_DB}" != "${TARGET_DB}" ]; then
        echo "ERROR: -VerifyDSN dbname must match -TargetDB (${TARGET_DB})" >&2
        exit 1
    fi
fi

command -v psql >/dev/null 2>&1 || { echo "ERROR: psql is not installed" >&2; exit 1; }
command -v pg_restore >/dev/null 2>&1 || { echo "ERROR: pg_restore is not installed" >&2; exit 1; }

echo "==> Creating target database ${TARGET_DB}"
target_exists="$(psql "${ADMIN_DSN}" -tAc "SELECT 1 FROM pg_database WHERE datname = '${TARGET_DB}'" || true)"
if [ "${target_exists}" = "1" ]; then
    echo "ERROR: target database ${TARGET_DB} already exists; refusing to overwrite" >&2
    exit 1
fi
psql "${ADMIN_DSN}" -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"${TARGET_DB}\"" >/dev/null

TARGET_DSN="$(python3 - "${ADMIN_DSN}" "${TARGET_DB}" <<'PY'
import sys

dsn, db = sys.argv[1], sys.argv[2]
parts = []
for part in dsn.split():
    key, _, value = part.partition("=")
    if key == "dbname":
        value = db
    parts.append(f"{key}={value}")
print(" ".join(parts))
PY
)"

cleanup() {
    psql "${ADMIN_DSN}" -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${TARGET_DB}' AND pid <> pg_backend_pid()" >/dev/null 2>&1 || true
    psql "${ADMIN_DSN}" -c "DROP DATABASE IF EXISTS \"${TARGET_DB}\"" >/dev/null 2>&1 || true
}

echo "==> Restoring into new database ${TARGET_DB}"
if ! pg_restore --no-owner --no-acl --exit-on-error -d "${TARGET_DSN}" < "${BACKUP_FILE}"; then
    echo "ERROR: pg_restore failed; dropping partial target database" >&2
    cleanup
    exit 1
fi

echo "==> Verifying migration ledger against the repository"
if [ -z "${VERIFY_DSN}" ]; then
    VERIFY_DSN="${TARGET_DSN}"
fi
if ! RESTORED_LEDGER_JSON="$(psql "${TARGET_DSN}" -tAc \
    "SELECT json_agg(json_build_object('version', version, 'filename', filename, 'checksum', checksum) ORDER BY version) FROM schema_migrations")"; then
    echo "ERROR: could not read the restored migration ledger" >&2
    cleanup
    exit 1
fi
if ! python3 - "${MANIFEST_FILE}" "${RESTORED_LEDGER_JSON}" <<'PY'
import json
import sys

manifest_path, restored_json = sys.argv[1:3]
with open(manifest_path, encoding="utf-8") as f:
    manifest = json.load(f)
expected = manifest.get("migrations")
actual = json.loads(restored_json)
if not isinstance(expected, list) or expected != actual:
    raise SystemExit(1)
PY
then
    echo "ERROR: backup manifest migration set does not match restored ledger" >&2
    cleanup
    exit 1
fi
if ! (cd "${BACKEND_DIR}" && go run ./cmd/migrate -DSN "${VERIFY_DSN}" -Dir "${MIGRATIONS_DIR}" \
    -Metadata "${MIGRATIONS_DIR}/metadata.json" -VerifyOnly); then
    echo "ERROR: restored database ledger does not match the repository" >&2
    cleanup
    exit 1
fi

echo "==> Running smoke checks"
SMOKE_LEDGER="$(psql "${TARGET_DSN}" -tAc "SELECT count(*) FROM schema_migrations")"
SMOKE_USERS="$(psql "${TARGET_DSN}" -tAc "SELECT count(*) FROM users")"
SMOKE_INVALID_CONSTRAINTS="$(psql "${TARGET_DSN}" -tAc "SELECT count(*) FROM pg_constraint WHERE NOT convalidated")"
echo "  ledger rows: ${SMOKE_LEDGER}, users rows: ${SMOKE_USERS}, unvalidated constraints: ${SMOKE_INVALID_CONSTRAINTS}"
if [ "${SMOKE_LEDGER}" -lt 1 ]; then
    echo "ERROR: restored database has no migration ledger" >&2
    cleanup
    exit 1
fi
if [ "${SMOKE_INVALID_CONSTRAINTS}" -ne 0 ]; then
    echo "ERROR: restored database has ${SMOKE_INVALID_CONSTRAINTS} unvalidated constraints" >&2
    cleanup
    exit 1
fi

FINISHED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "==> Restore verified: ${TARGET_DB} (${STARTED_AT} -> ${FINISHED_AT})"
