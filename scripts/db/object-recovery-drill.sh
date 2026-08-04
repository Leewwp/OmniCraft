#!/usr/bin/env bash
# =============================================================================
# OmniCraft object recovery drill
# =============================================================================
# Proves the versioned-object adapter behavior against a local MinIO stand-in:
# seed attachment keys in PostgreSQL and objects in the versioned bucket,
# delete an object, restore its version, verify the content checksum, and
# reconcile the full database key set against the object store.
#
# Usage:
#   bash scripts/db/object-recovery-drill.sh -ComposeFile ops/recovery/docker-compose.recovery.yml -ReportDir artifacts/ops-02
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

COMPOSE_FILE=""
REPORT_DIR=""

while [ $# -gt 0 ]; do
  case "$1" in
    -ComposeFile) COMPOSE_FILE="$2"; shift 2 ;;
    -ReportDir) REPORT_DIR="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ -z "$COMPOSE_FILE" ] || [ -z "$REPORT_DIR" ]; then
  echo "usage: object-recovery-drill.sh -ComposeFile <path> -ReportDir <dir>" >&2
  exit 2
fi
REPORT_DIR="$(cd "$(dirname "$REPORT_DIR")" && pwd)/$(basename "$REPORT_DIR")"
if [ ! -f "$COMPOSE_FILE" ]; then
  echo "compose file not found: $COMPOSE_FILE" >&2
  exit 1
fi
if ! grep -qE '^\s+(minio-init|postgres):' "$COMPOSE_FILE"; then
  echo "compose file $COMPOSE_FILE is not a recovery stack" >&2
  exit 1
fi

PROJ="omnicraft-object-$$"
MC_IMAGE="minio/mc@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727"
BUCKET="omnicraft-recovery"
mkdir -p "$REPORT_DIR"
mc_() {
  local mounts=()
  if [ -n "${SEED_DIR:-}" ]; then
    mounts=(-v "$SEED_DIR:/seed:ro")
  fi
  docker run --rm --network "${PROJ}_default" "${mounts[@]+"${mounts[@]}"}" \
    -e "MC_HOST_local=http://omnicraftrecovery:omnicraftrecovery-secret@minio:9000" \
    "$MC_IMAGE" "$@"
}


sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    if [ $# -gt 0 ]; then
      sha256sum "$1" | awk '{print $1}'
    else
      sha256sum | awk '{print $1}'
    fi
  else
    if [ $# -gt 0 ]; then
      shasum -a 256 "$1" | awk '{print $1}'
    else
      shasum -a 256 | awk '{print $1}'
    fi
  fi
}

cleanup() {
	local original_status=$?
	local teardown_status=0
	trap - EXIT
	set +e
  echo "==> teardown: removing object drill stack $PROJ"
	docker compose -p "$PROJ" -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1
	teardown_status=$?
  for d in "${SEED_DIR:-}"; do
    if [ -n "$d" ]; then
      rm -rf "$d"
    fi
  done
	if [ "$teardown_status" -ne 0 ]; then
		echo "ERROR: object recovery stack teardown failed for $PROJ" >&2
		if [ "$original_status" -eq 0 ]; then
			original_status="$teardown_status"
		fi
	fi
	exit "$original_status"
}
trap cleanup EXIT

PG_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
export RECOVERY_PG_PORT="$PG_PORT"
export RECOVERY_REDIS_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"

DRILL_STARTED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "==> object recovery drill started at $DRILL_STARTED"

echo "==> starting isolated recovery stack"
docker compose -p "$PROJ" -f "$COMPOSE_FILE" up -d >/dev/null
for _ in $(seq 1 90); do
  ready=1
  docker exec "$PROJ"-postgres-1 pg_isready -U omnicraft -d omnicraft >/dev/null 2>&1 || ready=0
  mc_ ready local >/dev/null 2>&1 || ready=0
  mc_ ls "local/$BUCKET" >/dev/null 2>&1 || ready=0
  [ "$ready" -eq 1 ] && break
  sleep 2
done
docker exec "$PROJ"-postgres-1 pg_isready -U omnicraft -d omnicraft >/dev/null 2>&1 || { echo "postgres did not become ready" >&2; exit 1; }
mc_ ready local >/dev/null 2>&1 || { echo "minio did not become ready" >&2; exit 1; }
mc_ ls "local/$BUCKET" >/dev/null 2>&1 || { echo "versioned bucket was not created" >&2; exit 1; }

echo "==> seeding attachment keys and objects"
docker exec -i "$PROJ"-postgres-1 psql -U omnicraft -d omnicraft -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
CREATE TABLE drill_attachments (
    id BIGSERIAL PRIMARY KEY,
    oss_key TEXT NOT NULL UNIQUE,
    sha256 TEXT NOT NULL
);
SQL
SEED_DIR="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-object-seed.XXXXXX")"
printf 'object-alpha-content' > "$SEED_DIR/alpha.bin"
printf 'object-beta-content' > "$SEED_DIR/beta.bin"
printf 'object-gamma-content' > "$SEED_DIR/gamma.bin"
for name in alpha beta gamma; do
  checksum="$(sha256_of "$SEED_DIR/$name.bin" | awk '{print $1}')"
  docker exec "$PROJ"-postgres-1 psql -U omnicraft -d omnicraft -v ON_ERROR_STOP=1 \
    -c "INSERT INTO drill_attachments (oss_key, sha256) VALUES ('objs/$name.bin', '$checksum')" >/dev/null
  mc_ cp "/seed/$name.bin" "local/$BUCKET/objs/$name.bin" >/dev/null
