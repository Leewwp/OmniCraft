#!/usr/bin/env bash
# =============================================================================
# OmniCraft SBOM generator: deterministic CycloneDX SBOMs for the Go, npm and
# Rust lockfiles plus the container images, bound into a machine-readable
# release manifest with artifact/image digests, migration-manifest digest,
# pinned generator identity and provenance references.
#
# Determinism: volatile fields (metadata.timestamp, serialNumber) are removed
# from the generated SBOMs; package identity is never edited. Runs the pinned
# syft image (digest in release/sbom-policy.json) exactly like CI.
#
# Usage:
#   bash scripts/release/generate-sbom.sh -OutputDir <dir>
#       [-RepoRoot <dir>] [-Policy <path>] [-Version <str>]
#       [-Commit <sha>] [-SkipImages]
#
# -SkipImages produces an unsigned preview manifest (lockfile ecosystems only)
# for pull-request builds; release manifests always include container SBOMs.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
POLICY="$REPO_ROOT/release/sbom-policy.json"
OUTPUT_DIR=""
VERSION=""
COMMIT=""
SKIP_IMAGES=0

while [ $# -gt 0 ]; do
  case "$1" in
    -OutputDir)
      [ $# -ge 2 ] || { echo "missing value for -OutputDir" >&2; exit 2; }
      OUTPUT_DIR="$2"; shift 2 ;;
    -RepoRoot)
      [ $# -ge 2 ] || { echo "missing value for -RepoRoot" >&2; exit 2; }
      REPO_ROOT="$2"; shift 2 ;;
    -Policy)
      [ $# -ge 2 ] || { echo "missing value for -Policy" >&2; exit 2; }
      POLICY="$2"; shift 2 ;;
    -Version)
      [ $# -ge 2 ] || { echo "missing value for -Version" >&2; exit 2; }
      VERSION="$2"; shift 2 ;;
    -Commit)
      [ $# -ge 2 ] || { echo "missing value for -Commit" >&2; exit 2; }
      COMMIT="$2"; shift 2 ;;
    -SkipImages)
      SKIP_IMAGES=1; shift ;;
    *)
      echo "unknown argument: $1" >&2
      echo "usage: generate-sbom.sh -OutputDir <dir> [-RepoRoot <dir>] [-Policy <path>] [-Version <str>] [-Commit <sha>] [-SkipImages]" >&2
      exit 2 ;;
  esac
done

if [ -z "$OUTPUT_DIR" ]; then
  echo "usage: generate-sbom.sh -OutputDir <dir> [-RepoRoot <dir>] [-Policy <path>] [-Version <str>] [-Commit <sha>] [-SkipImages]" >&2
  exit 2
fi

[ -d "$REPO_ROOT" ] || { echo "repo root not found: $REPO_ROOT" >&2; exit 1; }
[ -f "$POLICY" ] || { echo "policy not found: $POLICY" >&2; exit 1; }
[ -d "$REPO_ROOT/backend/migrations" ] || { echo "migrations dir not found: $REPO_ROOT/backend/migrations" >&2; exit 1; }

mkdir -p "$OUTPUT_DIR/sboms" "$OUTPUT_DIR/container" "$OUTPUT_DIR/stage"
OUT="$(cd "$OUTPUT_DIR" && pwd)"

if [ -z "$COMMIT" ]; then
  COMMIT="$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo "0000000000000000000000000000000000000000")"
fi
if [ -z "$VERSION" ]; then
  VERSION="$(git -C "$REPO_ROOT" describe --tags --always --dirty=-dev 2>/dev/null || echo "0.0.0-dev")"
fi

export OMNICRAFT_SBOM_OUT="$OUT"
export OMNICRAFT_SBOM_REPO="$REPO_ROOT"
export OMNICRAFT_SBOM_POLICY="$POLICY"
export OMNICRAFT_SBOM_COMMIT="$COMMIT"
export OMNICRAFT_SBOM_VERSION="$VERSION"
export OMNICRAFT_SBOM_SKIP_IMAGES="$SKIP_IMAGES"

python3 - <<'PY'
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tarfile
import datetime

out = os.environ["OMNICRAFT_SBOM_OUT"]
repo_root = os.environ["OMNICRAFT_SBOM_REPO"]
policy_path = os.environ["OMNICRAFT_SBOM_POLICY"]
commit = os.environ["OMNICRAFT_SBOM_COMMIT"]
version = os.environ["OMNICRAFT_SBOM_VERSION"]
skip_images = os.environ["OMNICRAFT_SBOM_SKIP_IMAGES"] == "1"

