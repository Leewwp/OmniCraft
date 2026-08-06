#!/usr/bin/env bash
# =============================================================================
# OmniCraft load test runner: validates the environment gate and target
# allowlist, then executes the k6 suite for the requested tier. The target
# must be local/staging unless the operator explicitly approves production
# with -AllowProduction plus a matching confirmation token. A summary is
# always written to the report directory, even when k6 fails; k6 threshold
# failures propagate the k6 exit code.
#
# Usage:
#   bash scripts/load/run-load-tests.sh \
#     -Environment Local -Tier Smoke -Target http://127.0.0.1:8080 \
#     -Profile tests/load/k6/release-profile.json \
#     -ReportDir artifacts/ops-07 -RunName smoke
#
# Optional:
#   -AllowProduction     allow a target outside the local/staging allowlist
#   -ConfirmationToken   token required together with -AllowProduction;
#                        must equal the OMNICRAFT_LOAD_PROD_CONFIRM_TOKEN env
#   -DryRun              validate arguments, profile and allowlist only
#   -K6Bin <path>        k6 executable (default: k6 on PATH)
#   -ScriptDir <dir>     k6 suite directory (default: tests/load/k6)
#   -SeedDb <container>  PostgreSQL container used to seed and clean up the
#                        isolated test identities declared in testdata.json;
#                        without it no DB seeding happens (contract tests,
#                        or workflows that pre-seed their own identities)
#   -DbDsn <dsn>         PostgreSQL DSN used for seeding/cleanup when the DB
#                        is not reachable through a docker container (CI
#                        service containers); requires psql on PATH
#   -ContentId <id>      content id used by the anonymous detail path
#                        (default: resolved from the seeded DB)
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
K6_DIR="$REPO_ROOT/tests/load/k6"

ENVIRONMENT=""
TIER=""
TARGET=""
PROFILE=""
REPORT_DIR=""
RUN_NAME=""
ALLOW_PRODUCTION=0
CONFIRMATION_TOKEN=""
DRY_RUN=0
K6_BIN="k6"
SCRIPT_DIR_OVERRIDE=""
SEED_DB=""
DB_DSN=""
CONTENT_ID=""
READINESS_TIMEOUT_SEC=30
SKIP_READY=0
SKIP_METRICS=0

while [ $# -gt 0 ]; do
  case "$1" in
    -Environment) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; ENVIRONMENT="$2"; shift 2 ;;
    -Tier) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; TIER="$2"; shift 2 ;;
    -Target) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; TARGET="$2"; shift 2 ;;
    -Profile) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; PROFILE="$2"; shift 2 ;;
    -ReportDir) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; REPORT_DIR="$2"; shift 2 ;;
    -RunName) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; RUN_NAME="$2"; shift 2 ;;
    -AllowProduction) ALLOW_PRODUCTION=1; shift ;;
    -ConfirmationToken) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; CONFIRMATION_TOKEN="$2"; shift 2 ;;
    -DryRun) DRY_RUN=1; shift ;;
    -K6Bin) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; K6_BIN="$2"; shift 2 ;;
    -ScriptDir) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; SCRIPT_DIR_OVERRIDE="$2"; shift 2 ;;
    -SeedDb) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; SEED_DB="$2"; shift 2 ;;
    -DbDsn) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; DB_DSN="$2"; shift 2 ;;
    -ContentId) [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }; CONTENT_ID="$2"; shift 2 ;;
    -SkipReady) SKIP_READY=1; shift ;;
    -SkipMetrics) SKIP_METRICS=1; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ -n "$SCRIPT_DIR_OVERRIDE" ]; then
  K6_DIR="$SCRIPT_DIR_OVERRIDE"
fi

usage() {
  echo "usage: run-load-tests.sh -Environment Local|Staging -Tier Smoke|Load|Stress -Target <url> -Profile <path> -ReportDir <dir> -RunName <name> [-AllowProduction -ConfirmationToken <token>] [-DryRun]" >&2
}

