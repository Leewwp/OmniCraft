#!/usr/bin/env bash
# =============================================================================
# OmniCraft alerting verification: validates the committed alerting stack
# before any drill or release. Runs pinned promtool (config + rules) and
# amtool (Alertmanager config) checks, enforces the alert rule contract
# (owner/severity/for/summary/impact/first_step/runbook + metric allowlist),
# asserts exporter targets are wired into prometheus.yml, rejects loopback
# routing inside Alertmanager, and validates the external heartbeat config
# against its schema.
#
# Usage:
#   bash scripts/ops/verify-alerts.sh [-ConfigDir ops/observability]
#       [-HeartbeatConfig /path/to/external-heartbeat.json]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONFIG_DIR="$REPO_ROOT/ops/observability"
HEARTBEAT_CONFIG=""
PROMETHEUS_IMAGE="prom/prometheus@sha256:2659f4c2ebb718e7695cb9b25ffa7d6be64db013daba13e05c875451cf51b0d3"
ALERTMANAGER_IMAGE="prom/alertmanager@sha256:27c475db5fb156cab31d5c18a4251ac7ed567746a2483ff264516437a39b15ba"

while [ $# -gt 0 ]; do
  case "$1" in
    -ConfigDir) CONFIG_DIR="$2"; shift 2 ;;
    -HeartbeatConfig) HEARTBEAT_CONFIG="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

CONFIG_DIR="$(cd "$CONFIG_DIR" 2>/dev/null && pwd)" || {
  echo "config dir not found: $CONFIG_DIR" >&2
  exit 1
}

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# ------------------------------------------------------------------ existence
for f in prometheus.yml prometheus-rules.yml alertmanager.yml blackbox.yml \
  exporter-targets.yml alert-contract.schema.json \
  external-heartbeat.schema.json external-heartbeat.example.json; do
  [ -f "$CONFIG_DIR/$f" ] || fail "missing file: $CONFIG_DIR/$f"
done

# ------------------------------------------------------------ pinned tooling
command -v docker >/dev/null 2>&1 || fail "docker is required to run promtool/amtool"

# Pull the pinned images explicitly with retries: fresh runners pull from
# Docker Hub, where transient failures and anonymous-pull rate limits are
# common. Retrying before the checks makes the gate robust without hiding
# the underlying pull error.
ensure_image() {
  local image="$1" attempt
  if docker image inspect "$image" >/dev/null 2>&1; then
    return 0
  fi
  for attempt in 1 2 3; do
    if docker pull "$image" >/dev/null 2>&1; then
      return 0
    fi
    echo "WARN: pull of $image failed (attempt $attempt/3), retrying" >&2
    sleep 5
  done
  return 1
}

ensure_image "$PROMETHEUS_IMAGE" || fail "could not pull $PROMETHEUS_IMAGE"
ensure_image "$ALERTMANAGER_IMAGE" || fail "could not pull $ALERTMANAGER_IMAGE"

if ! promtool_out="$(docker run --rm --entrypoint promtool -v "$CONFIG_DIR:/etc/prometheus:ro" \
  "$PROMETHEUS_IMAGE" check config /etc/prometheus/prometheus.yml 2>&1)"; then
  fail "promtool rejected prometheus.yml: $promtool_out"
fi
if ! promtool_out="$(docker run --rm --entrypoint promtool -v "$CONFIG_DIR:/etc/prometheus:ro" \
  "$PROMETHEUS_IMAGE" check rules /etc/prometheus/prometheus-rules.yml 2>&1)"; then
  fail "promtool rejected prometheus-rules.yml: $promtool_out"
fi
if ! amtool_out="$(docker run --rm --entrypoint amtool -v "$CONFIG_DIR:/etc/alertmanager:ro" \
  "$ALERTMANAGER_IMAGE" check-config /etc/alertmanager/alertmanager.yml 2>&1)"; then
  fail "amtool rejected alertmanager.yml: $amtool_out"
fi

# ------------------------------------------------ alert rule contract (ruby)
# Ruby ships with a YAML stdlib on macOS and CI; it is the only reliable
# YAML parser available without adding dependencies.
ruby - "$CONFIG_DIR/alert-contract.schema.json" "$CONFIG_DIR/prometheus-rules.yml" \
  "$CONFIG_DIR/prometheus.yml" "$CONFIG_DIR/exporter-targets.yml" <<'RUBY' || fail "alert rule contract violated"
require "json"
require "yaml"

schema_path, rules_path, prometheus_path, targets_path = ARGV
schema = JSON.parse(File.read(schema_path))
rules = YAML.load_file(rules_path)
prometheus = YAML.load_file(prometheus_path)

errors = []

def check_rule(rule, schema, errors, index)
  schema["required_rule_fields"].each do |field|
    errors << "rule #{index}: missing field #{field}" unless rule.key?(field) && !rule[field].to_s.empty?
  end
  schema["required_labels"].each do |label|
    value = rule.dig("labels", label)
    errors << "rule #{index}: missing label #{label}" if value.nil? || value.to_s.empty?
  end
  schema["required_annotations"].each do |ann|
    value = rule.dig("annotations", ann)
    errors << "rule #{index}: missing annotation #{ann}" if value.nil? || value.to_s.empty?
  end
  severity = rule.dig("labels", "severity")
  unless schema["allowed_severities"].include?(severity)
    errors << "rule #{index}: severity #{severity.inspect} not allowed"
  end
  for_val = rule["for"]
  unless for_val && for_val.match?(Regexp.new("\\A#{schema["for_regex"]}\\z"))
    errors << "rule #{index}: invalid for value #{for_val.inspect}"
  end
  runbook = rule.dig("annotations", "runbook")
  unless runbook && runbook.match?(Regexp.new(schema["runbook_anchor_regex"]))
    errors << "rule #{index}: runbook must anchor into single-server-beta-runbook.md"
  end