stage_dir = os.path.join(out, "stage")
sboms_dir = os.path.join(out, "sboms")
container_dir = os.path.join(out, "container")


def fail(message):
    print(f"generate-sbom: {message}", file=sys.stderr)
    sys.exit(1)


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


try:
    with open(policy_path, encoding="utf-8") as f:
        policy = json.load(f)
except (OSError, ValueError) as e:
    fail(f"policy is not valid JSON: {e}")

try:
    tool = dict(policy["tool_pins"]["syft"])
except (KeyError, TypeError):
    fail("policy must declare tool_pins.syft")

if "digest" not in tool or not tool["digest"].startswith("sha256:"):
    fail("policy tool_pins.syft.digest must be a pinned sha256 image digest")

syft_image = f'{tool["image"]}@{tool["digest"]}'
started_at = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def syft_version():
    result = subprocess.run(
        ["docker", "run", "--rm", syft_image, "version"],
        capture_output=True, text=True, check=False,
    )
    for line in result.stdout.splitlines():
        if line.strip().startswith("Version:"):
            return line.split(":", 1)[1].strip()
    return tool.get("tag", "")


def run_syft(source, out_name):
    out_rel = os.path.join("sboms", out_name)
    cmd = [
        "docker", "run", "--rm",
        "-v", f"{out}:/work:rw",
        syft_image,
        "scan", source,
        "-o", f"cyclonedx-json=/work/sboms/{out_name}",
    ]
    result = subprocess.run(cmd, capture_output=True, text=True, check=False)
    if result.returncode != 0:
        detail = (result.stderr or result.stdout).strip().splitlines()
        fail(f"syft failed for {out_name}: {' | '.join(detail[-3:])}")
    path = os.path.join(sboms_dir, out_name)
    try:
        with open(path, encoding="utf-8") as f:
            sbom = json.load(f)
    except (OSError, ValueError) as e:
        fail(f"syft output for {out_name} is not valid CycloneDX JSON: {e}")
    if sbom.get("bomFormat") != "CycloneDX":
        fail(f"syft output for {out_name} is not CycloneDX")
    normalized_fields = policy.get("determinism", {}).get("normalized_fields", [])
    sbom.get("metadata", {}).pop("timestamp", None)
    sbom.pop("serialNumber", None)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(sbom, f, indent=2)
        f.write("\n")
    return {
        "id": out_name.replace(".cdx.json", ""),
        "path": out_rel,
        "sha256": sha256_file(path),
        "component_count": len(sbom.get("components", [])),
        "normalized_fields": normalized_fields,
    }


def ensure_image(comp):
    image = comp["image"]
    tag = comp["tag"]
    ref = f"{image}:{tag}"
    result = subprocess.run(["docker", "image", "inspect", ref],
                            capture_output=True, text=True, check=False)
    if result.returncode == 0:
        return
    dockerfile = comp.get("dockerfile")
    context = comp.get("context")
    if not dockerfile or not context:
        fail(f"container image {ref} is not available and policy provides no dockerfile/context to build it")
    ctx = os.path.join(repo_root, context)
    df = os.path.join(ctx, dockerfile)
    if not os.path.isfile(df):
        fail(f"dockerfile not found: {df}")
    build = subprocess.run(
        ["docker", "build", "-q", "-t", ref,
         "--build-arg", f"VERSION={version}",
         "--build-arg", f"COMMIT={commit}",
         "-f", df, ctx],
        capture_output=True, text=True, check=False,
    )
    if build.returncode != 0:
        detail = (build.stderr or build.stdout).strip().splitlines()
        fail(f"docker build failed for {ref}: {' | '.join(detail[-3:])}")


def container_sbom(comp):
    ref = f'{comp["image"]}:{comp["tag"]}'
    cid = comp["id"]
    tar_path = os.path.join(container_dir, f"{cid}.tar")
    save = subprocess.run(["docker", "save", "-o", tar_path, ref],
                          capture_output=True, text=True, check=False)
    if save.returncode != 0:
        detail = (save.stderr or save.stdout).strip().splitlines()
        fail(f"docker save failed for {ref}: {' | '.join(detail[-3:])}")
    try:
        with tarfile.open(tar_path, "r") as t:
            manifest_blob = t.extractfile("manifest.json").read()
    except (KeyError, OSError) as e:
        fail(f"could not read docker-archive manifest from {ref}: {e}")
    image_digest = "sha256:" + hashlib.sha256(manifest_blob).hexdigest()
    sbom = run_syft(f"docker-archive:/work/container/{cid}.tar", f"{cid}.cdx.json")
    artifact = {
        "name": cid,
        "type": "container-image",
        "path": os.path.join("container", f"{cid}.tar"),
        "sha256": sha256_file(tar_path),
        "size": os.path.getsize(tar_path),
        "digest": image_digest,
        "image": comp["image"],
        "tag": comp["tag"],
    }
    return artifact, sbom


