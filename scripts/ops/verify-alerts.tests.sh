#!/usr/bin/env bash
# Contract tests for scripts/ops/verify-alerts.sh: argument validation,
# promtool/amtool acceptance of the alerting configuration, rule contract
# (owner/severity/for/runbook/expression references) and heartbeat schema
# validation. Runs without containers on tampered copies where possible;
# promtool/amtool checks run through the pinned images like the real verifier.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VERIFY="$SCRIPT_DIR/verify-alerts.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONFIG_DIR="$REPO_ROOT/ops/observability"

if [ ! -f "$VERIFY" ]; then
  echo "verify-alerts.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-verify-alerts.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

# ---------------------------------------------------------------- usage/args
# No arguments default to the repository config dir, so this must not be a
# usage error (exit 2); the real invocation in the plan runs with no args.
bash "$VERIFY" >/dev/null 2>&1
[ $? -ne 2 ] || { echo "FAIL: no-args must not be a usage error" >&2; exit 1; }
bash "$VERIFY" -ConfigDir "$TEMP_ROOT" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: empty ConfigDir must exit 1" >&2; exit 1; }
bash "$VERIFY" -Bogus x >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: unknown argument must exit 2" >&2; exit 1; }

# Missing files inside ConfigDir -> exit 1.
bash "$VERIFY" -ConfigDir "$TEMP_ROOT/does-not-exist" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: missing ConfigDir must exit 1" >&2; exit 1; }
mkdir -p "$TEMP_ROOT/cfg"
bash "$VERIFY" -ConfigDir "$TEMP_ROOT/cfg" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: ConfigDir without alerting files must exit 1" >&2; exit 1; }

# ---------------------------------------------------------------- file check
# A full valid copy must pass the static file check.
bash "$VERIFY" -ConfigDir "$CONFIG_DIR" >/dev/null 2>&1
[ $? -eq 0 ] || { echo "FAIL: valid config must pass verify-alerts.sh" >&2; exit 1; }

# The committed rules must cover actual outage/pool states without false
# positives from the backend process's default-zero migration gauge.
ruby - "$CONFIG_DIR/prometheus-rules.yml" <<'RUBY'
require "yaml"
rules = YAML.load_file(ARGV[0]).fetch("groups").flat_map { |group| group.fetch("rules") }
by_name = rules.to_h { |rule| [rule.fetch("alert"), rule] }

api_expr = by_name.fetch("ApiUnavailable").fetch("expr")
raise "ApiUnavailable must fire when probe_success is zero" unless api_expr.match?(/probe_success.*==\s*0/m)
raise "ApiUnavailable must fire when the probe series is absent" unless api_expr.include?("absent")

db_pool = by_name.fetch("DatabasePoolExhausted").fetch("expr")
raise "DatabasePoolExhausted must compare in-use and max-open metrics" unless
  db_pool.include?("omnicraft_db_pool_in_use_connections") &&
    db_pool.include?("omnicraft_db_pool_max_open_connections")
raise "DatabasePoolExhausted must ignore an unlimited max-open value of zero" unless
  db_pool.match?(/omnicraft_db_pool_max_open_connections\s*>\s*0/)

redis_pool = by_name.fetch("RedisPoolExhausted").fetch("expr")
raise "RedisPoolExhausted must use total and idle pool metrics" unless
  redis_pool.include?("omnicraft_redis_pool_total_connections") &&
    redis_pool.include?("omnicraft_redis_pool_idle_connections")

migration_expr = by_name.fetch("MigrationFailed").fetch("expr")
raise "MigrationFailed must use the node-exporter textfile metric" unless
  migration_expr.include?('job="node-exporter"')
RUBY
[ $? -eq 0 ] || { echo "FAIL: required alert semantics are missing" >&2; exit 1; }

# Production exporters must use the same external credentials as their
# protected dependencies, and node-exporter must inspect the host rootfs.
PROD_COMPOSE="$REPO_ROOT/docs/deploy/docker-compose.single-server.yml"
grep -qE 'DATA_SOURCE_USER:.*POSTGRES_USER' "$PROD_COMPOSE" \
  && grep -qE 'DATA_SOURCE_PASS:.*POSTGRES_PASSWORD' "$PROD_COMPOSE" \
  && grep -qE 'REDIS_PASSWORD:.*REDIS_PASSWORD' "$PROD_COMPOSE" \
  || { echo "FAIL: production exporters must consume external DB/Redis credentials" >&2; exit 1; }
