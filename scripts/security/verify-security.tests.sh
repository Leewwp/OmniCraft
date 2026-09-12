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

# Verdict-fixture variant: runs only the verdict (-VerdictOnly) over the
# reports already present in the given report dir, so crafted scanner reports
# can be judged without running the real scanners.
expect_verdict() {
  local expected="$1" label="$2" root="$3" gates="$4" report_dir="$5"
  local actual=0
  bash "$VERIFY" -RepoRoot "$root" -Gates "$gates" -ReportDir "$report_dir" -VerdictOnly \
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

[[allowlists]]
paths = [ "allowed/not-secret.txt" ]
EOF
# Build the planted secret at runtime so the committed source never contains
# the literal secret pattern gitleaks scans for; the generated fixture file in
# the temp dir still matches the default gitleaks rule (the historical AWS
# AKIA..EXAMPLE probe is ignored by gitleaks >= 8.19 stopword handling, so the
# fixture plants a generated high-entropy generic api key instead).
SECRET_PLANT="$(python3 -c 'import hashlib; print("sk-" + hashlib.sha256(b"secret-fixture").hexdigest()[:32])')"
printf '{"api_key": "%s"}\n' "$SECRET_PLANT" > "$SECRET_ROOT/sub/credentials.txt"
expect_exit 0 "secret fixture passes policy gate (isolation)" "$SECRET_ROOT" "policy"
expect_exit 1 "fake secret fails the gitleaks gate" "$SECRET_ROOT" "secrets"

# ------------------------------------ gitleaks scope counter-experiments (F-06)
# The content exemptions in the repo .gitleaks.toml must be conditioned on
# BOTH path and content, evaluated per finding. Three planted roots replay
# the audit counter-experiments so any future widening (config drift or a
# gitleaks upgrade changing allowlist semantics) fails here:
#   scope-green    digest lines only inside the two exempt paths (plus the
#                  drill's exact "wrong-token" negative-test line) -> pass
#   scope-outside  the same digest shape in a foreign file -> gitleaks fires
#   scope-inside   a foreign secret shape inside an exempt file (and a
#                  non-literal bearer token in the drill) -> gitleaks fires
# Secret-shaped values are generated at runtime so this test source never
# carries a literal secret pattern.
SCOPE_DIGEST="$(python3 -c 'import hashlib; print(hashlib.sha256(b"scope-digest").hexdigest())')"
SCOPE_SK="$(python3 -c 'import hashlib; print("sk-" + hashlib.sha256(b"scope-sk").hexdigest()[:32])')"
SCOPE_BEARER="$(python3 -c 'import hashlib; print("stolen-" + hashlib.sha256(b"scope-bearer").hexdigest()[:16])')"

make_scope_root() {
  local root="$1"
  mkdir -p "$root/security" \
    "$root/backend/internal/service/rag_eval/testdata" \
    "$root/docs/working" \
    "$root/scripts/ops" \
    "$root/planted"
  cp "$SECURITY_DIR"/*.json "$root/security/"
  cp "$REPO_ROOT/.gitleaks.toml" "$root/.gitleaks.toml"
  # digest lines in both exempt paths (same shape as the real artifacts)
  printf '{"chunk_key": "%s"}\n' "$SCOPE_DIGEST" \
    > "$root/backend/internal/service/rag_eval/testdata/ablation-judge-replay.jsonl"
  printf '{"case_key": "%s"}\n' "$SCOPE_DIGEST" \
    > "$root/docs/working/2026-08-29-agent-answer-eval-real.json"
  # drill negative-test line: exempt only for the exact "wrong-token" literal.
  # The bearer value is assembled at runtime ("wrong-" + "token") so this
  # test source never carries the literal header value the curl-auth-header
  # rule would flag (the gate scans this very file).
  printf 'BADTOKEN_CODE="$(curl -s -o /dev/null -w "%%{http_code}" -H "Authorization: Bearer %s" "$GATE_URL/x")"\n' \
    "wrong-token" \
    > "$root/scripts/ops/observability-drill.sh"
}

SCOPE_GREEN_ROOT="$TEMP_ROOT/scope-green"
make_scope_root "$SCOPE_GREEN_ROOT"
expect_exit 0 "gitleaks scope green - digest exemptions still cover the eval paths" "$SCOPE_GREEN_ROOT" "secrets"

SCOPE_OUTSIDE_ROOT="$TEMP_ROOT/scope-outside"
make_scope_root "$SCOPE_OUTSIDE_ROOT"
printf '{"chunk_key": "%s"}\n' "$SCOPE_DIGEST" > "$SCOPE_OUTSIDE_ROOT/planted/other.json"
expect_exit 1 "gitleaks scope red - same digest shape in a foreign file is detected" "$SCOPE_OUTSIDE_ROOT" "secrets"

SCOPE_INSIDE_ROOT="$TEMP_ROOT/scope-inside"
make_scope_root "$SCOPE_INSIDE_ROOT"
printf '{"api_key": "%s"}\n' "$SCOPE_SK" \
  >> "$SCOPE_INSIDE_ROOT/backend/internal/service/rag_eval/testdata/ablation-judge-replay.jsonl"
printf 'BADTOKEN_CODE="$(curl -s -o /dev/null -w "%%{http_code}" -H "Authorization: Bearer %s" "$GATE_URL/x")"\n' \
  "$SCOPE_BEARER" >> "$SCOPE_INSIDE_ROOT/scripts/ops/observability-drill.sh"
expect_exit 1 "gitleaks scope red - non-digest secrets inside exempt files stay detected" "$SCOPE_INSIDE_ROOT" "secrets"

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

# ------------------------------------------------- npm audit scope fixtures
# scan-policy npm_audit_scope=production audits only the runtime dependency
# tree (--omit=dev): a dev-only vulnerable lockfile must pass the gate (the
# dev-tree risk is characterized in npm_dev_risk_notes, not gated), while the
# same lockfile under scope=all must fail. Proves the omit flag is actually
# applied rather than ignored.
cat > "$NPM_ROOT/frontend/package.json" <<'EOF'
{
  "name": "fixture-dev-only",
  "version": "1.0.0",
  "devDependencies": {
    "lodash": "4.17.15"
  }
}
EOF
(
  cd "$NPM_ROOT/frontend" \
    && npm install --package-lock-only --registry=https://registry.npmjs.org --ignore-scripts >/dev/null 2>&1
)
expect_exit 0 "dev-only vulnerable lockfile passes npm gate under production scope" "$NPM_ROOT" "npm"
python3 - "$NPM_ROOT/security/scan-policy.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["npm_audit_scope"] = "all"
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "dev-only vulnerable lockfile fails npm gate under all scope" "$NPM_ROOT" "npm"

# Invalid scope value must be rejected by the policy gate (drift protection:
# the policy key the npm gate reads cannot silently rot).
SCOPE_ROOT="$TEMP_ROOT/scope"
mkdir -p "$SCOPE_ROOT/security"
cp "$SECURITY_DIR"/*.json "$SCOPE_ROOT/security/"
python3 - "$SCOPE_ROOT/security/scan-policy.json" <<'PY'
import json, sys
path = sys.argv[1]
d = json.load(open(path, encoding="utf-8"))
d["npm_audit_scope"] = "everything"
json.dump(d, open(path, "w"), indent=2)
PY
expect_exit 1 "policy with invalid npm_audit_scope rejected" "$SCOPE_ROOT" "policy"
# ------------------------------------------------- verdict parsing fixtures
# govulncheck -format json is a multi-document stream (config/progress/osv/
# finding documents); the historical single-object json.load raised "Extra
# data", the verdict swallowed it and always reported zero go findings
# (audit F-05 finding 1). A stream containing a finding must be judged.
VERDICT_ROOT="$TEMP_ROOT/verdict"
mkdir -p "$VERDICT_ROOT/security"
cp "$SECURITY_DIR"/*.json "$VERDICT_ROOT/security/"
GVC_REPORT="$TEMP_ROOT/report-govulncheck-stream"
mkdir -p "$GVC_REPORT"
python3 - "$GVC_REPORT/govulncheck.json" <<'PY'
import json, sys
docs = [
    {"config": {"modules": ["omnicraft/backend"]}},
    {"progress": {"message": "Scanning your code and 50 packages..."}},
    {"osv": {"id": "GO-9999-0001", "summary": "fixture osv"}},
    {"osv": {"id": "GO-9999-0002", "summary": "fixture osv (module-level only)"}},
    {"finding": {"osv": "GO-9999-0001", "fixed_version": "v0.3.8",
                 "trace": [{"module": "golang.org/x/text", "version": "v0.3.0"}]}},
]
with open(sys.argv[1], "w", encoding="utf-8") as f:
    for d in docs:
        f.write(json.dumps(d) + "\n")
PY
expect_verdict 1 "verdict reports findings from streaming govulncheck json" \
  "$VERDICT_ROOT" "go" "$GVC_REPORT"
python3 - "$GVC_REPORT/security-verdict.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
go = [f for f in d["findings"] if f["source"] == "govulncheck"]
assert any(f["id"] == "GO-9999-0001" and f["component"] == "golang.org/x/text"
           for f in go), go
assert not any(f["id"] == "scanner-failure:govulncheck" for f in d["findings"])
print("verdict stream finding asserted")
PY

# A missing trivy-fs report must FAIL the verdict (scanner failure is not
# zero findings; audit F-05 finding 2), and a valid empty report must pass.
TRIVY_MISSING="$TEMP_ROOT/report-trivy-missing"
mkdir -p "$TRIVY_MISSING"
expect_verdict 1 "verdict fails when trivy-fs report is missing" \
  "$VERDICT_ROOT" "trivy-fs" "$TRIVY_MISSING"
python3 - "$TRIVY_MISSING/security-verdict.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
assert any(f["id"] == "scanner-failure:trivy-fs" and f["severity"] == "critical"
           for f in d["blocked_findings"]), d["blocked_findings"]
print("trivy scanner-failure asserted")
PY
TRIVY_CLEAN="$TEMP_ROOT/report-trivy-clean"
mkdir -p "$TRIVY_CLEAN"
printf '{"SchemaVersion": 2, "Results": []}' > "$TRIVY_CLEAN/trivy-fs.json"
expect_verdict 0 "verdict passes on a valid empty trivy-fs report" \
  "$VERDICT_ROOT" "trivy-fs" "$TRIVY_CLEAN"

# A missing govulncheck report must equally fail the verdict (same scanner-
# failure class, covering the go gate's historical zero-findings swallow).
GO_MISSING="$TEMP_ROOT/report-govulncheck-missing"
mkdir -p "$GO_MISSING"
expect_verdict 1 "verdict fails when govulncheck report is missing" \
  "$VERDICT_ROOT" "go" "$GO_MISSING"

# ------------------------------------------- workflow trigger drift fixture
# scan-policy scan_triggers must match security.yml's actual `on:` block.
# Removing the pull_request trigger from a copy of the real workflow while
# the policy still declares it must fail the policy gate.
DRIFT_ROOT="$TEMP_ROOT/drift"
mkdir -p "$DRIFT_ROOT/security" "$DRIFT_ROOT/.github/workflows"
cp "$SECURITY_DIR"/*.json "$DRIFT_ROOT/security/"
sed 's/^  pull_request:$//' "$REPO_ROOT/.github/workflows/security.yml" \
  > "$DRIFT_ROOT/.github/workflows/security.yml"
expect_exit 1 "policy-rejects-workflow-trigger-drift" "$DRIFT_ROOT" "policy"
grep -q "scan_triggers.pull_request" "$TEMP_ROOT/policy-rejects-workflow-trigger-drift.err" || {
  echo "FAIL: trigger drift must be cited in the policy errors" >&2
  exit 1
}

echo "OK: verify-security contract tests passed"
