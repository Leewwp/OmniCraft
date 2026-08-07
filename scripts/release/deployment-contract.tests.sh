#!/usr/bin/env bash
# Contract tests for scripts/release/deploy.sh and rollback.sh: immutable
# digest enforcement, required evidence (preflight/backup/migration/
# readiness/smoke), previous-digest recording, rollback refusal of unknown or
# schema-incompatible digests, and the guarantee that rollback never runs
# destructive down SQL.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY="$SCRIPT_DIR/deploy.sh"
ROLLBACK="$SCRIPT_DIR/rollback.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCHEMA="$REPO_ROOT/release/deployment-manifest.schema.json"

if [ ! -f "$DEPLOY" ] || [ ! -f "$ROLLBACK" ]; then
  echo "deploy.sh/rollback.sh do not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-deploy.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

# make_fixture creates a valid deployment tree: env, override, candidate
# manifest, previous manifest and a history file.
make_fixture() {
  local dir="$1"
  mkdir -p "$dir"
  python3 - "$dir" <<'PY'
import json, os, sys
out = sys.argv[1]

with open(out + "/env", "w") as f:
    f.write("""POSTGRES_USER=omnicraft
POSTGRES_PASSWORD=correct-horse-battery-staple
POSTGRES_DB=omnicraft
OMNICRAFT_PRIVATE_DB_HOSTS=pgbouncer
DB_DSN=host=pgbouncer port=5432 user=omnicraft password=correct-horse-battery-staple dbname=omnicraft sslmode=disable
REDIS_ADDR=redis:6379
REDIS_PASSWORD=redis-strong-secret
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
""")

with open(out + "/override.yaml", "w") as f:
    f.write("""server:
  mode: "release"
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
captcha:
  provider: "aliyun_v2"
  prefix: "p"
  scene_id: "s"
smtp:
  mode: "smtp"
  host: "smtp.omnicraft.test"
  user: "mailer@omnicraft.test"
  password: "smtp-secret"
  from_address: "noreply@omnicraft.test"
legal:
  current_terms_version: "2026-08-07"
  current_privacy_version: "2026-08-07"
observability:
  metrics_port: "9091"
  log_level: "info"
  log_ip_hash_secret: "ip-hash-secret"
  log_ip_key_id: "current"
rate_limit:
  enabled: true
  normal_per_minute: 100
  upload_per_hour: 200
""")

def manifest(version, commit, digest, previous, head):
    return {
        "schema_version": "1.0",
        "version": version,
        "commit": commit,
        "created_at": "2026-08-07T00:00:00Z",
        "previous_digest": previous,
        "images": {
            "backend": {"ref": "registry.example/omnicraft-backend@" + digest, "digest": digest},
            "frontend": {"ref": "registry.example/omnicraft-frontend@" + digest, "digest": digest},
        },
        "migration": {"head": head},
        "schema_compat": {"max_head": head},
        "preflight": {"status": "ok", "summary": "preflight-summary.json"},
        "backup": {"id": "backup-001", "status": "ok"},
        "readiness": {"status": "ok"},
        "smoke": {"status": "ok"},
        "deployed_at": "2026-08-07T01:00:00Z",
    }

d1 = "sha256:" + "1" * 64
d2 = "sha256:" + "2" * 64
head_old = "060_fix_search_config_fallback.sql"
head_new = "061_add_source_fanwork_id.sql"

with open(out + "/candidate.json", "w") as f:
    json.dump(manifest("v0.2.0", "a" * 40, d2, d1, head_new), f, indent=2)
with open(out + "/previous.json", "w") as f:
    json.dump(manifest("v0.1.0", "b" * 40, d1, None, head_old), f, indent=2)
with open(out + "/history.json", "w") as f:
    json.dump([
        {"digest": d1, "commit": "b" * 40, "migration_head": head_old,
         "schema_compat_max_head": head_old},
    ], f, indent=2)
PY
}

expect_deploy() {
  local expected="$1" label="$2" dir="$3"
  shift 3
  local actual=0
  OMNICRAFT_PRIVATE_DB_HOSTS=pgbouncer \
    bash "$DEPLOY" -Manifest "$dir/candidate.json" -EnvFile "$dir/env" -OverrideFile "$dir/override.yaml" \
      -Schema "$SCHEMA" -ReportDir "$dir/report" -HistoryFile "$dir/history.json" "$@" \
      >"$TEMP_ROOT/$label.out" 2>"$TEMP_ROOT/$label.err" || actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL: $label: expected exit $expected, got $actual" >&2
    cat "$TEMP_ROOT/$label.err" >&2
    exit 1
  fi
  echo "OK: $label"
}

