#!/usr/bin/env bash
# Contract tests for scripts/release/verify-provenance.sh: release-manifest
# schema/digest binding, required ecosystem coverage, pinned generators,
# deterministic SBOM normalization, migration-manifest binding and provenance
# reference structure. Uses synthetic CycloneDX SBOM fixtures and the real
# repository policy.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VERIFY="$SCRIPT_DIR/verify-provenance.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
POLICY="$REPO_ROOT/release/sbom-policy.json"

if [ ! -f "$VERIFY" ]; then
  echo "verify-provenance.sh does not exist" >&2
  exit 1
fi

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/omnicraft-provenance.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

# Build a valid release fixture: synthetic CycloneDX SBOMs, migration manifest
# computed from the real repository and a manifest with correct digests.
make_fixture() {
  local dir="$1"
  python3 - "$dir" "$POLICY" "$REPO_ROOT/backend/migrations" <<'PY'
import hashlib, json, os, sys

out_dir, policy_path, migrations_dir = sys.argv[1:4]
policy = json.load(open(policy_path, encoding="utf-8"))
os.makedirs(out_dir + "/sboms", exist_ok=True)
os.makedirs(out_dir + "/artifacts", exist_ok=True)

def sbom(name):
    return {
        "bomFormat": "CycloneDX",
        "specVersion": "1.5",
        "version": 1,
        "metadata": {"component": {"type": "application", "name": name, "version": "1.0"}},
        "components": [{"type": "library", "name": name + "-pkg", "version": "1.0.0",
                        "purl": "pkg:generic/" + name + "-pkg@1.0.0"}],
    }

components = []
for comp in policy["components"]:
    sbom_path = "sboms/%s.cdx.json" % comp["id"]
    with open(out_dir + "/" + sbom_path, "w", encoding="utf-8") as f:
        json.dump(sbom(comp["id"]), f)
    components.append({
        "id": comp["id"],
        "ecosystem": comp["ecosystem"],
        "source": comp["source"],
        "sbom_path": sbom_path,
        "sbom_sha256": hashlib.sha256(open(out_dir + "/" + sbom_path, "rb").read()).hexdigest(),
    })

migration_entries = []
for name in sorted(os.listdir(migrations_dir)):
    if not name.endswith(".sql"):
        continue
    migration_entries.append({
        "file": name,
        "sha256": hashlib.sha256(open(os.path.join(migrations_dir, name), "rb").read()).hexdigest(),
    })
migration_manifest = {"schema_version": 1, "count": len(migration_entries), "migrations": migration_entries}
with open(out_dir + "/migration-manifest.json", "w", encoding="utf-8") as f:
    json.dump(migration_manifest, f, indent=2)
    f.write("\n")

artifacts = []
for comp in policy["components"]:
    if comp["ecosystem"] != "container":
        continue
    tar_path = "artifacts/%s.tar" % comp["id"]
    content = ("fixture container payload for %s\n" % comp["id"]).encode()
    with open(out_dir + "/" + tar_path, "wb") as f:
        f.write(content)
    tar_sha = hashlib.sha256(content).hexdigest()
    artifacts.append({
        "name": comp["id"],
        "type": "container-image",
        "path": tar_path,
        "sha256": tar_sha,
        "digest": "sha256:" + tar_sha,
    })

manifest = {
    "schema_version": "1.0",
    "preview": False,
    "commit": "1111111111111111111111111111111111111111",
    "version": "0.1.0-fixture",
    "created_at": "2026-08-06T00:00:00Z",
    "generators": {"syft": {
        "image": policy["tool_pins"]["syft"]["image"],
        "tag": policy["tool_pins"]["syft"]["tag"],
        "version": "1.19.0",
        "digest": policy["tool_pins"]["syft"]["digest"],
    }},
    "components": components,
    "artifacts": artifacts,
    "migration_manifest": {
        "path": "migration-manifest.json",
        "sha256": hashlib.sha256(open(out_dir + "/migration-manifest.json", "rb").read()).hexdigest(),
        "count": len(migration_entries),
    },
    "provenance": [{
        "type": "github-attestation",
        "scope": "release",
        "builder_id": "https://github.com/omnicraft/omnicraft/.github/workflows/sbom.yml@refs/tags/v0.1.0",
        "subject": "release-manifest.json",
        "verify_command": "gh attestation verify release-manifest.json --repo omnicraft/omnicraft",
    }],
}
with open(out_dir + "/release-manifest.json", "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")
PY
}

