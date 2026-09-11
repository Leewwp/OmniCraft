#!/usr/bin/env bash
set -Eeuo pipefail

# Repeatable local-only bootstrap for the local demo stack. This script
# never creates or rewrites .env; runtime overrides live under artifacts/.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE_DIR="${OMNICRAFT_LIVE_DEMO_STATE_DIR:-${ROOT_DIR}/artifacts/live-demo}"
LOG_DIR="${STATE_DIR}/logs"
PID_DIR="${STATE_DIR}/pids"
OVERRIDE_PATH="${STATE_DIR}/config-override.yaml"
BACKEND_PORT="${OMNICRAFT_LIVE_DEMO_BACKEND_PORT:-8080}"
FRONTEND_PORT="${OMNICRAFT_LIVE_DEMO_FRONTEND_PORT:-3000}"
OPENSEARCH_URL="${OMNICRAFT_LIVE_DEMO_OPENSEARCH_URL:-http://127.0.0.1:9200}"
TRACING_ENABLED=false
METRICS_PORT="${OMNICRAFT_LIVE_DEMO_METRICS_PORT:-9091}"

usage() {
    cat <<'EOF'
Usage:
  scripts/live-demo.sh start [--full-infra] [--skip-rich-seed] [--skip-rag-seed]
  scripts/live-demo.sh verify [--full-infra]
  scripts/live-demo.sh stop

start boots PostgreSQL/Redis, applies the migration ledger, loads local seed
data, optionally starts OpenSearch, then starts the host backend, worker and
frontend. Runtime logs and PIDs are stored under artifacts/live-demo/.
EOF
}

