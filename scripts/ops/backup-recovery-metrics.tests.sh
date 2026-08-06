#!/usr/bin/env bash
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-metrics-contract.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

assert_metric() {
  local file="$1" name="$2" value="$3"
  grep -Eq "^${name} ${value}(\\.[0-9]+)?$" "$file" \
    || { echo "FAIL: expected ${name}=${value} in ${file}" >&2; exit 1; }
}

FAKE_BIN="$TEMP_ROOT/bin"
mkdir -p "$FAKE_BIN"
cat > "$FAKE_BIN/psql" <<'SH'
#!/usr/bin/env bash
case "$*" in
  *to_regclass*) echo t ;;
  *server_version*) echo 16.0 ;;
  *) echo '[]' ;;
esac
SH
cat > "$FAKE_BIN/pg_dump" <<'SH'
#!/usr/bin/env bash
printf 'fake custom dump'
SH
chmod +x "$FAKE_BIN/psql" "$FAKE_BIN/pg_dump"

SUCCESS_DIR="$TEMP_ROOT/success"
mkdir -p "$SUCCESS_DIR"
PATH="$FAKE_BIN:$PATH" BACKUP_DIR="$SUCCESS_DIR" METRICS_TEXTFILE_DIR="$SUCCESS_DIR/metrics" \
  DB_PASSWORD=fixture DB_NAME=omnicraft bash "$REPO_ROOT/scripts/backup-db.sh" --retain 1 >/dev/null
assert_metric "$SUCCESS_DIR/metrics/omnicraft_backup.prom" omnicraft_backup_status 1
grep -Eq '^omnicraft_backup_last_success_timestamp_seconds [0-9]+' "$SUCCESS_DIR/metrics/omnicraft_backup.prom" \
  || { echo "FAIL: successful backup must record last-success timestamp" >&2; exit 1; }

FAIL_DIR="$TEMP_ROOT/failure"
mkdir -p "$FAIL_DIR/bin"
cat > "$FAIL_DIR/bin/psql" <<'SH'
#!/usr/bin/env bash
echo t
SH
chmod +x "$FAIL_DIR/bin/psql"
PATH="$FAIL_DIR/bin:$PATH" BACKUP_DIR="$FAIL_DIR/backups" METRICS_TEXTFILE_DIR="$FAIL_DIR/metrics" \
  DB_PASSWORD=fixture DB_NAME=omnicraft bash "$REPO_ROOT/scripts/backup-db.sh" >/dev/null 2>&1
[ "$?" -ne 0 ] || { echo "FAIL: backup without pg_dump must fail" >&2; exit 1; }
assert_metric "$FAIL_DIR/metrics/omnicraft_backup.prom" omnicraft_backup_status 0
if grep -q '^omnicraft_backup_last_success_timestamp_seconds' "$FAIL_DIR/metrics/omnicraft_backup.prom"; then
  echo "FAIL: failed backup must not invent last-success timestamp" >&2
  exit 1
fi

echo "OK: backup/recovery metric contract passed"
