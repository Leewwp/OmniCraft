#!/usr/bin/env bash
# Contract tests for scripts/release/preflight.sh: effective production
# configuration validation. Fixtures are generated with python3 into a temp
# tree; every case asserts an exact exit code from the preflight wrapper.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PREFLIGHT="$SCRIPT_DIR/preflight.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

if [ ! -f "$PREFLIGHT" ]; then
  echo "preflight.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-preflight-tests.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

# make_fixture writes a valid production .env + override.yaml pair into dir.
make_fixture() {
  local dir="$1"
  mkdir -p "$dir"
  python3 - "$dir" <<'PY'
import sys
out = sys.argv[1]

env = """POSTGRES_USER=omnicraft
POSTGRES_PASSWORD=correct-horse-battery-staple
POSTGRES_DB=omnicraft
OMNICRAFT_PRIVATE_DB_HOSTS=pgbouncer
DB_DSN=host=pgbouncer port=5432 user=omnicraft password=correct-horse-battery-staple dbname=omnicraft sslmode=disable
REDIS_ADDR=redis:6379
REDIS_PASSWORD=redis-strong-secret-8f3a
REDIS_DB=0
JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
LLM_KEY_ENCRYPTION_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef
ALLOWED_ORIGINS=https://app.omnicraft.test
OSS_ENDPOINT=https://oss-cn-hangzhou.aliyuncs.com
OSS_ACCESS_KEY_ID=LTAI5t-real-key
OSS_ACCESS_KEY_SECRET=real-oss-secret
OSS_BUCKET_NAME=omnicraft-private
OSS_DOMAIN=https://omnicraft-private.oss-cn-hangzhou.aliyuncs.com
GREEN_ACCESS_KEY_ID=LTAI5t-green-key
GREEN_ACCESS_KEY_SECRET=real-green-secret
GREEN_REGION=cn-shanghai
GREEN_CALLBACK_URL=https://api.omnicraft.test/api/v1/internal/ai-callback
GREEN_CALLBACK_ALLOWED_IPS=203.0.113.10
CAPTCHA_ACCESS_KEY_ID=LTAI5t-captcha-key
CAPTCHA_ACCESS_KEY_SECRET=real-captcha-secret
SMTP_PASSWORD=smtp-strong-secret
NEXT_PUBLIC_API_URL=https://api.omnicraft.test
INTERNAL_API_URL=https://api.omnicraft.test
NEXT_PUBLIC_SITE_URL=https://app.omnicraft.test
"""
with open(out + "/env", "w") as f:
    f.write(env)

override = """server:
  mode: "release"
  port: "8080"
web:
  public_base_url: "https://app.omnicraft.test"
security:
  allowed_origins:
    - "https://app.omnicraft.test"
  trusted_proxies:
    - "172.16.0.0/12"
features:
  payment_enabled: false
  creator_support_enabled: false
  desktop_deploy_enabled: false
client:
  download_enabled: false
  download_url: ""
  latest_version: ""
captcha:
  provider: "aliyun_v2"
  prefix: "cap-prefix"
  scene_id: "cap-scene"
  region: "cn"
smtp:
  mode: "smtp"
  host: "smtp.omnicraft.test"
  port: 587
  user: "mailer@omnicraft.test"
  from_address: "noreply@omnicraft.test"
legal:
  current_terms_version: "2026-08-07"
  current_privacy_version: "2026-08-07"
observability:
  metrics_port: "9091"
  log_level: "info"
  log_ip_hash_secret: "ip-hash-secret"
  log_ip_key_id: "current"
  readiness:
    db_timeout_sec: 3
    redis_timeout_sec: 3
rate_limit:
  enabled: true
  normal_per_minute: 100
  upload_per_hour: 200
agent:
  web_agent_enabled: false
"""
with open(out + "/override.yaml", "w") as f:
    f.write(override)
PY
}

