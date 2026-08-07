#!/usr/bin/env bash
# =============================================================================
# OmniCraft alert drill: starts the application plus the alerting stack
# (Prometheus with alert rules, Alertmanager, exporters, blackbox and the
# in-network alert-sink) under an isolated Compose project, waits until every
# exporter target reports up == 1, injects API error-rate, dependency-down
# and overdue-recovery-drill conditions, polls the alert-sink for firing and
# resolved payloads, restores health, then exercises the REAL independent
# external heartbeat (Healthchecks.io) and proves the missing-heartbeat
# notification path from the independent provider. Teardown always removes
# containers and volumes. Loopback routing inside Alertmanager and
# synthetic-only heartbeat evidence are forbidden.
#
# Usage:
#   bash scripts/ops/alert-drill.sh \
#     -Environment Local \
#     -ComposeFile ops/observability/docker-compose.observability.yml \
#     -WebhookSink http://alert-sink:8080/events \
#     -ReportDir artifacts/ops-04 \
#     [-HeartbeatEnv ~/.config/omnicraft/ops-04-healthchecks.env] \
#     -HeartbeatNotificationEvidence /path/to/redacted-notification.png \
#     [-DryRun]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OBS_DIR="$REPO_ROOT/ops/observability"
DEFAULT_HEARTBEAT_ENV="$HOME/.config/omnicraft/ops-04-healthchecks.env"
DEFAULT_SMTP_ENV="$HOME/.config/omnicraft/ops-04-smtp.env"

ENVIRONMENT=""
COMPOSE_FILE="$OBS_DIR/docker-compose.observability.yml"
WEBHOOK_SINK="http://alert-sink:8080/events"
REPORT_DIR=""
HEARTBEAT_ENV=""
SMTP_ENV=""
HEARTBEAT_NOTIFICATION_EVIDENCE=""
DRY_RUN=0

notification_evidence_is_fresh() {
  local evidence_path="$1" event_boundary_epoch="$2" evidence_mtime

  [ -s "$evidence_path" ] || return 1
  evidence_mtime="$(python3 -c 'import os, sys; print(int(os.path.getmtime(sys.argv[1])))' "$evidence_path" 2>/dev/null || echo 0)"
  [ "$evidence_mtime" -ge "$event_boundary_epoch" ] 2>/dev/null
}

# Public contract-test seam for the provider-event freshness rule. Keeping the
# check here lets the contract suite exercise the same behavior as a real run
# without starting Docker or calling the external heartbeat provider.
if [ "${1:-}" = "-CheckNotificationEvidenceFreshness" ]; then
  [ "$#" -eq 3 ] || exit 2
  notification_evidence_is_fresh "$2" "$3"
  exit $?
fi

while [ $# -gt 0 ]; do
  case "$1" in
    -Environment) ENVIRONMENT="$2"; shift 2 ;;
    -ComposeFile) COMPOSE_FILE="$2"; shift 2 ;;
    -WebhookSink) WEBHOOK_SINK="$2"; shift 2 ;;
    -ReportDir) REPORT_DIR="$2"; shift 2 ;;
    -HeartbeatEnv) HEARTBEAT_ENV="$2"; shift 2 ;;
    -HeartbeatNotificationEvidence) HEARTBEAT_NOTIFICATION_EVIDENCE="$2"; shift 2 ;;
    -SmptEnv|-SMTPEnv) SMTP_ENV="$2"; shift 2 ;;
    -DryRun) DRY_RUN=1; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ -z "$ENVIRONMENT" ] || [ -z "$REPORT_DIR" ]; then
  echo "usage: alert-drill.sh -Environment Local -ComposeFile <path> -ReportDir <dir>" >&2
  exit 2
fi
case "$ENVIRONMENT" in
  Local|Staging) ;;
  *) echo "ERROR: -Environment must be Local or Staging (got $ENVIRONMENT)" >&2; exit 2 ;;
esac