if [ -z "$ENVIRONMENT" ] || [ -z "$TIER" ] || [ -z "$TARGET" ] || [ -z "$PROFILE" ] || [ -z "$REPORT_DIR" ] || [ -z "$RUN_NAME" ]; then
  usage
  exit 2
fi
case "$ENVIRONMENT" in
  Local|Staging) ;;
  *) echo "ERROR: -Environment must be Local or Staging (got $ENVIRONMENT)" >&2; exit 2 ;;
esac
case "$TIER" in
  Smoke|Load|Stress) ;;
  *) echo "ERROR: -Tier must be Smoke, Load or Stress (got $TIER)" >&2; exit 2 ;;
esac

if [ ! -f "$PROFILE" ]; then
  echo "release profile not found: $PROFILE" >&2
  exit 1
fi

# ---------------------------------------------------------------- target safety
# Target URL must parse and carry a scheme we can drive with k6.
target_scheme=""
target_host=""
if ! python3 - "$TARGET" <<'PY' >/dev/null 2>&1
import sys
from urllib.parse import urlsplit
u = urlsplit(sys.argv[1])
if u.scheme not in ("http", "https") or not u.netloc:
    sys.exit(1)
print(u.scheme)
print(u.hostname)
PY
then
  echo "invalid target URL: $TARGET" >&2
  exit 1
fi
target_scheme="$(python3 - "$TARGET" <<'PY'
import sys
from urllib.parse import urlsplit
print(urlsplit(sys.argv[1]).scheme)
PY
)"
target_host="$(python3 - "$TARGET" <<'PY'
import sys
from urllib.parse import urlsplit
print(urlsplit(sys.argv[1]).hostname)
PY
)"

allowed=0
case "$ENVIRONMENT" in
  Local)
    case "$target_host" in
      127.0.0.1|localhost|::1) allowed=1 ;;
    esac
    ;;
  Staging)
    if python3 - "$PROFILE" "$TARGET" <<'PY' >/dev/null 2>&1
import json, sys
from urllib.parse import urlsplit
profile = json.load(open(sys.argv[1]))
target_host = urlsplit(sys.argv[2]).hostname
for entry in profile.get("staging_targets", []):
    if urlsplit(entry).hostname == target_host:
        sys.exit(0)
sys.exit(1)
PY
    then
      allowed=1
    fi
    ;;
esac

if [ "$allowed" -eq 0 ]; then
  if [ "$ALLOW_PRODUCTION" -eq 1 ] && [ -n "$CONFIRMATION_TOKEN" ]; then
    expected="${OMNICRAFT_LOAD_PROD_CONFIRM_TOKEN:-}"
    if [ -z "$expected" ]; then
      echo "ERROR: OMNICRAFT_LOAD_PROD_CONFIRM_TOKEN is not set; refusing production target" >&2
      exit 1
    fi
    if [ "$CONFIRMATION_TOKEN" != "$expected" ]; then
      echo "ERROR: confirmation token does not match OMNICRAFT_LOAD_PROD_CONFIRM_TOKEN" >&2
      exit 1
    fi
    allowed=1
  else
    echo "ERROR: target $TARGET is outside the $ENVIRONMENT allowlist; use -AllowProduction plus -ConfirmationToken only for an explicitly approved production run" >&2
    exit 1
  fi
fi

# ------------------------------------------------------------------- profile
# Profile must be valid JSON with the fields the runner needs.
if ! python3 - "$PROFILE" <<'PY' >/dev/null 2>&1
import json, sys
profile = json.load(open(sys.argv[1]))
for key in ("version", "endpoints", "tiers", "approval"):
    if key not in profile:
        sys.exit(1)
if not isinstance(profile["tiers"], dict) or not isinstance(profile["approval"], dict) or not isinstance(profile["endpoints"], dict):
    sys.exit(1)
