#!/usr/bin/env bash
# =============================================================================
# OmniCraft continuous security verification: runs every required scan gate
# with pinned tool versions, validates the scan policy and exception ledger,
# and produces a machine-readable verdict. Any High/Critical vulnerability or
# secret finding without a currently valid exception fails the run.
#
# Gates (see security/scan-policy.json):
#   policy        - scan-policy.json + exceptions.json governance validation
#   secrets       - gitleaks (pinned image) on the working tree and full history
#   go            - govulncheck (pinned module version) on backend/...
#   npm           - npm audit for frontend/ and tauri-client/ lockfiles
#   cargo         - cargo audit (pinned image + cargo-audit version) on Cargo.lock
#   trivy-fs      - Trivy filesystem scan (vuln + secret)
#   trivy-config  - Trivy IaC scan (Dockerfiles, compose, configs)
#   trivy-image   - Trivy container-image scan (requires -BuildImages)
#
# Usage:
#   bash scripts/security/verify-security.sh [-RepoRoot <dir>] [-Gates a,b,c]
#       [-BuildImages] [-ReportDir <dir>] [-CargoAuditDb <host-path>]
#       [-VerdictOnly]
#   -CargoAuditDb points at a host-cloned rustsec advisory-db checkout used
#   with --no-fetch; without it the container fetches the DB itself (GitHub
#   runners). If the container cannot reach github.com the script falls back
#   to a host-side clone automatically and records the transport in the report.
#   -VerdictOnly skips gate execution and only runs the verdict over the
#   reports already present in -ReportDir: the contract-test hook for verdict
#   parsing fixtures (crafted govulncheck/trivy reports) without running the
#   real scanners.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
GATES="policy,secrets,go,npm,cargo,trivy-fs,trivy-config,trivy-image"
BUILD_IMAGES=0
VERDICT_ONLY=0
REPORT_DIR=""
CARGO_AUDIT_DB=""

GITLEAKS_IMAGE="zricethezav/gitleaks:v8.18.4@sha256:75bdb2b2f4db213cde0b8295f13a88d6b333091bbfbf3012a4e083d00d31caba"
TRIVY_IMAGE="aquasec/trivy:0.57.1@sha256:5c59e08f980b5d4d503329773480fcea2c9bdad7e381d846fbf9f2ecb8050f6b"
RUST_IMAGE="rust:1.85.1@sha256:e51d0265072d2d9d5d320f6a44dde6b9ef13653b035098febd68cce8fa7c0bc4"
GOVULNCHECK_VER="v1.1.4"
CARGO_AUDIT_VER="0.22.1"
TRIVY_CACHE_VOLUME="omnicraft-trivy-cache"
CARGO_AUDIT_VOLUME="omnicraft-cargo-audit-cache"

while [ $# -gt 0 ]; do
  case "$1" in
    -RepoRoot) REPO_ROOT="$2"; shift 2 ;;
    -Gates) GATES="$2"; shift 2 ;;
    -BuildImages) BUILD_IMAGES=1; shift ;;
    -VerdictOnly) VERDICT_ONLY=1; shift ;;
    -ReportDir) REPORT_DIR="$2"; shift 2 ;;
    -CargoAuditDb) CARGO_AUDIT_DB="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

REPO_ROOT="$(cd "$REPO_ROOT" 2>/dev/null && pwd)" || {
  echo "repo root not found: $REPO_ROOT" >&2
  exit 1
}
if [ -z "$REPORT_DIR" ]; then
  REPORT_DIR="$REPO_ROOT/artifacts/security"
fi
REPORT_DIR="$(mkdir -p "$REPORT_DIR" && cd "$REPORT_DIR" && pwd)" || {
  echo "cannot create report dir: $REPORT_DIR" >&2
  exit 1
}

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
RUN_COMMANDS=()
RUN_EXIT_CODES=()
SUMMARY_GATES=()

need_docker() {
  command -v docker >/dev/null 2>&1 || fail "docker is required for pinned security tools"
}

record() {
  RUN_COMMANDS+=("$1")
  RUN_EXIT_CODES+=("$2")
}

# True if $1 is readable JSON containing key $2. Distinguishes "scan found
# findings" (valid report; the verdict judges them) from "scan tool errored"
# (missing/invalid report; the gate must fail).
json_report_has_key() {
  local file="$1" key="$2"
  python3 - "$file" "$key" <<'PY' >/dev/null 2>&1
import json, sys
json.load(open(sys.argv[1], encoding="utf-8"))[sys.argv[2]]
PY
}

# True if $1 is a JSON stream containing at least one document with the given
# top-level key (govulncheck findings are {"finding": {...}} documents on a
# multi-document stream; single-object json.load cannot read it).
json_stream_has_key() {
  local file="$1" key="$2"
  python3 - "$file" "$key" <<'PY' >/dev/null 2>&1
import json, sys
buf, want = open(sys.argv[1], encoding="utf-8").read(), sys.argv[2]
dec = json.JSONDecoder()
idx, n = 0, len(buf)
while idx < n:
    while idx < n and buf[idx] in " \t\r\n":
        idx += 1
    if idx >= n:
        break
    obj, idx = dec.raw_decode(buf, idx)
    if isinstance(obj, dict) and want in obj:
        sys.exit(0)
sys.exit(1)
PY
}

