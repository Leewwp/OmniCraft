#!/usr/bin/env bash
# =============================================================================
# OmniCraft historical fixture generator
# =============================================================================
# Generates the checksum-protected synthetic historical database fixture used
# by migration upgrade tests. The fixture is produced by applying the real
# migrations 001..<baseline> to the pinned pgvector/pgvector:pg16 image via
# the migrate runner, seeding a small amount of non-sensitive data, and
# exporting the resulting database with pg_dump.
#
# Usage:
#   bash scripts/db/build-historical-fixture.sh -Baseline 050 \
#       -OutputDir backend/internal/migration/testdata
#   bash scripts/db/build-historical-fixture.sh -ValidateOnly \
#       -Fixture backend/internal/migration/testdata/historical-050.sql \
#       -Manifest backend/internal/migration/testdata/historical-050.manifest.json \
#       -Sha256 backend/internal/migration/testdata/historical-050.sha256
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PINNED_IMAGE_NAME="pgvector/pgvector:pg16"
PINNED_IMAGE="pgvector/pgvector@sha256:a36250871de0833b8757561c72f2477ef1ddd1101afa4e617fb552e0de514c6b"
GENERATOR_VERSION="1"

BASELINE=""
OUTPUT_DIR=""
VALIDATE_ONLY=""
FIXTURE_SQL=""
FIXTURE_MANIFEST=""
FIXTURE_SHA256=""
FORCE=""

usage() {
  echo "usage: build-historical-fixture.sh -Baseline NNN -OutputDir <dir> [-Force]" >&2
  echo "       build-historical-fixture.sh -ValidateOnly -Fixture <sql> -Manifest <json> -Sha256 <sha>" >&2
  exit 2
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}
while [ $# -gt 0 ]; do
  case "$1" in
    -Baseline) BASELINE="$2"; shift 2 ;;
    -OutputDir) OUTPUT_DIR="$2"; shift 2 ;;
    -ValidateOnly) VALIDATE_ONLY="1"; shift ;;
    -Fixture) FIXTURE_SQL="$2"; shift 2 ;;
    -Manifest) FIXTURE_MANIFEST="$2"; shift 2 ;;
    -Sha256) FIXTURE_SHA256="$2"; shift 2 ;;
    -Force) FORCE="1"; shift ;;
    *) echo "unknown argument: $1" >&2; usage ;;
  esac
done

validate_fixture() {
  local sql_path="$1"
  local manifest_path="$2"
  local sha_path="$3"
  local migrations_dir="$4"

  [ -f "$sql_path" ] || { echo "fixture dump missing: $sql_path" >&2; return 1; }
  [ -f "$manifest_path" ] || { echo "fixture manifest missing: $manifest_path" >&2; return 1; }
  [ -f "$sha_path" ] || { echo "fixture sha256 sidecar missing: $sha_path" >&2; return 1; }

  local actual expected manifest_checksum
  actual="$(sha256_of "$sql_path")"
  expected="$(tr -d '[:space:]' < "$sha_path")"
  [ "$actual" = "$expected" ] || {
    echo "fixture dump checksum mismatch: file $actual, sidecar $expected" >&2
    return 1
  }

  manifest_checksum="$(python3 - "$manifest_path" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    print(json.load(f).get("dump_checksum", ""))
PY
)"
  [ -n "$manifest_checksum" ] || { echo "fixture manifest missing dump_checksum" >&2; return 1; }
  [ "$actual" = "$manifest_checksum" ] || {
    echo "fixture manifest dump_checksum mismatch: file $actual, manifest $manifest_checksum" >&2
    return 1
  }

  python3 - "$manifest_path" "$migrations_dir" "$sql_path" <<'PY' || { echo "fixture manifest invalid" >&2; return 1; }
import json
import os
import re
import sys

manifest_path, migrations_dir, sql_path = sys.argv[1:4]
with open(manifest_path, encoding="utf-8") as f:
    manifest = json.load(f)

required = ["schema_version", "generator_version", "baseline", "image",
            "image_digest", "dump_checksum", "pg_dump_version", "generation_command", "migrations", "ledger"]
for key in required:
    if manifest.get(key) in (None, "", []):
        print(f"manifest missing {key}", file=sys.stderr)
        sys.exit(1)

baseline = int(manifest["baseline"])
if manifest["image"] != "pgvector/pgvector:pg16":
    print("manifest image is not the pinned pgvector/pgvector:pg16", file=sys.stderr)
    sys.exit(1)
if manifest["image_digest"] != "pgvector/pgvector@sha256:a36250871de0833b8757561c72f2477ef1ddd1101afa4e617fb552e0de514c6b":
    print("manifest image digest is not the pinned pgvector/pgvector digest", file=sys.stderr)
    sys.exit(1)