APP_COMPOSE="$REPO_ROOT/docker-compose.yml"
if [ ! -f "$APP_COMPOSE" ]; then
  echo "application compose not found: $APP_COMPOSE" >&2
  exit 1
fi
if ! grep -qE '^\s+backend:' "$APP_COMPOSE"; then
  echo "application compose $APP_COMPOSE is missing the backend service" >&2
  exit 1
fi
if [ ! -f "$COMPOSE_FILE" ]; then
  echo "observability compose not found: $COMPOSE_FILE" >&2
  exit 1
fi
for service in prometheus alert-sink; do
  if ! grep -qE "^  ${service}:" "$COMPOSE_FILE"; then
    echo "observability compose $COMPOSE_FILE is missing the $service service" >&2
    exit 1
  fi
done
if echo "$WEBHOOK_SINK" | grep -qE "127\.0\.0\.1|localhost"; then
  echo "ERROR: Alertmanager webhook sink must be an in-network service name, not loopback" >&2
  exit 1
fi

REPORT_DIR="$(cd "$(dirname "$REPORT_DIR")" && pwd)/$(basename "$REPORT_DIR")"
mkdir -p "$REPORT_DIR"
chmod 700 "$REPORT_DIR"
umask 077

RUNTIME_DIR="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-alert-drill-runtime.XXXXXX")"
chmod 700 "$RUNTIME_DIR"
PROJ=""
OVERRIDE_FILE=""
STACK_STARTED=0

cleanup() {
  local status=$?
  local teardown_status=0
  trap - EXIT
  set +e
  if [ "$STACK_STARTED" = "1" ]; then
    echo "==> teardown: removing alert drill stack $PROJ"
    docker compose -p "$PROJ" -f "$APP_COMPOSE" -f "$COMPOSE_FILE" -f "$OVERRIDE_FILE" \
      down -v --remove-orphans >/dev/null 2>&1
    teardown_status=$?
  fi
  rm -rf -- "$RUNTIME_DIR"
  if [ "$status" -eq 0 ] && [ "$teardown_status" -ne 0 ]; then
    echo "ERROR: alert drill teardown failed for $PROJ" >&2
    status=$teardown_status
  fi
  exit "$status"
}
trap cleanup EXIT

[ -z "$HEARTBEAT_ENV" ] && [ -f "$DEFAULT_HEARTBEAT_ENV" ] && HEARTBEAT_ENV="$DEFAULT_HEARTBEAT_ENV"
if [ -n "$HEARTBEAT_ENV" ] && [ ! -f "$HEARTBEAT_ENV" ]; then
  echo "ERROR: heartbeat credentials file not found: $HEARTBEAT_ENV" >&2
  exit 1
fi
if [ -z "$HEARTBEAT_ENV" ]; then
  if [ "$DRY_RUN" = "1" ]; then
    echo "WARN: dry run without heartbeat credentials; real drill requires them" >&2
  else
    echo "🚫 BLOCKED: real independent-failure-domain heartbeat credentials required (set -HeartbeatEnv or $DEFAULT_HEARTBEAT_ENV)" >&2
    exit 1
  fi
fi
if [ "$DRY_RUN" = "0" ]; then
  if [ -z "$HEARTBEAT_NOTIFICATION_EVIDENCE" ]; then
    echo "🚫 BLOCKED: a path for fresh redacted missing-heartbeat notification evidence is required (-HeartbeatNotificationEvidence)" >&2
    exit 1
  fi
  if [ ! -d "$(dirname "$HEARTBEAT_NOTIFICATION_EVIDENCE")" ]; then
    echo "ERROR: notification evidence directory does not exist: $(dirname "$HEARTBEAT_NOTIFICATION_EVIDENCE")" >&2
    exit 1
  fi
  HEARTBEAT_NOTIFICATION_EVIDENCE="$(cd "$(dirname "$HEARTBEAT_NOTIFICATION_EVIDENCE")" && pwd)/$(basename "$HEARTBEAT_NOTIFICATION_EVIDENCE")"
