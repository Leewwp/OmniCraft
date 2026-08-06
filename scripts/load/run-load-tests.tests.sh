#!/usr/bin/env bash
# =============================================================================
# Contract tests for scripts/load/run-load-tests.sh: argument validation,
# environment gate, target allowlist, production confirmation token and the
# invariant that a summary is always saved, including when k6 fails.
# Uses a fake k6 binary; no real load test is executed from this file.
# =============================================================================
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNNER="$SCRIPT_DIR/run-load-tests.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

if [ ! -f "$RUNNER" ]; then
  echo "run-load-tests.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-load.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

# ------------------------------------------------------------------ fixtures
# Minimal but schema-conformant release profile for contract tests.
cat > "$TEMP_ROOT/release-profile.json" <<'JSON'
{
  "version": "1",
  "commit": "test",
  "environment": {
    "cpu": "8 cores",
    "memory": "16 GB",
    "db": "PostgreSQL 16 + pgvector",
    "dataset_size": "1000 contents",
    "dataset_seed": "fixture"
  },
  "staging_targets": ["https://staging.omnicraft.example"],
  "endpoints": {
    "health": 0.1,
    "feed": 0.3,
    "search": 0.2,
    "detail": 0.2,
    "authenticated_read": 0.15,
    "authenticated_write": 0.05
  },
  "tiers": {
    "smoke": { "vus": 2, "duration": "10s", "p95_ms": 500, "p99_ms": 800, "error_rate": 0.01 },
    "load": { "vus": 50, "duration": "2m", "p95_ms": 800, "p99_ms": 1200, "error_rate": 0.01 },
    "stress": { "vus": 200, "duration": "3m", "p95_ms": 1500, "p99_ms": 2000, "error_rate": 0.02 }
  },
  "approval": {
    "status": "pending",
    "approved_by": "",
    "approved_at": "",
    "reference": ""
  }
}
JSON

# Fake k6: exit code controlled by K6_STUB_EXIT, always writes the summary
# export file it was asked to write.
cat > "$TEMP_ROOT/fake-k6" <<'SH'
#!/usr/bin/env bash
exit_code="${K6_STUB_EXIT:-0}"
summary_export=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "--summary-export" ]; then
    summary_export="$arg"
  fi
  prev="$arg"
done
if [ -n "$summary_export" ]; then
  printf '{"root_group":{"checks":{"http_req_failed":{"passes":0,"fails":0}},"metrics":{"http_req_duration":{"values":{"p(95)":0,"p(99)":0}}}}}' > "$summary_export"
fi
exit "$exit_code"
SH
chmod +x "$TEMP_ROOT/fake-k6"

# Stub k6 scenario directory: contract tests never execute real suites.
mkdir -p "$TEMP_ROOT/k6-scripts"
printf 'export default function () {}' > "$TEMP_ROOT/k6-scripts/smoke.js"
printf 'export default function () {}' > "$TEMP_ROOT/k6-scripts/load.js"
printf 'export default function () {}' > "$TEMP_ROOT/k6-scripts/stress.js"

