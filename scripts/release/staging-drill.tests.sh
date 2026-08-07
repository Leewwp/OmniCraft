#!/usr/bin/env bash
# Contract tests for scripts/release/staging-drill.sh: input validation
# (missing real staging inputs must block with exit 3, placeholders refused)
# and the drill orchestration in -Drill mode (preflight -> deploy -> verify ->
# rollback -> verify -> redeploy).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DRILL_SCRIPT="$SCRIPT_DIR/staging-drill.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

if [ ! -f "$DRILL_SCRIPT" ]; then
  echo "staging-drill.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-staging.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

make_fixture() {
  local dir="$1"
  mkdir -p "$dir"
  python3 - "$dir" <<'PY'
import json, sys
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
def manifest(version, commit, digest, head):
    return {
        "schema_version": "1.0", "version": version, "commit": commit,
        "created_at": "2026-08-07T00:00:00Z", "previous_digest": None,
        "images": {
            "backend": {"ref": "registry.example/omnicraft-backend@" + digest, "digest": digest},
            "frontend": {"ref": "registry.example/omnicraft-frontend@" + digest, "digest": digest},
        },
        "migration": {"head": head}, "schema_compat": {"max_head": head},
        "preflight": {"status": "ok", "summary": "preflight-summary.json"},
        "backup": {"id": "backup-001", "status": "ok"},
        "readiness": {"status": "ok"}, "smoke": {"status": "ok"},
        "deployed_at": "2026-08-07T01:00:00Z",
    }
head = "060_fix_search_config_fallback.sql"
d1 = "sha256:" + "1" * 64
d2 = "sha256:" + "2" * 64
# Candidate and previous share the same schema head so the rollback leg is
# schema-compatible (a candidate that bumps the schema would legitimately be
# refused by rollback.sh; that refusal path is covered by the deployment
# contract tests).
json.dump(manifest("v0.2.0", "a" * 40, d2, head), open(out + "/candidate.json", "w"), indent=2)
json.dump(manifest("v0.1.0", "b" * 40, d1, head), open(out + "/previous.json", "w"), indent=2)
PY
}

# ------------------------------------------------------------ usage errors
rc=0
bash "$DRILL_SCRIPT" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || { echo "FAIL: missing args must exit 2" >&2; exit 1; }
echo "OK: usage errors"

# -------------------------------------------- missing staging inputs block
F="$TEMP_ROOT/blocked"
make_fixture "$F"
env -u OMNICRAFT_STAGING_ENV_FILE -u OMNICRAFT_STAGING_OVERRIDE_FILE -u OMNICRAFT_CANDIDATE_MANIFEST \
  -u OMNICRAFT_PREVIOUS_MANIFEST -u OMNICRAFT_STAGING_OSS_BUCKET -u OMNICRAFT_OFFSITE_ARCHIVE_URI \
  -u GITHUB_RELEASE_TAG \
  bash "$DRILL_SCRIPT" -EnvironmentFile "$F/env" -OverrideFile "$F/override.yaml" \
    -CandidateManifest "$F/candidate.json" -PreviousManifest "$F/previous.json" -ReportDir "$F/report" \
    >/dev/null 2>"$F/blocked.err" || rc=$?
[ "$rc" -eq 3 ] || { echo "FAIL: missing staging inputs must block with exit 3, got $rc" >&2; cat "$F/blocked.err" >&2; exit 1; }
echo "OK: missing staging inputs block the drill"

# ------------------------------------------------- placeholder inputs block
F="$TEMP_ROOT/placeholder"
make_fixture "$F"
rc=0
OMNICRAFT_STAGING_ENV_FILE="$F/env" OMNICRAFT_STAGING_OVERRIDE_FILE="$F/override.yaml" \
  OMNICRAFT_CANDIDATE_MANIFEST="$F/candidate.json" OMNICRAFT_PREVIOUS_MANIFEST="$F/previous.json" \
  OMNICRAFT_STAGING_OSS_BUCKET="<your-bucket>" OMNICRAFT_OFFSITE_ARCHIVE_URI="s3://<bucket>/ops" \
  GITHUB_RELEASE_TAG="v0.1.0" \
  bash "$DRILL_SCRIPT" -EnvironmentFile "$F/env" -OverrideFile "$F/override.yaml" \
    -CandidateManifest "$F/candidate.json" -PreviousManifest "$F/previous.json" -ReportDir "$F/report" \
    >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 3 ] || { echo "FAIL: placeholder staging inputs must block with exit 3, got $rc" >&2; exit 1; }
echo "OK: placeholder staging inputs block the drill"