fi
if [ -z "$SMTP_ENV" ] && [ -f "$DEFAULT_SMTP_ENV" ]; then
  SMTP_ENV="$DEFAULT_SMTP_ENV"
fi

set -a
if [ -n "$HEARTBEAT_ENV" ]; then
  source "$HEARTBEAT_ENV"
fi
[ -n "$SMTP_ENV" ] && [ -f "$SMTP_ENV" ] && source "$SMTP_ENV"
set +a

# ------------------------------------------------------------- pre-flight: heartbeat
HEARTBEAT_CONFIG="$RUNTIME_DIR/external-heartbeat.json"
if [ -n "$HEARTBEAT_ENV" ]; then
  python3 - "$OBS_DIR/external-heartbeat.schema.json" "$HEARTBEAT_CONFIG" <<'PY' 2>/dev/null && HEARTBEAT_CONFIG_EXISTS=1 || HEARTBEAT_CONFIG_EXISTS=0
import json, sys
schema = json.load(open(sys.argv[1], encoding="utf-8"))
config = json.load(open(sys.argv[2], encoding="utf-8"))
PY
  if [ "$HEARTBEAT_CONFIG_EXISTS" = "0" ]; then
  HC_API_KEY="${HC_API_KEY:-}"
  HC_CHECK_UUID="${HC_CHECK_UUID:-}"
  HC_PING_URL="${HC_PING_URL:-}"
  HC_EMAIL_CHANNEL_ID="${HC_EMAIL_CHANNEL_ID:-}"
  HC_API_BASE="${HC_API_BASE:-https://healthchecks.io/api/v1}"
  for var in HC_API_KEY HC_CHECK_UUID HC_PING_URL HC_EMAIL_CHANNEL_ID; do
    [ -n "${!var}" ] || { echo "🚫 BLOCKED: heartbeat env missing $var in $HEARTBEAT_ENV" >&2; exit 1; }
  done
  cat > "$HEARTBEAT_CONFIG" <<JSON
{
  "schema_version": 1,
  "provider": "healthchecks.io",
  "api_base": "$HC_API_BASE",
  "check_uuid": "$HC_CHECK_UUID",
  "ping_url": "$HC_PING_URL",
  "api_key_env": "HC_API_KEY",
  "email_channel_id": "$HC_EMAIL_CHANNEL_ID",
  "grace_seconds": 3600,
  "timeout_seconds": 3600
}
JSON
  fi
fi
VERIFY_ARGS=(-ConfigDir "$OBS_DIR")
if [ -n "$HEARTBEAT_ENV" ]; then
  VERIFY_ARGS+=(-HeartbeatConfig "$HEARTBEAT_CONFIG")
fi
bash "$SCRIPT_DIR/verify-alerts.sh" "${VERIFY_ARGS[@]}" >/dev/null \
  || { echo "ERROR: heartbeat config failed verification" >&2; exit 1; }

if [ "$DRY_RUN" = "1" ]; then
  echo "OK: alert drill static checks passed (dry run)"
  exit 0
fi
if [ -z "$HEARTBEAT_ENV" ]; then
  echo "🚫 BLOCKED: real heartbeat credentials required for the drill" >&2
  exit 1
fi

command -v docker >/dev/null 2>&1 || { echo "ERROR: docker is required" >&2; exit 1; }

# ------------------------------------------------------------- rendered Alertmanager config
# Receiver values never come from Git: render the committed placeholder
# template with operator SMTP values (if present) into a private temporary
# directory that cleanup removes, and
# route drill notifications to the in-network alert-sink.
RENDERED_AM="$RUNTIME_DIR/alertmanager.runtime.yml"
python3 - "$OBS_DIR/alertmanager.yml" "$RENDERED_AM" "$WEBHOOK_SINK" <<'PY' || { echo "ERROR: failed to render alertmanager config" >&2; exit 1; }
import os, sys
template, out, webhook_sink = sys.argv[1], sys.argv[2], sys.argv[3]
with open(template, encoding="utf-8") as f:
    text = f.read()