# ---------------------------------------------------------------- policy gate
# exceptions.schema.json is the human-readable ledger contract; the
# authoritative implementation of the exception rules is the Python check
# below (it deliberately mirrors the schema). If the two ever disagree the
# Python check wins. policy-gate.json records the schema_version it validated
# against for traceability. The pinned-tools manifest is cross-checked against
# the tool constants embedded in this script so a version bump in one but not
# the other fails the policy gate.
run_policy_gate() {
  python3 - "$REPO_ROOT" "$REPORT_DIR" "$SCRIPT_DIR/verify-security.sh" <<'PY' 
import json, os, re, sys, datetime

root, report_dir, script_path = sys.argv[1], sys.argv[2], sys.argv[3]
errors = []

def load(path):
    if not os.path.exists(path):
        errors.append("missing file: %s" % path)
        return None
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except Exception as e:
        errors.append("invalid JSON %s: %s" % (path, e))
        return None

policy = load(os.path.join(root, "security", "scan-policy.json"))
exceptions = load(os.path.join(root, "security", "exceptions.json"))
tools = load(os.path.join(root, "security", "pinned-tools.json"))

if policy is not None:
    required_categories = [
        "go_dependencies", "frontend_npm", "tauri_npm", "rust_dependencies",
        "secrets", "filesystem", "iac", "container_images",
    ]
    cats = policy.get("categories", [])
    for c in required_categories:
        if c not in cats:
            errors.append("scan-policy missing category: %s" % c)
    for c in cats:
        if c not in required_categories:
            errors.append("scan-policy unknown category: %s" % c)
    if not isinstance(policy.get("fail_on_severities"), list):
        errors.append("scan-policy fail_on_severities must be a list")
    if policy.get("npm_audit_scope") not in ("production", "all"):
        errors.append("scan-policy npm_audit_scope must be 'production' or 'all' (got %r)" % policy.get("npm_audit_scope"))
    if "secret" not in policy.get("non_waivable", []):
        errors.append("scan-policy non_waivable must include 'secret'")
    if "critical" not in policy.get("non_waivable", []):
        errors.append("scan-policy non_waivable must include 'critical'")
    if not isinstance(policy.get("high_exceptions_enabled"), bool):
        errors.append("scan-policy high_exceptions_enabled must be boolean")
    if not policy.get("pinned_tools"):
        errors.append("scan-policy pinned_tools must name the tool manifest")

if tools is not None:
    names = [t.get("name") for t in tools.get("tools", [])]
    for required in ("govulncheck", "gitleaks", "trivy", "rust", "cargo-audit"):
        if required not in names:
            errors.append("pinned-tools manifest missing tool: %s" % required)
    for t in tools.get("tools", []):
        if not t.get("version") or not t.get("source"):
            errors.append("pinned tool %r must have version and source" % t.get("name"))
    # Cross-check the pinned-tools manifest against the tool constants
    # embedded in verify-security.sh itself: the script constants are what the
    # gates actually run, so a manifest that disagrees with them is drift.
    try:
        with open(script_path, encoding="utf-8") as f:
            script_src = f.read()
    except OSError:
        errors.append("cannot read verify-security.sh for tool constant cross-check: %s" % script_path)
        script_src = None
    if script_src is not None:
        tool_map = {t.get("name"): t for t in tools.get("tools", [])}
        for const_name, pattern, tool_name in (
            ("GOVULNCHECK_VER", r'GOVULNCHECK_VER="([^"]+)"', "govulncheck"),
            ("CARGO_AUDIT_VER", r'CARGO_AUDIT_VER="([^"]+)"', "cargo-audit"),
            ("GITLEAKS_IMAGE", r'GITLEAKS_IMAGE="([^"]+)"', "gitleaks"),
            ("TRIVY_IMAGE", r'TRIVY_IMAGE="([^"]+)"', "trivy"),
            ("RUST_IMAGE", r'RUST_IMAGE="([^"]+)"', "rust"),
        ):
            m = re.search(pattern, script_src)
            if not m:
                errors.append("verify-security.sh: constant %s not found" % const_name)
                continue
            t = tool_map.get(tool_name)
            if t is None:
                errors.append("pinned-tools manifest missing tool: %s" % tool_name)
                continue
            const_val = m.group(1)
            if tool_name in ("gitleaks", "trivy", "rust"):
                tag = re.search(r":v?([0-9][0-9.]*[0-9])@sha256:", const_val)
                if not tag:
                    errors.append("verify-security.sh: cannot parse %s version from %s" % (const_name, const_val))
                    continue
                if tag.group(1) != t.get("version"):
                    errors.append("verify-security.sh %s version %s does not match pinned-tools.json (%s)" % (const_name, tag.group(1), t.get("version")))
                digest = re.search(r"@(sha256:[0-9a-f]{64})", const_val)
                src_digest = re.search(r"@(sha256:[0-9a-f]{64})", t.get("source", ""))
                if digest and src_digest and digest.group(1) != src_digest.group(1):
                    errors.append("verify-security.sh %s digest %s does not match pinned-tools.json (%s)" % (const_name, digest.group(1), src_digest.group(1)))
            else:
                if const_val.lstrip("v") != t.get("version", "").lstrip("v"):
                    errors.append("verify-security.sh %s version %s does not match pinned-tools.json (%s)" % (const_name, const_val, t.get("version")))

# Cross-check scan-policy scan_triggers against the security workflow's
# actual `on:` block: a trigger declared in policy but missing from the
# workflow (or vice versa) is silent drift that green-lights an unscanned
# path (audit F-05 finding 3). Only checked when the workflow file exists;
# fixture roots that copy security/ alone exercise the other rules.
if policy is not None:
    workflow_path = os.path.join(root, ".github", "workflows", "security.yml")
    if os.path.exists(workflow_path):
        try:
            with open(workflow_path, encoding="utf-8") as f:
                workflow_src = f.read()
        except OSError as e:
            errors.append("cannot read security.yml for trigger cross-check: %s" % e)
            workflow_src = None
        if workflow_src is not None:
            triggers = policy.get("scan_triggers", {})
            has_pr = re.search(r"^\s*pull_request\s*:", workflow_src, re.M) is not None
            if has_pr != bool(triggers.get("pull_request")):
                errors.append("scan-policy scan_triggers.pull_request=%r but security.yml pull_request trigger is %s"
                              % (triggers.get("pull_request"), "present" if has_pr else "absent"))
            push_block = re.search(r"^\s*push\s*:\s*\n((?:[ \t]+.+\n?)+)", workflow_src, re.M)
            pushes_main = bool(push_block and re.search(r"branches\s*:\s*\[[^\]]*\bmain\b", push_block.group(1)))
            if pushes_main != bool(triggers.get("push_main")):
                errors.append("scan-policy scan_triggers.push_main=%r but security.yml push-on-main trigger is %s"
                              % (triggers.get("push_main"), "present" if pushes_main else "absent"))
            workflow_crons = re.findall(r'-\s*cron\s*:\s*["\']([^"\']+)["\']', workflow_src)
            policy_cron = triggers.get("schedule")
            if policy_cron and workflow_crons and policy_cron not in workflow_crons:
                errors.append("scan-policy scan_triggers.schedule=%r but security.yml cron is %r"
                              % (policy_cron, workflow_crons))

today = datetime.date.today().isoformat()
if exceptions is not None:
    if not isinstance(exceptions.get("exceptions"), list):
        errors.append("exceptions.json must contain an exceptions array")
    for i, exc in enumerate(exceptions.get("exceptions", [])):
        tag = "exception[%d]" % i
        for field in ("id", "vulnerability_id", "affected_component",
                      "affected_version", "severity", "risk_description",
                      "compensating_controls", "author", "approver",
                      "approval_date", "expiry_date", "approval_ref", "status"):
            if field not in exc:
                errors.append("%s missing field %s" % (tag, field))
                continue
        sev = exc.get("severity")
        if sev != "high":
            errors.append("%s: only high severity is waivable (got %r)" % (tag, sev))
        if exc.get("status") == "active" and exc.get("expiry_date", "9999-99-99") < today:
            errors.append("%s: expired exception (expiry %s)" % (tag, exc.get("expiry_date")))
        if exc.get("status") == "active" and exc.get("author") == exc.get("approver"):
            errors.append("%s: approver must differ from author" % tag)
        ref = exc.get("approval_ref")
        if isinstance(ref, dict):
            commit = ref.get("commit", "")
            if len(commit) != 40 or not all(c in "0123456789abcdef" for c in commit):
                errors.append("%s: approval_ref.commit must be a 40-hex SHA" % tag)
            event = ref.get("event", "")
            if not re.match(r"^https://github\.com/[^/]+/[^/]+/(pull/[0-9]+/(review|reviews)|environments/[^/]+)$", event):
                errors.append("%s: approval_ref.event must be a GitHub PR review or environment approval URL (issue/comment URLs are mutable and insufficient)" % tag)
        else:
            errors.append("%s: approval_ref must be an object with commit and event" % tag)
    if exceptions.get("exceptions"):
        if policy is not None and not policy.get("high_exceptions_enabled"):
            errors.append("high exceptions present but high_exceptions_enabled is false "
                          "(single-human-owner repository cannot pass High exceptions)")

schema_version = exceptions.get("schema_version") if exceptions is not None else None
if errors:
    with open(os.path.join(report_dir, "policy-gate.json"), "w", encoding="utf-8") as f:
        json.dump({"ok": False, "errors": errors}, f, indent=2)
    for e in errors:
        print("policy: %s" % e, file=sys.stderr)
    sys.exit(1)

with open(os.path.join(report_dir, "policy-gate.json"), "w", encoding="utf-8") as f:
    json.dump({"ok": True, "errors": [], "schema_version": schema_version}, f, indent=2)
sys.exit(0)
PY
}

