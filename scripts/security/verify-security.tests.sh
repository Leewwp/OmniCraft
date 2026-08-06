#!/usr/bin/env bash
# Contract tests for scripts/security/verify-security.sh: policy/exception
# governance, secret-fixture detection, vulnerable-lockfile detection and
# gate isolation. Each fixture must fail only its intended gate. Runs
# without containers except the gitleaks and trivy fixtures which use the
# pinned images like the real verifier.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VERIFY="$SCRIPT_DIR/verify-security.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SECURITY_DIR="$REPO_ROOT/security"

if [ ! -f "$VERIFY" ]; then
  echo "verify-security.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-security.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

expect_exit() {
  local expected="$1" label="$2" root="$3" gates="$4"
  local actual=0
  bash "$VERIFY" -RepoRoot "$root" -Gates "$gates" -ReportDir "$TEMP_ROOT/report-$label" \
    >"$TEMP_ROOT/$label.out" 2>"$TEMP_ROOT/$label.err" || actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL: $label: expected exit $expected, got $actual" >&2
    cat "$TEMP_ROOT/$label.err" >&2
    exit 1
  fi
  echo "OK: $label"
}

# ------------------------------------------------------------- real policy
expect_exit 0 "repository policy gate passes" "$REPO_ROOT" "policy"

