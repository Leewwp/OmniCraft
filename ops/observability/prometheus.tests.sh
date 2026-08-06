#!/usr/bin/env bash
# Contract tests for the ops/observability stack: Prometheus/Loki/Alloy
# configuration, retention and access posture. Requires Docker (promtool via
# the pinned Prometheus image) and a local Go toolchain for the gate build.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG_DIR="$SCRIPT_DIR"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.observability.yml"
PROMETHEUS_IMAGE="prom/prometheus:v2.55.1"

while [ $# -gt 0 ]; do
  case "$1" in
    -ComposeFile) COMPOSE_FILE="$2"; shift 2 ;;
    -ConfigDir) CONFIG_DIR="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[ -f "$COMPOSE_FILE" ] || fail "compose file not found: $COMPOSE_FILE"
[ -f "$CONFIG_DIR/prometheus.yml" ] || fail "prometheus.yml not found"
[ -f "$CONFIG_DIR/loki.yml" ] || fail "loki.yml not found"
[ -f "$CONFIG_DIR/alloy-config.alloy" ] || fail "alloy-config.alloy not found"
[ -f "$CONFIG_DIR/log-retention-policy.json" ] || fail "log-retention-policy.json not found"
[ -f "$CONFIG_DIR/log-retention-policy.schema.json" ] || fail "log-retention-policy.schema.json not found"
[ -f "$CONFIG_DIR/gate/go.mod" ] || fail "gate/go.mod not found"
[ -f "$CONFIG_DIR/gate/main.go" ] || fail "gate/main.go not found"

# 1. Prometheus config must pass promtool.
if ! command -v docker >/dev/null 2>&1; then
  fail "docker is required to run promtool"
fi
docker run --rm --entrypoint promtool -v "$CONFIG_DIR:/etc/prometheus:ro" \
  "$PROMETHEUS_IMAGE" check config /etc/prometheus/prometheus.yml >/dev/null 2>&1 \
  || fail "promtool rejected prometheus.yml"

# 2. Scrape targets must be internal service names, never localhost/127.0.0.1
#    (the metrics endpoint must not be reachable from outside the network).
if rg -n "localhost|127\.0\.0\.1" "$CONFIG_DIR/prometheus.yml" >/dev/null 2>&1; then
  fail "prometheus.yml must not scrape localhost/127.0.0.1 targets"
fi
rg -n "backend:9091" "$CONFIG_DIR/prometheus.yml" >/dev/null 2>&1 \
  || fail "prometheus.yml must scrape the backend internal metrics port backend:9091"
rg -n "node-exporter:9100" "$CONFIG_DIR/prometheus.yml" >/dev/null 2>&1 \
  || fail "prometheus.yml must scrape the node exporter textfile collector"

# 3. Prometheus retention must be bounded by time and disk in the compose file.
rg -n -- "--storage.tsdb.retention.time=30d" "$COMPOSE_FILE" >/dev/null 2>&1 \
  || fail "compose must pin Prometheus time retention to 30d"
rg -n -- "--storage.tsdb.retention.size=" "$COMPOSE_FILE" >/dev/null 2>&1 \
  || fail "compose must pin Prometheus disk retention size"
rg -n "prometheus_data:/prometheus" "$COMPOSE_FILE" >/dev/null 2>&1 \
  || fail "compose must mount a named durable Prometheus volume"

# 4. Loki retention must be explicit (30 days) with a durable named volume.
rg -n "retention_period: 30d" "$CONFIG_DIR/loki.yml" >/dev/null 2>&1 \
  || fail "loki.yml must set retention_period: 30d"
rg -n "LOKI_MAX_DISK_BYTES|loki disk cap exceeded" "$COMPOSE_FILE" >/dev/null 2>&1 \
  || fail "compose must enforce the configured Loki disk cap"
rg -n "retention_enabled: true" "$CONFIG_DIR/loki.yml" >/dev/null 2>&1 \
  || fail "loki.yml must enable compactor retention"
rg -n "loki_data:/loki" "$COMPOSE_FILE" >/dev/null 2>&1 \
  || fail "compose must mount a named durable Loki volume"

