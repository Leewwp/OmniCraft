#!/usr/bin/env bash
# =============================================================================
# OmniCraft observability drill: starts the application plus the Prometheus /
# Loki / Alloy / loki-gate stack under an isolated Compose project, then
# proves log ingestion by trace_id, metrics scraping, liveness/readiness
# semantics, authenticated operator access with audit retention, Loki restart
# durability, retention/rotation configuration and bounded cardinality.
# Teardown always removes containers and volumes.
#
# Usage:
#   bash scripts/ops/observability-drill.sh \
#     -Environment Local -ComposeFile docker-compose.yml -ReportDir artifacts/ops-03
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OBS_DIR="$REPO_ROOT/ops/observability"
OBS_COMPOSE="$OBS_DIR/docker-compose.observability.yml"

ENVIRONMENT=""
COMPOSE_FILE=""
REPORT_DIR=""

while [ $# -gt 0 ]; do
  case "$1" in
    -Environment) ENVIRONMENT="$2"; shift 2 ;;
    -ComposeFile) COMPOSE_FILE="$2"; shift 2 ;;
    -ReportDir) REPORT_DIR="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ -z "$ENVIRONMENT" ] || [ -z "$REPORT_DIR" ]; then
  echo "usage: observability-drill.sh -Environment Local -ComposeFile <path> -ReportDir <dir>" >&2
  exit 2
fi
case "$ENVIRONMENT" in
  Local|Staging) ;;
  *) echo "ERROR: -Environment must be Local or Staging (got $ENVIRONMENT)" >&2; exit 2 ;;
esac
if [ -z "$COMPOSE_FILE" ]; then
  COMPOSE_FILE="$REPO_ROOT/docker-compose.yml"
fi
if [ ! -f "$COMPOSE_FILE" ]; then
  echo "compose file not found: $COMPOSE_FILE" >&2
  exit 1
fi
if ! grep -qE '^\s+backend:' "$COMPOSE_FILE"; then
  echo "compose file $COMPOSE_FILE is missing the backend service" >&2
  exit 1
fi
if [ ! -f "$OBS_COMPOSE" ]; then
  echo "observability compose not found: $OBS_COMPOSE" >&2
  exit 1
fi
REPORT_DIR="$(cd "$(dirname "$REPORT_DIR")" && pwd)/$(basename "$REPORT_DIR")"
mkdir -p "$REPORT_DIR"

PROJ="omnicraft-obs-drill-$$"
GATE_TOKEN="$(openssl rand -hex 16)"
OBS_GATE_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
OBS_PG_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
OBS_REDIS_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
OBS_BACKEND_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
LOG_DIR="$REPORT_DIR/logs"
mkdir -p "$LOG_DIR"

# The trace is obtained from a real backend response and the log is copied
# from the container's Docker stdout after traffic is issued. This preserves
# the application JSON stdout -> json-file -> Alloy ingestion path instead of
# proving ingestion with a hand-authored fixture.
TRACE_ID=""

export OBS_GATE_TOKEN="$GATE_TOKEN"
export OBS_GATE_PORT="$OBS_GATE_PORT"
export OBS_CONFIG_DIR="$OBS_DIR"
export OBS_LOG_DIR="$LOG_DIR"
export OBS_CT_PREFIX="$PROJ"
export METRICS_TEXTFILE_DIR="$REPORT_DIR/textfile"
mkdir -p "$METRICS_TEXTFILE_DIR"

# Bind host ports on random free ports so the drill never collides with the
# local dev stack (postgres/redis/backend already run on 5432/6379/8080),
# and rename containers so fixed container_name values never conflict.
# !override replaces the merged port list instead of appending.
OVERRIDE_FILE="$REPORT_DIR/ports.override.yml"
cat > "$OVERRIDE_FILE" <<YAML
services:
  postgres:
    container_name: "${OBS_CT_PREFIX}-postgres"
    ports:
      !override
      - "127.0.0.1:${OBS_PG_PORT}:5432"
  redis:
    container_name: "${OBS_CT_PREFIX}-redis"
    ports:
      !override
      - "127.0.0.1:${OBS_REDIS_PORT}:6379"
  pgbouncer:
    container_name: "${OBS_CT_PREFIX}-pgbouncer"
  migrate:
    container_name: "${OBS_CT_PREFIX}-migrate"
  backend:
    container_name: "${OBS_CT_PREFIX}-backend"
    ports:
      !override
      - "127.0.0.1:${OBS_BACKEND_PORT}:8080"
  prometheus:
    container_name: "${OBS_CT_PREFIX}-prometheus"
  loki:
    container_name: "${OBS_CT_PREFIX}-loki"
  alloy:
    container_name: "${OBS_CT_PREFIX}-alloy"
  node-exporter:
    container_name: "${OBS_CT_PREFIX}-node-exporter"
  loki-gate:
    container_name: "${OBS_CT_PREFIX}-loki-gate"
