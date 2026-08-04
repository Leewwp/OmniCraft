#!/usr/bin/env bash
# OmniCraft project verification gate (bash port of the former verify-project.ps1).
# Default: deterministic engineering gate; --full adds mocked Playwright contracts;
# --release adds the complete Playwright suite; --tauri adds desktop gates.
# -Scope restricts the command set to Backend|Frontend|Docs|All;
# -ReportDir writes per-command UTF-8 logs and a machine-readable summary.json.
set -u

REPO_ROOT=""
FULL=0
RELEASE=0
TAURI=0
SCOPE="All"
REPORT_DIR=""

while [ $# -gt 0 ]; do
  case "$1" in
    --repo-root)
      if [ $# -lt 2 ]; then
        echo "missing value for --repo-root" >&2
        exit 2
      fi
      shift
      REPO_ROOT="$1"
      ;;
    --full)
      FULL=1
      ;;
    --release)
      RELEASE=1
      ;;
    --tauri)
      TAURI=1
      ;;
    -Scope)
      if [ $# -lt 2 ]; then
        echo "missing value for -Scope" >&2
        exit 2
      fi
      shift
      SCOPE="$1"
      ;;
    -ReportDir)
      if [ $# -lt 2 ]; then
        echo "missing value for -ReportDir" >&2
        exit 2
      fi
      shift
      REPORT_DIR="$1"
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

if [ -z "$REPO_ROOT" ]; then
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  REPO_ROOT="$(dirname "$SCRIPT_DIR")"
fi
REPO_ROOT="$(cd "$REPO_ROOT" && pwd)"

case "$SCOPE" in
  Backend|Frontend|Docs|Tauri|All) ;;
  *)
    echo "unknown scope: $SCOPE (expected Backend|Frontend|Docs|Tauri|All)" >&2
    exit 2
    ;;
esac

if [ "$FULL" -eq 1 ] && [ "$RELEASE" -eq 1 ]; then
  echo "--full and --release are mutually exclusive verification tiers" >&2
  exit 2
fi

if [ "$RELEASE" -eq 1 ]; then
  TIER="release"
elif [ "$FULL" -eq 1 ]; then
  TIER="full"
else
  TIER="default"
fi

echo "OmniCraft verification tier: $TIER (scope: $SCOPE)"

if [ -n "$REPORT_DIR" ]; then
  mkdir -p "$REPORT_DIR"
  REPORT_DIR="$(cd "$REPORT_DIR" && pwd)"
fi

in_scope() {
  [ "$SCOPE" = "All" ] || [ "$SCOPE" = "$1" ]
}

COMMANDS=()
EXIT_CODES=()
EVIDENCE=()
TOOLS=()
HALT=0
STARTED_AT=""

if [ -n "$REPORT_DIR" ]; then
  STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi

add_tool() {
  local tools="${TOOLS[*]:-}"
  case " $tools " in
    *" $1 "*) ;;
    *) TOOLS+=("$1") ;;
  esac
}

run_checked() {
  local dir="$1"
  local label="$2"
  local slug="$3"
  shift 3
  if [ "$HALT" -eq 1 ]; then
    return 0
  fi
  echo "> $*"
  local rc=0
  if [ -n "$REPORT_DIR" ]; then
    mkdir -p "$REPORT_DIR"
    ( cd "$dir" && set -o pipefail; "$@" 2>&1 | tee "$REPORT_DIR/$slug.log" )
    rc=$?
    EVIDENCE+=("$REPORT_DIR/$slug.log")
  else
    ( cd "$dir" && "$@" )
    rc=$?
  fi
  COMMANDS+=("$label|$*")
  EXIT_CODES+=("$rc")
  case "$label" in
    backend|doc-validator)
      add_tool go
      ;;
    frontend)
      add_tool node
      add_tool npm
      ;;
    tauri-client)
      add_tool node
      add_tool npm
      add_tool cargo
      ;;
  esac
  if [ "$rc" -ne 0 ]; then
    HALT=1
  fi
}

