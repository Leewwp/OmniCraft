#!/usr/bin/env bash
# Contract tests for scripts/db/build-historical-fixture.sh. The generation
# path requires docker and is exercised once by the operator; these tests
# cover argument validation, overwrite protection and -ValidateOnly behavior
# against the committed fixture and mutated copies.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILDER="$SCRIPT_DIR/build-historical-fixture.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TESTDATA="$REPO_ROOT/backend/internal/migration/testdata"

if [ ! -f "$BUILDER" ]; then
  echo "build-historical-fixture.sh does not exist" >&2
  exit 1
fi
if [ ! -f "$TESTDATA/historical-050.sql" ] || [ ! -f "$TESTDATA/historical-050.sha256" ] || [ ! -f "$TESTDATA/historical-050.manifest.json" ]; then
  echo "committed historical-050 fixture is incomplete" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-build-fixture.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

expect_exit() {
  local want="$1"
  local message="$2"
  shift 2
  local got=0
  bash "$BUILDER" "$@" >/dev/null 2>&1 || got=$?
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $message (exit $got, want $want)" >&2
    exit 1
  fi
}

expect_ok() {
  local message="$1"
  shift
  if ! bash "$BUILDER" "$@" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

expect_fail() {
  local message="$1"
  shift
  if bash "$BUILDER" "$@" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

expect_exit 2 "missing arguments must be rejected" 
expect_exit 2 "non-three-digit baseline must be rejected" -Baseline 50 -OutputDir "$TEMP_ROOT/out"
expect_exit 2 "unknown baseline migration must be rejected" -Baseline 999 -OutputDir "$TEMP_ROOT/out"
expect_exit 2 "an output dir with existing files must refuse without -Force" -Baseline 050 -OutputDir "$TESTDATA"

expect_ok "the committed fixture must validate" -ValidateOnly \
  -Fixture "$TESTDATA/historical-050.sql" \
  -Manifest "$TESTDATA/historical-050.manifest.json" \
  -Sha256 "$TESTDATA/historical-050.sha256"

cp "$TESTDATA/historical-050.sql" "$TEMP_ROOT/tampered.sql"
printf -- '-- hand-edited\n' >> "$TEMP_ROOT/tampered.sql"
expect_fail "a hand-edited dump must fail checksum validation" -ValidateOnly \
  -Fixture "$TEMP_ROOT/tampered.sql" \
  -Manifest "$TESTDATA/historical-050.manifest.json" \
  -Sha256 "$TESTDATA/historical-050.sha256"

cp "$TESTDATA/historical-050.manifest.json" "$TEMP_ROOT/tampered-manifest.json"
python3 - "$TEMP_ROOT/tampered-manifest.json" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as f:
    manifest = json.load(f)
manifest["image"] = "postgres:16"
with open(path, "w", encoding="utf-8") as f:
    json.dump(manifest, f)
PY
expect_fail "a manifest with a non-pinned image must be rejected" -ValidateOnly \
  -Fixture "$TESTDATA/historical-050.sql" \
  -Manifest "$TEMP_ROOT/tampered-manifest.json" \
  -Sha256 "$TESTDATA/historical-050.sha256"

cp "$TESTDATA/historical-050.manifest.json" "$TEMP_ROOT/missing-checksum-manifest.json"
python3 - "$TEMP_ROOT/missing-checksum-manifest.json" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as f:
    manifest = json.load(f)
manifest["migrations"].pop(0)
manifest["ledger"].pop(0)
with open(path, "w", encoding="utf-8") as f:
    json.dump(manifest, f)
PY
expect_fail "a manifest with a missing source checksum must be rejected" -ValidateOnly \
  -Fixture "$TESTDATA/historical-050.sql" \
  -Manifest "$TEMP_ROOT/missing-checksum-manifest.json" \
  -Sha256 "$TESTDATA/historical-050.sha256"

cp "$TESTDATA/historical-050.sql" "$TEMP_ROOT/edited-ledger.sql"
cp "$TESTDATA/historical-050.sha256" "$TEMP_ROOT/edited-ledger.sha256"
python3 - "$TEMP_ROOT/edited-ledger.sql" "$TEMP_ROOT/edited-ledger.sha256" <<'PY'
import hashlib
import sys

sql_path, sha_path = sys.argv[1], sys.argv[2]
with open(sql_path, encoding="utf-8") as f:
    contents = f.read()
contents = contents.replace("INSERT INTO public.schema_migrations VALUES (50,", "INSERT INTO public.schema_migrations VALUES (51,", 1)
with open(sql_path, "w", encoding="utf-8") as f:
    f.write(contents)
with open(sha_path, "w", encoding="utf-8") as f:
    f.write(hashlib.sha256(contents.encode("utf-8")).hexdigest())
PY
python3 - "$TESTDATA/historical-050.manifest.json" "$TEMP_ROOT/edited-ledger.sql" "$TEMP_ROOT/edited-ledger.manifest.json" <<'PY'
import hashlib
import json
import sys

source_manifest, sql_path, output_manifest = sys.argv[1:4]
with open(source_manifest, encoding="utf-8") as f:
    manifest = json.load(f)
with open(sql_path, "rb") as f:
    manifest["dump_checksum"] = hashlib.sha256(f.read()).hexdigest()
with open(output_manifest, "w", encoding="utf-8") as f:
    json.dump(manifest, f)
PY
expect_fail "a fixture whose ledger does not represent the baseline must be rejected" -ValidateOnly \
  -Fixture "$TEMP_ROOT/edited-ledger.sql" \
  -Manifest "$TEMP_ROOT/edited-ledger.manifest.json" \
  -Sha256 "$TEMP_ROOT/edited-ledger.sha256"

echo "build-historical-fixture contract tests passed"