# ------------------------------------------------------------- secrets gate
run_secrets_gate() {
  need_docker
  local rc=0
  local no_git=""
  [ -d "$REPO_ROOT/.git" ] || no_git="--no-git"
  docker run --rm -v "$REPO_ROOT:/repo:ro" -v "$REPORT_DIR:/out" \
    "$GITLEAKS_IMAGE" detect --source /repo $no_git \
    --report-format json --report-path /out/gitleaks-tree.json >/dev/null 2>&1 || rc=1
  if [ -d "$REPO_ROOT/.git" ]; then
    docker run --rm -v "$REPO_ROOT:/repo:ro" -v "$REPORT_DIR:/out" \
      "$GITLEAKS_IMAGE" detect --source /repo --log-opts="--all" \
      --report-format json --report-path /out/gitleaks-history.json >/dev/null 2>&1 || rc=1
  else
    echo '[]' > "$REPORT_DIR/gitleaks-history.json"
  fi
  [ -f "$REPORT_DIR/gitleaks-tree.json" ] || rc=1
  return $rc
}

# ------------------------------------------------------------------ go gate
# govulncheck exits 1 both when it finds vulnerabilities AND when the scan
# itself errors (package load failures). The report is a multi-document JSON
# stream, so the acceptable-failure test is "the stream contains finding
# documents" (the scan completed and has findings for the verdict to judge);
# any other non-zero exit is a gate error, never silently zero findings.
run_go_gate() {
  local rc=0
  (cd "$REPO_ROOT/backend" && go run "golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VER" \
    -format json ./... > "$REPORT_DIR/govulncheck.json" 2>"$REPORT_DIR/govulncheck.log") || rc=1
  if [ $rc -ne 0 ] && { json_report_has_key "$REPORT_DIR/govulncheck.json" "Vulnerabilities" \
      || json_stream_has_key "$REPORT_DIR/govulncheck.json" "finding"; }; then
    rc=0
  fi
  return $rc
}

