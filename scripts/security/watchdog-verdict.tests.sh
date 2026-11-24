#!/usr/bin/env bash
# Contract tests for scripts/security/watchdog-verdict.sh. Verifies the three
# verdict fixture states (real advisory -> one needs-triage issue; scanner
# failure only -> no issue; missing/corrupt artifact -> no issue), severity
# scoping (critical/high only), open-issue dedupe by advisory id, and the
# operational-failure exit path. All gh invocations go through a fake gh on
# PATH so no network or repository is touched.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WATCHDOG="$SCRIPT_DIR/watchdog-verdict.sh"
if [ ! -f "$WATCHDOG" ]; then
  echo "watchdog-verdict.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-watchdog-tests.XXXXXX")"
FAKE_BIN="$TEMP_ROOT/bin"
FIXTURE_DIR="$TEMP_ROOT/fixtures"
GH_LOG="$TEMP_ROOT/gh-calls.log"
ISSUE_LIST_FILE="$TEMP_ROOT/issue-list.json"
trap 'rm -rf "$TEMP_ROOT"' EXIT
mkdir -p "$FAKE_BIN" "$FIXTURE_DIR"

cat > "$FAKE_BIN/gh" <<'FAKE'
#!/usr/bin/env bash
echo "gh $*" >> "$GH_LOG"
case "$1 $2" in
  "issue list")
    if [ -f "$ISSUE_LIST_FILE" ]; then cat "$ISSUE_LIST_FILE"; else echo "[]"; fi
    exit 0
    ;;
  "issue create")
    exit "${FAKE_CREATE_EXIT:-0}"
    ;;
  *)
    echo "fake gh: unsupported invocation: $*" >&2
    exit 1
    ;;
esac
FAKE
chmod +x "$FAKE_BIN/gh"
: > "$GH_LOG"
PATH="$FAKE_BIN:$PATH"
export GH_LOG ISSUE_LIST_FILE

write_verdict() {
  python3 - "$FIXTURE_DIR/security-verdict.json" "$1" <<'PY'
import json, sys

blocked = json.loads(sys.argv[2])
with open(sys.argv[1], "w", encoding="utf-8") as out:
    json.dump({"findings": blocked, "blocked_findings": blocked,
               "informational_findings": [], "counts": {},
               "ok": not blocked, "policy": {}}, out)
PY
}

RUN_URL="https://github.com/Leewwp/OmniCraft/actions/runs/1234567890"
no_create_expected() {
  local message="$1"
  if grep -q "issue create" "$GH_LOG"; then
    echo "FAIL: $message (unexpected issue create)" >&2
    exit 1
  fi
}

reset_state() {
  : > "$GH_LOG"
  rm -f "$ISSUE_LIST_FILE" "$FIXTURE_DIR/security-verdict.json"
  unset FAKE_CREATE_EXIT || true
}

reset_state
write_verdict '[{"id":"GO-2026-6278","component":"github.com/gorilla/websocket","version":"v1.5.3","severity":"critical","source":"govulncheck","detail":"fixed v1.5.4"},{"id":"scanner-failure:trivy-fs","component":"trivy-fs.json","version":"N/A","severity":"critical","source":"trivy-fs","detail":"report missing or invalid"}]'
if ! bash "$WATCHDOG" -ReportDir "$FIXTURE_DIR" -RunUrl "$RUN_URL"; then
  echo "FAIL: real advisory must exit 0 after opening the issue" >&2
  exit 1
fi
if ! grep -q "issue create" "$GH_LOG"; then
  echo "FAIL: real advisory must open an issue" >&2
  exit 1
fi
CREATE_LINE="$(grep 'issue create' "$GH_LOG" | head -1)"
case "$CREATE_LINE" in
  *GO-2026-6278*github.com/gorilla/websocket*v1.5.3*) : ;;
  *) echo "FAIL: issue title must contain advisory id, component, and version: $CREATE_LINE" >&2; exit 1 ;;