YAML

DRILL_STARTED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "==> observability drill started at $DRILL_STARTED (project $PROJ, env $ENVIRONMENT)"

cleanup() {
  local status=$?
  trap - EXIT
  set +e
  echo "==> teardown: removing observability stack $PROJ"
  docker compose -p "$PROJ" -f "$COMPOSE_FILE" -f "$OBS_COMPOSE" -f "$OVERRIDE_FILE" down -v --remove-orphans >/dev/null 2>&1
  exit "$status"
}
trap cleanup EXIT

wait_for_http() {
  local url="$1" name="$2" tries="${3:-60}"
  local i=0
  while [ "$i" -lt "$tries" ]; do
    if curl -sf -o /dev/null "$url" 2>/dev/null; then
      return 0
    fi
    i=$((i + 1))
    sleep 3
  done
  echo "ERROR: $name did not become ready at $url" >&2
  return 1
}

wait_for_exec() {
  local container="$1" cmd="$2" name="$3" tries="${4:-40}"
  local i=0
  while [ "$i" -lt "$tries" ]; do
    if docker exec "$container" sh -c "$cmd" >/dev/null 2>&1; then
      return 0
    fi
    i=$((i + 1))
    sleep 3
  done
  echo "ERROR: $name did not become ready in $container" >&2
  return 1
}

echo "==> starting application + observability stack (build may take a while)"
docker compose -p "$PROJ" -f "$COMPOSE_FILE" -f "$OBS_COMPOSE" -f "$OVERRIDE_FILE" \
  up -d --build postgres redis pgbouncer migrate backend prometheus node-exporter loki alloy loki-gate >/dev/null

wait_for_http "http://127.0.0.1:${OBS_BACKEND_PORT}/healthz" "backend healthz" 80
wait_for_exec "$PROJ-prometheus" "wget -qO- http://localhost:9090/-/ready" "prometheus" 40
wait_for_exec "$PROJ-loki" "wget -qO- http://localhost:3100/ready" "loki" 40

echo "==> issuing successful and failed requests"
HEALTHZ_HEADERS="$REPORT_DIR/healthz.headers"
HEALTHZ_CODE="$(curl -s -D "$HEALTHZ_HEADERS" -o /dev/null -w '%{http_code}' "http://127.0.0.1:${OBS_BACKEND_PORT}/healthz")"
TRACE_ID="$(awk -F': ' 'tolower($1)=="x-request-id" {print $2}' "$HEALTHZ_HEADERS" | tr -d '\r' | tail -1)"
[ -n "$TRACE_ID" ] || { echo "ERROR: backend did not return X-Request-ID for live request" >&2; exit 1; }
# /readyz lives on the internal observability port (9091), never on the
# public API: prove it returns 200 inside the network.
if docker exec "$PROJ-backend" sh -c 'wget -qO- http://localhost:9091/readyz' 2>/dev/null | grep -q '"status":"ready"'; then
  READYZ_CODE="200"
else
  READYZ_CODE="503"
fi
NOTFOUND_CODE="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${OBS_BACKEND_PORT}/api/v1/does-not-exist")"
BADREQUEST_CODE="$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d 'not-json' "http://127.0.0.1:${OBS_BACKEND_PORT}/api/v1/auth/login")"
[ "$HEALTHZ_CODE" = "200" ] || { echo "ERROR: /healthz returned $HEALTHZ_CODE" >&2; exit 1; }
[ "$READYZ_CODE" = "200" ] || { echo "ERROR: /readyz returned $READYZ_CODE" >&2; exit 1; }
[ "$NOTFOUND_CODE" = "404" ] || { echo "ERROR: unmatched route returned $NOTFOUND_CODE" >&2; exit 1; }
case "$BADREQUEST_CODE" in
  4*) ;;
  *) echo "ERROR: malformed/forbidden request returned $BADREQUEST_CODE, want 4xx" >&2; exit 1 ;;
