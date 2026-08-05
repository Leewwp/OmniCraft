#!/usr/bin/env bash
# Contract tests for scripts/ops/archive-audit-logs.sh: aggregation of only
# warning/error entries, AES encryption, upload to the sink, checksum and
# retention metadata verification, and refusal of invalid inputs.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ARCHIVER="$SCRIPT_DIR/archive-audit-logs.sh"
if [ ! -f "$ARCHIVER" ]; then
  echo "archive-audit-logs.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-audit-archive.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

SOURCE_DIR="$TEMP_ROOT/source"
SINK="$TEMP_ROOT/sink"
KEY_FILE="$TEMP_ROOT/key"
REPORT_DIR="$TEMP_ROOT/report"
mkdir -p "$SOURCE_DIR" "$SINK" "$REPORT_DIR"
openssl rand -base64 48 > "$KEY_FILE"

# Two source files: one with mixed levels, one with only info lines.
cat > "$SOURCE_DIR/backend.jsonl" <<'JSONL'
{"time":"2026-08-05T00:00:01Z","level":"info","msg":"request","route":"/healthz"}
{"time":"2026-08-05T00:00:02Z","level":"warning","msg":"slow query body user@example.invalid token=secret-token","trace_id":"abc","route":"/api/v1/content/:id"}
{"time":"2026-08-05T00:00:03Z","level":"error","msg":"upstream failure body","trace_id":"def","route":"/api/v1/upload"}
JSONL
cat > "$SOURCE_DIR/worker.jsonl" <<'JSONL'
{"time":"2026-08-05T00:00:04Z","level":"info","msg":"worker tick"}
JSONL

expect_ok() {
  local message="$1"
  shift
  if ! bash "$ARCHIVER" "$@" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

expect_fail() {
  local message="$1"
  shift
  if bash "$ARCHIVER" "$@" >/dev/null 2>&1; then
    echo "FAIL: $message" >&2
    exit 1
  fi
}

# Missing required arguments -> usage error.
expect_fail "missing required arguments"
expect_fail "missing key file" -SourceDir "$SOURCE_DIR" -Destination "$SINK"
expect_fail "missing destination" -SourceDir "$SOURCE_DIR" -KeyFile "$KEY_FILE"
bash "$ARCHIVER" -SourceDir "$SOURCE_DIR" -KeyFile "$KEY_FILE" -Destination "$SINK" -ReportDir "$REPORT_DIR" >/dev/null 2>&1 \
  || { echo "FAIL: valid archive invocation returned nonzero" >&2; exit 1; }

# Exactly one encrypted artifact + manifest must land in the sink.
ARTIFACT="$(ls "$SINK"/*.enc 2>/dev/null | head -1 || true)"
MANIFEST="$(ls "$SINK"/*.manifest.json 2>/dev/null | head -1 || true)"
if [ -z "$ARTIFACT" ] || [ -z "$MANIFEST" ]; then
  echo "FAIL: no encrypted artifact/manifest written to the sink" >&2
  exit 1
fi
if [ "$(ls "$SINK"/*.enc | wc -l | tr -d ' ')" != "1" ]; then
  echo "FAIL: expected exactly one encrypted artifact" >&2
  exit 1
fi

# The summary plaintext must contain only warning/error entries.
PLAINTEXT="$TEMP_ROOT/decrypted-summary.json"
if ! openssl enc -d -aes-256-cbc -pbkdf2 -pass "file:${KEY_FILE}" -in "$ARTIFACT" -out "$PLAINTEXT" 2>/dev/null; then
  echo "FAIL: decryption roundtrip failed" >&2
  exit 1
fi
python3 - "$PLAINTEXT" <<'PY'
import json, sys
s = json.load(open(sys.argv[1], encoding="utf-8"))
assert s["entry_count"] == 2, f"entry_count = {s['entry_count']}, want 2"
levels = {e["level"] for e in s["entries"]}
assert levels == {"warning", "error"}, f"levels = {levels}"
assert s["archived_at"], "archived_at missing"
assert s["retention_days"] == 365, f"retention_days = {s['retention_days']}"
serialized = json.dumps(s)
assert "slow query body" not in serialized, "archive must not retain log message bodies"
assert "user@example.invalid" not in serialized, "archive must not retain direct identifiers"
assert "secret-token" not in serialized, "archive must not retain tokens"
PY
[ $? -eq 0 ] || { echo "FAIL: summary content validation" >&2; exit 1; }

# Manifest must carry matching plaintext and ciphertext checksums.
python3 - "$MANIFEST" "$PLAINTEXT" "$ARTIFACT" <<'PY'
import hashlib, json, sys
manifest_path, plaintext_path, ciphertext_path = sys.argv[1], sys.argv[2], sys.argv[3]
m = json.load(open(manifest_path, encoding="utf-8"))
plain_sha = hashlib.sha256(open(plaintext_path, "rb").read()).hexdigest()
cipher_sha = hashlib.sha256(open(ciphertext_path, "rb").read()).hexdigest()
assert m["plaintext_sha256"] == plain_sha, "manifest plaintext checksum mismatch"
assert m["ciphertext_sha256"] == cipher_sha, "manifest ciphertext checksum mismatch"
assert m["retention_days"] == 365, "manifest retention days mismatch"
assert m["redaction_checked"] is True, "redaction_checked must be true"
assert m["cipher"] == "aes-256-cbc-pbkdf2"
PY
[ $? -eq 0 ] || { echo "FAIL: manifest checksum validation" >&2; exit 1; }

# Evidence file must exist in the report dir.
if [ ! -f "$REPORT_DIR/archive-evidence.json" ]; then
  echo "FAIL: archive evidence not written" >&2
  exit 1
fi

# Invalid inputs must fail.
expect_fail "missing source dir" -SourceDir "$TEMP_ROOT/nope" -KeyFile "$KEY_FILE" -Destination "$SINK"
expect_fail "missing key file" -SourceDir "$SOURCE_DIR" -KeyFile "$TEMP_ROOT/nope" -Destination "$SINK"
expect_fail "missing destination" -SourceDir "$SOURCE_DIR" -KeyFile "$KEY_FILE" -Destination "$TEMP_ROOT/nope"

echo "OK: archive-audit-logs contract tests passed"