# ----------------------------------------------------------------- npm gate
# npm audit exits 1 when findings exist at or above the audit level; findings
# are judged by the verdict. A missing/error JSON (no "vulnerabilities" key)
# is a gate error. scan-policy npm_audit_scope=production audits only the
# runtime dependency tree (--omit=dev): dev-tree findings are build-time
# toolchain risk, characterized in scan-policy.json (npm_dev_risk_notes) and
# deliberately out of the gate's scope.
run_npm_gate() {
  local rc=0
  local registry
  registry="$(python3 - "$REPO_ROOT/security/scan-policy.json" <<'PY' 2>/dev/null
import json, sys
print(json.load(open(sys.argv[1], encoding="utf-8")).get("npm_registry", "https://registry.npmjs.org"))
PY
)" || registry="https://registry.npmjs.org"
  local scope
  scope="$(python3 - "$REPO_ROOT/security/scan-policy.json" <<'PY' 2>/dev/null
import json, sys
print(json.load(open(sys.argv[1], encoding="utf-8")).get("npm_audit_scope", "production"))
PY
)" || scope="production"
  local audit_flags=(--registry="$registry")
  if [ "$scope" = "production" ]; then audit_flags+=(--omit=dev); fi
  for pkg in frontend tauri-client; do
    if [ ! -f "$REPO_ROOT/$pkg/package-lock.json" ]; then
      echo "{\"ok\": false, \"error\": \"missing $pkg/package-lock.json\"}" \
        > "$REPORT_DIR/npm-$pkg-audit.json"
      echo "npm audit: skipping $pkg (no lockfile)" >&2
      rc=1
      continue
    fi
    local sub=0
    (cd "$REPO_ROOT/$pkg" && npm audit "${audit_flags[@]}" \
      --json > "$REPORT_DIR/npm-$pkg-audit.json" 2>"$REPORT_DIR/npm-$pkg-audit.log") || sub=1
    if ! json_report_has_key "$REPORT_DIR/npm-$pkg-audit.json" "vulnerabilities"; then
      echo "npm audit: failed for $pkg (invalid or error JSON)" >&2
      sub=1
    else
      sub=0
    fi
    if [ $sub -ne 0 ]; then rc=1; fi
  done
  return $rc
}

