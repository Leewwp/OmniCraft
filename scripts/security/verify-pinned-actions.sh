#!/usr/bin/env bash
# =============================================================================
# OmniCraft pinned-actions verification: statically checks every GitHub
# Actions workflow for SHA-pinned actions, least-privilege permissions and
# stable security-gate naming. Floating action references (e.g. @v4, @main)
# or write-all permissions fail the gate; `|| true`/continue-on-error in the
# security workflow is forbidden because it would hide scan failures.
#
# Usage:
#   bash scripts/security/verify-pinned-actions.sh [-WorkflowDir <dir>] [-ReportDir <dir>]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WORKFLOW_DIR="$REPO_ROOT/.github/workflows"
REPORT_DIR=""

while [ $# -gt 0 ]; do
  case "$1" in
    -WorkflowDir) WORKFLOW_DIR="$2"; shift 2 ;;
    -ReportDir) REPORT_DIR="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

[ -d "$WORKFLOW_DIR" ] || { echo "workflow dir not found: $WORKFLOW_DIR" >&2; exit 1; }
if [ -z "$REPORT_DIR" ]; then
  REPORT_DIR="$REPO_ROOT/artifacts/security"
fi
mkdir -p "$REPORT_DIR"

FAILED=0
ISSUES=""
WORKFLOWS=""

for wf in "$WORKFLOW_DIR"/*.yml "$WORKFLOW_DIR"/*.yaml; do
  [ -f "$wf" ] || continue
  NAME="$(basename "$wf")"
  WORKFLOWS="$WORKFLOWS $NAME"

  # Every `uses:` reference must be pinned to a 40-hex commit SHA (for
  # owner/repo actions) or a sha256 image digest (for docker:// actions).
  # This grep is intentionally strict: comments are not allowed to hide
  # an unpinned reference on the same line.
  if grep -E "uses:[[:space:]]*[^#]" "$wf" | grep -Ev "uses:[[:space:]]*([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@[0-9a-f]{40}|docker://[A-Za-z0-9_.:/@-]+@sha256:[0-9a-f]{64})" >/dev/null 2>&1; then
    ISSUES="$ISSUES
$NAME: floating action reference (must be SHA-pinned)"
    FAILED=1
  fi

  # The security workflow must expose the stable aggregate job `security-gate`
  # and must never hide scan failures.
  if [ "$NAME" = "security.yml" ]; then
    if ! grep -Eq "^[[:space:]]*security-gate:[[:space:]]*$" "$wf"; then
      ISSUES="$ISSUES
security.yml: missing stable job name security-gate"
      FAILED=1
    fi
    if grep -Eq "continue-on-error" "$wf"; then
      ISSUES="$ISSUES
security.yml: continue-on-error is forbidden (would hide scan failures)"
      FAILED=1
    fi
    if grep -Eq "\|\|[[:space:]]*true" "$wf"; then
      ISSUES="$ISSUES
security.yml: '|| true' is forbidden (would hide scan failures)"
      FAILED=1
    fi
  fi
done

# write-all / `permissions: write` / `permissions: <scope>: write` are
# forbidden (least-privilege). Checked line-wise in Python: a grep for the
# literal "write-all" can be bypassed by the `permissions: write` shorthand
# or a scoped `contents: write` under a multi-line permissions block. Scope
# keys are only matched while inside a `permissions:` block, so `needs:` and
# comment lines cannot false-positive.
# Exception (Ops-06 reservation extension): sbom.yml's provenance job may
# request `attestations: write` and `id-token: write` — both are mandatory for
# actions/attest-build-provenance, the only provenance mechanism used, and the
# action reference itself is still SHA-pinned by the check above.
PERM_ISSUES="$(python3 - "$WORKFLOW_DIR" <<'PY'
import os, re, sys

workflow_dir = sys.argv[1]
SCOPES = ("contents", "packages", "pages", "security-events", "statuses",
          "id-token", "attestations")
ATTESTATION_WORKFLOWS = ("sbom.yml",)
scope_re = re.compile(r"^\s*(%s):\s*write\b" % "|".join(SCOPES))
issues = []
for wf in sorted(os.listdir(workflow_dir)):
    if not (wf.endswith(".yml") or wf.endswith(".yaml")):
        continue
    try:
        lines = open(os.path.join(workflow_dir, wf), encoding="utf-8").read().splitlines()
    except OSError as e:
        issues.append("%s: unreadable workflow: %s" % (wf, e))
        continue
    in_permissions = False
    perm_indent = -1
    for line in lines:
        code = line.split("#", 1)[0].rstrip()
        if not code.strip():
            continue
        indent = len(line) - len(line.lstrip())
        if in_permissions and indent <= perm_indent:
            in_permissions = False
        if "write-all" in code:
            issues.append("%s: write-all permissions are forbidden" % wf)
        elif re.match(r"^permissions:\s*write\s*$", code.strip()):
            issues.append("%s: permissions: write is forbidden (use explicit read-only scopes)" % wf)
        elif re.match(r"^permissions:\s*\{.*\bwrite\b", code.strip()):
            issues.append("%s: permissions flow-map with write is forbidden (least-privilege)" % wf)
        elif re.match(r"^permissions:\s*$", code.strip()):
            in_permissions = True
            perm_indent = indent
        if in_permissions and scope_re.match(code):
            scope = code.split(":")[0].strip()
            if scope in ("id-token", "attestations") and wf in ATTESTATION_WORKFLOWS:
                continue
            issues.append("%s: permissions: %s: write is forbidden (least-privilege)" % (wf, scope))
print("\n".join(issues))
PY
)"
if [ -n "$PERM_ISSUES" ]; then
  ISSUES="$ISSUES
$PERM_ISSUES"
  FAILED=1
fi

[ -n "$WORKFLOWS" ] || { echo "no workflow files found in $WORKFLOW_DIR" >&2; exit 1; }

python3 - "$REPORT_DIR" "$WORKFLOWS" "$ISSUES" "$FAILED" <<'PY'
import json, sys
report_dir, workflows, issues, failed = sys.argv[1], sys.argv[2].split(), sys.argv[3], int(sys.argv[4])
summary = {
    "workflows": sorted(workflows),
    "issues": [i for i in issues.split("\n") if i.strip()] if issues else [],
    "ok": failed == 0,
}
with open(report_dir + "/pinned-actions.json", "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2)
PY

if [ $FAILED -ne 0 ]; then
  echo "FAIL: pinned-actions verification failed:" >&2
  echo "$ISSUES" >&2
  exit 1
fi
echo "OK: all workflows use SHA-pinned actions with least-privilege permissions"
exit 0