# ---------------------------------------------------- real-path guardrails
# The default path must never accidentally inherit -Drill from the shell's
# non-empty DRILL=0 value, and real deploy/rollback must call Compose rather
# than only write manifest fixtures.
if grep -Fq '${DRILL:+-Drill}' "$DRILL_SCRIPT"; then
  echo "FAIL: default staging path must not pass -Drill when DRILL=0" >&2
  exit 1
fi
if ! grep -Fq 'docker compose' "$SCRIPT_DIR/deploy.sh"; then
  echo "FAIL: deploy.sh must execute Docker Compose on the real path" >&2
  exit 1
fi
if grep -Fq 'simulated compose rollback completed' "$SCRIPT_DIR/rollback.sh"; then
  echo "FAIL: rollback.sh must not report simulated compose rollback" >&2
  exit 1
fi
echo "OK: real release path is not manifest-only or drill-only"

# --------------------------------------------------------- valid drill run
F="$TEMP_ROOT/valid"
make_fixture "$F"
rc=0
bash "$DRILL_SCRIPT" -EnvironmentFile "$F/env" -OverrideFile "$F/override.yaml" \
  -CandidateManifest "$F/candidate.json" -PreviousManifest "$F/previous.json" -ReportDir "$F/report" -Drill \
  >"$TEMP_ROOT/valid.out" 2>"$TEMP_ROOT/valid.err" || rc=$?
[ "$rc" -eq 0 ] || { echo "FAIL: valid drill must exit 0, got $rc" >&2; cat "$TEMP_ROOT/valid.err" >&2; exit 1; }
[ -f "$F/report/deployment-manifest.json" ] || { echo "FAIL: deployment manifest missing" >&2; exit 1; }
[ -f "$F/report/rollback-manifest.json" ] || { echo "FAIL: rollback manifest missing" >&2; exit 1; }
[ -f "$F/report/history.json" ] || { echo "FAIL: history missing" >&2; exit 1; }
echo "OK: valid drill orchestrates deploy -> rollback -> redeploy"

# ---------------------------------------- recovery objective comparison step
F="$TEMP_ROOT/objectives-ok"
make_fixture "$F"
python3 - "$F" <<'PY'
import hashlib, json, sys
out = sys.argv[1]
objectives = {
    "schema_version": 1,
    "state": "approved",
    "measured": {"postgres_rpo": 0, "postgres_rto": 1, "object_restore_rto": 1,
                 "service_rpo": 0, "service_rto": 6, "measured_at": "2026-08-07T00:00:00Z", "last_drill": ""},
    "approved_targets": {"postgres_rpo": 5, "postgres_rto": 30, "object_restore_rto": 30,
                         "service_rpo": 5, "service_rto": 30},
    "approval": {"ref": "c" * 40, "approver": "tester", "approved_at": "2026-08-07T00:00:00Z"},
}
json.dump(objectives, open(out + "/objectives.json", "w"), indent=2)
evidence_path = out + "/recovery-evidence.json"
open(evidence_path, "w").write('{"restore": "measured"}\n')
evidence_sha = hashlib.sha256(open(evidence_path, "rb").read()).hexdigest()
measured = {"schema_version": 1, "drill_id": "ops-08-staging-recovery", "source_commit": "a" * 40,
            "source_evidence": [{"path": "recovery-evidence.json", "sha256": evidence_sha}],
            "postgres_rpo": 1, "postgres_rto": 3, "object_restore_rto": 2,
            "service_rpo": 0, "service_rto": 8, "measured_at": "2026-08-07T02:00:00Z", "last_drill": "real drill"}
json.dump(measured, open(out + "/measured.json", "w"), indent=2)
PY
rc=0
bash "$DRILL_SCRIPT" -EnvironmentFile "$F/env" -OverrideFile "$F/override.yaml" \
  -CandidateManifest "$F/candidate.json" -PreviousManifest "$F/previous.json" -ReportDir "$F/report2" -Drill \
  -RecoveryObjectives "$F/objectives.json" -Measured "$F/measured.json" \
  >"$TEMP_ROOT/objectives-ok.out" 2>"$TEMP_ROOT/objectives-ok.err" || rc=$?
[ "$rc" -eq 0 ] || { echo "FAIL: drill with met recovery objectives must exit 0, got $rc" >&2; cat "$TEMP_ROOT/objectives-ok.err" >&2; exit 1; }
[ -f "$F/report2/recovery-objective-comparison.json" ] || { echo "FAIL: comparison evidence missing" >&2; exit 1; }
python3 - "$F/report2/recovery-objective-comparison.json" <<'PY'
import json, sys
c = json.load(open(sys.argv[1], encoding="utf-8"))
if c.get("state") != "approved" or not c.get("all_met"):
    print("FAIL: comparison must be approved and all_met", file=sys.stderr)
    sys.exit(1)
