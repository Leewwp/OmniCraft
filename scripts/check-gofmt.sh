#!/usr/bin/env bash
# Backend gofmt gate: `gofmt -l` over backend/ must report nothing.
# The gate only names offending files on stderr; it never rewrites them.
# gofmt resolves from the active Go toolchain (go env GOROOT) so local runs
# and CI (which pins the Go version) enforce identical formatting rules.
# GOFMT_BIN may override the binary location (used by verify-project
# contract tests to drive the gate with a fake gofmt).
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$(cd "$SCRIPT_DIR/../backend" && pwd)"

GOFMT_BIN="${GOFMT_BIN:-}"
if [ -z "$GOFMT_BIN" ]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "check-gofmt: go not found on PATH (needed to locate the toolchain gofmt)" >&2
    exit 1
  fi
  GOFMT_BIN="$(go env GOROOT)/bin/gofmt"
fi
if [ ! -x "$GOFMT_BIN" ]; then
  echo "check-gofmt: gofmt binary not found or not executable: $GOFMT_BIN" >&2
  exit 1
fi

unformatted="$(cd "$BACKEND_DIR" && "$GOFMT_BIN" -l .)" || {
  echo "check-gofmt: gofmt invocation failed" >&2
  exit 1
}
if [ -n "$unformatted" ]; then
  echo "check-gofmt: unformatted Go files (run gofmt -w on them):" >&2
  printf '%s\n' "$unformatted" >&2
  exit 1
fi
exit 0