values = {
    "SMTP_HOST_PLACEHOLDER": os.environ.get("SMTP_HOST", "SMTP_HOST_PLACEHOLDER"),
    "SMTP_PORT_PLACEHOLDER": os.environ.get("SMTP_PORT", "587"),
    "SMTP_FROM_PLACEHOLDER": os.environ.get("SMTP_FROM_EMAIL", "SMTP_FROM_PLACEHOLDER"),
    "SMTP_USERNAME_PLACEHOLDER": os.environ.get("SMTP_USERNAME", "SMTP_USERNAME_PLACEHOLDER"),
    "SMTP_PASSWORD_PLACEHOLDER": os.environ.get("SMTP_PASSWORD", "SMTP_PASSWORD_PLACEHOLDER"),
    "OPS_EMAIL_TO_PLACEHOLDER": os.environ.get("OPS_EMAIL_TO", os.environ.get("SMTP_FROM_EMAIL", "OPS_EMAIL_TO_PLACEHOLDER")),
}
for k, v in values.items():
    text = text.replace(k, v)
# Route the drill to the in-network webhook sink; SMTP stays configured but
# unused so production routing semantics are preserved.
text = text.replace("receiver: ops-email", "receiver: drill-sink")
text = text.replace("url: http://alert-sink:8080/events", f"url: {webhook_sink}")
with open(out, "w", encoding="utf-8") as f:
    f.write(text)
PY
chmod 600 "$RENDERED_AM"
if grep -qE "127\.0\.0\.1|localhost" "$RENDERED_AM"; then
  echo "ERROR: rendered alertmanager config must not reference loopback" >&2
  exit 1
fi
docker run --rm --entrypoint amtool -v "$(dirname "$RENDERED_AM"):/etc/alertmanager:ro" \
  prom/alertmanager@sha256:27c475db5fb156cab31d5c18a4251ac7ed567746a2483ff264516437a39b15ba check-config /etc/alertmanager/alertmanager.runtime.yml >/dev/null 2>&1 \
  || { echo "ERROR: rendered alertmanager config failed amtool check" >&2; exit 1; }

# ------------------------------------------------------------- isolated project setup
PROJ="omnicraft-alert-drill-$$"
GATE_TOKEN="$(openssl rand -hex 16)"
OBS_GATE_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
OBS_PG_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
OBS_REDIS_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
OBS_BACKEND_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
LOG_DIR="$REPORT_DIR/logs"
mkdir -p "$LOG_DIR"

export OBS_GATE_TOKEN="$GATE_TOKEN"
export OBS_GATE_PORT="$OBS_GATE_PORT"
export OBS_CONFIG_DIR="$OBS_DIR"
export OBS_LOG_DIR="$LOG_DIR"
export OBS_CT_PREFIX="$PROJ"
export METRICS_TEXTFILE_DIR="$REPORT_DIR/textfile"
export ALERTMANAGER_CONFIG_FILE="$RENDERED_AM"
mkdir -p "$METRICS_TEXTFILE_DIR"

OVERRIDE_FILE="$RUNTIME_DIR/ports.override.yml"
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
  alert-sink:
    container_name: "${OBS_CT_PREFIX}-alert-sink"
  alertmanager:
    container_name: "${OBS_CT_PREFIX}-alertmanager"
  postgres-exporter:
    container_name: "${OBS_CT_PREFIX}-postgres-exporter"
  redis-exporter:
    container_name: "${OBS_CT_PREFIX}-redis-exporter"
  cadvisor:
    container_name: "${OBS_CT_PREFIX}-cadvisor"
  blackbox:
    container_name: "${OBS_CT_PREFIX}-blackbox"
YAML

DRILL_STARTED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "==> alert drill started at $DRILL_STARTED (project $PROJ, env $ENVIRONMENT)"

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

