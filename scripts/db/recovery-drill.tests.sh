#!/usr/bin/env bash
# Contract tests for scripts/db/recovery-drill.sh. The real drill requires
# docker and is exercised separately; these tests cover argument validation,
# compose-file existence and the evidence writer.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DRILL="$SCRIPT_DIR/recovery-drill.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE="$REPO_ROOT/ops/recovery/docker-compose.recovery.yml"

if [ ! -f "$DRILL" ]; then
  echo "recovery-drill.sh does not exist" >&2
  exit 1
fi
if [ ! -f "$COMPOSE" ]; then
  echo "ops/recovery/docker-compose.recovery.yml does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-recovery-drill.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

expect_exit() {
  local want="$1"
  local message="$2"
  shift 2
  local got=0
  bash "$DRILL" "$@" >/dev/null 2>&1 || got=$?
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $message (exit $got, want $want)" >&2
    exit 1
  fi
}

expect_exit 2 "missing -ReportDir must be rejected"
expect_exit 2 "unknown arguments must be rejected" -ReportDir "$TEMP_ROOT" -Nope
expect_exit 1 "a missing compose file must be rejected" -ComposeFile "$TEMP_ROOT/nope.yml" -ReportDir "$TEMP_ROOT"
expect_exit 1 "a compose file without the recovery stack must be rejected" \
  -ComposeFile "$REPO_ROOT/docker-compose.yml" -ReportDir "$TEMP_ROOT"

if grep -Fq 'down -v --remove-orphans >/dev/null 2>&1 || true' "$DRILL"; then
  echo "FAIL: recovery drill must propagate teardown failure" >&2
  exit 1
fi
if ! grep -q 'go build.*cmd/server' "$DRILL" || grep -q 'go run ./cmd/server' "$DRILL"; then
  echo "FAIL: recovery drill must run a built server binary so it can terminate the real process" >&2
  exit 1
fi
if ! grep -q '/api/v1/contents' "$DRILL"; then
  echo "FAIL: recovery smoke must call a database-backed API, not only /healthz" >&2
  exit 1
fi
if ! grep -q 'service_rto_seconds' "$DRILL" || ! grep -q 'OUTAGE_STARTED' "$DRILL"; then
  echo "FAIL: recovery evidence must measure the outage-to-service-recovery RTO and real backup-to-outage RPO" >&2
  exit 1
fi
python3 - "$DRILL" <<'PY'
import sys

text = open(sys.argv[1], encoding="utf-8").read()
stop_redis = text.find('docker compose -p "$PROJ" -f "$COMPOSE_FILE" stop redis')
restore_postgres = text.find('bash "$REPO_ROOT/scripts/restore-db.sh"')
object_reconcile = text.find('database-to-object reconciliation')
redis_rebuild = text.find('redis clear and rebuild from postgres')
start_redis = text.find('docker compose -p "$PROJ" -f "$COMPOSE_FILE" start redis')
server_smoke = text.find('backend readiness smoke against the restored database')
service_rto = text.find('SERVICE_RTO_SECONDS=')
if min(stop_redis, restore_postgres, object_reconcile, redis_rebuild, start_redis, server_smoke, service_rto) < 0:
    raise SystemExit("recovery drill is missing explicit storage/service recovery phases")
if not (stop_redis < restore_postgres < object_reconcile < redis_rebuild < start_redis < server_smoke < service_rto):
    raise SystemExit("recovery order must be PostgreSQL -> object reconciliation -> Redis rebuild -> DB-backed app smoke -> service RTO")
PY

# restore-db.sh contract cases that need no docker: argument validation,
# missing artifacts, checksum mismatch and refusal to overwrite the source.
RESTORE="$SCRIPT_DIR/../../scripts/restore-db.sh"

expect_exit_script() {
  local script="$1"
  local want="$2"
  local message="$3"
  shift 3
  local got=0
  bash "$script" "$@" >/dev/null 2>&1 || got=$?
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $message (exit $got, want $want)" >&2
    exit 1
  fi
}

expect_exit_script "$RESTORE" 2 "restore-db.sh without -Backup must be rejected" -TargetDB "omnicraft_probe"
expect_exit_script "$RESTORE" 2 "restore-db.sh without -TargetDB must be rejected" -Backup "$TEMP_ROOT/nope.custom"
expect_exit_script "$RESTORE" 2 "restore-db.sh must reject an unsafe target identifier" \
  -Backup "$TEMP_ROOT/nope.custom" -TargetDB "omnicraft_probe;DROP DATABASE x"
expect_exit_script "$RESTORE" 1 "restore-db.sh must reject a missing backup file" \
  -Backup "$TEMP_ROOT/nope.custom" -TargetDB "omnicraft_probe"
mkdir -p "$TEMP_ROOT/backups"
printf 'not a real dump' > "$TEMP_ROOT/backups/probe.custom"
expect_exit_script "$RESTORE" 1 "restore-db.sh must reject a missing backup manifest" \
  -Backup "$TEMP_ROOT/backups/probe.custom" -TargetDB "omnicraft_probe"