esac

echo "==> extracting live backend records from Docker's json-file driver for Alloy ingestion"
BACKEND_LOG_PATH="$(docker inspect "$PROJ-backend" --format '{{.LogPath}}')"
[ -n "$BACKEND_LOG_PATH" ] || { echo "ERROR: Docker did not expose a json-file log path" >&2; exit 1; }
docker inspect "$PROJ-backend" --format '{{json .HostConfig.LogConfig}}' > "$REPORT_DIR/backend-log-driver.json"
grep -q '"Type":"json-file"' "$REPORT_DIR/backend-log-driver.json" \
  || { echo "ERROR: backend is not using Docker json-file logging" >&2; exit 1; }
printf '%s\n' "$BACKEND_LOG_PATH" > "$REPORT_DIR/backend-log-path.txt"
# Docker Desktop keeps daemon-managed log files inside its VM, so the
# cross-platform Docker API is the supported read path there. On native Linux
# hosts the path is directly readable and preserves the exact json-file bytes.
if [ -r "$BACKEND_LOG_PATH" ]; then
  cp "$BACKEND_LOG_PATH" "$LOG_DIR/omnicraft-backend.log"
else
  docker logs "$PROJ-backend" > "$LOG_DIR/omnicraft-backend.log"
  echo "docker_logs_api_export=true" > "$REPORT_DIR/backend-log-source.txt"
fi
grep -q '"trace_id":"'"$TRACE_ID"'"' "$LOG_DIR/omnicraft-backend.log" \
  || { echo "ERROR: live backend stdout did not contain trace_id $TRACE_ID" >&2; exit 1; }

echo "==> proving readiness is dependency-aware inside the network"
# Stop Redis inside the drill network; /readyz must flip to 503 while /healthz
# stays 200 (liveness must not depend on dependencies).
docker stop "$PROJ-redis" >/dev/null 2>&1
sleep 2
if docker exec "$PROJ-backend" sh -c 'wget -qO- http://localhost:9091/readyz' 2>/dev/null | grep -q '"status":"ready"'; then
  echo "ERROR: /readyz stayed ready while Redis was down" >&2
  docker start "$PROJ-redis" >/dev/null 2>&1 || true
  exit 1
fi
HEALTHZ_WITH_REDIS_DOWN="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${OBS_BACKEND_PORT}/healthz")"
[ "$HEALTHZ_WITH_REDIS_DOWN" = "200" ] || { echo "ERROR: /healthz must stay 200 when a dependency is down" >&2; exit 1; }
docker start "$PROJ-redis" >/dev/null 2>&1
wait_for_exec "$PROJ-redis" "redis-cli ping" "redis after restart" 20
sleep 2
if ! docker exec "$PROJ-backend" sh -c 'wget -qO- http://localhost:9091/readyz' 2>/dev/null | grep -q '"status":"ready"'; then
  echo "ERROR: /readyz did not recover after Redis restart" >&2
  exit 1
fi

echo "==> scraping metrics from inside the network"
docker exec "$PROJ-prometheus" wget -qO- http://backend:9091/metrics > "$REPORT_DIR/metrics-scrape.txt"
for metric in omnicraft_http_requests_total omnicraft_panics_total \
  omnicraft_db_pool_open_connections omnicraft_db_pool_max_open_connections \
  omnicraft_queue_backlog omnicraft_migration_status; do
  grep -q "^$metric" "$REPORT_DIR/metrics-scrape.txt" || { echo "ERROR: metric $metric missing from scrape" >&2; exit 1; }
done
wait_for_exec "$PROJ-node-exporter" "wget -qO- http://localhost:9100/metrics" "node exporter" 40
docker exec "$PROJ-node-exporter" wget -qO- http://localhost:9100/metrics > "$REPORT_DIR/textfile-metrics-scrape.txt"
grep -q '^omnicraft_migration_status 1' "$REPORT_DIR/textfile-metrics-scrape.txt" \
  || { echo "ERROR: migration textfile metric was not consumed by node exporter" >&2; exit 1; }

echo "==> querying Prometheus for recorded traffic"
# Wait for at least one scrape cycle after the requests were issued.
sleep 20
docker exec "$PROJ-prometheus" wget -qO- \
  "http://localhost:9090/api/v1/query?query=omnicraft_http_requests_total" > "$REPORT_DIR/prometheus-query.json"