if len(c.get("comparisons", [])) != 5:
    print("FAIL: comparison must cover all five metrics", file=sys.stderr)
    sys.exit(1)
if c.get("approval_ref") != "c" * 40:
    print("FAIL: approval ref must be carried into the comparison", file=sys.stderr)
    sys.exit(1)
print("recovery objective comparison (met) assertions passed")
PY
echo "OK: drill compares measured values against approved recovery objectives"

rc=0
python3 - "$F/metric-only.json" <<'PY'
import json, sys
json.dump({"postgres_rpo": 1, "postgres_rto": 3, "object_restore_rto": 2,
           "service_rpo": 0, "service_rto": 8}, open(sys.argv[1], "w"), indent=2)
PY
bash "$DRILL_SCRIPT" -EnvironmentFile "$F/env" -OverrideFile "$F/override.yaml" \
  -CandidateManifest "$F/candidate.json" -PreviousManifest "$F/previous.json" -ReportDir "$F/report-metric-only" -Drill \
  -RecoveryObjectives "$F/objectives.json" -Measured "$F/metric-only.json" \
  >/dev/null 2>&1 || rc=$?
[ "$rc" -ne 0 ] || { echo "FAIL: metric-only recovery measurements must fail closed" >&2; exit 1; }
echo "OK: metric-only recovery measurements rejected"

F="$TEMP_ROOT/objectives-unmet"
make_fixture "$F"
python3 - "$F" <<'PY'
import hashlib, json, sys
out = sys.argv[1]
objectives = {
    "schema_version": 1,
    "state": "approved",
    "measured": {"postgres_rpo": 0, "postgres_rto": 1, "object_restore_rto": 1,
                 "service_rpo": 0, "service_rto": 6, "measured_at": "2026-08-07T00:00:00Z", "last_drill": ""},
    "approved_targets": {"postgres_rpo": 5, "postgres_rto": 30, "object_restore_rto": 30,
                         "service_rpo": 5, "service_rto": 30},
    "approval": {"ref": "c" * 40, "approver": "tester", "approved_at": "2026-08-07T00:00:00Z"},
}
json.dump(objectives, open(out + "/objectives.json", "w"), indent=2)
evidence_path = out + "/recovery-evidence.json"
open(evidence_path, "w").write('{"restore": "measured"}\n')
evidence_sha = hashlib.sha256(open(evidence_path, "rb").read()).hexdigest()
measured = {"schema_version": 1, "drill_id": "ops-08-staging-recovery", "source_commit": "a" * 40,
            "source_evidence": [{"path": "recovery-evidence.json", "sha256": evidence_sha}],
            "postgres_rpo": 60, "postgres_rto": 3, "object_restore_rto": 2,
            "service_rpo": 0, "service_rto": 8, "measured_at": "2026-08-07T02:00:00Z", "last_drill": "real drill"}
json.dump(measured, open(out + "/measured.json", "w"), indent=2)
PY
rc=0
bash "$DRILL_SCRIPT" -EnvironmentFile "$F/env" -OverrideFile "$F/override.yaml" \
  -CandidateManifest "$F/candidate.json" -PreviousManifest "$F/previous.json" -ReportDir "$F/report3" -Drill \
  -RecoveryObjectives "$F/objectives.json" -Measured "$F/measured.json" \
  >/dev/null 2>&1 || rc=$?
[ "$rc" -ne 0 ] || { echo "FAIL: unmet recovery objectives must fail the drill" >&2; exit 1; }
echo "OK: unmet recovery objectives fail the drill"

rc=0
bash "$DRILL_SCRIPT" -EnvironmentFile "$F/env" -OverrideFile "$F/override.yaml" \
  -CandidateManifest "$F/candidate.json" -PreviousManifest "$F/previous.json" -ReportDir "$F/report4" -Drill \
  -RecoveryObjectives "$F/objectives.json" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || { echo "FAIL: -RecoveryObjectives without -Measured must exit 2" >&2; exit 1; }
echo "OK: -RecoveryObjectives without -Measured rejected"