end

def check_expr(rule, schema, errors, index)
  expr = rule["expr"].to_s
  # Reduce the expression to bare metric identifiers: strip string literals,
  # label matcher blocks, time ranges and aggregation label groups, then scan
  # for identifier tokens. Anything left over must be a known PromQL keyword
  # or an allowlisted metric.
  reduced = expr.dup
  reduced.gsub!(/"[^"]*"/, "\"\"")
  reduced.gsub!(/\{[^{}]*\}/, "{}")
  reduced.gsub!(/\[[^\]]*\]/, "[]")
  reduced.gsub!(/\b(?:by|on|ignoring)\s*\([^()]*\)/, "")
  tokens = reduced.scan(/[a-z_][a-z0-9_]*/).uniq
  known = schema["promql_keywords"] + schema["allowed_metrics"]
  tokens.each do |token|
    next if known.include?(token)
    errors << "rule #{index} (#{rule["alert"]}): expression references unknown metric #{token}"
  end
end

rules.fetch("groups", []).each do |group|
  group.fetch("rules", []).each_with_index do |rule, i|
    index = "#{group["name"]}/#{i}"
    check_rule(rule, schema, errors, index)
    check_expr(rule, schema, errors, index)
  end
end

if errors.empty? && rules["groups"].nil?
  errors << "prometheus-rules.yml must contain groups"
end
raise errors.join("\n") unless errors.empty?
RUBY

# ------------------------------------------------------- exporter wiring check
ruby - "$CONFIG_DIR/exporter-targets.yml" "$CONFIG_DIR/prometheus.yml" <<'RUBY' || fail "exporter targets not wired"
require "yaml"
targets = YAML.load_file(ARGV[0])
prometheus = YAML.load_file(ARGV[1])
jobs = prometheus.fetch("scrape_configs", []).map { |c| c["job_name"] }
targets.each do |target, meta|
  unless jobs.include?(meta["job"])
    raise "exporter target #{target} (job #{meta["job"]}) is not scraped in prometheus.yml"
  end
end
RUBY

# --------------------------------------------------------- routing safety check
if grep -nE "127\.0\.0\.1|localhost|0\.0\.0\.0" "$CONFIG_DIR/alertmanager.yml" >/dev/null 2>&1; then
  fail "alertmanager.yml must not reference loopback/unspecified addresses"
fi
if grep -nE "SMTP_(HOST|PORT|FROM|USERNAME|PASSWORD)_PLACEHOLDER|OPS_EMAIL_TO_PLACEHOLDER" \
  "$CONFIG_DIR/alertmanager.yml" >/dev/null 2>&1; then
  :
else
  fail "alertmanager.yml must keep receiver values as placeholders outside Git"
fi

# ------------------------------------------------ external heartbeat schema
python3 - "$CONFIG_DIR/external-heartbeat.schema.json" "$CONFIG_DIR/external-heartbeat.example.json" <<'PY' \
  || fail "external-heartbeat.example.json failed schema validation"
import json, re, sys

schema_path, config_path = sys.argv[1], sys.argv[2]
schema = json.load(open(schema_path, encoding="utf-8"))
config = json.load(open(config_path, encoding="utf-8"))

required = schema.get("required", [])
missing = [f for f in required if f not in config]
if missing:
    print(f"heartbeat config missing required fields: {missing}", file=sys.stderr)
    sys.exit(1)

def const_ok(expected, value):
    return value == expected

checks = [
    ("schema_version", lambda: const_ok(1, config["schema_version"])),
    ("api_base is uri", lambda: bool(re.match(r"^https?://", config["api_base"]))),
    ("ping_url is https", lambda: config["ping_url"].startswith("https://")),
    ("api_key_env name", lambda: bool(re.match(r"^[A-Z_][A-Z0-9_]*$", config["api_key_env"]))),
    ("grace_seconds >= 60", lambda: config["grace_seconds"] >= 60),
    ("timeout_seconds >= 60", lambda: config["timeout_seconds"] >= 60),
]
for name, check in checks:
    if not check():
        print(f"heartbeat config violates: {name}", file=sys.stderr)
        sys.exit(1)
PY

# Optional real heartbeat config (outside Git) must also validate.
if [ -n "$HEARTBEAT_CONFIG" ]; then
  [ -f "$HEARTBEAT_CONFIG" ] || fail "heartbeat config not found: $HEARTBEAT_CONFIG"
  python3 - "$CONFIG_DIR/external-heartbeat.schema.json" "$HEARTBEAT_CONFIG" <<'PY' \
    || fail "heartbeat config failed schema validation"
import json, re, sys

schema_path, config_path = sys.argv[1], sys.argv[2]
schema = json.load(open(schema_path, encoding="utf-8"))
config = json.load(open(config_path, encoding="utf-8"))

required = schema.get("required", [])
missing = [f for f in required if f not in config]
if missing:
    print(f"heartbeat config missing required fields: {missing}", file=sys.stderr)
    sys.exit(1)
for f in ("check_uuid", "email_channel_id", "api_key_env"):
    if not isinstance(config.get(f), str) or not config[f].strip():
        print(f"heartbeat config field {f} must be a non-empty string", file=sys.stderr)
        sys.exit(1)
if not re.match(r"^https?://", config["ping_url"]):
    print("heartbeat ping_url must be an absolute URL", file=sys.stderr)
    sys.exit(1)
PY
fi

echo "OK: alerting configuration contract passed"
