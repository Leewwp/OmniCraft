#!/usr/bin/env bash
# Contract tests for scripts/release/generate-sbom.sh: deterministic CycloneDX
# SBOM generation with the pinned syft image, release-manifest binding and
# preview mode. Lockfile fixtures are copied from the real repo; the container
# fixture is a tiny alpine image built in the test so the full pipeline runs
# without the application images. Runs the pinned syft image like the real
# generator.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GENERATE="$SCRIPT_DIR/generate-sbom.sh"
VERIFY="$SCRIPT_DIR/verify-provenance.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
POLICY="$REPO_ROOT/release/sbom-policy.json"

if [ ! -f "$GENERATE" ]; then
  echo "generate-sbom.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-sbom.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

# ------------------------------------------------------- fixture repository
FIXTURE="$TEMP_ROOT/fixture"
mkdir -p "$FIXTURE/backend" "$FIXTURE/frontend" "$FIXTURE/tauri-client/src-tauri" "$FIXTURE/release"

cp "$REPO_ROOT/backend/go.mod" "$REPO_ROOT/backend/go.sum" "$FIXTURE/backend/"
cp -R "$REPO_ROOT/backend/migrations" "$FIXTURE/backend/"
cp "$REPO_ROOT/frontend/package-lock.json" "$FIXTURE/frontend/"
cp "$REPO_ROOT/tauri-client/package-lock.json" "$FIXTURE/tauri-client/"
cp "$REPO_ROOT/tauri-client/src-tauri/Cargo.lock" "$FIXTURE/tauri-client/src-tauri/"

# Fixture policy reuses the real tool pin but points the container component
# at the tiny test image so the full pipeline never needs the application
# images.
python3 - "$POLICY" "$FIXTURE/release/sbom-policy.json" <<'PY'
import json, sys

real = json.load(open(sys.argv[1], encoding="utf-8"))
policy = {
    "schema_version": real["schema_version"],
    "format": real["format"],
    "required_ecosystems": real["required_ecosystems"],
    "components": [
        c for c in real["components"]
        if c["ecosystem"] in ("go", "npm", "rust")
    ] + [{
        "id": "fixture-container",
        "ecosystem": "container",
        "source": "fixture alpine image",
        "image": "omnicraft-sbom-fixture",
        "tag": "test",
        "dockerfile": None,
        "context": None,
    }],
    "tool_pins": real["tool_pins"],
    "determinism": real["determinism"],
}
with open(sys.argv[2], "w", encoding="utf-8") as f:
    json.dump(policy, f, indent=2)
    f.write("\n")
PY

expect_exit() {
  local expected="$1" label="$2"
  shift 2
  local actual=0
  bash "$GENERATE" "$@" >"$TEMP_ROOT/$label.out" 2>"$TEMP_ROOT/$label.err" || actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL: $label: expected exit $expected, got $actual" >&2
    cat "$TEMP_ROOT/$label.err" >&2
    exit 1
  fi
  echo "OK: $label"
}

assert_manifest() {
  local out="$1" expected_components="$2" preview="$3"
  python3 - "$out" "$FIXTURE/release/sbom-policy.json" "$expected_components" "$preview" <<'PY'
import hashlib, json, sys

out, policy_path, expected_components, preview = sys.argv[1:5]
manifest = json.load(open(out + "/release-manifest.json", encoding="utf-8"))
policy = json.load(open(policy_path, encoding="utf-8"))

def fail(msg):
    print("FAIL: " + msg, file=sys.stderr)
    sys.exit(1)

if manifest.get("preview", False) != (preview == "true"):
    fail("preview flag mismatch")
for field in ("schema_version", "commit", "version", "created_at", "generators",
              "components", "artifacts", "migration_manifest", "provenance"):
    if field not in manifest:
        fail("manifest missing field " + field)
if len(manifest["components"]) != int(expected_components):
    fail("expected %s components, got %d" % (expected_components, len(manifest["components"])))
if manifest["generators"]["syft"]["digest"] != policy["tool_pins"]["syft"]["digest"]:
    fail("generator syft digest is not the pinned policy digest")
if preview == "false" and len(manifest["provenance"]) == 0:
    fail("release manifest must declare provenance references")
for comp in manifest["components"]:
    path = out + "/" + comp["sbom_path"]
    digest = hashlib.sha256(open(path, "rb").read()).hexdigest()
    if digest != comp["sbom_sha256"]:
        fail("sbom digest mismatch for " + comp["id"])
    sbom = json.load(open(path, encoding="utf-8"))
    if sbom.get("bomFormat") != "CycloneDX":
        fail("sbom " + comp["id"] + " is not CycloneDX")
    if "timestamp" in sbom.get("metadata", {}):
        fail("sbom " + comp["id"] + " still carries a volatile metadata.timestamp")
    if len(sbom.get("components", [])) == 0:
        fail("sbom " + comp["id"] + " has no components")
for art in manifest["artifacts"]:
    if art["type"] == "container-image":
        if not art.get("digest", "").startswith("sha256:") or len(art["digest"]) != 71:
            fail("container artifact digest is not sha256 hex: " + art.get("digest", ""))
        tar = out + "/" + art["path"]
        if not hashlib.sha256(open(tar, "rb").read()).hexdigest() == art["sha256"]:
            fail("container tar sha256 mismatch for " + art["name"])
print("manifest assertions passed")
PY
}