wait_for_up() {
  local container="$1" target="$2" tries="${3:-40}"
  local i=0
  while [ "$i" -lt "$tries" ]; do
    state=$(docker exec "$container" wget -qO- \
      "http://localhost:9090/api/v1/query?query=up%7Bjob%3D%22${target}%22%7D" 2>/dev/null || true)
    if echo "$state" | grep -q '"value":\[[0-9.]*,"1"\]'; then
      return 0
    fi
    i=$((i + 1))
    sleep 3
  done
  echo "ERROR: Prometheus does not report target $target up == 1" >&2
  return 1
}

echo "==> starting application + alerting stack (build may take a while)"
STACK_STARTED=1
docker compose -p "$PROJ" -f "$APP_COMPOSE" -f "$COMPOSE_FILE" -f "$OVERRIDE_FILE" \
  up -d --build postgres redis pgbouncer migrate backend prometheus alert-sink \
  alertmanager postgres-exporter redis-exporter cadvisor blackbox >/dev/null

wait_for_http "http://127.0.0.1:${OBS_BACKEND_PORT}/healthz" "backend healthz" 80
wait_for_exec "$PROJ-prometheus" "wget -qO- http://localhost:9090/-/ready" "prometheus" 40
wait_for_exec "$PROJ-alertmanager" "wget -qO- http://localhost:9093/-/ready" "alertmanager" 40
wait_for_exec "$PROJ-alert-sink" "wget -qO- http://localhost:8080/healthz" "alert-sink" 40

echo "==> waiting for every postgres/redis/node/cAdvisor/blackbox target up == 1"
# node-exporter comes from the observability overlay; cadvisor/blackbox/exporters
# from the application compose. Each must be scraped and report up.
docker compose -p "$PROJ" -f "$APP_COMPOSE" -f "$COMPOSE_FILE" -f "$OVERRIDE_FILE" \
  up -d node-exporter >/dev/null 2>&1 || true
wait_for_up "$PROJ-prometheus" "postgres-exporter" 40
wait_for_up "$PROJ-prometheus" "redis-exporter" 40
wait_for_up "$PROJ-prometheus" "node-exporter" 40
wait_for_up "$PROJ-prometheus" "cadvisor" 40
wait_for_up "$PROJ-prometheus" "blackbox" 40

echo "==> confirming baseline: no alerts firing before injection"
docker exec "$PROJ-prometheus" wget -qO- \
  "http://localhost:9090/api/v1/query?query=ALERTS%7Balertstate%3D%22firing%22%7D" \
  > "$REPORT_DIR/baseline-alerts.json"

echo "==> injecting API error-rate, dependency-down and overdue-recovery conditions"
# Stop PostgreSQL: DB-backed API routes now fail with 5xx and the
# postgres-exporter reports pg_up == 0 (PostgresDown). GET requests are
# used so the injection avoids CSRF enforcement and login rate limiting.
# The error rate must be SUSTAINED until the rule's pending (for) window
# elapses: a single burst leaves rate()[5m] at zero. The overdue-recovery
# textfile is written at the same time so all three conditions age in
# parallel.
docker stop "$PROJ-postgres" >/dev/null
sleep 5
cat > "$METRICS_TEXTFILE_DIR/omnicraft_recovery_drill.prom" <<EOF
# HELP omnicraft_recovery_drill_status Latest recovery drill outcome (1 success, 0 failure).
# TYPE omnicraft_recovery_drill_status gauge
omnicraft_recovery_drill_status 1
# HELP omnicraft_recovery_drill_last_success_timestamp_seconds Unix time of the last successful recovery drill.
# TYPE omnicraft_recovery_drill_last_success_timestamp_seconds gauge
omnicraft_recovery_drill_last_success_timestamp_seconds 1000000000
EOF
INJECT_DEADLINE=$((SECONDS + 180))
INJECTION_5XX=0
while [ $SECONDS -lt $INJECT_DEADLINE ]; do
  for i in 1 2 3 4 5; do
    code="$(curl -s -o /dev/null -w '%{http_code}' \
      "http://127.0.0.1:${OBS_BACKEND_PORT}/api/v1/categories?zone=fanwork" || true)"
    echo "$code" >> "$REPORT_DIR/injected-codes.txt"
    case "$code" in
      5*) INJECTION_5XX=$((INJECTION_5XX + 1)) ;;
    esac
  done
  sleep 10