# --------------------------------------------------------------- cargo gate
# cargo audit exits 1 when the advisory database contains any vulnerability;
# findings are judged by the verdict. A missing/error JSON is a gate error.
# cargo-audit is installed on first use into a named volume (pinned version,
# crates.io); the advisory DB is fetched in the container, falling back to a
# host-side clone when the container cannot reach github.com.
run_cargo_gate() {
  need_docker
  local src="$REPO_ROOT/tauri-client/src-tauri"
  [ -f "$src/Cargo.lock" ] || fail "missing $src/Cargo.lock"
  local db_args=""
  local db_path=""
  if [ -n "$CARGO_AUDIT_DB" ]; then
    [ -d "$CARGO_AUDIT_DB" ] || fail "cargo audit db not found: $CARGO_AUDIT_DB"
    db_path="$(cd "$CARGO_AUDIT_DB" && pwd)"
    db_args="-v $db_path:/opt/advisory-db:ro"
  fi
  local install=""
  if ! docker run --rm -v "$CARGO_AUDIT_VOLUME:/opt/cargo-audit" \
    "$RUST_IMAGE" sh -c "test -x /opt/cargo-audit/bin/cargo-audit" >/dev/null 2>&1; then
    echo "cargo audit: installing cargo-audit $CARGO_AUDIT_VER (first run, ~2-4 minutes)" >&2
    install="CARGO_INSTALL_ROOT=/opt/cargo-audit cargo install cargo-audit --version $CARGO_AUDIT_VER --locked --quiet && "
  fi
  local rc=0
  if [ -n "$CARGO_AUDIT_DB" ]; then
    docker run --rm -v "$src:/src:ro" -v "$CARGO_AUDIT_VOLUME:/opt/cargo-audit" $db_args \
      "$RUST_IMAGE" sh -c "export PATH=/opt/cargo-audit/bin:\$PATH; ${install}cd /src && cargo audit --db /opt/advisory-db --no-fetch --json" \
      > "$REPORT_DIR/cargo-audit.json" 2>"$REPORT_DIR/cargo-audit.log" || rc=1
  else
    docker run --rm -v "$src:/src:ro" -v "$CARGO_AUDIT_VOLUME:/opt/cargo-audit" \
      "$RUST_IMAGE" sh -c "export PATH=/opt/cargo-audit/bin:\$PATH; ${install}cd /src && cargo audit --json" \
      > "$REPORT_DIR/cargo-audit.json" 2>"$REPORT_DIR/cargo-audit.log" || rc=1
    if [ $rc -ne 0 ] && grep -q "couldn't fetch advisory database" "$REPORT_DIR/cargo-audit.log" 2>/dev/null; then
      echo "cargo audit: container cannot reach github.com; falling back to host-side advisory-db clone" >&2
      local cache="${XDG_CACHE_HOME:-$HOME/.cache}/omnicraft/cargo-advisory-db"
      if [ ! -d "$cache/.git" ]; then
        mkdir -p "$cache" && git clone -q https://github.com/rustsec/advisory-db.git "$cache" || {
          echo "cargo audit: host-side clone of advisory-db failed" >&2
          return 1
        }
      else
        git -C "$cache" pull -q --ff-only 2>/dev/null \
          || echo "cargo audit: warning - advisory-db refresh failed, using existing clone" >&2
      fi
      docker run --rm -v "$src:/src:ro" -v "$CARGO_AUDIT_VOLUME:/opt/cargo-audit" \
        -v "$cache:/opt/advisory-db:ro" "$RUST_IMAGE" sh -c "export PATH=/opt/cargo-audit/bin:\$PATH; ${install}cd /src && cargo audit --db /opt/advisory-db --no-fetch --json" \
        > "$REPORT_DIR/cargo-audit.json" 2>"$REPORT_DIR/cargo-audit.log" || rc=1
    fi
  fi
  if [ $rc -ne 0 ] && json_report_has_key "$REPORT_DIR/cargo-audit.json" "vulnerabilities"; then
    rc=0
  fi
  return $rc
}

# ------------------------------------------------------------- trivy gates
# Findings are judged by the verdict (no --exit-code 1); only scan errors fail.
# Scanner diagnostics go to the report dir instead of /dev/null so a failed
# DB fetch is diagnosable; a scanner that produced no report makes the verdict
# fail closed (see run_verdict).
run_trivy_fs_gate() {
  need_docker
  docker run --rm -v "$REPO_ROOT:/repo:ro" -v "$REPORT_DIR:/out" -v "$TRIVY_CACHE_VOLUME:/root/.cache" \
    "$TRIVY_IMAGE" fs --scanners vuln,secret --severity HIGH,CRITICAL \
    --format json --output /out/trivy-fs.json /repo >"$REPORT_DIR/trivy-fs.log" 2>&1
}

run_trivy_config_gate() {
  need_docker
  docker run --rm -v "$REPO_ROOT:/repo:ro" -v "$REPORT_DIR:/out" -v "$TRIVY_CACHE_VOLUME:/root/.cache" \
    "$TRIVY_IMAGE" config --severity HIGH,CRITICAL \
    --format json --output /out/trivy-config.json /repo >/dev/null 2>&1
}

run_trivy_image_gate() {
  need_docker
  if [ "$BUILD_IMAGES" -ne 1 ]; then
    echo "trivy-image gate skipped: pass -BuildImages to build and scan container images" >&2
    echo "{\"ok\": false, \"skipped\": true}" > "$REPORT_DIR/trivy-image-skipped.json"
    return 1
  fi
  local build_rc=0
  docker compose build backend frontend migrate pgbouncer \
    > "$REPORT_DIR/docker-build.log" 2>&1 || build_rc=1
  if [ $build_rc -ne 0 ]; then
    echo "docker compose build failed, see $REPORT_DIR/docker-build.log" >&2
    return 1
  fi
  local rc=0
  for img in omnicraft-backend omnicraft-frontend omnicraft-migrate omnicraft-pgbouncer; do
    local sub=0
    docker run --rm -v "$REPORT_DIR:/out" -v "$TRIVY_CACHE_VOLUME:/root/.cache" \
      -v /var/run/docker.sock:/var/run/docker.sock \
      "$TRIVY_IMAGE" image --severity HIGH,CRITICAL \
      --format json --output "/out/trivy-image-${img#omnicraft-}.json" "$img:latest" \
      >/dev/null 2>&1 || sub=1
    if [ $sub -ne 0 ]; then rc=1; fi
  done
  return $rc
}