expect_preflight() {
  local expected="$1" label="$2" dir="$3"
  shift 3
  local actual=0
  OMNICRAFT_PRIVATE_DB_HOSTS="${OMNICRAFT_PRIVATE_DB_HOSTS:-pgbouncer}" \
    bash "$PREFLIGHT" -EnvironmentFile "$dir/env" -OverrideFile "$dir/override.yaml" -ReportDir "$dir/report" "$@" \
    >"$TEMP_ROOT/$label.out" 2>"$TEMP_ROOT/$label.err" || actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL: $label: expected exit $expected, got $actual" >&2
    cat "$TEMP_ROOT/$label.err" >&2
    exit 1
  fi
  echo "OK: $label"
}

# ------------------------------------------------------------ usage errors
rc=0
bash "$PREFLIGHT" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || { echo "FAIL: missing args must exit 2" >&2; exit 1; }
rc=0
bash "$PREFLIGHT" -EnvironmentFile "$TEMP_ROOT/missing.env" -OverrideFile "$TEMP_ROOT/missing.yaml" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 1 ] || { echo "FAIL: missing files must exit 1" >&2; exit 1; }
echo "OK: usage errors"

# --------------------------------------------------------------- valid case
F="$TEMP_ROOT/valid"
make_fixture "$F"
expect_preflight 0 "valid production config accepted" "$F"
[ -f "$F/report/preflight-summary.json" ] || { echo "FAIL: summary missing" >&2; exit 1; }
grep -q '"ok": true' "$F/report/preflight-summary.json" || { echo "FAIL: summary not ok" >&2; exit 1; }
echo "OK: summary written"

# ------------------------------------------------------------ placeholder jwt
F="$TEMP_ROOT/placeholder-jwt"
make_fixture "$F"
python3 - "$F/env" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "<openssl-rand-base64-64>")
open(p, "w").write(s)
PY
expect_preflight 1 "placeholder jwt rejected" "$F"

# --------------------------------------------------- placeholder smtp host
F="$TEMP_ROOT/placeholder-smtp"
make_fixture "$F"
python3 - "$F/override.yaml" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace('host: "smtp.omnicraft.test"', 'host: "<smtp-host>"')
open(p, "w").write(s)
PY
expect_preflight 1 "placeholder smtp host rejected" "$F"

# -------------------------------------------------------------- default dsn
F="$TEMP_ROOT/default-dsn"
make_fixture "$F"
python3 - "$F/env" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace("password=correct-horse-battery-staple", "password=omnicraft")
open(p, "w").write(s)
PY
expect_preflight 1 "default database password rejected" "$F"

# -------------------------------------------------- tls policy: not verify
F="$TEMP_ROOT/tls-require"
make_fixture "$F"
python3 - "$F/env" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace("host=pgbouncer port=5432 user=omnicraft password=correct-horse-battery-staple dbname=omnicraft sslmode=disable",
                           "host=db.omnicraft.test port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=require")
open(p, "w").write(s)
PY
expect_preflight 1 "non-verify-full tls rejected" "$F"

# --------------------------------------------------- tls: private exception
F="$TEMP_ROOT/tls-private"
make_fixture "$F"
expect_preflight 0 "private pgbouncer host allowed with tls negotiation" "$F"

# --------------------------------------------------------- loopback proxies
F="$TEMP_ROOT/loopback-proxies"
make_fixture "$F"
python3 - "$F/override.yaml" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace('  trusted_proxies:\n    - "172.16.0.0/12"', '  trusted_proxies:\n    - "127.0.0.1"')
open(p, "w").write(s)
PY
expect_preflight 1 "loopback-only trusted proxies rejected" "$F"

# --------------------------------------------------------- missing proxies
F="$TEMP_ROOT/missing-proxies"
make_fixture "$F"
python3 - "$F/override.yaml" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace('  trusted_proxies:\n    - "172.16.0.0/12"', '  trusted_proxies: []')
open(p, "w").write(s)
PY
expect_preflight 1 "empty trusted proxies rejected" "$F"

