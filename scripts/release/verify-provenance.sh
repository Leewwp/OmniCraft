#!/usr/bin/env bash
# =============================================================================
# OmniCraft provenance verifier: re-verifies a release manifest against its
# schema, recomputes every SBOM/artifact/migration-manifest digest from the
# referenced files (manifest paths are relative to the manifest directory, so
# downloaded-artifact verification works unchanged), enforces policy component
# coverage and pinned generators, rejects volatile SBOM fields, and checks
# provenance reference identity. With -ImageDaemon the container images are
# cross-checked against the manifest commit via OCI labels.
#
# Usage:
#   bash scripts/release/verify-provenance.sh -Manifest <path>
#       [-Policy <path>] [-RepoRoot <dir>] [-ReportDir <dir>]
#       [-Preview] [-ImageDaemon]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
POLICY="$REPO_ROOT/release/sbom-policy.json"
MANIFEST=""
REPORT_DIR=""
PREVIEW=0
IMAGE_DAEMON=0

while [ $# -gt 0 ]; do
  case "$1" in
    -Manifest)
      [ $# -ge 2 ] || { echo "missing value for -Manifest" >&2; exit 2; }
      MANIFEST="$2"; shift 2 ;;
    -Policy)
      [ $# -ge 2 ] || { echo "missing value for -Policy" >&2; exit 2; }
      POLICY="$2"; shift 2 ;;
    -RepoRoot)
      [ $# -ge 2 ] || { echo "missing value for -RepoRoot" >&2; exit 2; }
      REPO_ROOT="$2"; shift 2 ;;
    -ReportDir)
      [ $# -ge 2 ] || { echo "missing value for -ReportDir" >&2; exit 2; }
      REPORT_DIR="$2"; shift 2 ;;
    -Preview)
      PREVIEW=1; shift ;;
    -ImageDaemon)
      IMAGE_DAEMON=1; shift ;;
    *)
      echo "unknown argument: $1" >&2
      echo "usage: verify-provenance.sh -Manifest <path> [-Policy <path>] [-RepoRoot <dir>] [-ReportDir <dir>] [-Preview] [-ImageDaemon]" >&2
      exit 2 ;;
  esac
done

if [ -z "$MANIFEST" ]; then
  echo "usage: verify-provenance.sh -Manifest <path> [-Policy <path>] [-RepoRoot <dir>] [-ReportDir <dir>] [-Preview] [-ImageDaemon]" >&2
  exit 2
fi

[ -f "$MANIFEST" ] || { echo "manifest not found: $MANIFEST" >&2; exit 1; }
[ -f "$POLICY" ] || { echo "policy not found: $POLICY" >&2; exit 1; }
[ -d "$REPO_ROOT" ] || { echo "repo root not found: $REPO_ROOT" >&2; exit 1; }

MANIFEST="$(cd "$(dirname "$MANIFEST")" && pwd)/$(basename "$MANIFEST")"
if [ -z "$REPORT_DIR" ]; then
  REPORT_DIR="$(dirname "$MANIFEST")"
fi
mkdir -p "$REPORT_DIR"
REPORT_DIR="$(cd "$REPORT_DIR" && pwd)"

export OMNICRAFT_PROV_MANIFEST="$MANIFEST"
export OMNICRAFT_PROV_POLICY="$POLICY"
export OMNICRAFT_PROV_REPO="$REPO_ROOT"
export OMNICRAFT_PROV_REPORT="$REPORT_DIR"
export OMNICRAFT_PROV_PREVIEW="$PREVIEW"
export OMNICRAFT_PROV_IMAGE_DAEMON="$IMAGE_DAEMON"
export OMNICRAFT_PROV_SCHEMA="$SCRIPT_DIR/../../release/release-manifest.schema.json"

python3 - <<'PY'
import hashlib
import json
import os
import re
import subprocess
import sys
import datetime

manifest_path = os.environ["OMNICRAFT_PROV_MANIFEST"]
policy_path = os.environ["OMNICRAFT_PROV_POLICY"]
repo_root = os.environ["OMNICRAFT_PROV_REPO"]
report_dir = os.environ["OMNICRAFT_PROV_REPORT"]
preview_mode = os.environ["OMNICRAFT_PROV_PREVIEW"] == "1"
image_daemon = os.environ["OMNICRAFT_PROV_IMAGE_DAEMON"] == "1"

manifest_dir = os.path.dirname(manifest_path)
checks = []
warnings = []


def check(name, ok, detail=""):
    checks.append({"name": name, "ok": bool(ok), "detail": detail})


def warn(name, detail):
    warnings.append({"name": name, "detail": detail})


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def rel(path):
    return os.path.join(manifest_dir, path)