die() {
    printf 'live-demo: %s\n' "$1" >&2
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

compose() {
    (cd "$ROOT_DIR" && docker compose "$@")
}

wait_for_http() {
    local url="$1"
    local attempts="${2:-60}"
    local i
    for ((i = 1; i <= attempts; i++)); do
        if curl -fsS --max-time 2 "$url" >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    die "timed out waiting for ${url}"
}

port_available() {
    python3 - "$1" <<'PY'
import socket
import sys

with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
    sock.settimeout(0.25)
    raise SystemExit(0 if sock.connect_ex(("127.0.0.1", int(sys.argv[1]))) != 0 else 1)
PY
}

select_host_ports() {
    if ! port_available "$BACKEND_PORT"; then
        local fallback="${OMNICRAFT_LIVE_DEMO_BACKEND_FALLBACK_PORT:-8091}"
        port_available "$fallback" || die "backend port ${BACKEND_PORT} is occupied and fallback port ${fallback} is unavailable; set OMNICRAFT_LIVE_DEMO_BACKEND_PORT"
        printf 'backend port %s is occupied; using fallback %s\n' "$BACKEND_PORT" "$fallback"
        BACKEND_PORT="$fallback"
    fi
    if ! port_available "$FRONTEND_PORT"; then
        local fallback="${OMNICRAFT_LIVE_DEMO_FRONTEND_FALLBACK_PORT:-3001}"
        port_available "$fallback" || die "frontend port ${FRONTEND_PORT} is occupied and fallback port ${fallback} is unavailable; set OMNICRAFT_LIVE_DEMO_FRONTEND_PORT"
        printf 'frontend port %s is occupied; using fallback %s\n' "$FRONTEND_PORT" "$fallback"
        FRONTEND_PORT="$fallback"
    fi
    if ! port_available "$METRICS_PORT"; then
        local fallback="${OMNICRAFT_LIVE_DEMO_METRICS_FALLBACK_PORT:-9092}"
        port_available "$fallback" || die "metrics port ${METRICS_PORT} is occupied and fallback port ${fallback} is unavailable; set OMNICRAFT_LIVE_DEMO_METRICS_PORT"
        printf 'metrics port %s is occupied; using fallback %s\n' "$METRICS_PORT" "$fallback"
        METRICS_PORT="$fallback"
    fi
}

write_runtime_override() {
    mkdir -p "$LOG_DIR" "$PID_DIR"
    # This file is disposable demo state, not a repository or production
    # configuration file. Keep the real provider disabled for local evidence.
    cat >"$OVERRIDE_PATH" <<EOF
features:
  rag_hybrid_enabled: true
agent:
  web_agent_enabled: true
  llm_api_key: ""
  llm_provider: "local_disabled"
observability:
  metrics_port: "$METRICS_PORT"
EOF
}

start_process() {
    local name="$1"
    local cwd="$2"
    local logfile="$LOG_DIR/${name}.log"
    local pidfile="$PID_DIR/${name}.pid"
    shift 2

    if [[ -f "$pidfile" ]]; then
        local old_pid
        old_pid="$(<"$pidfile")"
        if [[ "$old_pid" =~ ^[0-9]+$ ]] && kill -0 "$old_pid" 2>/dev/null; then
            printf 'reusing %s (pid %s)\n' "$name" "$old_pid"
            return 0
        fi
        rm -f "$pidfile"
    fi

    printf 'starting %s; log=%s\n' "$name" "$logfile"
    (
        cd "$cwd"
        exec nohup env \
            CONFIG_OVERRIDE_PATH="$OVERRIDE_PATH" \
            AGENT_WEB_AGENT_ENABLED=true \
            AGENT_LLM_PROVIDER=local_disabled \
            AGENT_LLM_API_KEY=local-disabled-no-network \
            RAG_INDEX_URL="$OPENSEARCH_URL" \
            OBSERVABILITY_TRACING_ENABLED="$TRACING_ENABLED" \
            OTEL_EXPORTER_OTLP_ENDPOINT="127.0.0.1:4317" \
            SERVER_PORT="$BACKEND_PORT" \
            "$@"
    ) >"$logfile" 2>&1 < /dev/null &
    printf '%s\n' "$!" >"$pidfile"
}

start_dependencies() {
    printf '==> PostgreSQL and Redis\n'
    compose up -d postgres redis
    compose ps postgres redis
}

apply_migrations() {
    printf '==> forward-only migration ledger\n'
    (cd "$ROOT_DIR" && DB_DSN="${DB_DSN:-host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=omnicraft sslmode=disable}" \
        bash ./scripts/init-db.sh -SummaryPath "$STATE_DIR/migration-summary.json")
    (cd "$ROOT_DIR" && docker compose exec -T postgres psql -U omnicraft -d omnicraft -At -c \
        "SELECT version FROM schema_migrations WHERE version IN (69,70,71,72) ORDER BY version" | \
        tr '\n' ' ' | grep -Eq '^69 70 71 72 $') || die "migration ledger does not contain 069..072"
}

seed_database() {
    local skip_rich="$1"
    local skip_rag="$2"
    if [[ "$skip_rich" != true ]]; then
        printf '==> local rich UI seed\n'
        (cd "$ROOT_DIR" && python3 scripts/seed_local_rich_data.py seed)
    fi
    if [[ "$skip_rag" != true ]]; then
        printf '==> RAG evaluation fixture\n'
        compose exec -T postgres psql -v ON_ERROR_STOP=1 -U omnicraft -d omnicraft < "$ROOT_DIR/backend/testdata/rag_eval_seed.sql" >/dev/null
    fi
}

start_full_infra() {
    printf '==> OpenSearch full-infra profile\n'
    if curl -fsS --max-time 2 "${OPENSEARCH_URL}/_cluster/health" >/dev/null 2>&1; then
        printf 'reusing reachable OpenSearch at %s (container lifecycle is owned by another local Compose project)\n' "$OPENSEARCH_URL"
    else
        compose --profile full-infra up -d opensearch
        wait_for_http "${OPENSEARCH_URL}/_cluster/health?wait_for_status=yellow" 90
    fi
    (cd "$ROOT_DIR" && OPENSEARCH_URL="$OPENSEARCH_URL" OPENSEARCH_INDEX="omnicraft-rag-v1" sh ops/opensearch/seed.sh)
    wait_for_http "${OPENSEARCH_URL}/_cluster/health?wait_for_status=yellow" 90
    (cd "$ROOT_DIR" && python3 scripts/seed_local_rag_projection.py --opensearch-url "$OPENSEARCH_URL")
    curl -fsS "${OPENSEARCH_URL}/_alias/omnicraft-rag-read" >/dev/null
    printf 'OpenSearch ready: generation=omnicraft-rag-v1 alias=omnicraft-rag-read\n'

    printf '==> Jaeger and OTel Collector (best effort; image failures are recorded)\n'
    if compose --profile full-infra up -d jaeger otel-collector >"$LOG_DIR/full-infra-telemetry-start.log" 2>&1; then
        wait_for_http "http://127.0.0.1:16686" 90
        TRACING_ENABLED=true
        printf 'Jaeger/Collector ready: UI=http://127.0.0.1:16686\n'
    else
        printf 'WARNING: Jaeger/Collector images could not start; see %s/full-infra-telemetry-start.log\n' "$LOG_DIR" >&2
        printf 'telemetry_status=unverified\n' >"$STATE_DIR/telemetry-status.txt"
    fi
}

start_host_services() {
    write_runtime_override
    printf '==> build host backend binaries\n'
    (cd "$ROOT_DIR/backend" && go build -o "$STATE_DIR/server" ./cmd/server && go build -o "$STATE_DIR/worker" ./cmd/worker)
    start_process backend "$ROOT_DIR/backend" "$STATE_DIR/server"
    wait_for_http "http://127.0.0.1:${BACKEND_PORT}/healthz"
    start_process worker "$ROOT_DIR/backend" "$STATE_DIR/worker"
    printf '==> frontend\n'
    start_process frontend "$ROOT_DIR/frontend" env NEXT_PUBLIC_API_URL="http://127.0.0.1:${BACKEND_PORT}" npm run dev -- --hostname 127.0.0.1 --port "$FRONTEND_PORT"
    wait_for_http "http://127.0.0.1:${FRONTEND_PORT}"
}

verify_stack() {
    local full_infra="$1"
    printf '==> service status\n'
    compose ps postgres redis
    wait_for_http "http://127.0.0.1:${BACKEND_PORT}/healthz"
    wait_for_http "http://127.0.0.1:${FRONTEND_PORT}"
    if [[ "$full_infra" == true ]]; then
        wait_for_http "${OPENSEARCH_URL}/_cluster/health?wait_for_status=yellow"
        curl -fsS "${OPENSEARCH_URL}/_alias/omnicraft-rag-read" >/dev/null
    fi
    printf 'live-demo verification passed\n'
}

stop_processes() {
    local pidfile name pid
    shopt -s nullglob
    for pidfile in "$PID_DIR"/*.pid; do
        name="$(basename "$pidfile" .pid)"
        pid="$(<"$pidfile")"
        if [[ "$pid" =~ ^[0-9]+$ ]] && kill -0 "$pid" 2>/dev/null; then
            printf 'stopping %s (pid %s)\n' "$name" "$pid"
            kill "$pid" 2>/dev/null || true
        fi
        rm -f "$pidfile"
    done
}

start() {
    local full_infra=false skip_rich=false skip_rag=false arg
    while [[ $# -gt 0 ]]; do
        arg="$1"
        case "$arg" in
            --full-infra) full_infra=true ;;
            --skip-rich-seed) skip_rich=true ;;
            --skip-rag-seed) skip_rag=true ;;
            *) die "unknown start option: $arg" ;;
        esac
        shift
    done

    [[ -f "$ROOT_DIR/.env" ]] || die ".env is required by Compose; copy .env.example manually, the script will not create it"
    require_command docker
    require_command go
    require_command python3
    require_command curl
    require_command npm
    mkdir -p "$LOG_DIR" "$PID_DIR"
    select_host_ports
    start_dependencies
    apply_migrations
    seed_database "$skip_rich" "$skip_rag"
    if [[ "$full_infra" == true ]]; then
        start_full_infra
    else
        printf 'OpenSearch skipped; Agent RAG will use the configured PostgreSQL fallback.\n'
    fi
    start_host_services
    verify_stack "$full_infra"
}

verify() {
    local full_infra=false
    if [[ "${1:-}" == "--full-infra" ]]; then
        full_infra=true
    elif [[ $# -gt 0 ]]; then
        die "unknown verify option: $1"
    fi
    require_command curl
    verify_stack "$full_infra"
}

stop() {
    stop_processes
    printf 'host demo processes stopped; Docker data is preserved.\n'
}

command_name="${1:-}"
shift || true
case "$command_name" in
    start) start "$@" ;;
    verify) verify "$@" ;;
    stop) [[ $# -eq 0 ]] || die "stop takes no options"; stop ;;
    -h|--help|"") usage ;;
    *) usage; die "unknown command: $command_name" ;;
esac