# ------------------------------------------------------------ bad callback
F="$TEMP_ROOT/bad-callback-ip"
make_fixture "$F"
python3 - "$F/env" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace("GREEN_CALLBACK_ALLOWED_IPS=203.0.113.10", "GREEN_CALLBACK_ALLOWED_IPS=not-an-ip")
open(p, "w").write(s)
PY
expect_preflight 1 "invalid callback ip rejected" "$F"

# ------------------------------------------------------------ captcha bypass
F="$TEMP_ROOT/captcha-bypass"
make_fixture "$F"
python3 - "$F/override.yaml" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace('provider: "aliyun_v2"', 'provider: "bypass"')
open(p, "w").write(s)
PY
expect_preflight 1 "bypass captcha rejected" "$F"

# -------------------------------------------------------------- smtp logger
F="$TEMP_ROOT/smtp-logger"
make_fixture "$F"
python3 - "$F/override.yaml" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace('mode: "smtp"', 'mode: "logger"')
open(p, "w").write(s)
PY
expect_preflight 1 "logger smtp rejected" "$F"

# ----------------------------------------------------- missing frontend url
F="$TEMP_ROOT/missing-site-url"
make_fixture "$F"
python3 - "$F/env" <<'PY'
import sys
p = sys.argv[1]
lines = [l for l in open(p).read().splitlines() if not l.startswith("NEXT_PUBLIC_SITE_URL")]
open(p, "w").write("\n".join(lines) + "\n")
PY
expect_preflight 1 "missing frontend site url rejected" "$F"

# ------------------------------------------------- site/api host mismatch
F="$TEMP_ROOT/site-api-mismatch"
make_fixture "$F"
python3 - "$F/env" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace("NEXT_PUBLIC_API_URL=https://api.omnicraft.test", "NEXT_PUBLIC_API_URL=https://api.other.org")
open(p, "w").write(s)
PY
expect_preflight 1 "site-api-host-mismatch rejected" "$F"

# ----------------------------------------------------------- desktop enabled
F="$TEMP_ROOT/desktop-enabled"
make_fixture "$F"
python3 - "$F/override.yaml" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace("desktop_deploy_enabled: false", "desktop_deploy_enabled: true")
open(p, "w").write(s)
PY
expect_preflight 1 "desktop deploy enabled rejected" "$F"

# --------------------------------------------------------- summary redacted
F="$TEMP_ROOT/redaction"
make_fixture "$F"
expect_preflight 0 "redaction fixture accepted" "$F"
if grep -q "correct-horse-battery-staple\|0123456789abcdef0123456789abcdef" "$F/report/preflight-summary.json"; then
  echo "FAIL: summary leaks secret values" >&2
  exit 1
fi
echo "OK: summary redacted"

# ------------------------------------------- config volume must be read-only
F="$TEMP_ROOT/volume-ro"
make_fixture "$F"
cat > "$F/compose.yml" <<'YAML'
services:
  backend:
    volumes:
      - /var/lib/omnicraft/config_override.yaml:/app/config_override.yaml:ro
YAML
expect_preflight 0 "read-only config volume accepted" "$F" -ComposeFile "$F/compose.yml"

F="$TEMP_ROOT/volume-rw"
make_fixture "$F"
cat > "$F/compose.yml" <<'YAML'
services:
  backend:
    volumes:
      - /var/lib/omnicraft/config_override.yaml:/app/config_override.yaml
YAML
expect_preflight 1 "writable config volume rejected" "$F" -ComposeFile "$F/compose.yml"

F="$TEMP_ROOT/volume-missing"
make_fixture "$F"
cat > "$F/compose.yml" <<'YAML'
services:
  backend:
    volumes: []
YAML
expect_preflight 1 "missing config volume rejected" "$F" -ComposeFile "$F/compose.yml"

echo "All preflight contract tests passed"