expect_rollback() {
  local expected="$1" label="$2" dir="$3"
  shift 3
  local actual=0
  OMNICRAFT_PRIVATE_DB_HOSTS=pgbouncer \
    bash "$ROLLBACK" -Manifest "$dir/previous.json" -EnvFile "$dir/env" -OverrideFile "$dir/override.yaml" \
      -Schema "$SCHEMA" -ReportDir "$dir/report" -HistoryFile "$dir/history.json" "$@" \
      >"$TEMP_ROOT/$label.out" 2>"$TEMP_ROOT/$label.err" || actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL: $label: expected exit $expected, got $actual" >&2
    cat "$TEMP_ROOT/$label.err" >&2
    exit 1
  fi
  echo "OK: $label"
}

expect_real_deploy() {
  local expected="$1" label="$2" dir="$3" docker_bin="$4" smoke_url="$5"
  local actual=0
  OMNICRAFT_PRIVATE_DB_HOSTS=pgbouncer OMNICRAFT_DOCKER_BIN="$docker_bin" OMNICRAFT_SMOKE_URL="$smoke_url" \
    bash "$DEPLOY" -Manifest "$dir/candidate.json" -EnvFile "$dir/env" -OverrideFile "$dir/override.yaml" \
      -Schema "$SCHEMA" -ReportDir "$dir/report-real" -HistoryFile "$dir/history.json" \
      >"$TEMP_ROOT/$label.out" 2>"$TEMP_ROOT/$label.err" || actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL: $label: expected exit $expected, got $actual" >&2
    cat "$TEMP_ROOT/$label.err" >&2
    exit 1
  fi
  echo "OK: $label"
}

expect_real_rollback() {
  local expected="$1" label="$2" dir="$3" docker_bin="$4" smoke_url="$5"
  local actual=0
  OMNICRAFT_PRIVATE_DB_HOSTS=pgbouncer OMNICRAFT_DOCKER_BIN="$docker_bin" OMNICRAFT_SMOKE_URL="$smoke_url" \
    bash "$ROLLBACK" -Manifest "$dir/previous.json" -EnvFile "$dir/env" -OverrideFile "$dir/override.yaml" \
      -Schema "$SCHEMA" -ReportDir "$dir/report-real" -HistoryFile "$dir/history.json" \
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
bash "$DEPLOY" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || { echo "FAIL: deploy without args must exit 2" >&2; exit 1; }
rc=0
bash "$ROLLBACK" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || { echo "FAIL: rollback without args must exit 2" >&2; exit 1; }
echo "OK: usage errors"

# ------------------------------------------------- floating image digest
F="$TEMP_ROOT/floating"
make_fixture "$F"
python3 - "$F/candidate.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1]))
m["images"]["backend"]["ref"] = "registry.example/omnicraft-backend:latest"
json.dump(m, open(sys.argv[1], "w"), indent=2)
PY
expect_deploy 1 "floating backend image rejected" "$F" -Drill

# ------------------------------------------------- malformed digest shape
F="$TEMP_ROOT/bad-digest"
make_fixture "$F"
python3 - "$F/candidate.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1]))
m["images"]["frontend"]["digest"] = "sha256:short"
m["images"]["frontend"]["ref"] = "registry.example/omnicraft-frontend@sha256:short"
json.dump(m, open(sys.argv[1], "w"), indent=2)
PY
expect_deploy 1 "malformed image digest rejected" "$F" -Drill

# ------------------------------------------------- missing migration head
F="$TEMP_ROOT/no-migration"
make_fixture "$F"
python3 - "$F/candidate.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1]))
del m["migration"]["head"]
json.dump(m, open(sys.argv[1], "w"), indent=2)
PY
expect_deploy 1 "missing migration head rejected" "$F" -Drill

# --------------------------------------------------- preflight fails first
F="$TEMP_ROOT/bad-preflight"
make_fixture "$F"
python3 - "$F/env" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read().replace("correct-horse-battery-staple", "<strong-postgres-password>")
open(p, "w").write(s)
PY
expect_deploy 1 "placeholder env fails preflight before deploy" "$F" -Drill

# ------------------------------------------------------- valid drill deploy
F="$TEMP_ROOT/valid"
make_fixture "$F"
expect_deploy 0 "valid candidate deployed in drill mode" "$F" -Drill
[ -f "$F/report/deployment-manifest.json" ] || { echo "FAIL: deployment manifest missing" >&2; exit 1; }
python3 - "$F/report/deployment-manifest.json" "$F/history.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1]))
h = json.load(open(sys.argv[2]))
assert len(h) >= 2, "history must contain the new and the previous deployment"
assert h[0]["digest"] == "sha256:" + "2" * 64, "new deployment must be first in history"
assert h[1]["digest"] == "sha256:" + "1" * 64, "previous deployment must be preserved"
assert m["previous_digest"] == h[1]["digest"], "previous digest must be recorded"
PY
echo "OK: deployment manifest records previous digest"

# ------------------------------------------------------- real Compose path
# This fake Docker CLI is only a hermetic command recorder. The release
# scripts still execute their real non-drill branches, including Compose
# config/pull/up/exec, health inspection and smoke verification.
F="$TEMP_ROOT/real-compose"
make_fixture "$F"
python3 - "$F/previous.json" "$F/history.json" <<'PY'
import json, sys
for path in sys.argv[1:]:
    data = json.load(open(path))
    if path.endswith("previous.json"):
        data["schema_compat"]["max_head"] = "061_add_source_fanwork_id.sql"
    else:
        data[0]["migration_head"] = "061_add_source_fanwork_id.sql"
    json.dump(data, open(path, "w"), indent=2)
PY
printf 'smoke ok\n' > "$F/smoke.txt"
FAKE_BIN="$F/bin"
mkdir -p "$FAKE_BIN"
cat > "$FAKE_BIN/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "$*" >> "${FAKE_DOCKER_LOG:?}"
if [ "$1" = "inspect" ]; then
  echo healthy
  exit 0
fi
if [ "$1" != "compose" ]; then
  echo "unexpected fake docker command: $*" >&2
  exit 1
fi
shift
while [ "$#" -gt 0 ]; do
  case "$1" in
    --env-file|-f) shift 2 ;;
    *) command_name="$1"; shift; break ;;
  esac
