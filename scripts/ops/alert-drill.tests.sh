#!/usr/bin/env bash
# Contract tests for scripts/ops/alert-drill.sh: argument validation,
# compose prerequisites, environment allowlist and target safety. The real
# drill (containers, firing/resolved alert lifecycle, real external
# heartbeat) is exercised separately with credentials.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DRILL="$SCRIPT_DIR/alert-drill.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OBS_COMPOSE="$REPO_ROOT/ops/observability/docker-compose.observability.yml"

if [ ! -f "$DRILL" ]; then
  echo "alert-drill.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-alert-drill.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

# Missing required arguments -> usage error (exit 2).
bash "$DRILL" >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: no-args must exit 2" >&2; exit 1; }
bash "$DRILL" -Environment Local >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: missing ReportDir must exit 2" >&2; exit 1; }

# Invalid environment -> usage error.
bash "$DRILL" -Environment Production -ReportDir "$TEMP_ROOT" >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: Production environment must be rejected" >&2; exit 1; }

# Unknown argument -> usage error.
bash "$DRILL" -Bogus x -Environment Local -ReportDir "$TEMP_ROOT" >/dev/null 2>&1
[ $? -eq 2 ] || { echo "FAIL: unknown argument must exit 2" >&2; exit 1; }

# Missing compose file -> failure (exit 1).
bash "$DRILL" -Environment Local -ComposeFile "$TEMP_ROOT/nope.yml" -ReportDir "$TEMP_ROOT" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: missing compose file must exit 1" >&2; exit 1; }

# Compose without the observability services -> failure.
cat > "$TEMP_ROOT/no-obs.yml" <<'YAML'
services:
  redis:
    image: redis:7-alpine
YAML
bash "$DRILL" -Environment Local -ComposeFile "$TEMP_ROOT/no-obs.yml" -ReportDir "$TEMP_ROOT" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: compose without observability services must exit 1" >&2; exit 1; }

# The real observability compose passes static checks. -DryRun validates
# arguments and prerequisites only: the drill is never started from contract
# tests, and heartbeat credentials are never required.
bash "$DRILL" -Environment Local -ComposeFile "$OBS_COMPOSE" -ReportDir "$TEMP_ROOT" -DryRun >/dev/null 2>&1
status=$?
[ "$status" -eq 0 ] || { echo "FAIL: valid static inputs with -DryRun must exit 0" >&2; exit 1; }
bash "$DRILL" -Environment Local -ComposeFile "$OBS_COMPOSE" -ReportDir "$TEMP_ROOT" -DryRun \
  -HeartbeatEnv "$TEMP_ROOT/no-heartbeat.env" >/dev/null 2>&1
status=$?
[ "$status" -eq 1 ] || { echo "FAIL: missing heartbeat credentials must exit 1" >&2; exit 1; }

# SMTP credentials must never be rendered into the archived report directory.
if grep -qE 'REPORT_DIR/alertmanager\.runtime\.yml' "$DRILL"; then
  echo "FAIL: runtime Alertmanager config must live in a private temporary directory" >&2
  exit 1
fi

# A real run must require separate evidence that the independent provider's
# notification was received; a provider status down-flip alone is insufficient.
grep -qE -- '-HeartbeatNotificationEvidence' "$DRILL" \
  || { echo "FAIL: drill must accept heartbeat notification delivery evidence" >&2; exit 1; }
grep -qE 'heartbeat-notification-evidence' "$DRILL" \
  || { echo "FAIL: drill must archive redacted heartbeat notification evidence" >&2; exit 1; }
if grep -qE -- '-ReuseHeartbeatEvidence|REUSE_HEARTBEAT_EVIDENCE' "$DRILL"; then
  echo "FAIL: a release drill must exercise a fresh heartbeat event in the same run" >&2
  exit 1
fi
if grep -qE '\[ ! -s "$HEARTBEAT_NOTIFICATION_EVIDENCE" \]' "$DRILL"; then
  echo "FAIL: notification evidence cannot be required before the fresh provider event exists" >&2
  exit 1
fi
grep -qE 'NOTIFICATION_EVIDENCE_DEADLINE' "$DRILL" \
  || { echo "FAIL: drill must wait for notification evidence after the provider event" >&2; exit 1; }
grep -qE 'notification_evidence_is_fresh.*HEARTBEAT_EVENT_CONFIRMED_EPOCH' "$DRILL" \
  || { echo "FAIL: drill must reject evidence older than the confirmed provider event" >&2; exit 1; }
grep -qE 'HEARTBEAT_NOTIFICATION_EVIDENCE=.*dirname.*HEARTBEAT_NOTIFICATION_EVIDENCE' "$DRILL" \
  || { echo "FAIL: notification evidence path must be canonicalized before same-file comparison" >&2; exit 1; }

# Behavioral freshness boundary: evidence captured after the overall drill
# started but before the provider confirmed DOWN must still be rejected.
printf 'redacted provider delivery' > "$TEMP_ROOT/notification.png"
notification_mtime="$(python3 -c 'import os, sys; print(int(os.path.getmtime(sys.argv[1])))' "$TEMP_ROOT/notification.png")"
bash "$DRILL" -CheckNotificationEvidenceFreshness "$TEMP_ROOT/notification.png" "$notification_mtime" >/dev/null 2>&1
[ $? -eq 0 ] || { echo "FAIL: evidence at the provider-event boundary must be accepted" >&2; exit 1; }
bash "$DRILL" -CheckNotificationEvidenceFreshness "$TEMP_ROOT/notification.png" "$((notification_mtime + 1))" >/dev/null 2>&1
[ $? -eq 1 ] || { echo "FAIL: evidence predating the provider-event boundary must be rejected" >&2; exit 1; }
if grep -qE '"url".*config\["ping_url"\]|temp_check_uuid|temp_check_channels|"check": created' "$DRILL"; then
  echo "FAIL: archived heartbeat JSON must omit URLs, UUIDs and channel identifiers" >&2
  exit 1
fi
grep -qE 'mktemp -d.*omnicraft-alert-drill-runtime' "$DRILL" \
  || { echo "FAIL: runtime credentials must use a private temporary directory" >&2; exit 1; }
grep -qE 'teardown_status=\$?' "$DRILL" \
  || { echo "FAIL: teardown failure must be captured and propagated" >&2; exit 1; }

echo "OK: alert-drill contract tests passed"