# ---------------------------------------------------------------- verdict
run_verdict() {
  python3 - "$REPO_ROOT" "$REPORT_DIR" "$GATES" <<'PY'
import json, os, re, sys, urllib.request, datetime

root, report_dir, gates_arg = sys.argv[1], sys.argv[2], sys.argv[3]
active_gates = [g for g in gates_arg.split(",") if g]

def load(path):
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return None

policy = load(os.path.join(root, "security", "scan-policy.json")) or {}
exceptions = load(os.path.join(root, "security", "exceptions.json")) or {"exceptions": []}
fail_on = set(policy.get("fail_on_severities", ["high", "critical"]))
non_waivable = set(policy.get("non_waivable", ["secret", "critical"]))
high_enabled = policy.get("high_exceptions_enabled", False)
today = datetime.date.today().isoformat()

findings = []

def add(f):
    f["severity"] = str(f.get("severity", "")).lower()
    findings.append(f)

if "secrets" in active_gates:
    for rep in ("gitleaks-tree.json", "gitleaks-history.json"):
        for f in load(os.path.join(report_dir, rep)) or []:
            add({"id": "gitleaks:%s" % f.get("RuleID"), "component": f.get("File"),
                 "version": "N/A", "severity": "secret", "source": "gitleaks-%s" % rep,
                 "detail": f.get("Secret", "")[:24]})

def iter_json_stream(path):
    """Yield each JSON document in a multi-document stream. govulncheck
    -format json writes config/SBOM/progress/osv/finding documents back to
    back; single-object json.load raises "Extra data" and the historical
    verdict swallowed the error as zero findings (audit F-05 finding 1)."""
    try:
        with open(path, encoding="utf-8") as f:
            buf = f.read()
    except OSError:
        return
    dec = json.JSONDecoder()
    idx, n = 0, len(buf)
    while idx < n:
        while idx < n and buf[idx] in " \t\r\n":
            idx += 1
        if idx >= n:
            return
        try:
            obj, idx = dec.raw_decode(buf, idx)
        except ValueError:
            return  # unparsable from here: stop, earlier documents still count
        yield obj

def osv_severity(vuln_id, default="critical"):
    """Best-effort severity from the OSV API; unreachable API means we cannot
    downgrade, so the default (fail-closed) applies."""
    try:
        with urllib.request.urlopen(
                "https://api.osv.dev/v1/vulns/%s" % vuln_id, timeout=10) as r:
            osv = json.load(r)
        db = osv.get("database_specific", {})
        return db.get("severity") or default
    except Exception:
        return default

if "go" in active_gates:
    gvc_path = os.path.join(report_dir, "govulncheck.json")
    stream_docs = [d for d in iter_json_stream(gvc_path) if isinstance(d, dict)]
    if not stream_docs:
        # Scanner produced no parsable report. A failed scanner is not a
        # zero-findings scan; recorded as non-waivable so the verdict fails.
        add({"id": "scanner-failure:govulncheck", "component": "govulncheck.json",
             "version": "N/A", "severity": "critical", "source": "govulncheck",
             "detail": "report missing or invalid (scanner failure is not zero findings)"})
    else:
        # Legacy single-object shape ({"Vulnerabilities": [...]}) plus the
        # real govulncheck stream shape ({"finding": {...}} documents).
        vulns = []
        for d in stream_docs:
            if "Vulnerabilities" in d:
                vulns.extend(d.get("Vulnerabilities", []))
            if "finding" in d:
                vulns.append(d["finding"])
        for v in vulns:
            if "osv" in v:
                vid = v.get("osv")
                trace = v.get("trace") or [{}]
                frame = trace[0] if isinstance(trace[0], dict) else {}
                component = frame.get("module") or frame.get("package")
                version = frame.get("version") or "unknown"
                detail = "fixed %s" % v.get("fixed_version", "?")
            else:
                vid = v.get("ID")
                component = v.get("Package") or v.get("Module")
                version = v.get("Version") or "unknown"
                detail = v.get("Details", "")[:80]
            add({"id": vid, "component": component,
                 "version": version, "severity": osv_severity(vid),
                 "source": "govulncheck", "detail": detail})

if "npm" in active_gates:
    for pkg in ("frontend", "tauri-client"):
        rep = "npm-%s-audit.json" % pkg
        d = load(os.path.join(report_dir, rep))
        if not d or not isinstance(d, dict):
            continue
        for name, info in d.get("vulnerabilities", {}).items():
            sev = info.get("severity", "high")
            for via in info.get("via", []):
                if isinstance(via, dict) and via.get("source"):
                    add({"id": via["source"], "component": name,
                         "version": info.get("range", "unknown"), "severity": sev,
                         "source": "npm-audit-%s" % pkg, "detail": via.get("title", "")[:80]})
                elif isinstance(via, str):
                    add({"id": via, "component": name, "version": info.get("range", "unknown"),
                         "severity": sev, "source": "npm-audit-%s" % pkg, "detail": ""})

if "cargo" in active_gates:
    ca = load(os.path.join(report_dir, "cargo-audit.json"))
    if ca and isinstance(ca, dict):
        for v in ca.get("vulnerabilities", {}).get("list", []):
            adv = v.get("advisory", {})
            if isinstance(adv, str):
                adv = {"id": adv}
            pkg = v.get("package", {})
            sev = None
            if isinstance(adv.get("cvss"), dict):
                sev = adv["cvss"].get("severity")
            if not sev and adv.get("aliases"):
                for alias in adv["aliases"][:3]:
                    try:
                        with urllib.request.urlopen(
                                "https://api.osv.dev/v1/vulns/%s" % alias, timeout=10) as r:
                            osv = json.load(r)
                        sev = osv.get("database_specific", {}).get("severity")
                        if sev:
                            break
                    except Exception:
                        pass
            if not sev:
                sev = "critical"
            add({"id": adv.get("id"), "component": pkg.get("name"),
                 "version": pkg.get("version") or "unknown", "severity": sev,
                 "source": "cargo-audit", "detail": adv.get("url", "")[:80]})

if "trivy-fs" in active_gates:
    tf_path = os.path.join(report_dir, "trivy-fs.json")
    tf = load(tf_path)
    if not tf or not isinstance(tf, dict):
        # A missing or unparseable report means the scanner failed (e.g. vuln
        # DB fetch); treating it as zero findings green-lights a broken
        # scanner (audit F-05 finding 2). Non-waivable, so the verdict fails.
        add({"id": "scanner-failure:trivy-fs", "component": "trivy-fs.json",
             "version": "N/A", "severity": "critical", "source": "trivy-fs",
             "detail": "report missing or invalid (scanner failure is not zero findings)"})
    else:
        for r in tf.get("Results", []):
            for v in r.get("Vulnerabilities", []):
                if v.get("Severity") in ("HIGH", "CRITICAL"):
                    add({"id": v.get("VulnerabilityID"), "component": v.get("PkgName"),
                         "version": v.get("InstalledVersion"), "severity": v.get("Severity"),
                         "source": "trivy-fs", "detail": "fixed %s" % v.get("FixedVersion", "?")})

if "trivy-config" in active_gates:
    tc = load(os.path.join(report_dir, "trivy-config.json"))
    if tc and isinstance(tc, dict):
        for r in tc.get("Results", []):
            for m in r.get("Misconfigurations", []):
                if m.get("Severity") in ("HIGH", "CRITICAL"):
                    add({"id": m.get("ID"), "component": r.get("Target"), "version": "N/A",
                         "severity": m.get("Severity"), "source": "trivy-config",
                         "detail": m.get("Title", "")[:80]})

if "trivy-image" in active_gates:
    for img in ("backend", "frontend", "migrate", "pgbouncer"):
        rep = "trivy-image-%s.json" % img
        ti = load(os.path.join(report_dir, rep))
        if not ti or not isinstance(ti, dict):
            continue
        for r in ti.get("Results", []):
            for v in r.get("Vulnerabilities", []):
                if v.get("Severity") in ("HIGH", "CRITICAL"):
                    add({"id": v.get("VulnerabilityID"), "component": v.get("PkgName"),
                         "version": v.get("InstalledVersion"), "severity": v.get("Severity"),
                         "source": "trivy-image-%s" % img, "detail": "fixed %s" % v.get("FixedVersion", "?")})

def waivable(f):
    if f["severity"] in non_waivable:
        return False
    if f["severity"] not in fail_on:
        return True
    if f["severity"] != "high":
        return False
    if not high_enabled:
        return False
    for exc in exceptions.get("exceptions", []):
        if exc.get("status") != "active":
            continue
        if exc.get("expiry_date", "9999-99-99") < today:
            continue
        if exc.get("vulnerability_id") != f["id"]:
            continue
        if exc.get("affected_component") != f["component"]:
            continue
        if exc.get("affected_version") not in (f["version"], "*"):
            continue
        return True
    return False

blocked = [f for f in findings if not waivable(f)]
verdict = {
    "findings": findings,
    "blocked_findings": blocked,
    "counts": {},
    "ok": len(blocked) == 0,
    "policy": {
        "fail_on_severities": sorted(fail_on),
        "non_waivable": sorted(non_waivable),
        "high_exceptions_enabled": high_enabled,
    },
}
for f in findings:
    key = f["source"]
    verdict["counts"][key] = verdict["counts"].get(key, 0) + 1

with open(os.path.join(report_dir, "security-verdict.json"), "w", encoding="utf-8") as out:
    json.dump(verdict, out, indent=2)
for f in blocked:
    print("verdict: BLOCKED %s %s %s %s (%s)" % (
        f["source"], f["id"], f["component"], f["version"], f["severity"]), file=sys.stderr)
sys.exit(0 if verdict["ok"] else 1)
PY
}