PY
then
  echo "invalid release profile JSON: $PROFILE" >&2
  exit 1
fi

# The tier must be declared in the profile.
if ! python3 - "$PROFILE" "$TIER" <<'PY' >/dev/null 2>&1
import json, sys
profile = json.load(open(sys.argv[1]))
if sys.argv[2].lower() not in profile["tiers"]:
    sys.exit(1)
PY
then
  echo "ERROR: tier $TIER is not declared in the release profile" >&2
  exit 1
fi

# --------------------------------------------------------------- k6 presence
if [ ! -x "$K6_BIN" ] && [ "$DRY_RUN" -eq 0 ]; then
  if command -v "$K6_BIN" >/dev/null 2>&1; then
    :
  else
    echo "k6 not found: $K6_BIN (install with 'brew install k6' or pass -K6Bin)" >&2
    exit 1
  fi
fi
TIER_FILE="$(printf '%s' "$TIER" | tr '[:upper:]' '[:lower:]')"
if [ ! -f "$K6_DIR/$TIER_FILE.js" ]; then
  echo "k6 scenario not found: $K6_DIR/$TIER_FILE.js" >&2
  exit 1
fi

REPORT_DIR_ABS="$(cd "$(dirname "$REPORT_DIR")" 2>/dev/null && pwd || printf '%s' "$(dirname "$REPORT_DIR")")/$(basename "$REPORT_DIR")"
REPORT_DIR="$REPORT_DIR_ABS"
mkdir -p "$REPORT_DIR"
RUN_NAME_SAFE="$(printf '%s' "$RUN_NAME" | tr -cd 'A-Za-z0-9._-')"
if [ -z "$RUN_NAME_SAFE" ]; then
  echo "ERROR: -RunName must contain only letters, digits, dot, dash or underscore" >&2
  exit 1
fi

# ---------------------------------------------------------------- DB seeding
# Seed the isolated test identities declared in testdata.json and resolve a
# real content id for the anonymous detail path. Cleanup runs on exit.
# Two transports: -SeedDb <container> (docker exec) or -DbDsn <dsn> (psql).
SEEDED_EMAILS=""
DB_RUN() {
  local sql="$1"
  if [ -n "$SEED_DB" ]; then
    docker exec "$SEED_DB" psql -U omnicraft -d omnicraft -v ON_ERROR_STOP=1 -tAc "$sql"
  elif [ -n "$DB_DSN" ]; then
    psql "$DB_DSN" -v ON_ERROR_STOP=1 -tAc "$sql"
  fi
}
cleanup_test_data() {
  if { [ -n "$SEED_DB" ] || [ -n "$DB_DSN" ]; } && [ -n "$SEEDED_EMAILS" ]; then
    if DB_RUN "DELETE FROM users WHERE email LIKE 'load-test-%@omnicraft.local'" >/dev/null 2>&1; then
      echo "cleaned up load-test identities (cascade removed their content)"
    else
      echo "ERROR: failed to clean up load-test identities" >&2
      exit 1
    fi
  fi
}
trap cleanup_test_data EXIT

seed_test_data() {
  local hash count email
  hash="$(python3 -c "import json;print(json.load(open('$K6_DIR/testdata.json'))['identities']['password_hash'])")"
  if ! python3 -c "import re,sys; sys.exit(0 if re.match(r'^\\\$(2a|2b|2y|argon2)', sys.argv[1]) else 1)" "$hash" 2>/dev/null; then
    echo "ERROR: invalid bcrypt password_hash in testdata.json" >&2
    return 1
  fi
  count="$(python3 -c "import json;print(json.load(open('$K6_DIR/testdata.json'))['identities']['count'])")"
  if ! python3 -c "import sys; sys.exit(0 if int(sys.argv[1]) in range(1, 100) else 1)" "$count" 2>/dev/null; then
    echo "ERROR: invalid identity count in testdata.json" >&2
    return 1
  fi
  for i in $(seq 1 "$count"); do
    email="$(printf 'load-test-%03d@omnicraft.local' "$i")"
    if DB_RUN "INSERT INTO users (email, username, password_hash, role, reputation, email_verified_at) VALUES ('$email', 'loadtest$i', '$hash', 'user', 10, NOW()) ON CONFLICT (email) DO NOTHING" \
      >/dev/null 2>&1; then
      SEEDED_EMAILS="$SEEDED_EMAILS $email"
    else
      echo "ERROR: failed to seed test identity $email" >&2
      return 1
    fi
  done
  echo "seeded $count load-test identities"
  return 0
}