sources = sorted(manifest["migrations"], key=lambda m: m["version"])
expected = []
for version in range(1, baseline + 1):
    path = os.path.join(migrations_dir, f"{version:03d}_*.sql")
    matches = sorted(p for p in os.scandir(migrations_dir) if p.is_file() and p.name.startswith(f"{version:03d}_"))
    if len(matches) != 1:
        print(f"migration {version:03d} not found or duplicated in {migrations_dir}", file=sys.stderr)
        sys.exit(1)
    expected.append({"version": version, "filename": matches[0].name,
                     "checksum": __import__("hashlib").sha256(open(matches[0].path, "rb").read()).hexdigest()})

if sources != expected:
    print("manifest source migration/checksum set does not match repository files", file=sys.stderr)
    sys.exit(1)

ledger = sorted(manifest["ledger"], key=lambda m: m["version"])
expected_ledger = [{"version": m["version"], "filename": m["filename"], "checksum": m["checksum"]} for m in expected]
if ledger != expected_ledger:
    print("manifest ledger does not exactly represent the baseline migration set", file=sys.stderr)
    sys.exit(1)
with open(sql_path, encoding="utf-8") as f:
    sql_text = f.read()
dump_ledger = [
    {"version": int(version), "filename": filename, "checksum": checksum}
    for version, filename, checksum in re.findall(
        r"^INSERT INTO public\.schema_migrations VALUES \(([0-9]+), '([^']+)', '([0-9a-f]{64})',",
        sql_text,
        flags=re.MULTILINE,
    )
]
dump_ledger.sort(key=lambda row: (row["version"], row["filename"]))
if dump_ledger != expected_ledger:
    print("fixture SQL ledger does not exactly represent the baseline migration set", file=sys.stderr)
    sys.exit(1)
print("fixture validated: checksum, manifest, source checksums and ledger rows match")
PY
}

if [ -n "$VALIDATE_ONLY" ]; then
  [ -n "$FIXTURE_SQL" ] && [ -n "$FIXTURE_MANIFEST" ] && [ -n "$FIXTURE_SHA256" ] || usage
  if validate_fixture "$FIXTURE_SQL" "$FIXTURE_MANIFEST" "$FIXTURE_SHA256" "$REPO_ROOT/backend/migrations"; then
    exit 0
  fi
  exit 1
fi

[ -n "$BASELINE" ] && [ -n "$OUTPUT_DIR" ] || usage
if ! echo "$BASELINE" | grep -qE '^[0-9]{3}$'; then
  echo "baseline must be a three-digit migration version" >&2
  exit 2
fi

MIGRATIONS_DIR="$REPO_ROOT/backend/migrations"
BASELINE_FILE=""
for candidate in "$MIGRATIONS_DIR/${BASELINE}"_*.sql; do
  [ -e "$candidate" ] && BASELINE_FILE="$candidate"
done
[ -n "$BASELINE_FILE" ] || { echo "baseline migration $BASELINE does not exist in $MIGRATIONS_DIR" >&2; exit 2; }

FIXTURE_PREFIX="historical-${BASELINE}"
SQL_OUT="$OUTPUT_DIR/${FIXTURE_PREFIX}.sql"
SHA_OUT="$OUTPUT_DIR/${FIXTURE_PREFIX}.sha256"
MANIFEST_OUT="$OUTPUT_DIR/${FIXTURE_PREFIX}.manifest.json"

mkdir -p "$OUTPUT_DIR"
if [ -z "$FORCE" ] && { [ -e "$SQL_OUT" ] || [ -e "$SHA_OUT" ] || [ -e "$MANIFEST_OUT" ]; }; then
  echo "fixture outputs already exist; rerun with -Force to regenerate" >&2
  exit 2
