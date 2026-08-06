#!/usr/bin/env bash
# Contract tests for scripts/security/verify-pinned-actions.sh: every GitHub
# Actions workflow must use SHA-pinned actions, least-privilege permissions,
# and the security workflow must expose the stable `security-gate` job without
# failure-hiding constructs. Runs on tampered fixture workflow dirs.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VERIFY="$SCRIPT_DIR/verify-pinned-actions.sh"

if [ ! -f "$VERIFY" ]; then
  echo "verify-pinned-actions.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-pinned-actions.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

expect_exit() {
  local expected="$1" label="$2" dir="$3"
  local actual=0
  bash "$VERIFY" -WorkflowDir "$dir" -ReportDir "$TEMP_ROOT/report-$label" \
    >/dev/null 2>&1 || actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL: $label: expected exit $expected, got $actual" >&2
    exit 1
  fi
  echo "OK: $label"
}

# ----------------------------------------------------------------- usage/args
rc=0
bash "$VERIFY" -Bogus x >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || { echo "FAIL: unknown argument must exit 2" >&2; exit 1; }
rc=0
bash "$VERIFY" -WorkflowDir "$TEMP_ROOT/does-not-exist" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 1 ] || { echo "FAIL: missing WorkflowDir must exit 1" >&2; exit 1; }

# ------------------------------------------------------------- valid fixture
mkdir -p "$TEMP_ROOT/valid"
cat > "$TEMP_ROOT/valid/ci.yml" <<'EOF'
name: CI
permissions:
  contents: read
jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
      - uses: docker://aquasec/trivy:0.57.1@sha256:5c59e08f980b5d4d503329773480fcea2c9bdad7e381d846fbf9f2ecb8050f6b
EOF
cat > "$TEMP_ROOT/valid/security.yml" <<'EOF'
name: Security
permissions:
  contents: read
jobs:
  security-gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
      - run: bash scripts/security/verify-security.sh
EOF
expect_exit 0 "valid workflow dir" "$TEMP_ROOT/valid"

# -------------------------------------------------------- floating action
mkdir -p "$TEMP_ROOT/floating"
cat > "$TEMP_ROOT/floating/ci.yml" <<'EOF'
permissions:
  contents: read
jobs:
  gate:
    steps:
      - uses: actions/checkout@v4
EOF
expect_exit 1 "floating @v4 action rejected" "$TEMP_ROOT/floating"

# ----------------------------------------------------------- short SHA pin
mkdir -p "$TEMP_ROOT/shortsha"
cat > "$TEMP_ROOT/shortsha/ci.yml" <<'EOF'
permissions:
  contents: read
jobs:
  gate:
    steps:
      - uses: actions/checkout@3d3c42e
EOF
expect_exit 1 "short SHA pin rejected" "$TEMP_ROOT/shortsha"

# ---------------------------------------------------------- write-all perms
mkdir -p "$TEMP_ROOT/writeall"
cat > "$TEMP_ROOT/writeall/ci.yml" <<'EOF'
permissions:
  contents: write-all
jobs:
  gate:
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
EOF
expect_exit 1 "write-all permissions rejected" "$TEMP_ROOT/writeall"

# ------------------------------------------------- permissions: write shorthand
mkdir -p "$TEMP_ROOT/permwrite"
cat > "$TEMP_ROOT/permwrite/ci.yml" <<'EOF'
permissions: write
jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
EOF
expect_exit 1 "permissions: write shorthand rejected" "$TEMP_ROOT/permwrite"

# ------------------------------------------- scoped write in a multi-line block
mkdir -p "$TEMP_ROOT/permscope"
cat > "$TEMP_ROOT/permscope/ci.yml" <<'EOF'
permissions:
  contents: write
jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
EOF
expect_exit 1 "scoped contents: write permissions rejected" "$TEMP_ROOT/permscope"

# ------------------------------------------------- permissions flow-map write
mkdir -p "$TEMP_ROOT/permflow"
cat > "$TEMP_ROOT/permflow/ci.yml" <<'EOF'
permissions: {contents: write}
jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
EOF
expect_exit 1 "permissions flow-map with write rejected" "$TEMP_ROOT/permflow"

# --------------------------------------------------- security-gate missing
mkdir -p "$TEMP_ROOT/nosecgate"
cat > "$TEMP_ROOT/nosecgate/security.yml" <<'EOF'
permissions:
  contents: read
jobs:
  other:
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
EOF
expect_exit 1 "security.yml without security-gate job rejected" "$TEMP_ROOT/nosecgate"

# ------------------------------------------------- continue-on-error hidden
mkdir -p "$TEMP_ROOT/contonerr"
cat > "$TEMP_ROOT/contonerr/security.yml" <<'EOF'
permissions:
  contents: read
jobs:
  security-gate:
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
      - run: bash scripts/security/verify-security.sh
        continue-on-error: true
EOF
expect_exit 1 "continue-on-error in security.yml rejected" "$TEMP_ROOT/contonerr"

# ------------------------------------------------------------ '|| true' hidden
mkdir -p "$TEMP_ROOT/pipetrue"
cat > "$TEMP_ROOT/pipetrue/security.yml" <<'EOF'
permissions:
  contents: read
jobs:
  security-gate:
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
      - run: bash scripts/security/verify-security.sh || true
EOF
expect_exit 1 "|| true in security.yml rejected" "$TEMP_ROOT/pipetrue"

# ------------------------------------------------------------- real workflows
bash "$VERIFY" -ReportDir "$TEMP_ROOT/report-real" >/dev/null 2>&1 || {
  echo "FAIL: repository workflows must pass pinned-actions verification" >&2
  exit 1
}
echo "OK: repository workflows"

echo "OK: pinned-actions contract tests passed"