done
[ "$INJECTION_5XX" -ge 10 ] \
  || { echo "ERROR: fewer than 10 5xx responses recorded during injection" >&2; exit 1; }

echo "==> waiting for firing payloads at the alert-sink"
SINK_EVENTS="$REPORT_DIR/sink-events.jsonl"
FIRING_SEEN=""
for i in $(seq 1 120); do
  docker exec "$PROJ-alert-sink" cat /events/events.jsonl > "$SINK_EVENTS" 2>/dev/null || true
  if grep -qE '"status":"firing"' "$SINK_EVENTS" 2>/dev/null; then
    FIRING_SEEN=1
    break
  fi
  sleep 5
done
[ -n "$FIRING_SEEN" ] || { echo "ERROR: no firing payloads delivered to the alert-sink" >&2; exit 1; }
for alert in PostgresDown RecoveryDrillOverdue ApiHigh5xxRate; do
  if ! grep -qE "$alert" "$SINK_EVENTS" 2>/dev/null; then
    echo "ERROR: alert $alert not delivered to the sink" >&2
    exit 1
  fi
done
python3 - "$SINK_EVENTS" <<'PY' || { echo "ERROR: firing evidence invalid" >&2; exit 1; }
import json, sys
firing = set()
for line in open(sys.argv[1], encoding="utf-8"):
    line = line.strip()
    if not line:
        continue
    payload = json.loads(line)
    if payload.get("status") != "firing":
        continue
    for alert in payload.get("alerts", []):
        firing.add(alert.get("labels", {}).get("alertname"))
required = {"PostgresDown", "RecoveryDrillOverdue", "ApiHigh5xxRate"}
missing = required - firing
assert not missing, f"missing firing alerts: {missing}"
PY
echo "==> firing evidence recorded: PostgresDown / RecoveryDrillOverdue / ApiHigh5xxRate"

echo "==> restoring health and waiting for resolved payloads"
docker start "$PROJ-postgres" >/dev/null
wait_for_exec "$PROJ-postgres" "pg_isready -U omnicraft" "postgres after restart" 30
# Restore the recovery drill metric to a fresh timestamp.
python3 - <<'PY'
import time
stamp = int(time.time())
with open(__import__("os").environ["METRICS_TEXTFILE_DIR"] + "/omnicraft_recovery_drill.prom", "w") as f:
    f.write("# HELP omnicraft_recovery_drill_status Latest recovery drill outcome (1 success, 0 failure).\n")
    f.write("# TYPE omnicraft_recovery_drill_status gauge\n")
    f.write("omnicraft_recovery_drill_status 1\n")
    f.write("# HELP omnicraft_recovery_drill_last_success_timestamp_seconds Unix time of the last successful recovery drill.\n")
    f.write("# TYPE omnicraft_recovery_drill_last_success_timestamp_seconds gauge\n")
    f.write(f"omnicraft_recovery_drill_last_success_timestamp_seconds {stamp}\n")
PY
RESOLVED_SEEN=""
for i in $(seq 1 240); do
  docker exec "$PROJ-alert-sink" cat /events/events.jsonl > "$SINK_EVENTS" 2>/dev/null || true
  if python3 - "$SINK_EVENTS" <<'PY' 2>/dev/null
import json, sys
resolved = set()
try:
    for line in open(sys.argv[1], encoding="utf-8"):
        line = line.strip()
        if not line:
            continue
        payload = json.loads(line)
        if payload.get("status") != "resolved":
            continue
        for alert in payload.get("alerts", []):
            resolved.add(alert.get("labels", {}).get("alertname"))
except OSError:
    pass