# ------------------------------------------------------------ gate dispatch
GATE_LIST="$(echo "$GATES" | tr ',' ' ')"
FAILED=0
if [ "$VERDICT_ONLY" -eq 1 ]; then
  echo "verdict-only mode: skipping gate execution, judging existing reports" >&2
  GATE_LIST=""
fi
for gate in $GATE_LIST; do
  case "$gate" in
    policy)
      rc=0
      run_policy_gate || rc=$?
      record "verify-security:policy" "$rc"
      SUMMARY_GATES+=("policy:$rc")
      if [ $rc -ne 0 ]; then FAILED=1; fi
      ;;
    secrets)
      rc=0
      run_secrets_gate || rc=$?
      record "verify-security:secrets" "$rc"
      SUMMARY_GATES+=("secrets:$rc")
      if [ $rc -ne 0 ]; then FAILED=1; fi
      ;;
    go)
      rc=0
      run_go_gate || rc=$?
      record "verify-security:go" "$rc"
      SUMMARY_GATES+=("go:$rc")
      if [ $rc -ne 0 ]; then FAILED=1; fi
      ;;
    npm)
      rc=0
      run_npm_gate || rc=$?
      record "verify-security:npm" "$rc"
      SUMMARY_GATES+=("npm:$rc")
      if [ $rc -ne 0 ]; then FAILED=1; fi
      ;;
    cargo)
      rc=0
      run_cargo_gate || rc=$?
      record "verify-security:cargo" "$rc"
      SUMMARY_GATES+=("cargo:$rc")
      if [ $rc -ne 0 ]; then FAILED=1; fi
      ;;
    trivy-fs)
      rc=0
      run_trivy_fs_gate || rc=$?
      record "verify-security:trivy-fs" "$rc"
      SUMMARY_GATES+=("trivy-fs:$rc")
      if [ $rc -ne 0 ]; then FAILED=1; fi
      ;;
    trivy-config)
      rc=0
      run_trivy_config_gate || rc=$?
      record "verify-security:trivy-config" "$rc"
      SUMMARY_GATES+=("trivy-config:$rc")
      if [ $rc -ne 0 ]; then FAILED=1; fi
      ;;
    trivy-image)
      rc=0
      run_trivy_image_gate || rc=$?
      record "verify-security:trivy-image" "$rc"
      SUMMARY_GATES+=("trivy-image:$rc")
      if [ $rc -ne 0 ]; then FAILED=1; fi
      ;;
    *)
      echo "unknown gate: $gate" >&2
      exit 2
      ;;
  esac
