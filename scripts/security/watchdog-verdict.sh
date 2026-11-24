#!/usr/bin/env bash
# Red-lock watchdog for main-line Security runs: opens a needs-triage issue
# when the security-evidence artifact's security-verdict.json contains real
# (non `scanner-failure:` prefixed) critical/high advisories, deduplicating
# against open issues by advisory id. Scanner failures — missing or
# unparseable artifact, `scanner-failure:` findings — never open issues: the
# security-gate job is already red for those, and duplicating scanner outages
# as advisories trains people to ignore the queue.
# Usage: watchdog-verdict.sh -ReportDir <dir> -RunUrl <url>
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
REPORT_DIR="$REPO_ROOT/artifacts/security"
RUN_URL=""

while [ $# -gt 0 ]; do
  case "$1" in
    -ReportDir)
      if [ $# -lt 2 ]; then
        echo "missing value for -ReportDir" >&2
        exit 2
      fi
      shift
      REPORT_DIR="$1"
      ;;
    -RunUrl)
      if [ $# -lt 2 ]; then
        echo "missing value for -RunUrl" >&2
        exit 2
      fi
      shift
      RUN_URL="$1"
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

TARGET_REPO="${GH_TARGET_REPO:-${GITHUB_REPOSITORY:-Leewwp/OmniCraft}}"

# Decision stage: emit one ADVISORY line per real critical/high finding.
# Fields are pipe-separated; detail is last so it may contain spaces.
ADVISORIES="$(python3 - "$REPORT_DIR" <<'PY'
import json, os, sys

report_dir = sys.argv[1]
path = os.path.join(report_dir, "security-verdict.json")
try:
    with open(path, encoding="utf-8") as f:
        verdict = json.load(f)
except Exception:
    print("watchdog: evidence artifact missing or unparseable (scanner failure; security-gate already red)")
    sys.exit(0)
if not isinstance(verdict, dict):
    print("watchdog: verdict is not an object (scanner failure; security-gate already red)")
    sys.exit(0)
blocked = verdict.get("blocked_findings")
if not isinstance(blocked, list):
    blocked = []
real = []
for f in blocked:
    if not isinstance(f, dict):
        continue
    fid = str(f.get("id", ""))
    severity = str(f.get("severity", "")).lower()
    if not fid or fid.startswith("scanner-failure:"):
        continue
    if severity not in ("critical", "high"):
        continue
    real.append(f)
if not real:
    print("watchdog: no real critical/high advisories (clean or scanner failure)")
    sys.exit(0)
for f in real:
    fields = [str(f.get(k, "")) for k in ("id", "component", "version", "severity", "source", "detail")]
    print("ADVISORY|" + "|".join(fields))
PY
)" || exit 1

if ! printf '%s\n' "$ADVISORIES" | grep -q '^ADVISORY|'; then
  printf '%s\n' "$ADVISORIES"
  exit 0
fi

UNCOVERED=""
COVERED_COUNT=0
while IFS= read -r line; do
  case "$line" in
    ADVISORY|*) : ;;
    *) continue ;;
  esac
  IFS='|' read -r _ ID COMPONENT VERSION SEVERITY SOURCE DETAIL <<LINE
$line
LINE
  if gh issue list --repo "$TARGET_REPO" --state open --search "$ID" --limit 20 --json number,title 2>/dev/null | grep -Fq "$ID"; then
    COVERED_COUNT=$((COVERED_COUNT + 1))
    continue
  fi
  UNCOVERED="${UNCOVERED}${line}
"
done <<EOF
$(printf '%s\n' "$ADVISORIES" | grep '^ADVISORY|')
EOF

if [ "$COVERED_COUNT" -gt 0 ]; then
  echo "watchdog: $COVERED_COUNT advisory/ies already tracked by open issues"
fi
if [ -z "$UNCOVERED" ]; then
  echo "watchdog: nothing new to report"
  exit 0
fi

TITLE=""
BODY_ITEMS=""
NEW_COUNT=0
while IFS= read -r line; do
  [ -n "$line" ] || continue
  IFS='|' read -r _ ID COMPONENT VERSION SEVERITY SOURCE DETAIL <<LINE
$line
LINE
  NEW_COUNT=$((NEW_COUNT + 1))
  if [ -z "$TITLE" ]; then
    TITLE="security: 红锁公告待修复 ${ID} ${COMPONENT}@${VERSION}"
  fi
  BODY_ITEMS="${BODY_ITEMS}- ${ID} — ${COMPONENT}@${VERSION}（${SEVERITY}，source: ${SOURCE}；${DETAIL}）
"
done <<EOF
$UNCOVERED
EOF
if [ "$NEW_COUNT" -gt 1 ]; then
  TITLE="${TITLE}（另 $((NEW_COUNT - 1)) 项）"
fi
BODY="Security 看门自动开票：main 线 Security 扫描发现真实公告（critical/high、非扫描器故障）。

$BODY_ITEMS
扫描 run：$RUN_URL

修复 PR 请以 \`Closes\` 引用本票关账；扫描频率与判据见 security/scan-policy.json 与 scripts/security/watchdog-verdict.sh。"

gh issue create --repo "$TARGET_REPO" --title "$TITLE" --body "$BODY" --label needs-triage || {
  echo "watchdog: gh issue create failed" >&2
  exit 1
}
echo "watchdog: opened tracking issue for $NEW_COUNT advisory/ies"