grep -qE -- '--path.rootfs=/host' "$PROD_COMPOSE" \
  && grep -qE '/:/host:ro' "$PROD_COMPOSE" \
  || { echo "FAIL: production node-exporter must inspect the host rootfs" >&2; exit 1; }

# The required aggregate check must fail when the alerting job fails.
python3 - "$REPO_ROOT/.github/workflows/ci.yml" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
match = re.search(r"(?ms)^  project-gate:\n(.*?)(?=^  [a-zA-Z0-9_-]+:|\Z)", text)
assert match, "project-gate job missing"
job = match.group(1)
assert re.search(r"needs:\s*\[[^\]]*ops-alerting", job), "project-gate does not need ops-alerting"
assert "needs.ops-alerting.result" in job, "project-gate does not assert ops-alerting result"
PY
[ $? -eq 0 ] || { echo "FAIL: project-gate must aggregate ops-alerting" >&2; exit 1; }

# ---------------------------------------------------------------- pinned tooling
# The verifier must run promtool/amtool from digest-pinned images so fresh
# runners and CI always execute the same tool binaries.
python3 - "$VERIFY" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
for var in ("PROMETHEUS_IMAGE", "ALERTMANAGER_IMAGE"):
    m = re.search(r'^%s="([^"]+)"$' % var, text, re.M)
    assert m, "%s is not declared" % var
    assert m.group(1).startswith(("prom/prometheus@", "prom/alertmanager@")), \
        "%s must be a digest-pinned image reference: %s" % (var, m.group(1))
    assert re.match(r"^[^@]+@sha256:[0-9a-f]{64}$", m.group(1)), \
        "%s must pin a full sha256 digest: %s" % (var, m.group(1))
assert "docker pull" in text, "verifier must pull images explicitly for retry"
assert "2>&1" in text, "verifier must surface docker error output"
print("pinned tooling assertions passed")
PY
[ $? -eq 0 ] || { echo "FAIL: verifier must use digest-pinned tool images" >&2; exit 1; }

# ---------------------------------------------------------------- contract: owner/runbook
# A rules file missing a required contract field must be rejected.
cp "$CONFIG_DIR/prometheus-rules.yml" "$TEMP_ROOT/rules-no-owner.yml"
sed -i '' 's/^          owner: /          # owner: /' "$TEMP_ROOT/rules-no-owner.yml" 2>/dev/null \
  || sed -i 's/^          owner: /          # owner: /' "$TEMP_ROOT/rules-no-owner.yml"
mkdir -p "$TEMP_ROOT/no-owner"
cp "$CONFIG_DIR"/prometheus.yml "$CONFIG_DIR"/alertmanager.yml "$CONFIG_DIR"/blackbox.yml \
  "$CONFIG_DIR"/exporter-targets.yml "$CONFIG_DIR"/alert-contract.schema.json \
  "$CONFIG_DIR"/external-heartbeat.schema.json "$CONFIG_DIR"/external-heartbeat.example.json \
  "$TEMP_ROOT/no-owner/"
cp "$TEMP_ROOT/rules-no-owner.yml" "$TEMP_ROOT/no-owner/prometheus-rules.yml"
bash "$VERIFY" -ConfigDir "$TEMP_ROOT/no-owner" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: rule without owner must be rejected" >&2; exit 1; }

# Rule without a runbook anchor must be rejected.
sed 's/^          runbook: /          # runbook: /' "$CONFIG_DIR/prometheus-rules.yml" \
  > "$TEMP_ROOT/rules-no-runbook.yml"
mkdir -p "$TEMP_ROOT/no-runbook"
cp "$CONFIG_DIR"/prometheus.yml "$CONFIG_DIR"/alertmanager.yml "$CONFIG_DIR"/blackbox.yml \
  "$CONFIG_DIR"/exporter-targets.yml "$CONFIG_DIR"/alert-contract.schema.json \
  "$CONFIG_DIR"/external-heartbeat.schema.json "$CONFIG_DIR"/external-heartbeat.example.json \
  "$TEMP_ROOT/no-runbook/"