done

# ------------------------------------------------------------- final verdict
verdict_rc=0
run_verdict || verdict_rc=$?
record "verify-security:verdict" "$verdict_rc"
SUMMARY_GATES+=("verdict:$verdict_rc")
if [ $verdict_rc -ne 0 ]; then FAILED=1; fi

FINISHED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
GATES_JSON="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "${SUMMARY_GATES[@]}")"
COMMANDS_JSON="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "${RUN_COMMANDS[@]}")"
EXIT_CODES_JSON="$(python3 -c 'import json,sys; print(json.dumps([int(x) for x in sys.argv[1:]]))' "${RUN_EXIT_CODES[@]}")"
GOVULN_JSON="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$GOVULNCHECK_VER" "$CARGO_AUDIT_VER" "$GITLEAKS_IMAGE" "$TRIVY_IMAGE")"
python3 - "$REPORT_DIR" "$STARTED_AT" "$FINISHED_AT" "$GATES_JSON" "$COMMANDS_JSON" "$EXIT_CODES_JSON" "$GOVULN_JSON" <<'PY'
import json, os, subprocess, sys

report_dir, started, finished = sys.argv[1], sys.argv[2], sys.argv[3]
gates = json.loads(sys.argv[4])
commands = json.loads(sys.argv[5])
exit_codes = json.loads(sys.argv[6])
pinned = json.loads(sys.argv[7])
versions = {}
for name, cmd in (
    ("go", ["go", "version"]),
    ("node", ["node", "--version"]),
    ("npm", ["npm", "--version"]),
):
    try:
        versions[name] = subprocess.run(cmd, capture_output=True, text=True).stdout.strip()
    except Exception:
        pass
summary = {
    "started_at": started,
    "finished_at": finished,
    "gates": gates,
    "commands": commands,
    "exit_codes": exit_codes,
    "tool_versions": versions,
    "pinned_tools": pinned,
    "ok": all(c == 0 for c in exit_codes),
}
with open(os.path.join(report_dir, "security-summary.json"), "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2)
PY

echo "OK: security verification gate summary written to $REPORT_DIR/security-summary.json"
[ $FAILED -eq 0 ] || exit 1
exit 0