# ------------------------------------------------------------ usage errors
rc=0
bash "$GENERATE" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || { echo "FAIL: missing -OutputDir must exit 2" >&2; exit 1; }
rc=0
bash "$GENERATE" -OutputDir "$TEMP_ROOT/x" -RepoRoot "$TEMP_ROOT/does-not-exist" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 1 ] || { echo "FAIL: missing RepoRoot must exit 1" >&2; exit 1; }
echo "OK: usage errors"

# ------------------------------------------------- preview generation (red first: missing script)
OUT1="$TEMP_ROOT/preview1"
expect_exit 0 "preview sbom generation" -RepoRoot "$FIXTURE" -Policy "$FIXTURE/release/sbom-policy.json" -OutputDir "$OUT1" -SkipImages
assert_manifest "$OUT1" 4 true

# ------------------------------------------------ determinism across runs
OUT2="$TEMP_ROOT/preview2"
expect_exit 0 "second preview run" -RepoRoot "$FIXTURE" -Policy "$FIXTURE/release/sbom-policy.json" -OutputDir "$OUT2" -SkipImages
for f in "$OUT1"/sboms/*.cdx.json; do
  base="$(basename "$f")"
  if ! cmp -s "$f" "$OUT2/sboms/$base"; then
    echo "FAIL: sbom $base is not deterministic across runs" >&2
    exit 1
  fi
done
echo "OK: sboms deterministic across runs"

# ------------------------------------------------- full mode with container
docker build -q -t omnicraft-sbom-fixture:test - <<'EOF' >/dev/null
FROM alpine:3.23
LABEL org.opencontainers.image.title="OmniCraft SBOM fixture"
LABEL org.opencontainers.image.revision="fixture-revision"
LABEL org.opencontainers.image.version="fixture"
EOF
OUT3="$TEMP_ROOT/full"
expect_exit 0 "full sbom generation with container" -RepoRoot "$FIXTURE" -Policy "$FIXTURE/release/sbom-policy.json" -OutputDir "$OUT3"
assert_manifest "$OUT3" 5 false
rc=0
bash "$VERIFY" -Manifest "$OUT3/release-manifest.json" -Policy "$FIXTURE/release/sbom-policy.json" -RepoRoot "$FIXTURE" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 0 ] || { echo "FAIL: verify-provenance must accept the generated full manifest (got $rc)" >&2; exit 1; }
echo "OK: full manifest accepted by verify-provenance"

# -------------------------------------------------- missing lockfile fails
rm "$FIXTURE/frontend/package-lock.json"
MISSING_LABEL="missing frontend lockfile rejected"
expect_exit 1 "$MISSING_LABEL" -RepoRoot "$FIXTURE" -Policy "$FIXTURE/release/sbom-policy.json" -OutputDir "$TEMP_ROOT/missing"
if ! grep -q "frontend-npm" "$TEMP_ROOT/$MISSING_LABEL.err"; then
  echo "FAIL: missing-lockfile error must name the component" >&2
  cat "$TEMP_ROOT/$MISSING_LABEL.err" >&2
  exit 1
fi
echo "OK: missing-lockfile rejection cites the component"

echo "All generate-sbom contract tests passed"
