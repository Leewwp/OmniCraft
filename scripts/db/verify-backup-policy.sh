#!/usr/bin/env bash
# =============================================================================
# OmniCraft backup policy verifier
# =============================================================================
# Validates release/backup-policy.json and release/recovery-objectives.json
# against their schema contracts and writes machine-readable evidence.
#
# Usage:
#   bash scripts/db/verify-backup-policy.sh -Policy release/backup-policy.json \
#       -Objectives release/recovery-objectives.json -ReportDir artifacts/ops-02/policy
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

POLICY=""
OBJECTIVES=""
REPORT_DIR=""

while [ $# -gt 0 ]; do
  case "$1" in
    -Policy) POLICY="$2"; shift 2 ;;
    -Objectives) OBJECTIVES="$2"; shift 2 ;;
    -ReportDir) REPORT_DIR="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

[ -n "$POLICY" ] || { echo "missing -Policy" >&2; exit 2; }
[ -n "$OBJECTIVES" ] || { echo "missing -Objectives" >&2; exit 2; }
[ -n "$REPORT_DIR" ] || { echo "missing -ReportDir" >&2; exit 2; }

python3 - "$POLICY" "$OBJECTIVES" "$REPORT_DIR" <<'PY'
import json
import os
import sys

policy_path, objectives_path, report_dir = sys.argv[1], sys.argv[2], sys.argv[3]

errors = []


def load(path, label):
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except Exception as exc:  # noqa: BLE001 - contract verifier reports any parse error
        errors.append(f"{label}: cannot parse: {exc}")
        return None


policy = load(policy_path, "backup-policy")
objectives = load(objectives_path, "recovery-objectives")

if policy is not None:
    if policy.get("schema_version", 0) < 1:
        errors.append("backup-policy: schema_version must be >= 1")
    if policy.get("state") != "active":
        errors.append("backup-policy: state must be active")

    postgres_full = policy.get("postgres_full") or {}
    if postgres_full.get("frequency") != "daily":
        errors.append("backup-policy: postgres_full.frequency must be daily")
    if postgres_full.get("pre_migration") is not True:
        errors.append("backup-policy: postgres_full.pre_migration must be true")
    if postgres_full.get("format") != "custom":
        errors.append("backup-policy: postgres_full.format must be custom (no plain dumps)")
    if postgres_full.get("checksum_manifest") is not True:
        errors.append("backup-policy: postgres_full.checksum_manifest must be true")
    if postgres_full.get("migration_manifest") is not True:
        errors.append("backup-policy: postgres_full.migration_manifest must be true")

    local_retention = policy.get("local_retention") or {}
    if local_retention.get("copies") != 7:
        errors.append("backup-policy: local_retention.copies must be 7")

    off_host = policy.get("off_host") or {}
    if off_host.get("enabled") is not True:
        errors.append("backup-policy: off_host.enabled must be true")
    if not off_host.get("encryption"):
        errors.append("backup-policy: off_host.encryption must be declared (transit and at-rest)")
    if off_host.get("immutable_or_versioned") is not True:
        errors.append("backup-policy: off_host.immutable_or_versioned must be true")
    if off_host.get("retention_days") != 30:
        errors.append("backup-policy: off_host.retention_days must be 30")
    if off_host.get("verify_after_upload") is not True:
        errors.append("backup-policy: off_host.verify_after_upload must be true")

    restore_drill = policy.get("restore_drill") or {}
    if restore_drill.get("cadence") != "monthly":
        errors.append("backup-policy: restore_drill.cadence must be monthly")
    if restore_drill.get("max_age_days") != 30:
        errors.append("backup-policy: restore_drill.max_age_days must be 30 (drill within 30 days of every schema change)")
    if restore_drill.get("new_database_only") is not True:
        errors.append("backup-policy: restore_drill.new_database_only must be true")

    classification = policy.get("storage_classification") or []
    classified = {entry.get("system") for entry in classification if isinstance(entry, dict)}
    for system in ("postgresql", "oss", "redis"):
        if system not in classified:
            errors.append(f"backup-policy: storage_classification must classify {system}")

    restore_order = policy.get("restore_order") or []
    if restore_order[:1] != ["postgresql"]:
        errors.append("backup-policy: restore_order must start with postgresql (source of truth first)")
    if len(set(restore_order)) != len(restore_order) or set(restore_order) != {"postgresql", "oss", "redis"}:
        errors.append("backup-policy: restore_order must list postgresql, oss and redis exactly once")

if objectives is not None:
    if objectives.get("schema_version", 0) < 1:
        errors.append("recovery-objectives: schema_version must be >= 1")
    state = objectives.get("state")
    if state not in ("baseline_only", "approved"):
        errors.append("recovery-objectives: state must be baseline_only or approved")

    measured = objectives.get("measured")
    if not isinstance(measured, dict):
        errors.append("recovery-objectives: measured measurements must be present")
    else:
        for key in ("postgres_rpo", "postgres_rto", "object_restore_rto", "service_rpo", "service_rto", "measured_at", "last_drill"):
            if key not in measured:
                errors.append(f"recovery-objectives: measured.{key} is missing")

    if state == "approved":
        targets = objectives.get("approved_targets")
        approval = objectives.get("approval") or {}
        if not isinstance(targets, dict) or targets.get("postgres_rpo") is None or targets.get("postgres_rto") is None:
            errors.append("recovery-objectives: approved state requires numeric approved postgres RPO/RTO targets")
        if not approval.get("ref") or not approval.get("approver") or not approval.get("approved_at"):
            errors.append("recovery-objectives: approved state requires a commit-bound approval ref, approver and date")

os.makedirs(report_dir, exist_ok=True)
summary = {
    "policy_path": policy_path,
    "objectives_path": objectives_path,
    "valid": not errors,
    "errors": errors,
}
report = os.path.join(report_dir, "backup-policy-validation.json")
with open(report, "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2)
    f.write("\n")

if errors:
    for error in errors:
        print(f"VIOLATION: {error}", file=sys.stderr)
    sys.exit(1)
print("backup policy and recovery objectives validated")
PY