expect_fail() {
  local message="$1"
  shift
  if bash "$RUNNER" "$@" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

expect_pass() {
  local message="$1"
  shift
  if ! bash "$RUNNER" "$@" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

BASE_ARGS=(
  -Environment Local
  -Tier Smoke
  -Target http://127.0.0.1:8080
  -Profile "$TEMP_ROOT/release-profile.json"
  -ReportDir "$TEMP_ROOT/report"
  -RunName smoke
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts"
)

# ------------------------------------------------------- argument validation
bash "$RUNNER" >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: no-args must exit 2" >&2; exit 1; }
expect_fail "missing Environment"
expect_fail "missing Tier" -Environment Local
expect_fail "missing Target" -Environment Local -Tier Smoke
expect_fail "missing Profile" -Environment Local -Tier Smoke -Target http://127.0.0.1:8080
expect_fail "missing ReportDir" -Environment Local -Tier Smoke -Target http://127.0.0.1:8080 -Profile "$TEMP_ROOT/release-profile.json"
expect_fail "missing RunName" -Environment Local -Tier Smoke -Target http://127.0.0.1:8080 -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report"

bash "$RUNNER" -Environment Production -Tier Smoke -Target http://127.0.0.1:8080 \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: Production environment must be rejected as test environment" >&2; exit 1; }
bash "$RUNNER" -Environment Local -Tier Slow -Target http://127.0.0.1:8080 \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: invalid tier must exit 2" >&2; exit 1; }
bash "$RUNNER" -Bogus x "${BASE_ARGS[@]}" >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: unknown argument must exit 2" >&2; exit 1; }

# ------------------------------------------------------------- target safety
# Local environment accepts loopback targets.
expect_pass "Local + 127.0.0.1 target must pass dry-run" "${BASE_ARGS[@]}" -DryRun
expect_pass "Local + localhost target must pass dry-run" \
  -Environment Local -Tier Smoke -Target http://localhost:8080 \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" -DryRun

# Production-shaped target without explicit permission must be rejected.
printf 'omnicraft-prod-load-confirmed' > "$TEMP_ROOT/token"
expect_fail "production target rejected without -AllowProduction" \
  -Environment Local -Tier Smoke -Target https://api.omnicraft.com \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" -DryRun
expect_fail "production target rejected with -AllowProduction but no confirmation token" \
  -Environment Local -Tier Smoke -Target https://api.omnicraft.com \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" -AllowProduction -DryRun
expect_fail "wrong confirmation token rejected" \
  -Environment Local -Tier Smoke -Target https://api.omnicraft.com \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" -AllowProduction -ConfirmationToken wrong -DryRun
expect_fail "confirmation token without -AllowProduction rejected" \
  -Environment Local -Tier Smoke -Target https://api.omnicraft.com \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" -ConfirmationToken "$(cat "$TEMP_ROOT/token")" -DryRun

# Correct token + -AllowProduction passes dry-run for a production target,
# with the expected token provided through the environment.
export OMNICRAFT_LOAD_PROD_CONFIRM_TOKEN="$(cat "$TEMP_ROOT/token")"
expect_pass "production target with -AllowProduction and correct token passes dry-run" \
  -Environment Local -Tier Smoke -Target https://api.omnicraft.com \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" -AllowProduction -ConfirmationToken "$(cat "$TEMP_ROOT/token")" -DryRun
unset OMNICRAFT_LOAD_PROD_CONFIRM_TOKEN

# Staging environment only allows hosts listed in the profile.
expect_pass "Staging + profile-listed staging target passes dry-run" \
  -Environment Staging -Tier Load -Target https://staging.omnicraft.example \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName load \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" -DryRun
expect_fail "Staging + non-listed target rejected" \
  -Environment Staging -Tier Load -Target https://other.example.com \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName load \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" -DryRun

# ---------------------------------------------------------- profile presence
expect_fail "missing profile file must exit 1" \
  -Environment Local -Tier Smoke -Target http://127.0.0.1:8080 \
  -Profile "$TEMP_ROOT/nope.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" -DryRun
printf 'not json' > "$TEMP_ROOT/bad.json"
expect_fail "invalid profile JSON must exit 1" \
  -Environment Local -Tier Smoke -Target http://127.0.0.1:8080 \
  -Profile "$TEMP_ROOT/bad.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" -DryRun

# ------------------------------------------------- k6 failure propagation and
# ---------------------------------------------------------- always-save-summary
# -SkipReady/-SkipMetrics keep these cases hermetic: no network probe and no
# docker dependency; the fake k6 binary is the only executable invoked.
rm -rf "$TEMP_ROOT/report"
mkdir -p "$TEMP_ROOT/report"
K6_STUB_EXIT=99 bash "$RUNNER" -Environment Local -Tier Smoke -Target http://127.0.0.1:8080 \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" \
  -SkipReady -SkipMetrics >/dev/null 2>&1
status=$?
[ "$status" -ne 0 ] || { echo "FAIL: k6 threshold failure must propagate nonzero" >&2; exit 1; }
[ -f "$TEMP_ROOT/report/smoke-summary.json" ] || { echo "FAIL: summary must be saved even when k6 fails" >&2; exit 1; }
[ -f "$TEMP_ROOT/report/smoke-k6.json" ] || { echo "FAIL: k6 summary export must be saved even when k6 fails" >&2; exit 1; }

rm -rf "$TEMP_ROOT/report"
mkdir -p "$TEMP_ROOT/report"
K6_STUB_EXIT=0 bash "$RUNNER" -Environment Local -Tier Smoke -Target http://127.0.0.1:8080 \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" \
  -SkipReady -SkipMetrics >/dev/null 2>&1
status=$?
[ "$status" -eq 0 ] || { echo "FAIL: k6 success must exit 0" >&2; exit 1; }
[ -f "$TEMP_ROOT/report/smoke-summary.json" ] || { echo "FAIL: summary must be saved on success" >&2; exit 1; }

# Missing k6 binary must fail before any run.
expect_fail "missing k6 binary must exit 1" \
  -Environment Local -Tier Smoke -Target http://127.0.0.1:8080 \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/not-a-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" \
  -SkipReady -SkipMetrics

# A flag that requires a value must not accept a missing value silently.
bash "$RUNNER" -Environment Local -Tier Smoke -Target http://127.0.0.1:8080 \
  -Profile "$TEMP_ROOT/release-profile.json" -ReportDir "$TEMP_ROOT/report" -RunName smoke \
  -K6Bin "$TEMP_ROOT/fake-k6" -ScriptDir "$TEMP_ROOT/k6-scripts" \
  -SkipReady -SkipMetrics -ContentId >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: missing flag value must exit 2" >&2; exit 1; }

echo "All run-load-tests contract cases passed."
