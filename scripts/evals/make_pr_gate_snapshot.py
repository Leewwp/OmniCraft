#!/usr/bin/env python3
"""Build the frozen PR-gate snapshot from a rag-eval generation run (SP-22 E3).

The gate itself (backend/cmd/rag-gate) must run with zero provider calls, zero
LLM cost and no database access, so both sides of every assertion are frozen
here at snapshot time:

- expected side: golden labels (relevant_content_ids) from PostgreSQL,
- actual side:   the recorded generation run's retrieved_ids + answer status.

Re-run this script only when the golden set or the retrieval snapshot is
intentionally refreshed; the snapshot's dataset_checksum pins which golden
fixture it was taken against.

Usage:
    python3 scripts/evals/make_pr_gate_snapshot.py \
        --runs artifacts/sp22-e1/runs-all2.jsonl \
        --out backend/testdata/rag_pr_gate_snapshot.jsonl
"""

from __future__ import annotations

import argparse
import json
import sys
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import evals_lib as lib

SNAPSHOT_VERSION = 1


def load_golden_labels() -> dict:
    """case_key -> {expected_ids, layer} straight from the golden table.

    v2 cases carry their expected retrieval set in expected_citations (content
    id + version); relevant_content_ids is the older v1 shape and is kept as a
    fallback so the generator works across both generations.
    """
    sql = (
        "SELECT COALESCE(json_agg(t), '[]'::json) FROM ("
        "SELECT case_key, relevant_content_ids, expected_citations, classification "
        "FROM eval_golden_cases WHERE is_active ORDER BY case_key) t"
    )
    out = {}
    for row in json.loads(lib._psql(sql).strip() or "[]"):
        rel = row.get("relevant_content_ids") or []
        if isinstance(rel, str):
            rel = json.loads(rel)
        cites = row.get("expected_citations") or []
        if isinstance(cites, str):
            cites = json.loads(cites)
        expected = [int(c["content_id"]) for c in cites if c.get("content_id")]
        if not expected:
            expected = [int(x) for x in rel]
        cls = row.get("classification") or {}
        if isinstance(cls, str):
            cls = json.loads(cls)
        out[row["case_key"]] = {
            "expected_ids": sorted(set(expected)),
            "split": cls.get("split", ""),
            "layer": cls.get("primary_layer", ""),
        }
    return out


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--runs", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    header, cases = lib.load_generation_rows(args.runs)
    labels = load_golden_labels()

    rows = []
    missing = []
    for case in cases:
        label = labels.get(case.case_key)
        if not label:
            missing.append(case.case_key)
            continue
        rows.append({
            "case_key": case.case_key,
            "split": label["split"],
            "layer": label["layer"],
            "expected_ids": label["expected_ids"],
            "retrieved_ids": sorted(case.retrieved_ids),
            "answer_status": case.status,
            "answer_kind": case.answer_kind,
        })

    with open(args.out, "w", encoding="utf-8") as fh:
        json.dump({
            "snapshot_version": SNAPSHOT_VERSION,
            "kind": "rag-pr-gate-snapshot",
            "dataset_checksum": header.get("dataset_checksum", ""),
            "source_label": header.get("label", ""),
            "switches": header.get("switches", {}),
            "runtime": header.get("runtime", {}),
            "cases": rows,
        }, fh, ensure_ascii=False, indent=1)

    print(f"wrote {len(rows)} cases to {args.out}")
    if missing:
        print(f"WARNING: {len(missing)} cases had no golden label: {missing[:5]}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
