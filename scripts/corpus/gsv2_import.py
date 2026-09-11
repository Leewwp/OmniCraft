#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Golden Set v2 importer — #291 step 5 freeze (annotation spec §13.3).

Imports the frozen v2 split JSONL into eval_golden_cases (migration 069)
with the explicit draft→069 field mapping and fail-closed cardinality
checks. The draft keeps acceptable/forbidden annotation at the TOP level
(corpus_item_key domain) while the runner reads only the 069 columns
(ParseAnswerRubric → AcceptableContentIDs / ForbiddenReasons in
content_id domain). A naive answer_rubric passthrough silently drops
both tiers — this importer is the only sanctioned path.

Mapping (spec §13.3, mirroring §3):
  acceptable_keys        → answer_rubric.acceptable_content_ids   [content_id]
  forbidden_reasons      → answer_rubric.forbidden_reasons        {content_id: reason}
  expected_citation_keys → expected_citations                     [{content_id, content_version}]
  forbidden_keys         → forbidden_content_ids                  [content_id]
  relevant_refs          → relevant_evidence                     (span payload as-is)
  viewer_context         → viewer_context                        (principal_key, no numeric ids)
  classification         → classification  (+ provenance.human_reviewed: true;
                           draft annotation_status=pending_review is asserted
                           as an import precondition then DROPPED — it has no
                           069 column)
  relevant_content_ids   → '[]' (v1 legacy column; the v2 expected tier lives
                           in expected_citations, the runner's fallback path
                           stays intentionally unused)

Fail-closed gates:
  G1  split file canonical sha16 == SPLIT_SHA pin (rev 3, 2026-09-05)
  G2  every corpus_item_key resolves in injection/mapping.jsonl
  G3  global cardinalities: acceptable entries == 40, forbidden reasons == 186
  G4  per-case post-import re-read: acceptable / forbidden-reason / span
      counts equal the draft; any mismatch aborts with the offending case

Usage:
  python3 gsv2_import.py verify      # mapping + G1..G3 only, no writes
  python3 gsv2_import.py import      # upsert + G4 verification (idempotent)
  python3 gsv2_import.py deactivate-legacy
                                      # is_active=false for schema_version<2
                                      # rows (v1 golden set superseded)

Environment:
  PSQL         default docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft -X -q
  GSPLIT       default <repo>/artifacts/corpus-v2/golden-set/v2-cases-split.jsonl
  MAPPING      default <repo>/artifacts/corpus-v2/injection/mapping.jsonl