required = {"PostgresDown", "RecoveryDrillOverdue", "ApiHigh5xxRate"}
sys.exit(0 if required <= resolved else 1)
PY
  then
    RESOLVED_SEEN=1
    break
  fi
  sleep 5
done
[ -n "$RESOLVED_SEEN" ] || { echo "ERROR: no resolved payloads delivered to the alert-sink" >&2; exit 1; }
python3 - "$SINK_EVENTS" <<'PY' || { echo "ERROR: resolved evidence invalid" >&2; exit 1; }
import json, sys
resolved = set()
for line in open(sys.argv[1], encoding="utf-8"):
    line = line.strip()
    if not line:
        continue
    payload = json.loads(line)
    if payload.get("status") != "resolved":
        continue
    for alert in payload.get("alerts", []):
        resolved.add(alert.get("labels", {}).get("alertname"))
required = {"PostgresDown", "RecoveryDrillOverdue", "ApiHigh5xxRate"}
missing = required - resolved
assert not missing, f"missing resolved alerts: {missing}"
PY
echo "==> resolved evidence recorded for all injected alerts"

# ------------------------------------------------------------- real external heartbeat
echo "==> recording independent external heartbeat and notification evidence"
python3 - "$HEARTBEAT_CONFIG" "$REPORT_DIR/heartbeat-evidence.json" <<'PY' || { echo "ERROR: heartbeat drill failed" >&2; exit 1; }
import json, os, subprocess, sys, time, urllib.request, urllib.error

config = json.load(open(sys.argv[1], encoding="utf-8"))
evidence_path = sys.argv[2]
api_key = os.environ.get(config["api_key_env"], "")
if not api_key:
    raise SystemExit("missing API key")