expect_verify() {
  local expected="$1" label="$2" manifest="$3"
  shift 3
  local actual=0
  bash "$VERIFY" -Manifest "$manifest" -Policy "$POLICY" -RepoRoot "$REPO_ROOT" "$@" \
    >"$TEMP_ROOT/$label.out" 2>"$TEMP_ROOT/$label.err" || actual=$?
  if [ "$actual" -ne "$expected" ]; then
    echo "FAIL: $label: expected exit $expected, got $actual" >&2
    cat "$TEMP_ROOT/$label.err" >&2
    exit 1
  fi
  echo "OK: $label"
}

# ------------------------------------------------------------ usage errors
rc=0
bash "$VERIFY" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 2 ] || { echo "FAIL: missing -Manifest must exit 2" >&2; exit 1; }
rc=0
bash "$VERIFY" -Manifest "$TEMP_ROOT/does-not-exist.json" >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 1 ] || { echo "FAIL: missing manifest file must exit 1" >&2; exit 1; }
echo "OK: usage errors"

# --------------------------------------------------------------- valid case
VALID="$TEMP_ROOT/valid"
make_fixture "$VALID"
expect_verify 0 "valid release manifest accepted" "$VALID/release-manifest.json"
[ -f "$VALID/provenance-verification.json" ] || { echo "FAIL: verification report missing" >&2; exit 1; }
echo "OK: verification report written"

# ------------------------------------------------------ tampered SBOM digest
CASE="$TEMP_ROOT/tampered-sbom"
make_fixture "$CASE"
printf '\n' >> "$CASE/sboms/backend-go.cdx.json"
expect_verify 1 "tampered sbom rejected" "$CASE/release-manifest.json"

# ---------------------------------------------------- missing component entry
CASE="$TEMP_ROOT/missing-component"
make_fixture "$CASE"
python3 - "$CASE/release-manifest.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
m["components"] = [c for c in m["components"] if c["id"] != "backend-go"]
json.dump(m, open(sys.argv[1], "w"), indent=2)
PY
expect_verify 1 "missing component rejected" "$CASE/release-manifest.json"

# --------------------------------------- policy/component set mismatch (extra)
CASE="$TEMP_ROOT/extra-component"
make_fixture "$CASE"
python3 - "$CASE/release-manifest.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
m["components"].append({"id": "unexpected-extra", "ecosystem": "go",
                        "source": "x", "sbom_path": "sboms/backend-go.cdx.json",
                        "sbom_sha256": m["components"][0]["sbom_sha256"]})
json.dump(m, open(sys.argv[1], "w"), indent=2)
PY
expect_verify 1 "component set differing from policy rejected" "$CASE/release-manifest.json"

# -------------------------------------------------------------- preview mode
CASE="$TEMP_ROOT/preview"
make_fixture "$CASE"
python3 - "$CASE/release-manifest.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
m["preview"] = True
m["components"] = [c for c in m["components"] if c["id"] not in ("backend-image", "frontend-image")]
m["artifacts"] = []
m["provenance"] = []
json.dump(m, open(sys.argv[1], "w"), indent=2)
PY
expect_verify 1 "preview manifest rejected for release verification" "$CASE/release-manifest.json"
expect_verify 0 "preview manifest accepted with -Preview" "$CASE/release-manifest.json" -Preview

# --------------------------------------------------------- unpinned generator
CASE="$TEMP_ROOT/unpinned"
make_fixture "$CASE"
python3 - "$CASE/release-manifest.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
m["generators"]["syft"]["digest"] = "sha256:" + "0" * 64
json.dump(m, open(sys.argv[1], "w"), indent=2)
PY
expect_verify 1 "unpinned generator rejected" "$CASE/release-manifest.json"

# ------------------------------------------------------ volatile timestamp
CASE="$TEMP_ROOT/timestamp"
make_fixture "$CASE"
python3 - "$CASE" <<'PY'
import hashlib, json, sys
base, rel = sys.argv[1], "sboms/backend-go.cdx.json"
path = base + "/" + rel
m = json.load(open(base + "/release-manifest.json", encoding="utf-8"))
sbom = json.load(open(path, encoding="utf-8"))
sbom["metadata"]["timestamp"] = "2026-08-06T00:00:00Z"
json.dump(sbom, open(path, "w"))
for c in m["components"]:
    if c["id"] == "backend-go":
        c["sbom_sha256"] = hashlib.sha256(open(path, "rb").read()).hexdigest()
json.dump(m, open(base + "/release-manifest.json", "w"), indent=2)
PY
expect_verify 1 "volatile sbom timestamp rejected" "$CASE/release-manifest.json"