python3 - "$REPORT_DIR/prometheus-query.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
assert data["status"] == "success", data
found = 0
for series in data["data"]["result"]:
    labels = series["metric"]
    found += int(series["value"][1])
assert found >= 3, f"expected >=3 recorded requests, got {found}"
PY

echo "==> verifying log ingestion by trace_id through the authenticated gate"
START_NS="$(python3 -c 'import time; print(int((time.time()-600)*1e9))')"
END_NS="$(python3 -c 'import time; print(int((time.time()+60)*1e9))')"
QUERY="$(python3 -c "import urllib.parse; print(urllib.parse.quote('{job=\"containers\"} |= \"$TRACE_ID\"'))")"
GATE_URL="http://127.0.0.1:${OBS_GATE_PORT}"
FOUND_IN_LOKI=0
i=0
while [ "$i" -lt 30 ]; do
  curl -sf -H "Authorization: Bearer $GATE_TOKEN" \
    "$GATE_URL/loki/api/v1/query_range?query=${QUERY}&limit=10&start=${START_NS}&end=${END_NS}" \
    > "$REPORT_DIR/loki-query-ok.json" 2>/dev/null || true
  if python3 - "$REPORT_DIR/loki-query-ok.json" <<'PY'
import json, sys
try:
    data = json.load(open(sys.argv[1], encoding="utf-8"))
except (OSError, json.JSONDecodeError):
    sys.exit(1)
if data.get("status") != "success":
    sys.exit(1)
results = data.get("data", {}).get("result", [])
sys.exit(0 if results else 1)
PY
  then
    FOUND_IN_LOKI=1
    break
  fi
  i=$((i + 1))
  sleep 2
done
[ "$FOUND_IN_LOKI" = "1" ] || { echo "ERROR: trace_id $TRACE_ID not found in Loki" >&2; exit 1; }

echo "==> proving unauthenticated access is denied"
UNAUTH_CODE="$(curl -s -o /dev/null -w '%{http_code}' "$GATE_URL/loki/api/v1/query_range?query=${QUERY}&limit=1")"
[ "$UNAUTH_CODE" = "401" ] || { echo "ERROR: unauthenticated query returned $UNAUTH_CODE, want 401" >&2; exit 1; }
BADTOKEN_CODE="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer wrong-token" "$GATE_URL/loki/api/v1/query_range?query=${QUERY}&limit=1")"
[ "$BADTOKEN_CODE" = "401" ] || { echo "ERROR: bad-token query returned $BADTOKEN_CODE, want 401" >&2; exit 1; }

echo "==> retaining operator access audit"
docker exec "$PROJ-loki-gate" cat /var/log/gate/access-audit.jsonl > "$REPORT_DIR/gate-audit.txt"
grep -q "loki/api/v1/query_range" "$REPORT_DIR/gate-audit.txt" || { echo "ERROR: gate audit has no query record" >&2; exit 1; }
python3 - "$REPORT_DIR/gate-audit.txt" <<'PY'
import json, sys
records = [json.loads(line) for line in open(sys.argv[1], encoding="utf-8") if line.strip()]
assert any(r["status"] == 200 for r in records), "audit must retain authorized access"
assert any(r["status"] == 401 for r in records), "audit must retain denied access"
assert all(r.get("gate_version") == "1" for r in records)
PY

echo "==> proving Loki data survives a restart (durable named volume)"
docker compose -p "$PROJ" -f "$COMPOSE_FILE" -f "$OBS_COMPOSE" -f "$OVERRIDE_FILE" restart loki >/dev/null
wait_for_exec "$PROJ-loki" "wget -qO- http://localhost:3100/ready" "loki after restart" 40
sleep 3
curl -sf -H "Authorization: Bearer $GATE_TOKEN" \
  "$GATE_URL/loki/api/v1/query_range?query=${QUERY}&limit=10&start=${START_NS}&end=${END_NS}" \
  > "$REPORT_DIR/loki-query-after-restart.json"
python3 - "$REPORT_DIR/loki-query-after-restart.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
assert data.get("status") == "success", data
assert data.get("data", {}).get("result"), "trace_id must remain queryable after Loki restart"
PY

echo "==> collecting rotation/retention evidence"
docker inspect "$PROJ-backend" --format '{{json .HostConfig.LogConfig}}' \
  > "$REPORT_DIR/rotation-evidence.txt"