if [ "$DRY_RUN" -eq 1 ]; then
  :
elif [ -n "$SEED_DB" ] || [ -n "$DB_DSN" ]; then
  seed_test_data || exit 1
  if [ -z "$CONTENT_ID" ]; then
    CONTENT_ID="$(DB_RUN "SELECT id FROM content_items WHERE status IN ('approved','published') AND deleted_at IS NULL ORDER BY id LIMIT 1" 2>/dev/null | tr -d ' ' || true)"
    if [ -z "$CONTENT_ID" ]; then
      echo "ERROR: no approved content available for the detail path; seed the dataset first" >&2
      exit 1
    fi
    echo "resolved content id $CONTENT_ID for detail path"
  fi
else
  if [ -z "$CONTENT_ID" ]; then
    echo "WARNING: -ContentId not provided and no -SeedDb/-DbDsn; detail path will use id 1" >&2
  fi
fi

# ---------------------------------------------------------------- readiness
# Wait for the target health endpoint before starting the run.
wait_ready() {
  local i=0
  while [ "$i" -lt "$READINESS_TIMEOUT_SEC" ]; do
    if curl -sf --max-time 2 "$TARGET/healthz" >/dev/null 2>&1; then
      echo "target ready: $TARGET/healthz"
      return 0
    fi
    i=$((i + 2))
    sleep 2
  done
  echo "ERROR: target $TARGET did not become ready within ${READINESS_TIMEOUT_SEC}s" >&2
  return 1
}

# ----------------------------------------------------------- resource metrics
# Docker stats snapshot for backend/postgres/redis containers merged with the
# native backend process when it runs outside docker. Emits a single JSON
# array document. Missing metrics fail the run (plan: metrics fail the run)
# unless -SkipMetrics is set (contract tests).
capture_metrics() {
  local out="$REPORT_DIR/$RUN_NAME_SAFE-resources.json"
  local raw="$REPORT_DIR/$RUN_NAME_SAFE-resources.raw"
  rm -f "$raw"
  if command -v docker >/dev/null 2>&1; then
    local found=""
    found="$(docker ps --format '{{.Names}}' 2>/dev/null | grep -E 'omnicraft-(backend|postgres|redis)' | head -5 || true)"
    if [ -n "$found" ]; then
      docker stats --no-stream --format json $found > "$raw" 2>/dev/null || rm -f "$raw"
    fi
  fi
  # Native backend process (outside docker) plus any docker output, merged
  # into one JSON array document. The process pattern covers both the dev
  # `go run cmd/server/main.go` invocation and the compiled CI binary.
  {
    if [ -f "$raw" ]; then
      cat "$raw"
    fi
    ps aux | grep -E 'cmd/server/main|omnicraft-server' | grep -v grep | \
      awk '{printf "{\"process\":\"backend\",\"cpu_percent\":\"%s\",\"mem_percent\":\"%s\"}\n", $3, $4}'
  } | python3 -c '
import json, sys
lines = [l for l in sys.stdin if l.strip()]
items = []
for l in lines:
    try:
        items.append(json.loads(l))
    except Exception:
        continue
if not items:
    sys.exit(1)
print(json.dumps(items))
' > "$out" 2>/dev/null || { rm -f "$out"; }
  rm -f "$raw"
  if [ -s "$out" ]; then
    echo "captured resource metrics: $out"
    return 0
  fi
  echo "ERROR: failed to capture resource metrics" >&2
  return 1
}

STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
K6_EXIT=0
RECOVERY_SEC=""
if [ "$DRY_RUN" -eq 1 ]; then
  echo "dry-run: would execute k6 $TIER against $TARGET"
else
  if [ "$SKIP_READY" -eq 0 ]; then
    wait_ready || exit 1
  fi
  K6_ENV_ARGS=(-e TARGET="$TARGET" -e TIER="$TIER" -e PROFILE="$(cd "$(dirname "$PROFILE")" && pwd)/$(basename "$PROFILE")")
  if [ -n "$CONTENT_ID" ]; then
    K6_ENV_ARGS+=(-e CONTENT_ID="$CONTENT_ID")
  fi
  set +e
  "$K6_BIN" run \
    --summary-export "$REPORT_DIR/$RUN_NAME_SAFE-k6.json" \
    --out json="$REPORT_DIR/$RUN_NAME_SAFE-k6-log.json" \
    "${K6_ENV_ARGS[@]}" \
    "$K6_DIR/$TIER_FILE.js"
  K6_EXIT=$?
  set -e
  if [ "$SKIP_METRICS" -eq 0 ]; then
    capture_metrics || exit 1
  fi
  # Stress recovery observation: time until /healthz latency returns to a
  # stable fast value after the aggressive ramp-down (plan: recovery time).
  if [ "$TIER" = "Stress" ]; then
    RECOVERY_SEC="$(python3 - "$TARGET" <<'PY'
import subprocess, sys, time
target = sys.argv[1]
deadline = time.time() + 30
while time.time() < deadline:
    try:
        r = subprocess.run(["curl", "-sf", "--max-time", "2", "-o", "/dev/null",
                            "-w", "%{time_total}", f"{target}/healthz"],
                           capture_output=True, text=True, timeout=3)
        if r.returncode == 0 and float(r.stdout) < 0.2:
            print(f"{float(r.stdout)*1000:.0f}")
            sys.exit(0)
    except Exception:
        pass
    time.sleep(1)
sys.exit(0)
PY
)"
    echo "stress recovery healthz latency: ${RECOVERY_SEC:-n/a}ms"
  fi
fi
FINISHED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# -------------------------------------------------------------------- summary
# Always save a summary, including on k6 failure.
K6_TOOL_VERSION=""
if command -v "$K6_BIN" >/dev/null 2>&1; then
  K6_TOOL_VERSION="$("$K6_BIN" version 2>/dev/null | head -1 || true)"
fi
python3 - "$REPORT_DIR" "$RUN_NAME_SAFE" "$ENVIRONMENT" "$TIER" "$TARGET" "$PROFILE" "$STARTED_AT" "$FINISHED_AT" "$K6_EXIT" "$K6_TOOL_VERSION" "$DRY_RUN" "$CONTENT_ID" "$SEED_DB" "$RECOVERY_SEC" <<'PY'
import json, sys
report_dir, run_name, env, tier, target, profile, started, finished, k6_exit, k6_version, dry_run, content_id, seed_db, recovery_sec = sys.argv[1:]
summary = {
    "run_name": run_name,
    "environment": env,
    "tier": tier,
    "target": target,
    "profile": profile,
    "content_id": content_id,
    "seed_db": seed_db,
    "started_at": started,
    "finished_at": finished,
    "k6_exit_code": int(k6_exit),
    "k6_version": k6_version,
    "dry_run": dry_run == "1",
    "recovery_healthz_ms": int(recovery_sec) if recovery_sec else None,
    "status": "pass" if k6_exit == "0" else "fail",
}
with open(f"{report_dir}/{run_name}-summary.json", "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2)
PY

if [ "$DRY_RUN" -eq 1 ]; then
  exit 0
fi
exit "$K6_EXIT"
