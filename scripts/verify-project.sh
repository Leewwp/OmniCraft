#!/usr/bin/env bash
# OmniCraft project verification gate (bash port of the former verify-project.ps1).
# Default: deterministic engineering gate; --full adds mocked Playwright contracts;
# --release adds the complete Playwright suite; --tauri adds desktop gates.
set -u

REPO_ROOT=""
FULL=0
RELEASE=0
TAURI=0

while [ $# -gt 0 ]; do
  case "$1" in
    --repo-root)
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

if [ "$FULL" -eq 1 ] && [ "$RELEASE" -eq 1 ]; then
  echo "-Full and -Release are mutually exclusive verification tiers" >&2
  exit 2
fi

if [ "$RELEASE" -eq 1 ]; then
  TIER="release"
elif [ "$FULL" -eq 1 ]; then
  TIER="full"
else
  TIER="default"
fi

echo "OmniCraft verification tier: $TIER"

run_checked() {
  local dir="$1"
  shift
  echo "> $*"
  ( cd "$dir" && "$@" ) || exit 1
}

run_checked "$REPO_ROOT/backend" go test ./...
run_checked "$REPO_ROOT/backend" go vet ./...
run_checked "$REPO_ROOT/backend" go build ./...

run_checked "$REPO_ROOT/frontend" npm run test:unit
run_checked "$REPO_ROOT/frontend" npm run lint
run_checked "$REPO_ROOT/frontend" npm run build

run_checked "$REPO_ROOT/tools/doc-validator" go test ./...
run_checked "$REPO_ROOT/tools/doc-validator" go run . --check --profile release

if [ "$FULL" -eq 1 ]; then
  run_checked "$REPO_ROOT/frontend" npm run test:contracts
fi

if [ "$RELEASE" -eq 1 ]; then
  run_checked "$REPO_ROOT/frontend" npm run test:e2e
fi

if [ "$TAURI" -eq 1 ]; then
  run_checked "$REPO_ROOT/tauri-client" npm run build
  run_checked "$REPO_ROOT/tauri-client" cargo test --manifest-path src-tauri/Cargo.toml
fi

echo "OmniCraft $TIER verification passed"
