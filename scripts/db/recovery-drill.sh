#!/usr/bin/env bash
# =============================================================================
# OmniCraft database recovery drill
# =============================================================================
# Full recovery drill: builds an isolated PostgreSQL/Redis/MinIO stack, seeds
# fixture data, takes a policy-compliant backup, destroys the source database,
# restores into a NEW database, verifies the ledger and backend readiness,
# restores a deleted object version, reconciles DB attachment keys against the
# versioned object store, rebuilds Redis from PostgreSQL, measures RPO/RTO and
# writes evidence. Teardown always removes containers and volumes.
#
# Usage:
#   bash scripts/db/recovery-drill.sh -ReportDir artifacts/ops-02
#   bash scripts/db/recovery-drill.sh -ComposeFile ops/recovery/docker-compose.recovery.yml -ReportDir artifacts/ops-02
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"
MIGRATIONS_DIR="$BACKEND_DIR/migrations"

COMPOSE_FILE=""
REPORT_DIR=""

while [ $# -gt 0 ]; do
  case "$1" in
    -ComposeFile) COMPOSE_FILE="$2"; shift 2 ;;
    -ReportDir) REPORT_DIR="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ -z "$COMPOSE_FILE" ]; then
  COMPOSE_FILE="$REPO_ROOT/ops/recovery/docker-compose.recovery.yml"
fi
if [ -z "$REPORT_DIR" ]; then
  echo "usage: recovery-drill.sh -ReportDir <dir> [-ComposeFile <path>]" >&2
  exit 2
fi
REPORT_DIR="$(cd "$(dirname "$REPORT_DIR")" && pwd)/$(basename "$REPORT_DIR")"
if [ ! -f "$COMPOSE_FILE" ]; then
  echo "compose file not found: $COMPOSE_FILE" >&2
  exit 1
fi
if ! grep -qE '^\s+(postgres|redis|minio-init):' "$COMPOSE_FILE"; then
  echo "compose file $COMPOSE_FILE is not a recovery stack (missing postgres/redis/minio-init)" >&2
  exit 1
fi

PROJ="omnicraft-recovery-$$"
MC_IMAGE="minio/mc@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727"
BUCKET="omnicraft-recovery"
BACKUP_DIR="$REPORT_DIR/backup"
mkdir -p "$BACKUP_DIR"
mc_() {
  local mounts=()
  if [ -n "${SEED_DIR:-}" ]; then
    mounts=(-v "$SEED_DIR:/seed:ro")
  fi
  docker run --rm --network "${PROJ}_default" "${mounts[@]+"${mounts[@]}"}" \
    -e "MC_HOST_local=http://omnicraftrecovery:omnicraftrecovery-secret@minio:9000" \
    "$MC_IMAGE" "$@"
}


sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    if [ $# -gt 0 ]; then
      sha256sum "$1" | awk '{print $1}'
    else
      sha256sum | awk '{print $1}'
    fi
  else
    if [ $# -gt 0 ]; then
      shasum -a 256 "$1" | awk '{print $1}'
    else
      shasum -a 256 | awk '{print $1}'
    fi
  fi
}

# Emit bounded recovery-drill success/failure/last-success metrics to the
# Prometheus textfile collector directory when METRICS_TEXTFILE_DIR is set.
emit_recovery_metric() {
	local status="$1"
	[ -z "${METRICS_TEXTFILE_DIR:-}" ] && return 0
	mkdir -p "$METRICS_TEXTFILE_DIR"
	local outcome=0
	local last_success=""
	if [ "$status" -eq 0 ]; then
		outcome=1
		last_success="$(python3 -c 'import time; print(time.time())')"
	elif [ -f "${METRICS_TEXTFILE_DIR}/omnicraft_recovery_drill.prom" ]; then
		last_success="$(awk '/^omnicraft_recovery_drill_last_success_timestamp_seconds / {print $2}' \
			"${METRICS_TEXTFILE_DIR}/omnicraft_recovery_drill.prom")"
	fi
	{
		echo "# HELP omnicraft_recovery_drill_status Latest recovery drill outcome (1 success, 0 failure)."
		echo "# TYPE omnicraft_recovery_drill_status gauge"
		echo "omnicraft_recovery_drill_status $outcome"
	if [ -n "$last_success" ]; then
		echo "# HELP omnicraft_recovery_drill_last_success_timestamp_seconds Unix time of the last successful recovery drill."
		echo "# TYPE omnicraft_recovery_drill_last_success_timestamp_seconds gauge"
		echo "omnicraft_recovery_drill_last_success_timestamp_seconds $last_success"
	fi
  } > "${METRICS_TEXTFILE_DIR}/omnicraft_recovery_drill.prom.tmp"
  mv "${METRICS_TEXTFILE_DIR}/omnicraft_recovery_drill.prom.tmp" "${METRICS_TEXTFILE_DIR}/omnicraft_recovery_drill.prom"
}

cleanup() {
	local original_status=$?
	local teardown_status=0
	trap - EXIT
	set +e
  if [ -n "${SERVER_PID:-}" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" >/dev/null 2>&1
    wait "$SERVER_PID" >/dev/null 2>&1
  fi
  emit_recovery_metric "$original_status"
  echo "==> teardown: removing recovery stack $PROJ"
	docker compose -p "$PROJ" -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1
	teardown_status=$?
  for d in "${SEED_DIR:-}" "${OVERRIDE_DIR:-}" "${TOOL_DIR:-}"; do
    if [ -n "$d" ]; then
      rm -rf "$d"
    fi
  done
	if [ "$teardown_status" -ne 0 ]; then
		echo "ERROR: recovery stack teardown failed for $PROJ" >&2
		if [ "$original_status" -eq 0 ]; then
			original_status="$teardown_status"
		fi
	fi
	exit "$original_status"
}
trap cleanup EXIT

PG_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
REDIS_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
export RECOVERY_PG_PORT="$PG_PORT"
export RECOVERY_REDIS_PORT="$REDIS_PORT"

DRILL_STARTED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "==> recovery drill started at $DRILL_STARTED (project $PROJ)"

echo "==> starting isolated recovery stack"
docker compose -p "$PROJ" -f "$COMPOSE_FILE" up -d >/dev/null

for _ in $(seq 1 90); do
  ready=1
  docker exec "$PROJ"-postgres-1 pg_isready -U omnicraft -d omnicraft >/dev/null 2>&1 || ready=0
  docker exec "$PROJ"-redis-1 redis-cli ping >/dev/null 2>&1 || ready=0
  mc_ ready local >/dev/null 2>&1 || ready=0
  mc_ ls "local/$BUCKET" >/dev/null 2>&1 || ready=0
  if [ "$ready" -eq 1 ]; then
    break
  fi
  sleep 2
done
docker exec "$PROJ"-postgres-1 pg_isready -U omnicraft -d omnicraft >/dev/null 2>&1 || { echo "postgres did not become ready" >&2; exit 1; }
docker exec "$PROJ"-redis-1 redis-cli ping >/dev/null 2>&1 || { echo "redis did not become ready" >&2; exit 1; }
mc_ ready local >/dev/null 2>&1 || { echo "minio did not become ready" >&2; exit 1; }
mc_ ls "local/$BUCKET" >/dev/null 2>&1 || { echo "versioned bucket was not created" >&2; exit 1; }

PG_DSN="host=127.0.0.1 port=$PG_PORT user=omnicraft password=omnicraft dbname=omnicraft sslmode=disable"
ADMIN_DSN="host=127.0.0.1 port=$PG_PORT user=omnicraft password=omnicraft dbname=postgres sslmode=disable"
RESTORED_DSN="host=127.0.0.1 port=$PG_PORT user=omnicraft password=omnicraft dbname=omnicraft_restored sslmode=disable"

echo "==> applying full schema to the drill database"
( cd "$BACKEND_DIR" && go run ./cmd/migrate -DSN "$PG_DSN" -Dir "$MIGRATIONS_DIR" -Metadata "$MIGRATIONS_DIR/metadata.json" -SummaryPath "$BACKUP_DIR/schema-migration-summary.json" )

echo "==> seeding fixture data and attachment objects"
SEED_DIR="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-recovery-seed.XXXXXX")"
printf 'object-one-content' > "$SEED_DIR/att1.bin"
printf 'object-two-content' > "$SEED_DIR/att2.bin"
printf 'object-three-content' > "$SEED_DIR/att3.bin"
ATTACHMENT_KEYS=("fixture/att1.bin" "fixture/att2.bin" "fixture/att3.bin")

docker exec -i "$PROJ"-postgres-1 psql -U omnicraft -d omnicraft -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
INSERT INTO users (email, password_hash, username, preferred_locale) VALUES
  ('recovery-seed@example.invalid', '$2a$10$drill-hash-do-not-use', 'recovery-seed-user', 'zh-CN');
INSERT INTO tags (name, category, usage_count) VALUES
  ('drill-a', 'drill', 1),
  ('drill-b', 'drill', 2);
INSERT INTO content_items (title, author_id, zone, content_type, status)
  VALUES ('drill item', 1, 'original', 'article', 'published');
INSERT INTO content_attachments (content_item_id, file_type, oss_key, file_size, mime_type, is_primary) VALUES
  (1, 'image', 'fixture/att1.bin', 17, 'application/octet-stream', true),
  (1, 'image', 'fixture/att2.bin', 17, 'application/octet-stream', false),
  (1, 'image', 'fixture/att3.bin', 17, 'application/octet-stream', false);
SQL
for key in "${ATTACHMENT_KEYS[@]}"; do
  mc_ cp "/seed/$(basename "$key")" "local/$BUCKET/$key" >/dev/null
done
EXPECTED_OBJ_CHECKSUM="$(sha256_of "$SEED_DIR/att2.bin" | awk '{print $1}')"

echo "==> taking a policy-compliant backup"
# The drill host may lack PostgreSQL client tools, so point the backup/restore
# scripts at container-native wrappers while keeping the scripts themselves as
# the single source of truth for the backup policy.
TOOL_DIR="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-recovery-tools.XXXXXX")"
cat > "$TOOL_DIR/psql" <<EOF
#!/bin/sh
export PGPASSWORD=omnicraft
exec docker exec -i "$PROJ-postgres-1" psql -h 127.0.0.1 -p 5432 -U omnicraft "\$@"
EOF
cat > "$TOOL_DIR/pg_dump" <<EOF
#!/bin/sh
export PGPASSWORD=omnicraft
exec docker exec -i "$PROJ-postgres-1" pg_dump -h 127.0.0.1 -p 5432 -U omnicraft "\$@"
EOF
cat > "$TOOL_DIR/pg_restore" <<EOF
#!/bin/sh
export PGPASSWORD=omnicraft
exec docker exec -i "$PROJ-postgres-1" pg_restore "\$@"
EOF
chmod +x "$TOOL_DIR"/*

BACKUP_START="$(date +%s)"
PATH="$TOOL_DIR:$PATH" BACKUP_DB_NAME="omnicraft" DB_HOST="127.0.0.1" DB_PORT="5432" DB_USER="omnicraft" DB_PASSWORD="omnicraft" \
  BACKUP_DIR="$BACKUP_DIR" RETAIN_COUNT="7" bash "$REPO_ROOT/scripts/backup-db.sh" >/dev/null
BACKUP_FILE="$(ls -1t "$BACKUP_DIR"/omnicraft_*.custom | head -1)"
[ -n "$BACKUP_FILE" ] || { echo "no backup produced" >&2; exit 1; }
DUMP_CHECKSUM="$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['dump_checksum_sha256'])" "$BACKUP_FILE.manifest.json")"
BACKUP_END="$(date +%s)"

echo "==> seeding stale redis state before the simulated outage"
docker exec "$PROJ"-redis-1 redis-cli SET "drill:cache:user:1" "stale-value" >/dev/null
docker exec "$PROJ"-redis-1 redis-cli RPUSH "drill:queue:pending" "stale-task" >/dev/null

echo "==> simulating a crash: dropping the source database"
OUTAGE_STARTED_EPOCH="$(date +%s)"
docker compose -p "$PROJ" -f "$COMPOSE_FILE" stop redis >/dev/null
docker exec -i "$PROJ"-postgres-1 psql -U omnicraft -d postgres -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'omnicraft' AND pid <> pg_backend_pid();
DROP DATABASE omnicraft;
SQL
RPO_SECONDS=$(( OUTAGE_STARTED_EPOCH - BACKUP_END ))

echo "==> restoring into a new database"
RESTORE_START="$(date +%s)"
CONTAINER_ADMIN_DSN="host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=postgres sslmode=disable"
CONTAINER_RESTORED_DSN="host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=omnicraft_restored sslmode=disable"
VERIFY_DSN="host=127.0.0.1 port=$PG_PORT user=omnicraft password=omnicraft dbname=omnicraft_restored sslmode=disable"
PATH="$TOOL_DIR:$PATH" ADMIN_DB_DSN="$CONTAINER_ADMIN_DSN" bash "$REPO_ROOT/scripts/restore-db.sh" \
  -Backup "$BACKUP_FILE" -TargetDB omnicraft_restored -VerifyDSN "$VERIFY_DSN"
RESTORE_END="$(date +%s)"

echo "==> deleted-object version restore"
OBJECT_RESTORE_START="$(date +%s)"
mc_ rm "local/$BUCKET/fixture/att2.bin" >/dev/null
mc_ undo "local/$BUCKET/fixture/att2.bin" >/dev/null
RESTORED_CONTENT="$(mc_ cat "local/$BUCKET/fixture/att2.bin" 2>/dev/null)"
RESTORED_CHECKSUM="$(printf '%s' "$RESTORED_CONTENT" | sha256_of)"
if [ "$RESTORED_CHECKSUM" != "$EXPECTED_OBJ_CHECKSUM" ]; then
  echo "object version restore produced a different checksum: $RESTORED_CHECKSUM vs $EXPECTED_OBJ_CHECKSUM" >&2
  exit 1
fi
OBJECT_RESTORE_RTO_SECONDS=$(( $(date +%s) - OBJECT_RESTORE_START ))

echo "==> database-to-object reconciliation"
DB_KEYS="$(docker exec "$PROJ"-postgres-1 psql -U omnicraft -d omnicraft_restored -tAc "SELECT oss_key FROM content_attachments ORDER BY oss_key")"
OBJ_KEYS="$(mc_ ls --recursive --json "local/$BUCKET" 2>/dev/null \
  | python3 -c 'import json,sys
out=set()
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    obj=json.loads(line)
    if obj.get("type")=="file":
        out.add(obj["key"])
print("\n".join(sorted(out)))')"
RECONCILE_OK="yes"
[ "$DB_KEYS" = "$OBJ_KEYS" ] || { RECONCILE_OK="no"; echo "reconciliation mismatch:" >&2; echo "DB keys: $DB_KEYS" >&2; echo "Object keys: $OBJ_KEYS" >&2; }
if [ "$RECONCILE_OK" != "yes" ]; then
  exit 1
fi

echo "==> redis clear and rebuild from postgres"
REDIS_REBUILD_START="$(date +%s)"
docker compose -p "$PROJ" -f "$COMPOSE_FILE" start redis >/dev/null
for _ in $(seq 1 60); do
  docker exec "$PROJ"-redis-1 redis-cli ping >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$PROJ"-redis-1 redis-cli ping >/dev/null 2>&1 || { echo "redis did not recover" >&2; exit 1; }
docker exec "$PROJ"-redis-1 redis-cli FLUSHALL >/dev/null
docker exec "$PROJ"-postgres-1 psql -U omnicraft -d omnicraft_restored -tAc \
  "SELECT 'drill:cache:user:' || id || ' ' || username FROM users WHERE id = 1" \
  | while read -r key value; do docker exec "$PROJ"-redis-1 redis-cli SET "$key" "$value" >/dev/null; done
CACHE_VALUE="$(docker exec "$PROJ"-redis-1 redis-cli GET "drill:cache:user:1")"
if [ "$CACHE_VALUE" != "recovery-seed-user" ]; then
  echo "redis rebuild produced wrong value: $CACHE_VALUE" >&2
  exit 1
fi
REDIS_REBUILD_SECONDS=$(( $(date +%s) - REDIS_REBUILD_START ))

echo "==> backend readiness smoke against the restored database"
OVERRIDE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-recovery-override.XXXXXX")"
SERVER_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
printf 'server:\n  port: "%s"\n' "$SERVER_PORT" > "$OVERRIDE_DIR/config_override.yaml"
SERVER_BIN="$TOOL_DIR/omnicraft-recovery-server"
(cd "$BACKEND_DIR" && go build -o "$SERVER_BIN" ./cmd/server)
DB_DSN="$RESTORED_DSN" REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  CONFIG_OVERRIDE_PATH="$OVERRIDE_DIR/config_override.yaml" \
  "$SERVER_BIN" > "$BACKUP_DIR/backend-smoke.log" 2>&1 &
SERVER_PID=$!
SMOKE_OK="no"
for _ in $(seq 1 90); do
  if curl -fsS "http://127.0.0.1:$SERVER_PORT/api/v1/contents?page=1&page_size=1" \
    > "$BACKUP_DIR/backend-smoke-response.json" 2>/dev/null; then
    SMOKE_OK="yes"
    break
  fi
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    break
  fi
  sleep 2
done
kill "$SERVER_PID" >/dev/null 2>&1 || true
wait "$SERVER_PID" >/dev/null 2>&1 || true
SERVER_PID=""
if [ "$SMOKE_OK" != "yes" ]; then
  echo "backend readiness smoke failed; log:" >&2
  tail -20 "$BACKUP_DIR/backend-smoke.log" >&2 || true
  exit 1
fi
SERVICE_RTO_SECONDS=$(( $(date +%s) - OUTAGE_STARTED_EPOCH ))

echo "==> collecting evidence"
RESTORE_RTO_SECONDS=$(( RESTORE_END - RESTORE_START ))
BACKUP_DURATION_SECONDS=$(( BACKUP_END - BACKUP_START ))
LEDGER_ROWS="$(docker exec "$PROJ"-postgres-1 psql -U omnicraft -d omnicraft_restored -tAc "SELECT count(*) FROM schema_migrations")"
DRILL_FINISHED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

RECONCILE_OK_BOOL="false"
[ "$RECONCILE_OK" = "yes" ] && RECONCILE_OK_BOOL="true"
REDIS_OK_BOOL="false"
[ "$CACHE_VALUE" = "recovery-seed-user" ] && REDIS_OK_BOOL="true"
SMOKE_OK_BOOL="false"
[ "$SMOKE_OK" = "yes" ] && SMOKE_OK_BOOL="true"

python3 - "$REPORT_DIR/recovery-drill-evidence.json" \
  "$DRILL_STARTED" "$DRILL_FINISHED" "$BACKUP_FILE" "$DUMP_CHECKSUM" "$LEDGER_ROWS" \
  "$BACKUP_DURATION_SECONDS" "$RESTORE_RTO_SECONDS" "$RPO_SECONDS" "$RESTORED_CHECKSUM" \
  "$OBJECT_RESTORE_RTO_SECONDS" "$REDIS_REBUILD_SECONDS" "$SERVICE_RTO_SECONDS" \
  "$RECONCILE_OK_BOOL" "$REDIS_OK_BOOL" "$SMOKE_OK_BOOL" <<'PY'
import json
import sys

out, started, finished, backup_file, checksum, ledger, backup_dur, restore_dur, rpo, obj_checksum, object_rto, redis_rebuild, service_rto, recon_ok, redis_ok, smoke_ok = sys.argv[1:17]

evidence = {
    "drill": "recovery-drill",
    "started_at": started,
    "finished_at": finished,
    "backup_file": backup_file,
    "dump_checksum_sha256": checksum,
    "restored_database": "omnicraft_restored",
    "restored_ledger_rows": int(ledger),
    "backup_duration_seconds": int(backup_dur),
    "restore_duration_seconds": int(restore_dur),
    "rpo_seconds": int(rpo),
    "rto_seconds": int(restore_dur),
    "object_restore_checksum": obj_checksum,
    "object_restore_rto_seconds": int(object_rto),
    "redis_rebuild_seconds": int(redis_rebuild),
    "service_rto_seconds": int(service_rto),
    "reconciliation_ok": recon_ok == "true",
    "redis_rebuild_ok": redis_ok == "true",
    "backend_smoke_ok": smoke_ok == "true",
    "blockers": ["real Aliyun OSS versioning, off-host storage and service-level RPO/RTO require Ops-08 staging with real credentials"],
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(evidence, f, indent=2)
    f.write("\n")
print(json.dumps(evidence, indent=2))
PY

echo "==> recovery drill passed"