# --------------------------------------------------- expired exception
# The fixture enables high_exceptions_enabled so the ONLY failure reason left
# is the expired-exception rule: with the flag false the gate would fail for
# "high exceptions disabled" regardless of the expiry check, hiding its loss
# of discrimination. The report is then asserted to cite the expiry rule.
EXPIRED_ROOT="$TEMP_ROOT/expired"
mkdir -p "$EXPIRED_ROOT/security"
cp "$SECURITY_DIR"/*.json "$EXPIRED_ROOT/security/"
python3 - "$EXPIRED_ROOT/security/scan-policy.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["high_exceptions_enabled"] = True
json.dump(d, open(path, "w"), indent=2)
PY
python3 - "$EXPIRED_ROOT/security/exceptions.json" <<'PY'
import json, sys, datetime
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["exceptions"] = [{
    "id": "EXP-001",
    "vulnerability_id": "GHSA-0000-0000-0000",
    "affected_component": "fixture-pkg",
    "affected_version": "1.0.0",
    "severity": "high",
    "risk_description": "fixture",
    "compensating_controls": "fixture controls",
    "author": "alice",
    "approver": "bob",
    "approval_date": "2020-01-01",
    "expiry_date": "2020-02-01",
    "approval_ref": {
        "commit": "1111111111111111111111111111111111111111",
        "event": "https://github.com/omnicraft/omnicraft/pull/1/reviews",
    },
    "status": "active",
}]
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "expired-exception-rejected" "$EXPIRED_ROOT" "policy"
# The fixture must fail ONLY on the expiry rule: the report must cite it and
# must not cite the high-exceptions-disabled rule (the flag is enabled) or any
# other rule (the exception record is otherwise valid).
python3 - "$TEMP_ROOT/report-expired-exception-rejected/policy-gate.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
expected = ["exception[0]: expired exception (expiry 2020-02-01)"]
assert d["errors"] == expected, d["errors"]
PY

# ------------------------------------------------ malformed exception batch
MALFORMED_ROOT="$TEMP_ROOT/malformed"
mkdir -p "$MALFORMED_ROOT/security"
cp "$SECURITY_DIR"/*.json "$MALFORMED_ROOT/security/"

# missing approval_ref.commit
python3 - "$MALFORMED_ROOT/security/exceptions.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["exceptions"] = [{
    "id": "EXP-BAD-REF",
    "vulnerability_id": "GHSA-1111-1111-1111",
    "affected_component": "fixture-pkg",
    "affected_version": "1.0.0",
    "severity": "high",
    "risk_description": "fixture",
    "compensating_controls": "fixture controls",
    "author": "alice",
    "approver": "bob",
    "approval_date": "2099-01-01",
    "expiry_date": "2099-02-01",
    "approval_ref": {
        "event": "https://github.com/omnicraft/omnicraft/issues/1",
    },
    "status": "active",
}]
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "exception with mutable issue URL and missing commit rejected" "$MALFORMED_ROOT" "policy"

# approver must differ from author
python3 - "$MALFORMED_ROOT/security/exceptions.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["exceptions"] = [{
    "id": "EXP-SAME",
    "vulnerability_id": "GHSA-2222-2222-2222",
    "affected_component": "fixture-pkg",
    "affected_version": "1.0.0",
    "severity": "high",
    "risk_description": "fixture",
    "compensating_controls": "fixture controls",
    "author": "alice",
    "approver": "alice",
    "approval_date": "2099-01-01",
    "expiry_date": "2099-02-01",
    "approval_ref": {
        "commit": "2222222222222222222222222222222222222222",
        "event": "https://github.com/omnicraft/omnicraft/pull/1/reviews/1",
    },
    "status": "active",
}]
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "exception with approver equal to author rejected" "$MALFORMED_ROOT" "policy"

# secret finding is non-waivable
python3 - "$MALFORMED_ROOT/security/exceptions.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["exceptions"] = [{
    "id": "EXP-SECRET",
    "vulnerability_id": "gitleaks:generic-api-key",
    "affected_component": "some/file.go",
    "affected_version": "*",
    "severity": "high",
    "risk_description": "fixture",
    "compensating_controls": "fixture controls",
    "author": "alice",
    "approver": "bob",
    "approval_date": "2099-01-01",
    "expiry_date": "2099-02-01",
    "approval_ref": {
        "commit": "3333333333333333333333333333333333333333",
        "event": "https://github.com/omnicraft/omnicraft/pull/1/reviews/1",
    },
    "status": "active",
}]
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "exception ledger may not waive secret findings" "$MALFORMED_ROOT" "policy"

# missing affected_version
python3 - "$MALFORMED_ROOT/security/exceptions.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["exceptions"] = [{
    "id": "EXP-NO-VER",
    "vulnerability_id": "GHSA-5555-5555-5555",
    "affected_component": "fixture-pkg",
    "severity": "high",
    "risk_description": "fixture",
    "compensating_controls": "fixture controls",
    "author": "alice",
    "approver": "bob",
    "approval_date": "2099-01-01",
    "expiry_date": "2099-02-01",
    "approval_ref": {
        "commit": "5555555555555555555555555555555555555555",
        "event": "https://github.com/omnicraft/omnicraft/pull/1/reviews/1",
    },
    "status": "active",
}]
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "exception-missing-affected-version-rejected" "$MALFORMED_ROOT" "policy"
grep -q "missing field affected_version" "$TEMP_ROOT/exception-missing-affected-version-rejected.err" || {
  echo "FAIL: missing affected_version must be cited in the policy errors" >&2
  exit 1
}

# missing compensating_controls
python3 - "$MALFORMED_ROOT/security/exceptions.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["exceptions"] = [{
    "id": "EXP-NO-CTRL",
    "vulnerability_id": "GHSA-6666-6666-6666",
    "affected_component": "fixture-pkg",
    "affected_version": "1.0.0",
    "severity": "high",
    "risk_description": "fixture",
    "author": "alice",
    "approver": "bob",
    "approval_date": "2099-01-01",
    "expiry_date": "2099-02-01",
    "approval_ref": {
        "commit": "6666666666666666666666666666666666666666",
        "event": "https://github.com/omnicraft/omnicraft/pull/1/reviews/1",
    },
    "status": "active",
}]
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "exception-missing-compensating-controls-rejected" "$MALFORMED_ROOT" "policy"
grep -q "missing field compensating_controls" "$TEMP_ROOT/exception-missing-compensating-controls-rejected.err" || {
  echo "FAIL: missing compensating_controls must be cited in the policy errors" >&2
  exit 1
}

# missing approval_date (must trigger the field-missing check on its own,
# independent of the other date fields)
python3 - "$MALFORMED_ROOT/security/exceptions.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["exceptions"] = [{
    "id": "EXP-NO-DATE",
    "vulnerability_id": "GHSA-7777-7777-7777",
    "affected_component": "fixture-pkg",
    "affected_version": "1.0.0",
    "severity": "high",
    "risk_description": "fixture",
    "compensating_controls": "fixture controls",
    "author": "alice",
    "approver": "bob",
    "expiry_date": "2099-02-01",
    "approval_ref": {
        "commit": "7777777777777777777777777777777777777777",
        "event": "https://github.com/omnicraft/omnicraft/pull/1/reviews/1",
    },
    "status": "active",
}]
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "exception-missing-approval-date-rejected" "$MALFORMED_ROOT" "policy"
grep -q "missing field approval_date" "$TEMP_ROOT/exception-missing-approval-date-rejected.err" || {
  echo "FAIL: missing approval_date must be cited in the policy errors" >&2
  exit 1
}

# ----------------------------------------------------- missing categories
NOCAT_ROOT="$TEMP_ROOT/nocat"
mkdir -p "$NOCAT_ROOT/security"
cp "$SECURITY_DIR"/*.json "$NOCAT_ROOT/security/"
python3 - "$NOCAT_ROOT/security/scan-policy.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["categories"] = ["go_dependencies"]
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "policy with missing scan categories rejected" "$NOCAT_ROOT" "policy"

# --------------------------------------------------- high exception disabled
HIGH_ROOT="$TEMP_ROOT/high"
mkdir -p "$HIGH_ROOT/security"
cp "$SECURITY_DIR"/*.json "$HIGH_ROOT/security/"
python3 - "$HIGH_ROOT/security/exceptions.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["exceptions"] = [{
    "id": "EXP-HIGH",
    "vulnerability_id": "GHSA-4444-4444-4444",
    "affected_component": "fixture-pkg",
    "affected_version": "1.0.0",
    "severity": "high",
    "risk_description": "fixture",
    "compensating_controls": "fixture controls",
    "author": "alice",
    "approver": "bob",
    "approval_date": "2099-01-01",
    "expiry_date": "2099-02-01",
    "approval_ref": {
        "commit": "4444444444444444444444444444444444444444",
        "event": "https://github.com/omnicraft/omnicraft/pull/1/reviews/1",
    },
    "status": "active",
}]
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "high exception rejected while high_exceptions_enabled is false" "$HIGH_ROOT" "policy"

# ------------------------------------------------------------ secret fixture
SECRET_ROOT="$TEMP_ROOT/secret"
mkdir -p "$SECRET_ROOT/sub" "$SECRET_ROOT/security"
cp "$SECURITY_DIR"/*.json "$SECRET_ROOT/security/"
cat > "$SECRET_ROOT/.gitleaks.toml" <<'EOF'
title = "fixture"

[extend]
useDefault = true

[allowlist]
paths = [ "allowed/not-secret.txt" ]
EOF
# Build the AWS key pattern at runtime so the committed source never contains
# the literal secret pattern gitleaks scans for; the generated fixture file in
# the temp dir still matches the default gitleaks rule.
printf 'AWS_ACCESS_KEY_ID=AKIA%sEXAMPLE\n' 'IOSFODNN7' > "$SECRET_ROOT/sub/credentials.txt"
expect_exit 0 "secret fixture passes policy gate (isolation)" "$SECRET_ROOT" "policy"
expect_exit 1 "fake secret fails the gitleaks gate" "$SECRET_ROOT" "secrets"

# ------------------------------------------------ vulnerable lockfile fixture
# run_npm_gate audits BOTH frontend and tauri-client lockfiles; the fixture
# must provide a clean tauri-client lockfile so the vulnerable frontend
# lockfile is the only reason the npm gate can fail (a missing tauri-client
# lockfile alone would fail the gate even if vulnerability detection broke).
# The paired clean-root fixture asserts the reverse: two clean lockfiles must
# pass the npm gate, proving the gate is discriminative.
NPM_ROOT="$TEMP_ROOT/npm"
mkdir -p "$NPM_ROOT/frontend" "$NPM_ROOT/tauri-client" "$NPM_ROOT/security"
cp "$SECURITY_DIR"/*.json "$NPM_ROOT/security/"
cat > "$NPM_ROOT/frontend/package.json" <<'EOF'
{
  "name": "fixture",
  "version": "1.0.0",
  "dependencies": {
    "lodash": "4.17.15"
  }
}
EOF
cat > "$NPM_ROOT/tauri-client/package.json" <<'EOF'
{
  "name": "fixture-tauri-client",
  "version": "1.0.0"
}
EOF
(
  cd "$NPM_ROOT/frontend" \
    && npm install --package-lock-only --registry=https://registry.npmjs.org --ignore-scripts >/dev/null 2>&1
)
(
  cd "$NPM_ROOT/tauri-client" \
    && npm install --package-lock-only --registry=https://registry.npmjs.org --ignore-scripts >/dev/null 2>&1
)
expect_exit 0 "vulnerable lockfile fixture passes policy gate (isolation)" "$NPM_ROOT" "policy"
expect_exit 1 "vulnerable fixture lockfile fails the npm audit gate" "$NPM_ROOT" "npm"

# Reverse assertion: regenerate the same frontend as a clean lockfile (the
# tauri-client clean lockfile stays). The npm gate must now exit 0 - before
# this fixture provided tauri-client/ the gate failed on the missing lockfile
# even when detection was fully broken, so this contrast proves the gate
# discriminates on actual vulnerability findings.
cat > "$NPM_ROOT/frontend/package.json" <<'EOF'
{
  "name": "fixture-clean",
  "version": "1.0.0"
}
EOF
(
  cd "$NPM_ROOT/frontend" \
    && npm install --package-lock-only --registry=https://registry.npmjs.org --ignore-scripts >/dev/null 2>&1
)
expect_exit 0 "clean lockfiles pass the npm audit gate (discrimination)" "$NPM_ROOT" "npm"

echo "OK: verify-security contract tests passed"
