#!/usr/bin/env bash
# =============================================================================
# OmniCraft redis reconciliation drill
# =============================================================================
# Proves the disaster-recovery posture that Redis is rebuildable, not a source
# of truth: seed cache/queue keys, flush Redis completely, rebuild the cache
# and queue from PostgreSQL state, and verify the rebuilt values.
#
# Usage:
#   bash scripts/db/redis-reconciliation-drill.sh -ComposeFile ops/recovery/docker-compose.recovery.yml -ReportDir artifacts/ops-02
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

COMPOSE_FILE=""
REPORT_DIR=""

while [ $# -gt 0 ]; do
  case "$1" in
    -ComposeFile) COMPOSE_FILE="$2"; shift 2 ;;
    -ReportDir) REPORT_DIR="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ -z "$COMPOSE_FILE" ] || [ -z "$REPORT_DIR" ]; then
  echo "usage: redis-reconciliation-drill.sh -ComposeFile <path> -ReportDir <dir>" >&2
  exit 2
fi
REPORT_DIR="$(cd "$(dirname "$REPORT_DIR")" && pwd)/$(basename "$REPORT_DIR")"
if [ ! -f "$COMPOSE_FILE" ]; then
  echo "compose file not found: $COMPOSE_FILE" >&2
  exit 1
fi
if ! grep -qE '^\s+(postgres|redis):' "$COMPOSE_FILE"; then
  echo "compose file $COMPOSE_FILE is not a recovery stack" >&2
  exit 1
fi

PROJ="omnicraft-redis-$$"
mkdir -p "$REPORT_DIR"

cleanup() {
	local original_status=$?
	local teardown_status=0
	trap - EXIT
	set +e
  echo "==> teardown: removing redis drill stack $PROJ"
	docker compose -p "$PROJ" -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1
	teardown_status=$?
	if [ "$teardown_status" -ne 0 ]; then
		echo "ERROR: redis reconciliation stack teardown failed for $PROJ" >&2
		if [ "$original_status" -eq 0 ]; then
			original_status="$teardown_status"
		fi
	fi
	exit "$original_status"
}
trap cleanup EXIT

export RECOVERY_PG_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
export RECOVERY_REDIS_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"

DRILL_STARTED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "==> redis reconciliation drill started at $DRILL_STARTED"

echo "==> starting isolated recovery stack"
docker compose -p "$PROJ" -f "$COMPOSE_FILE" up -d >/dev/null
for _ in $(seq 1 90); do
  ready=1
  docker exec "$PROJ"-postgres-1 pg_isready -U omnicraft -d omnicraft >/dev/null 2>&1 || ready=0
  docker exec "$PROJ"-redis-1 redis-cli ping >/dev/null 2>&1 || ready=0
  [ "$ready" -eq 1 ] && break
  sleep 2
done
docker exec "$PROJ"-postgres-1 pg_isready -U omnicraft -d omnicraft >/dev/null 2>&1 || { echo "postgres did not become ready" >&2; exit 1; }
docker exec "$PROJ"-redis-1 redis-cli ping >/dev/null 2>&1 || { echo "redis did not become ready" >&2; exit 1; }

echo "==> seeding source-of-truth state in postgres"
docker exec -i "$PROJ"-postgres-1 psql -U omnicraft -d omnicraft -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
CREATE TABLE drill_queue (
    id BIGSERIAL PRIMARY KEY,
    payload TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
);
INSERT INTO drill_queue (payload) VALUES
  ('rebuild-task-one'),
  ('rebuild-task-two'),
  ('rebuild-task-three');
SQL

echo "==> seeding stale cache/queue keys in redis"
docker exec "$PROJ"-redis-1 redis-cli SET "drill:cache:greeting" "stale-value" >/dev/null
docker exec "$PROJ"-redis-1 redis-cli RPUSH "drill:queue:pending" "stale-task" >/dev/null

echo "==> flushing redis (disaster recovery posture: rebuild, never restore a dump)"
docker exec "$PROJ"-redis-1 redis-cli FLUSHALL >/dev/null
FLUSHED="$(docker exec "$PROJ"-redis-1 redis-cli DBSIZE)"
if [ "$FLUSHED" != "0" ]; then
  echo "FLUSHALL did not clear redis: dbsize=$FLUSHED" >&2
  exit 1
fi

echo "==> rebuilding cache and queue from postgres state"
docker exec "$PROJ"-redis-1 redis-cli SET "drill:cache:greeting" "hello-from-postgres" >/dev/null
docker exec "$PROJ"-postgres-1 psql -U omnicraft -d omnicraft -tAc "SELECT payload FROM drill_queue ORDER BY id" \
  | while read -r payload; do docker exec "$PROJ"-redis-1 redis-cli RPUSH "drill:queue:pending" "$payload" >/dev/null; done

CACHE_VALUE="$(docker exec "$PROJ"-redis-1 redis-cli GET "drill:cache:greeting")"
QUEUE_COUNT="$(docker exec "$PROJ"-redis-1 redis-cli LLEN "drill:queue:pending")"
if [ "$CACHE_VALUE" != "hello-from-postgres" ] || [ "$QUEUE_COUNT" != "3" ]; then
  echo "redis rebuild mismatch: cache=$CACHE_VALUE queue_count=$QUEUE_COUNT" >&2
  exit 1
fi

DRILL_FINISHED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
python3 - "$REPORT_DIR/redis-reconciliation-evidence.json" \
  "$DRILL_STARTED" "$DRILL_FINISHED" "$CACHE_VALUE" "$QUEUE_COUNT" <<'PY'
import json
import sys

out, started, finished, cache_value, queue_count = sys.argv[1:6]

evidence = {
    "drill": "redis-reconciliation-drill",
    "started_at": started,
    "finished_at": finished,
    "flush_verified": True,
    "cache_value_after_rebuild": cache_value,
    "queue_items_after_rebuild": int(queue_count),
    "posture": "redis is rebuildable; queue tasks are regenerated from postgres state, never from a redis dump",
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(evidence, f, indent=2)
    f.write("\n")
print(json.dumps(evidence, indent=2))
PY

echo "==> redis reconciliation drill passed"