# 5. Alloy must have no Docker control capability and only read-only mounts.
if rg -n "docker\.(sd|socket|containers)|discovery\.docker" "$CONFIG_DIR/alloy-config.alloy" >/dev/null 2>&1; then
  fail "alloy-config.alloy must not use Docker discovery/control components"
fi
rg -n "alloy-data|alloy_config|alloy-config.alloy:ro" "$COMPOSE_FILE" >/dev/null 2>&1 \
  || fail "compose must mount the alloy config read-only"
rg -n "/var/log/containers:ro" "$COMPOSE_FILE" >/dev/null 2>&1 \
  || fail "compose must mount the log directory read-only for Alloy"

# 6. Prometheus/Loki must not publish public ports; only the gate may bind a
#    host port, and only on 127.0.0.1.
if rg -n -- "- \"[0-9]+:(9090|3100)\"|- \"127\.0\.0\.1:[0-9]+:(9090|3100)\"" "$COMPOSE_FILE" >/dev/null 2>&1; then
  fail "Prometheus/Loki must not publish host ports"
fi
rg -n '127\.0\.0\.1:\$\{OBS_GATE_PORT[^}]*\}:8080' "$COMPOSE_FILE" >/dev/null 2>&1 \
  || fail "loki-gate must publish only on 127.0.0.1"

# 7. The log retention policy must match its schema (Python subset validator;
#    no external jsonschema dependency on macOS/CI runners).
python3 - "$CONFIG_DIR/log-retention-policy.schema.json" "$CONFIG_DIR/log-retention-policy.json" <<'PY'
import json, sys

schema_path, policy_path = sys.argv[1], sys.argv[2]
schema = json.load(open(schema_path, encoding="utf-8"))
policy = json.load(open(policy_path, encoding="utf-8"))

def const_ok(expected, value):
    return value == expected

missing = [f for f in schema["required"] if f not in policy]
if missing:
    print(f"policy missing required fields: {missing}", file=sys.stderr)
    sys.exit(1)

checks = [
    ("schema_version", lambda: const_ok(1, policy["schema_version"])),
    ("docker_log_driver", lambda: const_ok("json-file", policy["docker_log_driver"])),
    ("loki.retention_days >= 30", lambda: policy["loki"]["retention_days"] >= 30),
    ("loki.storage.max_bytes", lambda: bool(policy["loki"]["storage"].get("max_bytes"))),
    ("alloy.log_mount read-only", lambda: const_ok("read-only", policy["alloy"]["log_mount"])),
    ("alloy.docker_control false", lambda: const_ok(False, policy["alloy"]["docker_control"])),
    ("warning_error_archive.encrypted", lambda: const_ok(True, policy["warning_error_archive"]["encrypted"])),
    ("warning_error_archive.retention_days >= 30", lambda: policy["warning_error_archive"]["retention_days"] >= 30),
    ("reviewed_at", lambda: bool(policy.get("reviewed_at"))),
    ("reviewed_by", lambda: bool(policy.get("reviewed_by"))),
]
for name, check in checks:
    if not check():
        print(f"policy violates: {name}", file=sys.stderr)
        sys.exit(1)
PY
[ $? -eq 0 ] || fail "log-retention-policy.json failed schema validation"

# 8. The gate must build with the local Go toolchain (stdlib only).
if command -v go >/dev/null 2>&1; then
  (cd "$CONFIG_DIR/gate" && GOFLAGS=-mod=mod go build ./...) >/dev/null 2>&1 \
    || fail "loki-gate failed to build"
fi

# 9. All services must pin exact image versions (no floating tags).
for img in prom/prometheus grafana/loki grafana/alloy prom/node-exporter; do
  if rg -n "${img}:[^\"']*" "$COMPOSE_FILE" | rg -v ":[0-9a-zA-Z.]+(\"|')?$" >/dev/null 2>&1; then
    fail "image $img must use a pinned version tag"
  fi
done

echo "OK: observability stack configuration contract passed"