esac
if ! grep -q -- '--label needs-triage' "$GH_LOG"; then
  echo "FAIL: issue create must carry the needs-triage label" >&2
  exit 1
fi
if ! grep -Fq "$RUN_URL" "$GH_LOG"; then
  echo "FAIL: issue body must contain the run URL" >&2
  exit 1
fi
if grep -q 'scanner-failure' "$GH_LOG"; then
  echo "FAIL: scanner-failure findings must not reach any gh invocation" >&2
  exit 1
fi
if ! grep -q 'issue list.*GO-2026-6278' "$GH_LOG"; then
  echo "FAIL: dedupe must search open issues for the advisory id" >&2
  exit 1
fi
echo "PASS: real advisory opens one labeled issue with id/component/version"

reset_state
write_verdict '[{"id":"scanner-failure:trivy-fs","component":"trivy-fs.json","version":"N/A","severity":"critical","source":"trivy-fs","detail":"report missing or invalid"}]'
if ! bash "$WATCHDOG" -ReportDir "$FIXTURE_DIR" -RunUrl "$RUN_URL"; then
  echo "FAIL: scanner failure only must exit 0 (security-gate is already red)" >&2
  exit 1
fi
no_create_expected "scanner failure only must not open an issue"
echo "PASS: scanner-failure-only opens nothing"

reset_state
rm -f "$FIXTURE_DIR/security-verdict.json"
if ! bash "$WATCHDOG" -ReportDir "$FIXTURE_DIR" -RunUrl "$RUN_URL"; then
  echo "FAIL: missing artifact must exit 0" >&2
  exit 1
fi
no_create_expected "missing artifact must not open an issue"
printf 'not json {' > "$FIXTURE_DIR/security-verdict.json"
if ! bash "$WATCHDOG" -ReportDir "$FIXTURE_DIR" -RunUrl "$RUN_URL"; then
  echo "FAIL: corrupt artifact must exit 0" >&2
  exit 1
fi
no_create_expected "corrupt artifact must not open an issue"
echo "PASS: missing/corrupt artifact opens nothing"

reset_state
write_verdict '[{"id":"CVE-2099-1111","component":"npm/foo","version":"1.0.0","severity":"MEDIUM","source":"trivy-fs","detail":"fixed 1.0.1"}]'
if ! bash "$WATCHDOG" -ReportDir "$FIXTURE_DIR" -RunUrl "$RUN_URL"; then
  echo "FAIL: non-critical/high severity must exit 0" >&2
  exit 1
fi
no_create_expected "medium severity is out of the watchdog scope"
echo "PASS: severity scoped to critical/high"

reset_state
write_verdict '[{"id":"GO-2026-6278","component":"github.com/gorilla/websocket","version":"v1.5.3","severity":"critical","source":"govulncheck","detail":"fixed v1.5.4"}]'
printf '[{"number":42,"title":"security: red lock GO-2026-6278 pending fix"}]' > "$ISSUE_LIST_FILE"
if ! bash "$WATCHDOG" -ReportDir "$FIXTURE_DIR" -RunUrl "$RUN_URL"; then
  echo "FAIL: covered advisory must exit 0" >&2
  exit 1
fi
no_create_expected "an open issue already tracking the advisory id"
echo "PASS: dedupe skips advisories already tracked"

reset_state
write_verdict '[{"id":"GO-2026-6278","component":"github.com/gorilla/websocket","version":"v1.5.3","severity":"critical","source":"govulncheck","detail":"fixed v1.5.4"}]'
export FAKE_CREATE_EXIT=1
if bash "$WATCHDOG" -ReportDir "$FIXTURE_DIR" -RunUrl "$RUN_URL" >/dev/null 2>&1; then
  echo "FAIL: gh operational failure must exit non-zero" >&2
  exit 1
fi
echo "PASS: gh operational failure surfaces as a red job"

unset FAKE_CREATE_EXIT || true
echo "watchdog-verdict contract tests passed"