printf '{"schema_version":1,"backup_file":"probe.custom","source_db":"omnicraft","started_at":"2026-08-04T00:00:00Z","finished_at":"2026-08-04T00:00:01Z","dump_checksum_sha256":"deadbeef","pg_version":"16.14","migrations":[]}' \
  > "$TEMP_ROOT/backups/probe.custom.manifest.json"
expect_exit_script "$RESTORE" 1 "restore-db.sh must reject a checksum mismatch" \
  -Backup "$TEMP_ROOT/backups/probe.custom" -TargetDB "omnicraft_probe"

python3 - "$TEMP_ROOT/backups/probe.custom.manifest.json" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as f:
    manifest = json.load(f)
manifest["dump_checksum_sha256"] = __import__("hashlib").sha256(b"not a real dump").hexdigest()
with open(path, "w", encoding="utf-8") as f:
    json.dump(manifest, f)
PY
expect_exit_script "$RESTORE" 1 "restore-db.sh must refuse to overwrite the source database" \
  -Backup "$TEMP_ROOT/backups/probe.custom" -TargetDB "omnicraft"

verify_output="$(bash "$RESTORE" -Backup "$TEMP_ROOT/backups/probe.custom" -TargetDB omnicraft_probe \
  -VerifyDSN 'host=127.0.0.1 dbname=some_other_database' 2>&1 || true)"
if ! printf '%s' "$verify_output" | grep -q 'VerifyDSN dbname must match -TargetDB'; then
  echo "FAIL: restore-db.sh must reject ledger verification against a different database" >&2
  exit 1
fi
if ! grep -q 'convalidated' "$RESTORE"; then
  echo "FAIL: restore-db.sh must reject unvalidated PostgreSQL constraints" >&2
  exit 1
fi

# The manifest start timestamp must be captured before pg_dump begins.
mkdir -p "$TEMP_ROOT/bin" "$TEMP_ROOT/backup-output"
cat > "$TEMP_ROOT/bin/psql" <<'SH'
#!/bin/sh
case "$*" in
  *to_regclass*) printf 't\n' ;;
  *server_version*) printf '16.14\n' ;;
  *json_agg*)
    if [ -n "${FAKE_LEDGER_COUNTER:-}" ]; then
      count=0
      [ ! -f "$FAKE_LEDGER_COUNTER" ] || count="$(cat "$FAKE_LEDGER_COUNTER")"
      count=$((count + 1))
      printf '%s\n' "$count" > "$FAKE_LEDGER_COUNTER"
      if [ "$count" -eq 1 ]; then
        printf '[]\n'
      else
        printf '[{"version":1,"filename":"001_users.sql","checksum":"changed"}]\n'
      fi
    else
      printf '[]\n'
    fi
    ;;
  *) exit 1 ;;
esac
SH
cat > "$TEMP_ROOT/bin/pg_dump" <<'SH'
#!/bin/sh
date -u +%Y-%m-%dT%H:%M:%SZ > "$FAKE_PG_DUMP_STARTED"
sleep 1
printf 'custom-dump-bytes'
SH
chmod +x "$TEMP_ROOT/bin/psql" "$TEMP_ROOT/bin/pg_dump"
FAKE_PG_DUMP_STARTED="$TEMP_ROOT/pg-dump-started.txt" PATH="$TEMP_ROOT/bin:$PATH" \
  BACKUP_DIR="$TEMP_ROOT/backup-output" bash "$REPO_ROOT/scripts/backup-db.sh" >/dev/null
python3 - "$TEMP_ROOT/backup-output" "$TEMP_ROOT/pg-dump-started.txt" <<'PY'
import datetime
import glob
import json
import sys

backup_dir, marker = sys.argv[1:3]
manifest_path = glob.glob(backup_dir + "/*.manifest.json")[0]
with open(manifest_path, encoding="utf-8") as f:
    started = datetime.datetime.fromisoformat(json.load(f)["started_at"].replace("Z", "+00:00"))
with open(marker, encoding="utf-8") as f:
    dump_started = datetime.datetime.fromisoformat(f.read().strip().replace("Z", "+00:00"))
if started > dump_started:
    raise SystemExit("backup manifest started_at was captured after pg_dump began")
PY

# A migration committed during pg_dump must invalidate the backup instead of
# producing a sidecar manifest that describes a different ledger than the dump.
mkdir -p "$TEMP_ROOT/racy-backup-output"
racy_status=0
FAKE_LEDGER_COUNTER="$TEMP_ROOT/ledger-counter.txt" PATH="$TEMP_ROOT/bin:$PATH" \
  BACKUP_DIR="$TEMP_ROOT/racy-backup-output" bash "$REPO_ROOT/scripts/backup-db.sh" >/dev/null 2>&1 || racy_status=$?
if [ "$racy_status" -eq 0 ]; then
  echo "FAIL: backup must reject a migration ledger change during pg_dump" >&2
  exit 1
fi
if find "$TEMP_ROOT/racy-backup-output" -name '*.custom' -type f | grep -q .; then
  echo "FAIL: an inconsistent dump must be removed" >&2
  exit 1
fi

if ! grep -q 'backup manifest migration set does not match restored ledger' "$RESTORE"; then
  echo "FAIL: restore must compare the restored ledger with the backup migration manifest" >&2
  exit 1
fi

echo "recovery-drill contract tests passed"