# ------------------------------------------ real-mode full orchestration
# The non-drill path must run the full Compose deploy/rollback orchestration
# without crashing on uninitialized drill-args state, and emit the RPO/RTO
# comparison evidence.
F="$TEMP_ROOT/real-mode"
make_fixture "$F"
python3 - "$F" <<'PY'
import hashlib, json, sys
out = sys.argv[1]
candidate = json.load(open(out + "/candidate.json"))
candidate["commit"] = "c" * 40
json.dump(candidate, open(out + "/candidate.json", "w"), indent=2)
objectives = {
    "schema_version": 1,
    "state": "approved",
    "measured": {"postgres_rpo": 0, "postgres_rto": 1, "object_restore_rto": 1,
                 "service_rpo": 0, "service_rto": 6, "measured_at": "2026-08-07T00:00:00Z", "last_drill": ""},
    "approved_targets": {"postgres_rpo": 1440, "postgres_rto": 30, "object_restore_rto": 60,
                         "service_rpo": 0, "service_rto": 30},
    "approval": {"ref": "c" * 40, "approver": "tester", "approved_at": "2026-08-07T00:00:00Z"},
}
json.dump(objectives, open(out + "/objectives.json", "w"), indent=2)
evidence_path = out + "/recovery-evidence.json"
open(evidence_path, "w").write('{"restore": "measured"}\n')
evidence_sha = hashlib.sha256(open(evidence_path, "rb").read()).hexdigest()
measured = {"schema_version": 1, "drill_id": "ops-08-staging-recovery", "source_commit": "c" * 40,
            "source_evidence": [{"path": "recovery-evidence.json", "sha256": evidence_sha}],
            "postgres_rpo": 0, "postgres_rto": 1, "object_restore_rto": 1,
            "service_rpo": 0, "service_rto": 6, "measured_at": "2026-08-07T02:00:00Z", "last_drill": "real drill"}
json.dump(measured, open(out + "/measured.json", "w"), indent=2)
with open(out + "/compose.yml", "w") as f:
    f.write("""services:
  migrate:
    image: alpine:3.20
  backend:
    image: alpine:3.20
  frontend:
    image: alpine:3.20
  nginx:
    image: alpine:3.20
""")
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
    -f|--env-file) shift 2 ;;
    --*) shift ;;
    *) command_name="$1"; shift; break ;;
  esac
done
case "${command_name:-}" in
  config|pull|up|stop|start|rm) exit 0 ;;
  ps) echo fake-container; exit 0 ;;
  exec) printf 'fake pg dump\n'; exit 0 ;;
  *) echo "unexpected fake compose command: ${command_name:-}" >&2; exit 1 ;;
esac
EOF
chmod +x "$FAKE_BIN/docker"
rc=0
OMNICRAFT_STAGING_ENV_FILE="$F/env" OMNICRAFT_STAGING_OVERRIDE_FILE="$F/override.yaml" \
  OMNICRAFT_CANDIDATE_MANIFEST="$F/candidate.json" OMNICRAFT_PREVIOUS_MANIFEST="$F/previous.json" \
  OMNICRAFT_STAGING_COMPOSE_FILE="$F/compose.yml" OMNICRAFT_STAGING_OSS_BUCKET="omnicraft-private" \
  OMNICRAFT_OFFSITE_ARCHIVE_URI="oss://omnicraft-ops-archive/ops-evidence" \
  GITHUB_RELEASE_TAG="v0.1.0" OMNICRAFT_RECOVERY_OBJECTIVES="$F/objectives.json" \
  OMNICRAFT_MEASURED="$F/measured.json" OMNICRAFT_SMOKE_URL="file://$F/smoke.txt" \
  OMNICRAFT_DOCKER_BIN="$FAKE_BIN/docker" FAKE_DOCKER_LOG="$F/docker.log" \
  bash "$DRILL_SCRIPT" -EnvironmentFile "$F/env" -OverrideFile "$F/override.yaml" \
    -CandidateManifest "$F/candidate.json" -PreviousManifest "$F/previous.json" \
    -ComposeFile "$F/compose.yml" -ReportDir "$F/report-real" \
    -RecoveryObjectives "$F/objectives.json" -Measured "$F/measured.json" \
    >/dev/null 2>"$F/real.err" || rc=$?
[ "$rc" -eq 0 ] || { echo "FAIL: real-mode drill must complete, got $rc" >&2; cat "$F/real.err" >&2; exit 1; }
[ -f "$F/report-real/recovery-objective-comparison.json" ] || {
  echo "FAIL: real-mode drill must emit recovery-objective-comparison.json" >&2; exit 1; }
grep -Eq 'compose .*config|compose .*up|compose .*exec|inspect' "$F/docker.log" || {
  echo "FAIL: real-mode drill did not exercise Compose" >&2; exit 1; }
echo "OK: real-mode drill completes with Compose orchestration and RPO/RTO comparison"

echo "All staging-drill contract tests passed"