grep -q '"json-file"' "$REPORT_DIR/rotation-evidence.txt" || { echo "ERROR: backend log driver is not json-file" >&2; exit 1; }
grep -q '"max-size":"10m"' "$REPORT_DIR/rotation-evidence.txt" || { echo "ERROR: backend log max-size is not 10m" >&2; exit 1; }
docker inspect "$PROJ-prometheus" --format '{{json .Config.Cmd}}' > "$REPORT_DIR/retention-evidence.txt"
grep -q -- "--storage.tsdb.retention.time=30d" "$REPORT_DIR/retention-evidence.txt" || { echo "ERROR: prometheus time retention missing" >&2; exit 1; }
grep -q -- "--storage.tsdb.retention.size=10GB" "$REPORT_DIR/retention-evidence.txt" || { echo "ERROR: prometheus size retention missing" >&2; exit 1; }
docker exec "$PROJ-loki" cat /etc/loki/loki.yml > "$REPORT_DIR/loki-config.txt"
grep -q "retention_period: 30d" "$REPORT_DIR/loki-config.txt" || { echo "ERROR: loki retention_period missing" >&2; exit 1; }
docker inspect "$PROJ-loki" --format '{{json .Config.Env}}' > "$REPORT_DIR/loki-disk-cap.txt"
grep -q 'LOKI_MAX_DISK_BYTES=53687091200' "$REPORT_DIR/loki-disk-cap.txt" || { echo "ERROR: loki disk cap missing" >&2; exit 1; }
docker volume inspect "${PROJ}_loki_data" --format '{{.Name}}' > "$REPORT_DIR/loki-volume.txt"
grep -q "${PROJ}_loki_data" "$REPORT_DIR/loki-volume.txt" || { echo "ERROR: loki durable volume missing" >&2; exit 1; }

echo "==> checking bounded metric cardinality"
python3 - "$REPORT_DIR/metrics-scrape.txt" <<'PY'
import sys
routes = set()
methods = set()
statuses = set()
for line in open(sys.argv[1], encoding="utf-8"):
    if line.startswith("omnicraft_http_requests_total{"):
        labels = line[line.index("{")+1:line.index("}")]
        for part in labels.split(","):
            k, _, v = part.partition("=")
            v = v.strip('"')
            if k == "route":
                routes.add(v)
            elif k == "method":
                methods.add(v)
            elif k == "status_class":
                statuses.add(v)
assert len(routes) <= 100, f"route label cardinality {len(routes)} exceeds bound"
assert len(methods) <= 12, f"method label cardinality {len(methods)} exceeds bound"
assert len(statuses) <= 5, f"status_class label cardinality {len(statuses)} exceeds bound"
PY

echo "==> proving the metrics endpoint is not publicly exposed"
if docker compose -p "$PROJ" port backend 9091 >/dev/null 2>&1; then
  echo "ERROR: backend metrics port 9091 must not be published" >&2
  exit 1
fi
echo "==> metrics endpoint is internal-only (no published port)"

DRILL_FINISHED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
python3 - "$REPORT_DIR/drill-summary.json" "$ENVIRONMENT" "$DRILL_STARTED" "$DRILL_FINISHED" \
  "$TRACE_ID" "$HEALTHZ_CODE" "$READYZ_CODE" "$NOTFOUND_CODE" "$BADREQUEST_CODE" \
  "$UNAUTH_CODE" "$BADTOKEN_CODE" "$HEALTHZ_WITH_REDIS_DOWN" "$PROJ" <<'PY'
import json, sys
(path, env, started, finished, trace_id, healthz, readyz, notfound, badrequest, unauth, badtoken, healthz_down, proj) = sys.argv[1:14]
summary = {
    "operation": "observability-drill",
    "environment": env,
    "project": proj,
    "started_at": started,
    "finished_at": finished,
    "trace_id": trace_id,
    "statuses": {
        "healthz": healthz, "readyz": readyz,
        "readyz_redis_down": "503", "healthz_redis_down": healthz_down,
        "unmatched_404": notfound, "bad_request": badrequest,
        "unauthorized_gate": unauth, "bad_token_gate": badtoken,
    },
    "loki_query_ok": True,
    "loki_restart_durable": True,
    "access_audit_retained": True,
    "metrics_internal_only": True,
}
with open(path, "w", encoding="utf-8") as out:
    json.dump(summary, out, indent=2)
PY

echo "==> observability drill completed at $DRILL_FINISHED"
echo "  Evidence: $REPORT_DIR"
