#!/usr/bin/env bash
# =============================================================================
# OmniCraft Database Initialization Script
# =============================================================================
# Applies all forward-only migrations to the target database through the
# migrate CLI, which maintains the schema_migrations ledger, rejects checksum
# drift, and serializes concurrent runners with an advisory lock. Local
# bootstrap only; production runs the same migrate binary as a one-shot
# container before the backend starts.
#
# Usage:
#   ./scripts/init-db.sh                          # uses env vars or defaults
#   DB_DSN="host=... dbname=omnicraft" ./scripts/init-db.sh
#   ./scripts/init-db.sh -ReconcileVersions 047,049 -ReconcileApproval CHG-1234
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="${SCRIPT_DIR}/../backend"
MIGRATIONS_DIR="${BACKEND_DIR}/migrations"

RECONCILE=""
RECONCILE_APPROVAL=""
SUMMARY_PATH=""
while [ $# -gt 0 ]; do
    case "$1" in
        -ReconcileVersions)
            RECONCILE="$2"
            shift 2
            ;;
		-ReconcileApproval)
			RECONCILE_APPROVAL="$2"
			shift 2
			;;
        -SummaryPath)
            SUMMARY_PATH="$2"
            shift 2
            ;;
        *)
            echo "unknown argument: $1" >&2
            exit 2
            ;;
    esac
done

# Database connection string. DB_DSN takes precedence, otherwise build one
# from the conventional per-part env vars.
DSN="${DB_DSN:-}"
if [ -z "${DSN}" ]; then
    DB_HOST="${DB_HOST:-localhost}"
    DB_PORT="${DB_PORT:-5432}"
    DB_USER="${DB_USER:-omnicraft}"
    DB_PASSWORD="${DB_PASSWORD:-omnicraft}"
    DB_NAME="${DB_NAME:-omnicraft}"
    DSN="host=${DB_HOST} port=${DB_PORT} user=${DB_USER} password=${DB_PASSWORD} dbname=${DB_NAME} sslmode=disable"
fi

ARGS=(-DSN "${DSN}" -Dir "${MIGRATIONS_DIR}" -Metadata "${MIGRATIONS_DIR}/metadata.json")
if [ -n "${RECONCILE}" ]; then
    ARGS+=(-ReconcileVersions "${RECONCILE}")
fi
if [ -n "${RECONCILE_APPROVAL}" ]; then
	ARGS+=(-ReconcileApproval "${RECONCILE_APPROVAL}")
fi
if [ -n "${SUMMARY_PATH}" ]; then
    ARGS+=(-SummaryPath "${SUMMARY_PATH}")
fi

echo "==> Running migrations with the ledger runner (connection details redacted)"
(cd "${BACKEND_DIR}" && go run ./cmd/migrate "${ARGS[@]}")
echo "==> Migrations complete"