cp "$TEMP_ROOT/rules-no-runbook.yml" "$TEMP_ROOT/no-runbook/prometheus-rules.yml"
bash "$VERIFY" -ConfigDir "$TEMP_ROOT/no-runbook" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: rule without runbook anchor must be rejected" >&2; exit 1; }

# Rule without resolution semantics must be rejected.
sed 's/^        for: /        # for: /' "$CONFIG_DIR/prometheus-rules.yml" \
  > "$TEMP_ROOT/rules-no-for.yml"
mkdir -p "$TEMP_ROOT/no-for"
cp "$CONFIG_DIR"/prometheus.yml "$CONFIG_DIR"/alertmanager.yml "$CONFIG_DIR"/blackbox.yml \
  "$CONFIG_DIR"/exporter-targets.yml "$CONFIG_DIR"/alert-contract.schema.json \
  "$CONFIG_DIR"/external-heartbeat.schema.json "$CONFIG_DIR"/external-heartbeat.example.json \
  "$TEMP_ROOT/no-for/"
cp "$TEMP_ROOT/rules-no-for.yml" "$TEMP_ROOT/no-for/prometheus-rules.yml"
bash "$VERIFY" -ConfigDir "$TEMP_ROOT/no-for" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: rule without 'for' must be rejected" >&2; exit 1; }

# A rule referencing an unknown metric must be rejected.
sed 's/omnicraft_http_requests_total/omnicraft_http_requests_bogus_total/' \
  "$CONFIG_DIR/prometheus-rules.yml" > "$TEMP_ROOT/rules-bad-expr.yml"
mkdir -p "$TEMP_ROOT/bad-expr"
cp "$CONFIG_DIR"/prometheus.yml "$CONFIG_DIR"/alertmanager.yml "$CONFIG_DIR"/blackbox.yml \
  "$CONFIG_DIR"/exporter-targets.yml "$CONFIG_DIR"/alert-contract.schema.json \
  "$CONFIG_DIR"/external-heartbeat.schema.json "$CONFIG_DIR"/external-heartbeat.example.json \
  "$TEMP_ROOT/bad-expr/"
cp "$TEMP_ROOT/rules-bad-expr.yml" "$TEMP_ROOT/bad-expr/prometheus-rules.yml"
bash "$VERIFY" -ConfigDir "$TEMP_ROOT/bad-expr" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: rule with unknown metric reference must be rejected" >&2; exit 1; }

# ---------------------------------------------------------------- alertmanager
# Alertmanager routing must never reference loopback targets.
sed 's#http://alert-sink:8080/events#http://127.0.0.1:8080/events#' \
  "$CONFIG_DIR/alertmanager.yml" > "$TEMP_ROOT/am-loopback.yml"
mkdir -p "$TEMP_ROOT/am-loopback"
cp "$CONFIG_DIR"/prometheus.yml "$CONFIG_DIR"/prometheus-rules.yml "$CONFIG_DIR"/blackbox.yml \
  "$CONFIG_DIR"/exporter-targets.yml "$CONFIG_DIR"/alert-contract.schema.json \
  "$CONFIG_DIR"/external-heartbeat.schema.json "$CONFIG_DIR"/external-heartbeat.example.json \
  "$TEMP_ROOT/am-loopback/"
cp "$TEMP_ROOT/am-loopback.yml" "$TEMP_ROOT/am-loopback/alertmanager.yml"
bash "$VERIFY" -ConfigDir "$TEMP_ROOT/am-loopback" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: loopback Alertmanager receiver must be rejected" >&2; exit 1; }

# ---------------------------------------------------------------- heartbeat schema
# The heartbeat example must validate; a tampered one must be rejected.
cp "$CONFIG_DIR/external-heartbeat.example.json" "$TEMP_ROOT/hb-bad.json"
sed -i '' 's/"api_key_env": "[^"]*"/"api_key_env": ""/' "$TEMP_ROOT/hb-bad.json" 2>/dev/null \
  || sed 's/"api_key_env": "[^"]*"/"api_key_env": ""/' "$CONFIG_DIR/external-heartbeat.example.json" \
  > "$TEMP_ROOT/hb-bad.json"
bash "$VERIFY" -HeartbeatConfig "$TEMP_ROOT/hb-bad.json" -ConfigDir "$CONFIG_DIR" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: invalid heartbeat config must be rejected" >&2; exit 1; }

echo "OK: verify-alerts contract tests passed"