if in_scope Backend; then
  run_checked "$REPO_ROOT/backend" backend backend-go-test go test ./...
  run_checked "$REPO_ROOT/backend" backend backend-go-vet go vet ./...
  run_checked "$REPO_ROOT/backend" backend backend-go-build go build ./...
fi

if in_scope Frontend; then
  run_checked "$REPO_ROOT/frontend" frontend frontend-npm-test-unit npm run test:unit
  run_checked "$REPO_ROOT/frontend" frontend frontend-npm-lint npm run lint
  run_checked "$REPO_ROOT/frontend" frontend frontend-npm-lint-ui npm run lint:ui
  run_checked "$REPO_ROOT/frontend" frontend frontend-npm-build npm run build
fi

if in_scope Docs; then
  run_checked "$REPO_ROOT/tools/doc-validator" doc-validator doc-validator-go-test go test ./...
  run_checked "$REPO_ROOT/tools/doc-validator" doc-validator doc-validator-go-check go run . --check --profile release
fi

if [ "$FULL" -eq 1 ] && in_scope Frontend; then
  run_checked "$REPO_ROOT/frontend" frontend frontend-npm-contracts npm run test:contracts
fi

if [ "$RELEASE" -eq 1 ] && in_scope Frontend; then
  run_checked "$REPO_ROOT/frontend" frontend frontend-npm-e2e npm run test:e2e
fi

if [ "$SCOPE" = "Tauri" ] || [ "$TAURI" -eq 1 ]; then
  run_checked "$REPO_ROOT/tauri-client" tauri-client tauri-npm-build npm run build
  run_checked "$REPO_ROOT/tauri-client" tauri-client tauri-cargo-test cargo test --manifest-path src-tauri/Cargo.toml
fi

if [ -n "$REPORT_DIR" ]; then
  mkdir -p "$REPORT_DIR"
  state_file="$REPORT_DIR/.summary-state.tmp"
  : > "$state_file"
  for i in "${!COMMANDS[@]}"; do
    printf '%s\t%s\t%s\n' "${COMMANDS[$i]}" "${EXIT_CODES[$i]}" "${EVIDENCE[$i]}" >> "$state_file"
  done
  versions_file="$REPORT_DIR/.summary-versions.tmp"
  : > "$versions_file"
  for tool in "${TOOLS[@]}"; do
    if [ "$tool" = "go" ]; then
      version="$("$tool" version 2>/dev/null | head -n 1)"
    else
      version="$("$tool" --version 2>/dev/null | head -n 1)"
    fi
    printf '%s\t%s\n' "$tool" "$version" >> "$versions_file"
  done
  task_name="$(basename "$REPORT_DIR")"
  finished_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  python3 - "$REPORT_DIR/summary.json" "$state_file" "$versions_file" "$task_name" "$STARTED_AT" "$finished_at" <<'PY'
import json, sys

summary_path, state_file, versions_file, task_name, started_at, finished_at = sys.argv[1:]
commands = []
exit_codes = []
evidence = []
for line in open(state_file, encoding="utf-8"):
    cmd, code, ev = line.rstrip("\n").split("\t")
    commands.append(cmd)
    exit_codes.append(int(code))
    evidence.append(ev)
tool_versions = {}
for line in open(versions_file, encoding="utf-8"):
    tool, version = line.rstrip("\n").split("\t")
    tool_versions[tool] = version
summary = {
    "task": task_name,
    "commit": None,
    "started_at": started_at,
    "finished_at": finished_at,
    "commands": commands,
    "exit_codes": exit_codes,
    "tool_versions": tool_versions,
    "evidence": evidence,
    "redaction_checked": False,
    "blockers": [],
}
with open(summary_path, "w", encoding="utf-8") as out:
    json.dump(summary, out, ensure_ascii=False, indent=2)
    out.write("\n")
PY
  rm -f "$state_file" "$versions_file"
fi

if [ "$HALT" -eq 1 ]; then
  exit 1
fi

echo "OmniCraft $TIER verification passed"