evidence = {"provider": config["provider"], "steps": [], "started_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}

def api(method, path, body=None):
    url = config["api_base"].rstrip("/") + path
    req = urllib.request.Request(url, method=method)
    req.add_header("X-Api-Key", api_key)
    req.add_header("Content-Type", "application/json")
    data = json.dumps(body).encode() if body is not None else None
    try:
        with urllib.request.urlopen(req, data=data, timeout=30) as resp:
            return resp.status, json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        return e.code, {}

def ping(url):
    try:
        req = urllib.request.Request(url, method="POST")
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.status
    except urllib.error.HTTPError as e:
        return e.code

# 1. A real ping to the configured check proves the heartbeat is alive.
status = ping(config["ping_url"])
evidence["steps"].append({"step": "ping_configured_check", "status": status})
assert status == 200, f"configured check ping returned {status}"

# 2. Create a temporary check with the minimum timeout/grace attached to the
#    operator's email channel, prove it flips to "down" after the grace
#    window without pings, then clean it up. This is the real
#    missing-heartbeat notification path from the independent provider.
status, created = api("POST", "/checks/", {
    "name": f"omnicraft-ops04-drill-{int(time.time())}",
    "timeout": 60,
    "grace": 60,
    "channels": config["email_channel_id"],
})
check_name = created.get("name", "")
evidence["check_name"] = check_name
evidence["steps"].append({"step": "create_temp_check", "status": status})
assert status in (200, 201), f"create check failed: {status} {created}"
uuid = created.get("uuid")
temp_ping = created.get("ping_url", "")
assert config["email_channel_id"] in created.get("channels", ""), "email channel not attached"

ping_status = ping(temp_ping)
evidence["steps"].append({"step": "ping_temp_check", "status": ping_status})
assert ping_status == 200, f"temp check ping returned {ping_status}"

# Wait out timeout + grace + margin; the check must flip to "down".
deadline = time.time() + 60 + 60 + 30
state = "new"
while time.time() < deadline:
    status, check = api("GET", f"/checks/{uuid}")
    state = check.get("status", state)
    if state == "down":
        break
    time.sleep(10)
evidence["steps"].append({"step": "wait_missing_heartbeat", "final_status": state,
                          "waited_seconds": 60 + 60 + 30})
assert state == "down", f"temp check did not flip to down (final {state})"

# Cleanup: remove the temporary check.
status, _ = api("DELETE", f"/checks/{uuid}")
evidence["steps"].append({"step": "delete_temp_check", "status": status})
evidence["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())

with open(evidence_path, "w", encoding="utf-8") as f:
    json.dump(evidence, f, indent=2)
PY
HEARTBEAT_EVENT_CONFIRMED_EPOCH="$(date +%s)"

# The provider event must be fresh, so accept only notification evidence that
# the operator captures or refreshes after this run confirms the DOWN event and
# cleans up the temporary check. This wait makes it possible to save the
# email/provider delivery screenshot after the notification arrives.
NOTIFICATION_EVIDENCE_DEADLINE=$(( $(date +%s) + 300 ))
echo "==> waiting up to 300s for fresh redacted notification evidence: $HEARTBEAT_NOTIFICATION_EVIDENCE"
while [ "$(date +%s)" -lt "$NOTIFICATION_EVIDENCE_DEADLINE" ]; do
  if notification_evidence_is_fresh "$HEARTBEAT_NOTIFICATION_EVIDENCE" "$HEARTBEAT_EVENT_CONFIRMED_EPOCH"; then
    break
  fi
  sleep 5
done
if [ ! -s "$HEARTBEAT_NOTIFICATION_EVIDENCE" ]; then
  echo "🚫 BLOCKED: fresh notification evidence was not supplied within 300s" >&2
  exit 1
fi
if ! notification_evidence_is_fresh "$HEARTBEAT_NOTIFICATION_EVIDENCE" "$HEARTBEAT_EVENT_CONFIRMED_EPOCH"; then
  echo "🚫 BLOCKED: notification evidence predates the confirmed DOWN event; capture the current provider delivery" >&2
  exit 1
fi
NOTIFICATION_DEST="$REPORT_DIR/heartbeat-notification-evidence.png"
if [ "$HEARTBEAT_NOTIFICATION_EVIDENCE" != "$NOTIFICATION_DEST" ]; then
  cp "$HEARTBEAT_NOTIFICATION_EVIDENCE" "$NOTIFICATION_DEST"
fi
chmod 600 "$REPORT_DIR/heartbeat-evidence.json" "$REPORT_DIR/heartbeat-notification-evidence.png"
echo "==> heartbeat evidence recorded (real provider down-flip + received notification)"

# ------------------------------------------------------------- final evidence
docker exec "$PROJ-prometheus" wget -qO- \
  "http://localhost:9090/api/v1/query?query=ALERTS" > "$REPORT_DIR/alerts-query.json" 2>/dev/null || true
docker exec "$PROJ-prometheus" wget -qO- \
  "http://localhost:9090/api/v1/targets" > "$REPORT_DIR/targets.json" 2>/dev/null || true

DRILL_FINISHED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
python3 - "$REPORT_DIR/drill-summary.json" "$ENVIRONMENT" "$DRILL_STARTED" "$DRILL_FINISHED" "$PROJ" <<'PY'
import json, sys
(path, env, started, finished, proj) = sys.argv[1:6]
summary = {
    "operation": "alert-drill",
    "environment": env,
    "project": proj,
    "started_at": started,
    "finished_at": finished,
    "targets_up": True,
    "injected": ["ApiHigh5xxRate", "PostgresDown", "RecoveryDrillOverdue"],
    "firing_delivered": True,
    "resolved_delivered": True,
    "external_heartbeat": True,
    "receiver": "webhook-sink",
    "evidence": ["sink-events.jsonl", "heartbeat-evidence.json", "heartbeat-notification-evidence.png", "alerts-query.json", "targets.json", "injected-codes.txt"],
}
with open(path, "w", encoding="utf-8") as out:
    json.dump(summary, out, indent=2)
PY

echo "==> alert drill completed at $DRILL_FINISHED"
echo "  Evidence: $REPORT_DIR"