done

echo "==> deleting one object and restoring its version"
mc_ rm "local/$BUCKET/objs/beta.bin" >/dev/null
mc_ undo "local/$BUCKET/objs/beta.bin" >/dev/null
RESTORED_CONTENT="$(mc_ cat "local/$BUCKET/objs/beta.bin" 2>/dev/null)"
RESTORED_CHECKSUM="$(printf '%s' "$RESTORED_CONTENT" | sha256_of)"
EXPECTED_CHECKSUM="$(docker exec "$PROJ"-postgres-1 psql -U omnicraft -d omnicraft -tAc \
  "SELECT sha256 FROM drill_attachments WHERE oss_key = 'objs/beta.bin'")"
if [ "$RESTORED_CHECKSUM" != "$EXPECTED_CHECKSUM" ]; then
  echo "object version restore checksum mismatch: $RESTORED_CHECKSUM vs $EXPECTED_CHECKSUM" >&2
  exit 1
fi

echo "==> reconciling database keys against the object store"
DB_KEYS="$(docker exec "$PROJ"-postgres-1 psql -U omnicraft -d omnicraft -tAc "SELECT oss_key FROM drill_attachments ORDER BY oss_key")"
OBJ_KEYS="$(mc_ ls --recursive --json "local/$BUCKET" 2>/dev/null \
  | python3 -c 'import json,sys
out=set()
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    obj=json.loads(line)
    if obj.get("type")=="file":
        out.add(obj["key"])
print("\n".join(sorted(out)))')"
if [ "$DB_KEYS" != "$OBJ_KEYS" ]; then
  echo "reconciliation mismatch:" >&2
  echo "DB keys: $DB_KEYS" >&2
  echo "Object keys: $OBJ_KEYS" >&2
  exit 1
fi

DRILL_FINISHED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
RECONCILED_COUNT="$(printf '%s\n' "$DB_KEYS" | sed '/^$/d' | wc -l | tr -d ' ')"
python3 - "$REPORT_DIR/object-recovery-evidence.json" \
  "$DRILL_STARTED" "$DRILL_FINISHED" "$BUCKET" "$RESTORED_CHECKSUM" "$EXPECTED_CHECKSUM" "$RECONCILED_COUNT" <<'PY'
import json
import sys

out, started, finished, bucket, restored, expected, reconciled = sys.argv[1:8]

evidence = {
    "drill": "object-recovery-drill",
    "started_at": started,
    "finished_at": finished,
    "bucket": bucket,
    "deleted_object": "objs/beta.bin",
    "restored_checksum": restored,
    "expected_checksum": expected,
    "reconciled_keys": int(reconciled),
    "adapter": "minio local stand-in; real Aliyun OSS versioning remains an Ops-08 blocker",
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(evidence, f, indent=2)
    f.write("\n")
print(json.dumps(evidence, indent=2))
PY

echo "==> object recovery drill passed"