done
case "${command_name:-}" in
  config|pull|up) exit 0 ;;
  ps) echo fake-container; exit 0 ;;
  exec) printf 'fake pg dump\n'; exit 0 ;;
  *) echo "unexpected fake compose command: ${command_name:-}" >&2; exit 1 ;;
esac
EOF
chmod +x "$FAKE_BIN/docker"
FAKE_DOCKER_LOG="$F/docker.log" OMNICRAFT_DOCKER_BIN="$FAKE_BIN/docker" \
  expect_real_deploy 0 "real compose deploy path" "$F" "$FAKE_BIN/docker" "file://$F/smoke.txt"
FAKE_DOCKER_LOG="$F/docker.log" OMNICRAFT_DOCKER_BIN="$FAKE_BIN/docker" \
  expect_real_rollback 0 "real compose rollback path" "$F" "$FAKE_BIN/docker" "file://$F/smoke.txt"
grep -Eq 'compose .*config|compose .*pull|compose .*up|compose .*exec|inspect' "$F/docker.log" || {
  echo "FAIL: real path did not exercise Compose and health inspection" >&2
  exit 1
}
echo "OK: real Compose path exercised"

# ------------------------------------------------- rollback: unknown digest
F="$TEMP_ROOT/rollback-unknown"
make_fixture "$F"
python3 - "$F/history.json" <<'PY'
import json, sys
h = json.load(open(sys.argv[1]))
h[0]["digest"] = "sha256:" + "9" * 64
json.dump(h, open(sys.argv[1], "w"), indent=2)
PY
expect_rollback 1 "rollback to unknown digest refused" "$F" -Drill

# ------------------------------------------- rollback: schema incompatible
F="$TEMP_ROOT/rollback-schema"
make_fixture "$F"
python3 - "$F/previous.json" "$F/history.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1]))
# Previous app only understands migrations up to 060; DB is at 061 now.
m["schema_compat"]["max_head"] = "060_fix_search_config_fallback.sql"
json.dump(m, open(sys.argv[1], "w"), indent=2)
h = json.load(open(sys.argv[2]))
h[0]["migration_head"] = "061_add_source_fanwork_id.sql"
json.dump(h, open(sys.argv[2], "w"), indent=2)
PY
expect_rollback 1 "rollback to schema-incompatible digest refused" "$F" -Drill

# ------------------------------------------------ rollback: valid drill
F="$TEMP_ROOT/rollback-valid"
make_fixture "$F"
expect_rollback 0 "valid rollback executed in drill mode" "$F" -Drill
[ -f "$F/report/rollback-manifest.json" ] || { echo "FAIL: rollback manifest missing" >&2; exit 1; }
echo "OK: rollback manifest written"

# ----------------------------- rollback must never run destructive down SQL
if grep -n "migrate.*down\|--down\|Down" "$ROLLBACK" >/dev/null 2>&1; then
  echo "FAIL: rollback.sh must not run destructive down SQL" >&2
  exit 1
fi
echo "OK: rollback never runs destructive down SQL"

echo "All deployment contract tests passed"