# ------------------------------------------------------- invalid SBOM format
CASE="$TEMP_ROOT/not-cyclonedx"
make_fixture "$CASE"
python3 - "$CASE" <<'PY'
import hashlib, json, sys
base, rel = sys.argv[1], "sboms/backend-go.cdx.json"
path = base + "/" + rel
m = json.load(open(base + "/release-manifest.json", encoding="utf-8"))
sbom = json.load(open(path, encoding="utf-8"))
sbom["bomFormat"] = "SPDX"
json.dump(sbom, open(path, "w"))
for c in m["components"]:
    if c["id"] == "backend-go":
        c["sbom_sha256"] = hashlib.sha256(open(path, "rb").read()).hexdigest()
json.dump(m, open(base + "/release-manifest.json", "w"), indent=2)
PY
expect_verify 1 "non-CycloneDX sbom rejected" "$CASE/release-manifest.json"

# ----------------------------------------------------------- empty SBOM
CASE="$TEMP_ROOT/empty-sbom"
make_fixture "$CASE"
python3 - "$CASE" <<'PY'
import hashlib, json, sys
base, rel = sys.argv[1], "sboms/backend-go.cdx.json"
path = base + "/" + rel
m = json.load(open(base + "/release-manifest.json", encoding="utf-8"))
sbom = json.load(open(path, encoding="utf-8"))
sbom["components"] = []
json.dump(sbom, open(path, "w"))
for c in m["components"]:
    if c["id"] == "backend-go":
        c["sbom_sha256"] = hashlib.sha256(open(path, "rb").read()).hexdigest()
json.dump(m, open(base + "/release-manifest.json", "w"), indent=2)
PY
expect_verify 1 "empty sbom rejected" "$CASE/release-manifest.json"

# ------------------------------------------------------- tampered artifact
CASE="$TEMP_ROOT/tampered-artifact"
make_fixture "$CASE"
printf 'evil\n' >> "$CASE/artifacts/backend-image.tar"
expect_verify 1 "tampered container artifact rejected" "$CASE/release-manifest.json"

# ---------------------------------------------------- bad image digest shape
CASE="$TEMP_ROOT/bad-image-digest"
make_fixture "$CASE"
python3 - "$CASE/release-manifest.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
for a in m["artifacts"]:
    if a["type"] == "container-image":
        a["digest"] = "sha256:short"
json.dump(m, open(sys.argv[1], "w"), indent=2)
PY
expect_verify 1 "malformed image digest rejected" "$CASE/release-manifest.json"

# ------------------------------------------- migration manifest digest mismatch
CASE="$TEMP_ROOT/migration-tampered"
make_fixture "$CASE"
python3 - "$CASE" <<'PY'
import json, sys
base = sys.argv[1]
m = json.load(open(base + "/migration-manifest.json", encoding="utf-8"))
m["count"] = 0
json.dump(m, open(base + "/migration-manifest.json", "w"), indent=2)
PY
expect_verify 1 "tampered migration manifest rejected" "$CASE/release-manifest.json"

# ----------------------------------- migration entry not in repository
CASE="$TEMP_ROOT/migration-ghost"
make_fixture "$CASE"
python3 - "$CASE" <<'PY'
import hashlib, json, sys
base = sys.argv[1]
path = base + "/migration-manifest.json"
m = json.load(open(base + "/release-manifest.json", encoding="utf-8"))
mm = json.load(open(path, encoding="utf-8"))
mm["migrations"].append({"file": "999_ghost.sql", "sha256": "0" * 64})
json.dump(mm, open(path, "w"))
m["migration_manifest"]["sha256"] = hashlib.sha256(open(path, "rb").read()).hexdigest()
m["migration_manifest"]["count"] = len(mm["migrations"])
json.dump(m, open(base + "/release-manifest.json", "w"), indent=2)
PY
expect_verify 1 "migration entry absent from repository rejected" "$CASE/release-manifest.json"

# ------------------------------------------------------------ empty provenance
CASE="$TEMP_ROOT/no-provenance"
make_fixture "$CASE"
python3 - "$CASE/release-manifest.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
m["provenance"] = []
json.dump(m, open(sys.argv[1], "w"), indent=2)
PY
expect_verify 1 "missing provenance references rejected" "$CASE/release-manifest.json"

# ---------------------------------------------------- non-github builder_id
CASE="$TEMP_ROOT/bad-builder"
make_fixture "$CASE"
python3 - "$CASE/release-manifest.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
m["provenance"][0]["builder_id"] = "https://example.com/unknown"
json.dump(m, open(sys.argv[1], "w"), indent=2)
PY
expect_verify 1 "foreign provenance builder rejected" "$CASE/release-manifest.json"

echo "All verify-provenance contract tests passed"