"""
from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import corpus_lib as lib  # noqa: E402

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
GSPLIT = os.environ.get(
    "GSPLIT", os.path.join(REPO, "artifacts/corpus-v2/golden-set/v2-cases-split.jsonl")
)
MAPPING = os.environ.get(
    "MAPPING", os.path.join(REPO, "artifacts/corpus-v2/injection/mapping.jsonl")
)
PSQL = os.environ.get(
    "PSQL", "docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft -X -q"
)

# Frozen rev-3 split (2026-09-05). Supersedes f0764976f350135d (rev 2, 15/16)
# and 1b0cc79a8ed78695 (first cut, two-tier graph).
SPLIT_SHA = "f090172bbb24f9d7"
EXPECT_ACCEPTABLE = 40
EXPECT_FORBIDDEN_REASONS = 186


def psql(sql: str) -> str:
    proc = subprocess.run(
        PSQL.split(), input=sql.encode("utf-8"), capture_output=True
    )
    if proc.returncode != 0:
        raise RuntimeError(
            "psql failed: %s\nSQL: %s" % (proc.stderr.decode()[:400], sql[:200])
        )
    return proc.stdout.decode("utf-8")


def psql_scalar(sql: str):
    proc = subprocess.run(
        PSQL.split() + ["-t", "-A", "-c", sql], capture_output=True
    )
    if proc.returncode != 0:
        raise RuntimeError(
            "psql failed: %s\nSQL: %s" % (proc.stderr.decode()[:400], sql[:200])
        )
    out = proc.stdout.decode("utf-8").strip()
    return None if out == "" else out


def sha16(rows) -> str:
    s = "\n".join(json.dumps(r, ensure_ascii=False, sort_keys=True) for r in rows)
    return hashlib.sha256(s.encode("utf-8")).hexdigest()[:16]


def load_rows():
    rows = [json.loads(l) for l in open(GSPLIT, encoding="utf-8")]
    got = sha16(rows)
    if got != SPLIT_SHA:
        raise SystemExit(
            "G1 FAIL: split sha=%s want=%s — regenerate via gsv2_split.py "
            "and update the pin deliberately" % (got, SPLIT_SHA)
        )
    return rows


def load_mapping():
    keymap = {}
    for line in open(MAPPING, encoding="utf-8"):
        m = json.loads(line)
        keymap[m["corpus_item_key"]] = (m["content_id"], m["versions"])
    return keymap


def resolve(keymap, key, case_key, field):
    if key not in keymap:
        raise SystemExit(
            "G2 FAIL: case %s field %s references unknown corpus key %s"
            % (case_key, field, key)
        )
    return keymap[key]


def build_row(r, keymap):
    """Draft row → 069 column dict. Returns (columns, draft_counts)."""
    if r.get("annotation_status") != "pending_review":
        raise SystemExit(
            "precondition FAIL: case %s annotation_status=%r (want pending_review)"
            % (r["case_key"], r.get("annotation_status"))
        )
    refs = r["relevant_refs"]
    ref_versions = {s["corpus_item_key"]: s["content_version"] for s in refs}

    def cid(key, field):
        content_id, versions = resolve(keymap, key, r["case_key"], field)
        if key in ref_versions and ref_versions[key] != versions:
            raise SystemExit(
                "version drift: case %s key %s ref_version=%d mapping_versions=%d"
                % (r["case_key"], key, ref_versions[key], versions)
            )
        return content_id, versions

    citations = []
    for key in r["expected_citation_keys"]:
        content_id, versions = cid(key, "expected_citation_keys")
        citations.append({"content_id": content_id, "content_version": versions})
    forbidden_ids = [
        cid(key, "forbidden_keys")[0] for key in r["forbidden_keys"]
    ]
    acceptable_ids = [
        cid(key, "acceptable_keys")[0] for key in r["acceptable_keys"]
    ]
    reasons = {}
    for key, reason in (r.get("forbidden_reasons") or {}).items():
        reasons[str(cid(key, "forbidden_reasons")[0])] = reason

    rubric = dict(r["answer_rubric"])  # judge fields pass through verbatim
    if acceptable_ids:
        rubric["acceptable_content_ids"] = acceptable_ids
    if reasons:
        rubric["forbidden_reasons"] = reasons

    classification = dict(r["classification"])
    classification["provenance"] = dict(classification.get("provenance") or {})
    classification["provenance"]["human_reviewed"] = True

    cols = {
        "case_key": r["case_key"],
        "schema_version": r["schema_version"],
        "query": r["query"],
        "query_language": r["query_language"],
        "viewer_context": json.dumps(r["viewer_context"], ensure_ascii=False),
        "relevant_evidence": json.dumps(refs, ensure_ascii=False),
        "relevant_content_ids": "[]",
        "expected_citations": json.dumps(citations),
        "forbidden_content_ids": json.dumps(forbidden_ids),
        "answer_rubric": json.dumps(rubric, ensure_ascii=False),
        "classification": json.dumps(classification, ensure_ascii=False),
    }
    draft_counts = (
        len(acceptable_ids),
        len(reasons),
        len(refs),
        len(citations),
        len(forbidden_ids),
    )
    return cols, draft_counts


def jval(v):
    return lib.sql_quote(v)


def upsert(cols):
    sql = (
        "INSERT INTO eval_golden_cases "
        "(case_key, schema_version, query, query_language, viewer_context, "
        "relevant_evidence, relevant_content_ids, expected_citations, "
        "forbidden_content_ids, answer_rubric, classification) VALUES ("
        "%s, %d, %s, %s, %s::jsonb, %s::jsonb, %s::jsonb, %s::jsonb, "
        "%s::jsonb, %s::jsonb, %s::jsonb) "
        "ON CONFLICT (case_key) DO UPDATE SET "
        "schema_version=EXCLUDED.schema_version, query=EXCLUDED.query, "
        "query_language=EXCLUDED.query_language, "
        "viewer_context=EXCLUDED.viewer_context, "
        "relevant_evidence=EXCLUDED.relevant_evidence, "
        "relevant_content_ids=EXCLUDED.relevant_content_ids, "
        "expected_citations=EXCLUDED.expected_citations, "
        "forbidden_content_ids=EXCLUDED.forbidden_content_ids, "
        "answer_rubric=EXCLUDED.answer_rubric, "
        "classification=EXCLUDED.classification, is_active=true;"
        % (
            jval(cols["case_key"]),
            cols["schema_version"],
            jval(cols["query"]),
            jval(cols["query_language"]),
            jval(cols["viewer_context"]),
            jval(cols["relevant_evidence"]),
            jval(cols["relevant_content_ids"]),
            jval(cols["expected_citations"]),
            jval(cols["forbidden_content_ids"]),
            jval(cols["answer_rubric"]),
            jval(cols["classification"]),
        )
    )
    psql(sql)


def verify_against_db(rows, keymap):
    """G4: re-read every imported case and compare tier/span cardinalities."""
    failures = []
    for r in rows:
        cols, draft = build_row(r, keymap)
        out = psql_scalar(
            "SELECT answer_rubric::text FROM eval_golden_cases "
            "WHERE case_key = %s AND is_active" % jval(r["case_key"])
        )
        if out is None:
            failures.append((r["case_key"], "row missing/inactive"))
            continue
        rubric = json.loads(out)
        acc = len(rubric.get("acceptable_content_ids") or [])
        fr = len(rubric.get("forbidden_reasons") or {})
        n_spans = int(
            psql_scalar(
                "SELECT jsonb_array_length(relevant_evidence) "
                "FROM eval_golden_cases WHERE case_key = %s" % jval(r["case_key"])
            )
        )
        n_cit = int(
            psql_scalar(
                "SELECT jsonb_array_length(expected_citations) "
                "FROM eval_golden_cases WHERE case_key = %s" % jval(r["case_key"])
            )
        )
        n_forb = int(
            psql_scalar(
                "SELECT jsonb_array_length(forbidden_content_ids) "
                "FROM eval_golden_cases WHERE case_key = %s" % jval(r["case_key"])
            )
        )
        got = (acc, fr, n_spans, n_cit, n_forb)
        if got != draft:
            failures.append((r["case_key"], "db=%s draft=%s" % (got, draft)))
    if failures:
        for case, why in failures:
            print("G4 FAIL: %s — %s" % (case, why), file=sys.stderr)
        raise SystemExit(1)
    total = int(psql_scalar(
        "SELECT count(*) FROM eval_golden_cases WHERE is_active AND "
        "schema_version = 2"
    ))
    if total != len(rows):
        raise SystemExit(
            "G4 FAIL: active v2 rows=%d want=%d" % (total, len(rows))
        )
    legacy = int(psql_scalar(
        "SELECT count(*) FROM eval_golden_cases WHERE is_active AND "
        "schema_version < 2"
    ))
    if legacy:
        print(
            "NOTE: %d active legacy (schema_version<2) rows remain — the "
            "v1 golden set is superseded by this freeze; deactivate them "
            "so eval runs read exactly the 196 frozen cases" % legacy
        )
    print("G4 OK: %d cases, per-case cardinalities match the draft" % len(rows))


def main():
    cmd = sys.argv[1] if len(sys.argv) > 1 else "verify"
    rows = load_rows()
    keymap = load_mapping()
    acc_total = sum(len(r.get("acceptable_keys") or []) for r in rows)
    fr_total = sum(len(r.get("forbidden_reasons") or {}) for r in rows)
    print("G1 OK: split sha16=%s (%d cases)" % (SPLIT_SHA, len(rows)))
    if acc_total != EXPECT_ACCEPTABLE:
        raise SystemExit(
            "G3 FAIL: acceptable entries=%d want=%d" % (acc_total, EXPECT_ACCEPTABLE)
        )
    if fr_total != EXPECT_FORBIDDEN_REASONS:
        raise SystemExit(
            "G3 FAIL: forbidden reasons=%d want=%d"
            % (fr_total, EXPECT_FORBIDDEN_REASONS)
        )
    print("G3 OK: acceptable=%d forbidden_reasons=%d" % (acc_total, fr_total))

    if cmd == "verify":
        for r in rows:
            build_row(r, keymap)  # exercises G2 + preconditions, no writes
        print("G2 OK: all corpus keys resolve; preconditions hold (dry run)")
        return
    if cmd == "deactivate-legacy":
        n = psql_scalar(
            "WITH x AS (UPDATE eval_golden_cases SET is_active=false WHERE "
            "is_active AND schema_version < 2 RETURNING 1) "
            "SELECT count(*) FROM x"
        )
        print("deactivated %s legacy rows" % n)
        return
    if cmd != "import":
        raise SystemExit("usage: gsv2_import.py [verify|import]")

    for r in rows:
        cols, _ = build_row(r, keymap)
        upsert(cols)
    print("imported %d cases (idempotent upsert)" % len(rows))
    verify_against_db(rows, keymap)
    acc_db = int(psql_scalar(
        "SELECT coalesce(sum(jsonb_array_length("
        "answer_rubric->'acceptable_content_ids')),0) FROM eval_golden_cases "
        "WHERE is_active"
    ))
    fr_db = int(psql_scalar(
        "SELECT coalesce(sum((SELECT count(*) FROM jsonb_object_keys("
        "answer_rubric->'forbidden_reasons'))),0) FROM eval_golden_cases "
        "WHERE is_active AND answer_rubric ? 'forbidden_reasons'"
    ))
    print("db totals: acceptable=%d forbidden_reasons=%d" % (acc_db, fr_db))


if __name__ == "__main__":
    main()
