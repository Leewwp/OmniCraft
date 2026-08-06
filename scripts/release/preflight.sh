#!/usr/bin/env bash
# =============================================================================
# OmniCraft production preflight: validates the effective production
# configuration (environment file + operator override YAML merged over the
# repository default config.yaml) against release-mode rules, including
# placeholder rejection, HTTPS/origin rules, database TLS policy, trusted-proxy
# topology, callback IP allowlist, frontend/API URL consistency and unsafe
# feature flags. Writes a redacted preflight summary (never echo secrets).
#
# Usage:
#   bash scripts/release/preflight.sh -EnvironmentFile <path> -OverrideFile <path>
#       [-Schema <path>] [-ReportDir <dir>] [-Binary <path>] [-ComposeFile <path>]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENV_FILE=""
OVERRIDE_FILE=""
SCHEMA=""
REPORT_DIR=""
BINARY=""
COMPOSE_FILE=""

while [ $# -gt 0 ]; do
  case "$1" in
    -EnvironmentFile)
      [ $# -ge 2 ] || { echo "missing value for -EnvironmentFile" >&2; exit 2; }
      ENV_FILE="$2"; shift 2 ;;
    -OverrideFile)
      [ $# -ge 2 ] || { echo "missing value for -OverrideFile" >&2; exit 2; }
      OVERRIDE_FILE="$2"; shift 2 ;;
    -Schema)
      [ $# -ge 2 ] || { echo "missing value for -Schema" >&2; exit 2; }
      SCHEMA="$2"; shift 2 ;;
    -ReportDir)
      [ $# -ge 2 ] || { echo "missing value for -ReportDir" >&2; exit 2; }
      REPORT_DIR="$2"; shift 2 ;;
    -Binary)
      [ $# -ge 2 ] || { echo "missing value for -Binary" >&2; exit 2; }
      BINARY="$2"; shift 2 ;;
    -ComposeFile)
      [ $# -ge 2 ] || { echo "missing value for -ComposeFile" >&2; exit 2; }
      COMPOSE_FILE="$2"; shift 2 ;;
    *)
      echo "unknown argument: $1" >&2
      echo "usage: preflight.sh -EnvironmentFile <path> -OverrideFile <path> [-Schema <path>] [-ReportDir <dir>] [-Binary <path>] [-ComposeFile <path>]" >&2
      exit 2 ;;
  esac
done

if [ -z "$ENV_FILE" ] || [ -z "$OVERRIDE_FILE" ]; then
  echo "usage: preflight.sh -EnvironmentFile <path> -OverrideFile <path> [-Schema <path>] [-ReportDir <dir>] [-Binary <path>] [-ComposeFile <path>]" >&2
  exit 2
fi

[ -f "$ENV_FILE" ] || { echo "environment file not found: $ENV_FILE" >&2; exit 1; }
[ -f "$OVERRIDE_FILE" ] || { echo "override file not found: $OVERRIDE_FILE" >&2; exit 1; }
if [ -n "$SCHEMA" ] && [ ! -f "$SCHEMA" ]; then
  echo "schema not found: $SCHEMA" >&2
  exit 1
fi

ENV_FILE="$(cd "$(dirname "$ENV_FILE")" && pwd)/$(basename "$ENV_FILE")"
OVERRIDE_FILE="$(cd "$(dirname "$OVERRIDE_FILE")" && pwd)/$(basename "$OVERRIDE_FILE")"
if [ -n "$REPORT_DIR" ]; then
  mkdir -p "$REPORT_DIR"
  REPORT_DIR="$(cd "$REPORT_DIR" && pwd)"
fi

ARGS=(-EnvironmentFile "$ENV_FILE" -OverrideFile "$OVERRIDE_FILE")
[ -n "$SCHEMA" ] && ARGS+=(-Schema "$SCHEMA")
[ -n "$REPORT_DIR" ] && ARGS+=(-ReportDir "$REPORT_DIR")
[ -n "$COMPOSE_FILE" ] && ARGS+=(-ComposeFile "$COMPOSE_FILE")

if [ -z "$BINARY" ]; then
  BUILD_DIR="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-preflight.XXXXXX")"
  trap 'rm -rf "$BUILD_DIR"' EXIT
  if ! (cd "$REPO_ROOT/backend" && go build -o "$BUILD_DIR/release-preflight" ./cmd/release-preflight); then
    echo "preflight: failed to build release-preflight" >&2
    exit 1
  fi
  BINARY="$BUILD_DIR/release-preflight"
fi

"$BINARY" "${ARGS[@]}"