def load_json(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def valid_types(value, types):
    if isinstance(types, str):
        types = [types]
    for t in types:
        if t == "string" and isinstance(value, str):
            return True
        if t == "array" and isinstance(value, list):
            return True
        if t == "object" and isinstance(value, dict):
            return True
        if t == "boolean" and isinstance(value, bool):
            return True
        if t == "integer" and isinstance(value, int) and not isinstance(value, bool):
            return True
    return False


def validate_schema(node, schema, path):
    if "type" in schema and not valid_types(node, schema["type"]):
        return f"{path}: expected type {schema['type']}"
    if schema.get("const") is not None and node != schema["const"]:
        return f"{path}: expected const {schema['const']}"
    if "enum" in schema and node not in schema["enum"]:
        return f"{path}: unexpected enum value {node!r}"
    if "pattern" in schema and isinstance(node, str) and not re.fullmatch(schema["pattern"], node):
        return f"{path}: pattern mismatch"
    if "minLength" in schema and isinstance(node, str) and len(node) < schema["minLength"]:
        return f"{path}: too short"
    if "minimum" in schema and isinstance(node, int) and node < schema["minimum"]:
        return f"{path}: below minimum"
    if isinstance(node, dict):
        for key, subschema in (schema.get("properties") or {}).items():
            if key in node:
                error = validate_schema(node[key], subschema, f"{path}.{key}")
                if error:
                    return error
    if isinstance(node, list) and "items" in schema:
        for i, item in enumerate(node):
            error = validate_schema(item, schema["items"], f"{path}[{i}]")
            if error:
                return error
    return None


# ---------------------------------------------------------------- parse
try:
    with open(manifest_path, encoding="utf-8") as f:
        manifest = json.load(f)
except (OSError, ValueError) as e:
    print(f"verify-provenance: manifest is not valid JSON: {e}", file=sys.stderr)
    sys.exit(1)
try:
    policy = load_json(policy_path)
except (OSError, ValueError) as e:
    print(f"verify-provenance: policy is not valid JSON: {e}", file=sys.stderr)
    sys.exit(1)

schema_path = os.environ["OMNICRAFT_PROV_SCHEMA"]
try:
    schema = load_json(schema_path)
except (OSError, ValueError) as e:
    print(f"verify-provenance: schema is not valid JSON: {e}", file=sys.stderr)
    sys.exit(1)

# ---------------------------------------------------------------- checks
missing = [f for f in schema.get("required", []) if f not in manifest]
check("schema.required_fields", not missing, f"missing: {', '.join(missing)}" if missing else "")
schema_error = validate_schema(manifest, schema, "manifest")
check("schema.structure", schema_error is None, schema_error or "")

expected_components = policy.get("components", [])
expected_by_id = {c["id"]: c for c in expected_components}
actual_by_id = {c["id"]: c for c in manifest.get("components", [])}

if preview_mode:
    policy_lockfile = [c["id"] for c in expected_components if c["ecosystem"] != "container"]
    mismatch = sorted(set(policy_lockfile) ^ set(actual_by_id))
    check("policy.component_coverage", not mismatch,
          f"preview components must equal policy lockfile set, diff: {mismatch}" if mismatch else "")
else:
    mismatch = sorted(set(expected_by_id) ^ set(actual_by_id))
    check("policy.component_coverage", not mismatch,
          f"manifest components must equal policy components, diff: {mismatch}" if mismatch else "")
    ecosystems = {c["ecosystem"] for c in manifest.get("components", [])}
    required = set(policy.get("required_ecosystems", []))
    check("policy.required_ecosystems", required <= ecosystems,
          f"missing ecosystems: {sorted(required - ecosystems)}" if required - ecosystems else "")

sbom_format_ok = True
sbom_digest_ok = True
sbom_deterministic_ok = True
sbom_populated_ok = True
for cid, comp in sorted(actual_by_id.items()):
    sbom_path = rel(comp["sbom_path"])
    if not os.path.isfile(sbom_path):
        sbom_digest_ok = False
        continue
    if sha256_file(sbom_path) != comp["sbom_sha256"]:
        sbom_digest_ok = False
        continue
    try:
        sbom = load_json(sbom_path)
    except ValueError:
        sbom_format_ok = False
        continue
    if sbom.get("bomFormat") != "CycloneDX":
        sbom_format_ok = False
        continue
    if "timestamp" in sbom.get("metadata", {}) or "serialNumber" in sbom:
        sbom_deterministic_ok = False
        continue
    if not sbom.get("components"):
        sbom_populated_ok = False
check("sbom.digests", sbom_digest_ok, "all referenced SBOM files must exist and match their sha256")
check("sbom.format", sbom_format_ok, "every SBOM must be CycloneDX")
check("sbom.determinism", sbom_deterministic_ok,
      "volatile fields metadata.timestamp/serialNumber must be normalized away")
check("sbom.populated", sbom_populated_ok, "every SBOM must contain components")

artifact_ok = True
image_digest_shape_ok = True
for artifact in manifest.get("artifacts", []):
    path = rel(artifact["path"])
    if not os.path.isfile(path) or sha256_file(path) != artifact["sha256"]:
        artifact_ok = False
    if artifact["type"] == "container-image":
        if not re.fullmatch(r"sha256:[0-9a-f]{64}", artifact.get("digest", "")):
            image_digest_shape_ok = False
check("artifacts.digests", artifact_ok, "every artifact must exist and match its sha256")
check("artifacts.image_digest", image_digest_shape_ok, "container digests must be sha256:64-hex")

mm = manifest.get("migration_manifest", {})
mm_path = rel(mm.get("path", ""))
mm_ok = os.path.isfile(mm_path) and sha256_file(mm_path) == mm.get("sha256")
migration_set = []
if mm_ok:
    try:
        migration_manifest = load_json(mm_path)
        migration_set = [(m["file"], m["sha256"]) for m in migration_manifest.get("migrations", [])]
        if len(migration_set) != mm.get("count"):
            mm_ok = False
    except (ValueError, KeyError, TypeError):
        mm_ok = False
check("migration_manifest.digest", mm_ok, "migration manifest must exist and match its sha256")
repo_migrations = sorted(os.listdir(os.path.join(repo_root, "backend", "migrations")))
repo_migration_set = [
    (name, sha256_file(os.path.join(repo_root, "backend", "migrations", name)))
    for name in repo_migrations if name.endswith(".sql")
]
check("migration_manifest.repository",
      sorted(migration_set) == sorted(repo_migration_set),
      "migration manifest entries must match backend/migrations exactly")

generators = manifest.get("generators", {})
syft_pin = policy.get("tool_pins", {}).get("syft", {})
gen_ok = (generators.get("syft", {}).get("digest") == syft_pin.get("digest"))
check("generators.pinned", gen_ok, "syft generator digest must equal the policy tool pin")
check("generators.version", bool(generators.get("syft", {}).get("version")), "generator version must be present")

provenance = manifest.get("provenance", [])
if preview_mode or manifest.get("preview"):
    prov_ok = True
    detail = ""
else:
    if not provenance:
        prov_ok = False
        detail = "release manifest must declare provenance references"
    else:
        prov_ok = True
        detail = ""
        for ref in provenance:
            if ref.get("type") != "github-attestation":
                prov_ok = False
                detail = "provenance type must be github-attestation"
                break
            if not re.fullmatch(
                r"https://github\.com/omnicraft/omnicraft/\.github/workflows/sbom\.yml@refs/.+",
                ref.get("builder_id", "")):
                prov_ok = False
                detail = f"foreign builder_id: {ref.get('builder_id')}"
                break
            if ref.get("scope") not in ("preview", "release") or not ref.get("subject") or not ref.get("verify_command"):
                prov_ok = False
                detail = "provenance reference must declare scope, subject and verify_command"
                break
check("provenance.references", prov_ok, detail or "provenance references structurally valid")

preview_consistent = True
if manifest.get("preview"):
    if not preview_mode:
        preview_consistent = False
if not manifest.get("preview") and not preview_mode:
    container_ids = [c["id"] for c in expected_components if c["ecosystem"] == "container"]
    if not all(cid in actual_by_id for cid in container_ids):
        preview_consistent = False
check("manifest.preview_consistency", preview_consistent,
      "preview flags and component sets must agree")

# ------------------------------------------------- image label binding
if image_daemon:
    label_ok = True
    label_detail = ""
    for artifact in manifest.get("artifacts", []):
        if artifact["type"] != "container-image" or not artifact.get("image") or not artifact.get("tag"):
            continue
        ref = f'{artifact["image"]}:{artifact["tag"]}'
        result = subprocess.run(
            ["docker", "image", "inspect", "--format", "{{index .Config.Labels \"org.opencontainers.image.revision\"}}", ref],
            capture_output=True, text=True, check=False)
        if result.returncode != 0:
            label_ok = False
            label_detail = f"image {ref} not present in daemon"
            break
        revision = result.stdout.strip()
        if revision != manifest.get("commit"):
            label_ok = False
            label_detail = f"image {ref} labels revision {revision}, manifest commit {manifest.get('commit')}"
            break
    check("artifacts.image_commit_binding", label_ok, label_detail)
else:
    warn("artifacts.image_commit_binding",
         "-ImageDaemon not used; OCI label binding checked in CI where images are built")

failed = [c for c in checks if not c["ok"]]
verified_at = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
report = {
    "manifest": os.path.basename(manifest_path),
    "manifest_sha256": sha256_file(manifest_path),
    "verified_at": verified_at,
    "tool": "scripts/release/verify-provenance.sh",
    "preview": manifest.get("preview", False),
    "checks": checks,
    "warnings": warnings,
    "ok": not failed,
}
report_path = os.path.join(report_dir, "provenance-verification.json")
with open(report_path, "w", encoding="utf-8") as f:
    json.dump(report, f, indent=2)
    f.write("\n")

for c in checks:
    mark = "ok " if c["ok"] else "FAIL"
    print(f"  [{mark}] {c['name']} {c['detail']}".rstrip())
if failed:
    print(f"verify-provenance: {len(failed)} check(s) failed", file=sys.stderr)
    sys.exit(1)
print(f"verify-provenance: all checks passed ({report_path})")
PY
