#!/usr/bin/env bash
# =============================================================================
# OmniCraft Database Initialization Script
# =============================================================================
# Runs all migration files in order against the target database.
# Usage:
#   ./scripts/init-db.sh                          # uses env vars or defaults
#   DB_DSN="host=... dbname=omnicraft" ./scripts/init-db.sh
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIGRATIONS_DIR="${SCRIPT_DIR}/../backend/migrations"

# Database connection string
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-omnicraft}"
DB_PASSWORD="${DB_PASSWORD:-omnicraft}"
DB_NAME="${DB_NAME:-omnicraft}"

export PGPASSWORD="${DB_PASSWORD}"

echo "==> Initializing database: ${DB_NAME} on ${DB_HOST}:${DB_PORT}"

# Check if psql is available
if ! command -v psql &> /dev/null; then
    echo "ERROR: psql is not installed. Please install PostgreSQL client."
    exit 1
fi

# Check connectivity
if ! psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -c "SELECT 1;" &> /dev/null; then
    echo "ERROR: Cannot connect to PostgreSQL at ${DB_HOST}:${DB_PORT}"
    echo "  Make sure PostgreSQL is running and credentials are correct."
    exit 1
fi

echo "==> Found $(ls -1 "${MIGRATIONS_DIR}"/*.sql 2>/dev/null | wc -l) migration files"

# Run each migration in order
FAILED=0
for migration in "${MIGRATIONS_DIR}"/*.sql; do
    filename=$(basename "${migration}")
    echo -n "  Running ${filename} ... "

    if psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
        -v ON_ERROR_STOP=1 -f "${migration}" &> /dev/null; then
        echo "OK"
    else
        echo "FAILED"
        echo "  Error details:"
        psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
            -v ON_ERROR_STOP=1 -f "${migration}" 2>&1 | head -20
        FAILED=1
        echo "  WARNING: Continuing with remaining migrations..."
    fi
done

echo ""
if [ "${FAILED}" -eq 0 ]; then
    echo "==> All migrations completed successfully."
else
    echo "==> Some migrations failed. Review errors above."
    exit 1
fi