components = []
artifacts = []
images = []
expected = policy.get("components", [])
if not expected:
    fail("policy must declare components")

for comp in expected:
    ecosystem = comp["ecosystem"]
    cid = comp["id"]
    if ecosystem == "container":
        if skip_images:
            continue
        ensure_image(comp)
        artifact, sbom = container_sbom(comp)
        artifacts.append(artifact)
        images.append(artifact)
        components.append({
            "id": cid,
            "ecosystem": ecosystem,
            "source": comp["source"],
            "sbom_path": sbom["path"],
            "sbom_sha256": sbom["sha256"],
        })
        continue
    files = comp.get("files") or []
    missing = [f for f in files if not os.path.isfile(os.path.join(repo_root, f))]
    if missing:
        fail(f"missing lockfile for component {cid}: {', '.join(missing)}")
    stage_comp = os.path.join(stage_dir, cid)
    os.makedirs(stage_comp, exist_ok=True)
    for f in files:
        shutil.copy2(os.path.join(repo_root, f), os.path.join(stage_comp, os.path.basename(f)))
    sbom = run_syft(f"dir:/work/stage/{cid}", f"{cid}.cdx.json")
    components.append({
        "id": cid,
        "ecosystem": ecosystem,
        "source": comp["source"],
        "sbom_path": sbom["path"],
        "sbom_sha256": sbom["sha256"],
    })

migration_entries = []
migrations_dir = os.path.join(repo_root, "backend", "migrations")
for name in sorted(os.listdir(migrations_dir)):
    if not name.endswith(".sql"):
        continue
    migration_entries.append({
        "file": name,
        "sha256": sha256_file(os.path.join(migrations_dir, name)),
    })
if not migration_entries:
    fail("no migration files found in backend/migrations")
migration_manifest = {
    "schema_version": 1,
    "count": len(migration_entries),
    "migrations": migration_entries,
}
migration_path = os.path.join(out, "migration-manifest.json")
with open(migration_path, "w", encoding="utf-8") as f:
    json.dump(migration_manifest, f, indent=2)
    f.write("\n")

if skip_images:
    provenance = []
else:
    provenance = [{
        "type": "github-attestation",
        "scope": "release",
        "builder_id": f"https://github.com/omnicraft/omnicraft/.github/workflows/sbom.yml@refs/tags/v{version}",
        "subject": "release-manifest.json",
        "verify_command": "gh attestation verify release-manifest.json --repo omnicraft/omnicraft",
    }]

manifest = {
    "schema_version": "1.0",
    "preview": skip_images,
    "commit": commit,
    "version": version,
    "created_at": started_at,
    "generators": {
        "syft": {
            "image": tool["image"],
            "tag": tool["tag"],
            "version": syft_version(),
            "digest": tool["digest"],
        }
    },
    "components": components,
    "artifacts": artifacts,
    "migration_manifest": {
        "path": "migration-manifest.json",
        "sha256": sha256_file(migration_path),
        "count": len(migration_entries),
    },
    "provenance": provenance,
}
manifest_path = os.path.join(out, "release-manifest.json")
with open(manifest_path, "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")

report = {
    "commit": commit,
    "version": version,
    "preview": skip_images,
    "started_at": started_at,
    "finished_at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "generator": {
        "image": tool["image"],
        "tag": tool["tag"],
        "version": manifest["generators"]["syft"]["version"],
        "digest": tool["digest"],
    },
    "sboms": [{
        "id": c["id"],
        "path": c["sbom_path"],
        "sha256": c["sbom_sha256"],
    } for c in components],
    "images": [{
        "id": i["name"],
        "image": i["image"],
        "tag": i["tag"],
        "digest": i["digest"],
        "tar_sha256": i["sha256"],
        "tar_size": i["size"],
    } for i in images],
    "migration_count": len(migration_entries),
    "manifest_path": "release-manifest.json",
    "manifest_sha256": sha256_file(manifest_path),
}
with open(os.path.join(out, "sbom-generation.json"), "w", encoding="utf-8") as f:
    json.dump(report, f, indent=2)
    f.write("\n")

print(f"generate-sbom: {len(components)} components, {len(artifacts)} artifacts, "
      f"manifest {manifest_path} ({report['manifest_sha256'][:12]}...)")
PY