fi

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
docker image inspect "$PINNED_IMAGE" >/dev/null 2>&1 || { echo "pulling $PINNED_IMAGE" >&2; docker pull "$PINNED_IMAGE" >/dev/null; }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-fixture.XXXXXX")"
CONTAINER="omnicraft-fixture-gen-$$"
PORT=""
cleanup() {
  if [ -n "$CONTAINER" ]; then
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT

# Stage only the baseline migrations plus metadata into a temp dir.
mkdir -p "$WORK/migrations"
for migration in "$MIGRATIONS_DIR"/[0-9][0-9][0-9]_*.sql; do
  name="$(basename "$migration")"
  version="${name%%_*}"
  if [ "$((10#$version))" -le "$((10#$BASELINE))" ]; then
    cp "$migration" "$WORK/migrations/"
  fi
done
cp "$MIGRATIONS_DIR/metadata.json" "$WORK/migrations/"

# Pick a free port for the generator container.
PORT="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"

echo "==> starting $PINNED_IMAGE (port $PORT)"
docker run -d --name "$CONTAINER" -p "127.0.0.1:$PORT:5432" \
  -e POSTGRES_USER=omnicraft -e POSTGRES_PASSWORD=omnicraft -e POSTGRES_DB=omnicraft \
  "$PINNED_IMAGE" >/dev/null

for _ in $(seq 1 60); do
  if docker exec "$CONTAINER" pg_isready -U omnicraft >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$CONTAINER" pg_isready -U omnicraft >/dev/null 2>&1 || { echo "generator database did not become ready" >&2; exit 1; }

DSN="host=127.0.0.1 port=$PORT user=omnicraft password=omnicraft dbname=omnicraft sslmode=disable"
GENERATION_COMMAND="bash scripts/db/build-historical-fixture.sh -Baseline $BASELINE -OutputDir $OUTPUT_DIR"
echo "==> applying real migrations 001..$BASELINE via the migrate runner"
( cd "$REPO_ROOT/backend" && go run ./cmd/migrate -DSN "$DSN" -Dir "$WORK/migrations" -Metadata "$WORK/migrations/metadata.json" -SummaryPath "$WORK/migration-summary.json" )

# Seed a small amount of non-sensitive synthetic data after the schema
# exists. -i is required so stdin (the heredoc) reaches psql.
docker exec -i "$CONTAINER" psql -U omnicraft -d omnicraft -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
INSERT INTO tags (name, category, usage_count) VALUES
  ('synthetic-a', 'seed', 1),
  ('synthetic-b', 'seed', 2),
  ('synthetic-c', 'seed', 3);
INSERT INTO users (email, password_hash, username, preferred_locale)
  VALUES ('fixture-seed@example.invalid', '$2a$10$synthetic-hash-do-not-use', 'fixture-seed-user', 'zh-CN');
SQL
docker exec "$CONTAINER" psql -U omnicraft -d omnicraft -tAc "SELECT count(*) FROM tags" | grep -q "^3$" || {
  echo "fixture seed data was not inserted" >&2
  exit 1
}

IMAGE_DIGEST="$(docker inspect --format='{{index .RepoDigests 0}}' "$CONTAINER" 2>/dev/null || true)"
if [ -z "$IMAGE_DIGEST" ]; then
  IMAGE_DIGEST="$(docker image inspect --format='{{index .RepoDigests 0}}' "$PINNED_IMAGE" 2>/dev/null || echo "")"
fi

echo "==> exporting fixture dump"
docker exec "$CONTAINER" pg_dump -U omnicraft -d omnicraft \
  --format=plain --inserts --no-owner --no-acl > "$SQL_OUT"
PG_DUMP_VERSION="$(docker exec "$CONTAINER" pg_dump --version | tr -d '\r')"

shasum -a 256 "$SQL_OUT" | awk '{print $1}' > "$SHA_OUT"

# Capture ledger rows exactly as recorded in the generated database.
LEDGER_JSON="$(docker exec "$CONTAINER" psql -U omnicraft -d omnicraft -tAc \
  "SELECT json_agg(json_build_object('version', version, 'filename', filename, 'checksum', checksum) ORDER BY version) FROM schema_migrations")"
[ -n "$LEDGER_JSON" ] && [ "$LEDGER_JSON" != "null" ] || { echo "generated database has no migration ledger" >&2; exit 1; }

python3 - "$MANIFEST_OUT" "$BASELINE" "$GENERATOR_VERSION" "$PINNED_IMAGE_NAME" "$IMAGE_DIGEST" "$GENERATION_COMMAND" "$LEDGER_JSON" "$MIGRATIONS_DIR" "$(cat "$SHA_OUT")" "$PG_DUMP_VERSION" <<'PY'
import datetime
import hashlib
import json
import os
import sys

out, baseline, generator_version, image, image_digest, command, ledger_json, migrations_dir, dump_checksum, pg_dump_version = sys.argv[1:11]
baseline_int = int(baseline)

migrations = []
for version in range(1, baseline_int + 1):
    name = next(p.name for p in os.scandir(migrations_dir) if p.is_file() and p.name.startswith(f"{version:03d}_"))
    path = os.path.join(migrations_dir, name)
    checksum = hashlib.sha256(open(path, "rb").read()).hexdigest()
    migrations.append({"version": version, "filename": name, "checksum": checksum})

manifest = {
    "schema_version": 1,
    "generator_version": generator_version,
    "baseline": baseline,
    "image": image,
    "image_digest": image_digest,
    "dump_checksum": dump_checksum,
    "pg_dump_version": pg_dump_version,
    "generated_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "generation_command": command,
    "migrations": migrations,
    "ledger": json.loads(ledger_json),
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2, ensure_ascii=False)
    f.write("\n")
PY

echo "==> validating generated fixture"
if ! validate_fixture "$SQL_OUT" "$MANIFEST_OUT" "$SHA_OUT" "$MIGRATIONS_DIR"; then
  echo "generated fixture failed validation" >&2
  exit 1
fi

echo "==> fixture written to $OUTPUT_DIR"
ls -la "$SQL_OUT" "$SHA_OUT" "$MANIFEST_OUT"
