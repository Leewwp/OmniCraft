#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Assemble Golden Set v2 (196 cases) from the frozen selection + authored
literals (#291 step 4, contract docs/working/2026-09-04-golden-set-v2-annotation-spec.md).

Inputs:
- artifacts/corpus-v2/golden-set/v2-selection.json (gsv2_targets.py, seeded)
- scripts/corpus/gsv2_cases.py (hand-authored SD/BE/HN/NA content + tiers)
- v1 draft + migration-map-v2.jsonl (keep/slot dispositions)
- DB (docker psql): authoritative body text + current chunk snapshots for spans

Outputs (all under artifacts/corpus-v2/golden-set/):
- v2-cases.jsonl            196 rows, draft keys (freeze maps key->content_id)
- v2-validation-report.json every automated gate V1..V11
- materials-v3.md           review sheets (rendered separately, see render_review_materials.py)

Draft row shape mirrors the 069 physical mapping (contract §3):
case_key, schema_version=2, query, query_language, viewer_context{principal_key},
relevant_refs[{corpus_item_key, content_id, content_version, source_start,
source_end, chunk_key, chunking_version, reason}],
expected_citation_keys, acceptable_keys, forbidden_keys, forbidden_reasons,
answer_rubric{judge_criteria, deterministic_assertions, must_not_claim,
ideal_answer}, classification{primary_layer, query_form, ip_scope(bool),
ip_name, language, temperature_band, corpus_visibility, query_register,
content_category, no_answer_strategy?, ascii_colon_subset?, provenance}.
split is intentionally absent (group-aware split is step 4b, contract §7).
"""
from __future__ import annotations

import hashlib
import json
import os
import re
import subprocess
import sys
from collections import Counter

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import corpus_lib as lib  # noqa: E402
import gsv2_cases as authored  # noqa: E402

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
BASE = os.path.join(REPO, "artifacts", "corpus-v2")
GS = os.path.join(BASE, "golden-set")
PSQL = ["docker", "compose", "exec", "-T", "postgres", "psql", "-U", "omnicraft", "-d", "omnicraft"]
GENERATOR = "zcode gsv2: selection(gsv2_targets r2) + hand-authored literals + per-fact BE spans (rev 2026-09-05)"
GENERATED_AT = "2026-09-05T12:00:00+08:00"  # fixed for deterministic regeneration (bumped at the 2026-09-05 per-fact span rev)
LAYER_ORDER = ["known_item_exact", "semantic_discovery", "body_evidence",
               "hard_neighbor", "no_answer", "visibility"]
INPUT_MASKS = {
    "semantic_discovery": "title withheld from query authoring (body packet only)",
    "body_evidence": "title withheld from question authoring (chunk texts only)",
    "hard_neighbor": "query authored from title-free body excerpt; tiers judged "
                     "against four-config retrieval union with titles",
    "known_item_exact": "none (query IS the title)",
    "no_answer": "none; zero-correspondence verified by live retrieval evidence",
    "visibility": "none (query IS the restricted doc title)",
}


def psql_rows(sql: str):
    res = subprocess.run(PSQL + ["-t", "-A", "-c", sql], capture_output=True, text=True, cwd=REPO)
    if res.returncode != 0:
        raise RuntimeError(res.stderr[:500])
    return [l for l in res.stdout.split("\n") if l != ""]


def load():
    idx = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(f"{BASE}/index.jsonl", encoding="utf-8"))}
    mapping = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(f"{BASE}/injection/mapping.jsonl", encoding="utf-8"))}
    v1 = {c["case_key"]: c for c in (json.loads(l) for l in open(f"{GS}/golden-cases-draft.jsonl", encoding="utf-8"))}
    mmap = [json.loads(l) for l in open(f"{GS}/migration-map-v2.jsonl", encoding="utf-8")]
    sel = json.load(open(f"{GS}/v2-selection.json", encoding="utf-8"))
    ev = json.load(open(f"{GS}/v2-retrieval-evidence/union.json", encoding="utf-8"))
    return idx, mapping, v1, mmap, sel, ev


def fetch_db(targets, mapping):
    """Bodies (corpus source == DB coordinate space, chunk-slice verified) +
    current chunk rows for the span targets."""
    import glob
    bodies_by_key = {}
    for path in sorted(glob.glob(os.path.join(BASE, "corpus-batch-*.jsonl"))):
        for line in open(path, encoding="utf-8"):
            r = json.loads(line)
            bodies_by_key[r["corpus_item_key"]] = r["body_md"]
    bodies = {mapping[k]["content_id"]: bodies_by_key[k] for k in targets if k in bodies_by_key}
    missing = [k for k in targets if k not in bodies_by_key]
    if missing:
        raise RuntimeError("bodies missing for %s" % missing[:5])
    ids = sorted({mapping[k]["content_id"] for k in targets})
    id_list = ",".join(str(i) for i in ids)
    chunks = {}
    for row in psql_rows(
        "SELECT rc.content_id || chr(31) || rc.source_start || chr(31) || rc.source_end || chr(31) || "
        "rc.content_version || chr(31) || rc.chunking_version || chr(31) || btrim(rc.chunk_key::text) || chr(31) || "
        "replace(rc.text, chr(10), '\\n') FROM rag_chunks rc "
        "JOIN index_projection_status ips ON ips.content_id = rc.content_id AND ips.index_version = rc.index_version "
        "WHERE rc.content_id IN (%s) AND ips.is_current = TRUE AND ips.state = 'ready' "
        "ORDER BY rc.content_id, rc.source_start" % id_list):
        cid, ss, se, cv, chunkv, ckey, text = row.split(chr(31), 6)
        chunks.setdefault(int(cid), []).append({
            "source_start": int(ss), "source_end": int(se), "content_version": int(cv),
            "chunking_version": int(chunkv), "chunk_key": ckey,
            "text": text.replace("\\n", "\n")})
    return bodies, chunks


def opening_span(body: str):
    """Deterministic opening-paragraph anchor for ke/hn evidence spans."""
    head = body.split("\n\n", 1)[0].strip("\n")
    if len(head) > 200:
        head = head[:200]
    return head


def span_of(body: str, anchor: str):
    pos = body.find(anchor)
    if pos < 0:
        return None, 0
    return pos, body.count(anchor)


def chunk_covering(chunks, start, end):
    for c in chunks:
        if c["source_start"] <= start and end <= c["source_end"]:
            return c
    for c in chunks:  # fall back to any overlap
        if c["source_start"] < end and start < c["source_end"]:
            return c
    return None


def make_ref(key, mapping, bodies, chunks, start, end, reason):
    cid = mapping[key]["content_id"]
    c = chunk_covering(chunks.get(cid, []), start, end)
    return {
        "corpus_item_key": key, "content_id": cid,
        "content_version": (c or {}).get("content_version", 1),
        "source_start": start, "source_end": end,
        "chunk_key": (c or {}).get("chunk_key", ""),
        "chunking_version": (c or {}).get("chunking_version", 0),
        "reason": reason,
    }


def provenance(layer):
    return {"generator": GENERATOR, "input_mask": INPUT_MASKS[layer],
            "generated_at": GENERATED_AT, "human_reviewed": False,
            "evidence": "v2-retrieval-evidence/union.json (four A-04 configs)"}


def never_retrieved(ev, sel):
    """Evidence-backed cases whose expected target never entered any config's
    corpus-filtered Top-10 (sd_diag + hn entries of the union evidence).
    hn targets live in v2-selection.json (the evidence file omits them); the
    sd evidence keys carry the sd-diag- prefix and are normalized to case keys."""
    hn_target = {e["v2_key"]: e["target"] for e in sel["hn"]}
    out = []
    for v2, e in ev.items():
        if e["kind"] not in ("sd_diag", "hn"):
            continue
        target = e["target"] if e["kind"] == "sd_diag" else hn_target.get(v2)
        if not target:
            continue
        hit = any(h["key"] == target for c in
                  ("off-off", "exp-on", "rerank-on", "exp-rerank") for h in e[c]["top"])
        if not hit:
            case_key = v2[len("sd-diag-"):] if v2.startswith("sd-diag-") else v2
            out.append((case_key, target))
    return out


NEVER_NOTE = ("目标未进四配置任一 Top-10（深候选 top-30 亦无，投影与嵌入健全）：故意保留的极限难例，"
              "用于检索区分度诊断；A-04 分层报告需单列，不计入层均分的常规解读")


def build():
    idx, mapping, v1, mmap, sel, ev = load()
    ke_swap = {e["v2_key"]: e for e in sel["ke"]}
    sd_swap = {e["v2_key"]: e for e in sel["sd"]}
    hn_sel = {e["v2_key"]: e for e in sel["hn"]}
    be_sel = {e["v2_key"]: e for e in sel["be"]}

    # evidence targets needing spans: ke/sd targets + be + hn
    span_targets = set()
    for r in mmap:
        if r["v2_layer"] == "known_item_exact":
            span_targets.add(ke_swap[r["v2_case_key"]]["target"] if r["disposition"] == "target_swap"
                             else v1[r["old_case_key"]]["expected_citation_keys"][0])
        elif r["v2_layer"] == "semantic_discovery":
            span_targets.add(sd_swap[r["v2_case_key"]]["target"] if r["disposition"] == "target_swap+rewrite"
                             else v1[r["old_case_key"]]["expected_citation_keys"][0])
    span_targets.update(e["target"] for e in sel["be"])
    span_targets.update(e["target"] for e in sel["hn"])
    bodies, chunks = fetch_db(span_targets, mapping)

    def body_of(key):
        return bodies[mapping[key]["content_id"]]

    rows = []

    # ---------------- ke ----------------
    for r in mmap:
        if r["v2_layer"] != "known_item_exact":
            continue
        v2 = r["v2_case_key"]
        old = v1[r["old_case_key"]]
        if r["disposition"] == "target_swap":
            e = ke_swap[v2]
            tgt = e["target"]
            ip_scoped = e["ip_scoped"]
            disposition = "target_swap"
        else:
            tgt = old["expected_citation_keys"][0]
            ip_scoped = old["classification"]["ip_scope"] == "ip_localized"
            disposition = "keep"
        # v1 keep queries inherited a .strip("《") artifact on embedded-bracket
        # titles (e.g. 《西游记》文物展…); every ke query is rebuilt from the
        # current canonical title, stripping only a true outer 《》 pair.
        prefix = "在%s内：" % idx[tgt]["ip"] if ip_scoped else ""
        query = prefix + lib.normalized_title(idx[tgt]["title"])
        body = body_of(tgt)
        anchor = opening_span(body)
        start, _ = span_of(body, anchor)
        rows.append({
            "case_key": v2, "schema_version": 2, "query": query,
            "query_language": idx[tgt]["language"],
            "viewer_context": {"principal_key": "anon"},
            "relevant_refs": [make_ref(tgt, mapping, bodies, chunks, start, start + len(anchor),
                                       "opening paragraph anchor (title-match diagnostic)")],
            "expected_citation_keys": [tgt], "acceptable_keys": [], "forbidden_keys": [],
            "forbidden_reasons": {},
            "answer_rubric": {"judge_criteria": ["回答引用且仅锚定查询所问的那篇作品"],
                              "deterministic_assertions": [], "must_not_claim": [],
                              "ideal_answer": "the exact work %s" % idx[tgt]["title"]},
            "classification": {
                "primary_layer": "known_item_exact", "query_form": "exact",
                "ip_scope": ip_scoped,
                "ip_name": idx[tgt]["ip"], "language": idx[tgt]["language"],
                "temperature_band": idx[tgt]["temperature"],
                "corpus_visibility": idx[tgt]["visibility"],
                "query_register": "neutral",
                "content_category": idx[tgt]["category"],
                "ascii_colon_subset": ":" in query,
                "provenance": provenance("known_item_exact"),
                "migration": {"old_case_key": r["old_case_key"], "disposition": disposition},
            },
            "annotation_status": "pending_review",
        })

    # ---------------- sd ----------------
    for r in mmap:
        if r["v2_layer"] != "semantic_discovery":
            continue
        v2 = r["v2_case_key"]
        a = authored.SD[v2]
        j = authored.SD_JUDGE[v2]
        tgt = (sd_swap[v2]["target"] if r["disposition"] == "target_swap+rewrite"
               else v1[r["old_case_key"]]["expected_citation_keys"][0])
        body = body_of(tgt)
        start, occ = span_of(body, a["anchor"])
        assert start >= 0, v2
        rows.append({
            "case_key": v2, "schema_version": 2, "query": a["query"],
            "query_language": idx[tgt]["language"],
            "viewer_context": {"principal_key": "anon"},
            "relevant_refs": [make_ref(tgt, mapping, bodies, chunks, start, start + len(a["anchor"]),
                                       "passage matching the query premise (hidden-title authoring)")],
            "expected_citation_keys": [tgt],
            "acceptable_keys": list(j["acceptable"]),
            "forbidden_keys": [k for k, _ in j["forbidden"]],
            "forbidden_reasons": {k: reason for k, reason in j["forbidden"]},
            "answer_rubric": {"judge_criteria": ["回答推荐 expected 且不把 forbidden 当作满足查询前提的作品"],
                              "deterministic_assertions": [], "must_not_claim": [],
                              "ideal_answer": a["evidence_note"]},
            "classification": {
                "primary_layer": "semantic_discovery", "query_form": "semantic",
                "ip_scope": False, "ip_name": idx[tgt]["ip"],
                "language": idx[tgt]["language"], "temperature_band": idx[tgt]["temperature"],
                "corpus_visibility": idx[tgt]["visibility"],
                "query_register": a["register"], "content_category": idx[tgt]["category"],
                "provenance": provenance("semantic_discovery"),
                "migration": {"old_case_key": r["old_case_key"], "disposition": r["disposition"]},
            },
            "annotation_status": "pending_review",
        })

    # ---------------- be ----------------
    for e in sel["be"]:
        v2 = e["v2_key"]
        a = authored.BE[v2]
        tgt = e["target"]
        body = body_of(tgt)
        start, occ = span_of(body, a["anchor"])
        assert start >= 0, v2
        anchor_text = body[start:start + len(a["anchor"])]
        refs = [make_ref(tgt, mapping, bodies, chunks, start, start + len(a["anchor"]),
                         "body passage that states the answer")]
        # Per-fact evidence spans (2026-09-05 re-audit rev): every deterministic
        # assertion must be supported by at least one span (gate V13, contract
        # §3 "正文中真正支持答案的区间"). Facts already inside the reviewed
        # anchor span need no extra ref; every other fact gets its own span,
        # located via an authored disambiguating context (short or
        # multi-occurrence facts — never a bare first-occurrence find) or via
        # the fact string itself when it is unique in the body.
        contexts = a.get("fact_contexts", {})
        by_span = {}
        for fact in a["answer_keys"]:
            if fact in anchor_text:
                continue
            ctx = contexts.get(fact, fact)
            s2, c2 = span_of(body, ctx)
            assert s2 >= 0, (v2, fact, "fact context not found in body")
            assert c2 == 1, (v2, fact, "fact context not unique — author fact_contexts entry")
            by_span.setdefault((s2, s2 + len(ctx)), []).append(fact)
        for (s2, e2), facts in sorted(by_span.items()):
            refs.append(make_ref(tgt, mapping, bodies, chunks, s2, e2,
                                 "body passage stating the answer fact%s: %s" %
                                 ("s" if len(facts) > 1 else "", "；".join(facts))))
        rows.append({
            "case_key": v2, "schema_version": 2, "query": a["question"],
            "query_language": idx[tgt]["language"],
            "viewer_context": {"principal_key": "anon"},
            "relevant_refs": refs,
            "expected_citation_keys": [tgt],
            "acceptable_keys": list(a["acceptable"]),
            "forbidden_keys": [k for k, _ in a["forbidden"]],
            "forbidden_reasons": {k: reason for k, reason in a["forbidden"]},
            "answer_rubric": {
                "judge_criteria": ["回答必须取自正文事实并引用 expected"],
                "deterministic_assertions": list(a["answer_keys"]),
                "must_not_claim": [], "ideal_answer": "；".join(a["answer_keys"])},
            "classification": {
                "primary_layer": "body_evidence", "query_form": "body",
                "ip_scope": False, "ip_name": idx[tgt]["ip"],
                "language": idx[tgt]["language"], "temperature_band": idx[tgt]["temperature"],
                "corpus_visibility": idx[tgt]["visibility"],
                "query_register": a["register"], "content_category": idx[tgt]["category"],
                "provenance": provenance("body_evidence"),
                "migration": {"old_case_key": "-", "disposition": "new"},
            },
            "annotation_status": "pending_review",
        })

    # ---------------- hn ----------------
    for e in sel["hn"]:
        v2 = e["v2_key"]
        a = authored.HN[v2]
        j = authored.HN_JUDGE[v2]
        tgt = e["target"]
        body = body_of(tgt)
        anchor = opening_span(body)
        start, _ = span_of(body, anchor)
        rows.append({
            "case_key": v2, "schema_version": 2, "query": a["query"],
            "query_language": idx[tgt]["language"],
            "viewer_context": {"principal_key": "anon"},
            "relevant_refs": [make_ref(tgt, mapping, bodies, chunks, start, start + len(anchor),
                                       "target opening (theme anchor for the neighbor query)")],
            "expected_citation_keys": [tgt],
            "acceptable_keys": list(j["acceptable"]),
            "forbidden_keys": [k for k, _ in j["forbidden"]],
            "forbidden_reasons": {k: reason for k, reason in j["forbidden"]},
            "answer_rubric": {"judge_criteria": ["回答区分 expected 与仅词面相近的 forbidden 并逐条给出理由"],
                              "deterministic_assertions": [], "must_not_claim": [],
                              "ideal_answer": j["evidence_note"]},
            "classification": {
                "primary_layer": "hard_neighbor", "query_form": "semantic",
                "ip_scope": False, "ip_name": idx[tgt]["ip"],
                "language": idx[tgt]["language"], "temperature_band": idx[tgt]["temperature"],
                "corpus_visibility": idx[tgt]["visibility"],
                "query_register": a["register"], "content_category": idx[tgt]["category"],
                "provenance": provenance("hard_neighbor"),
                "migration": {"old_case_key": "-", "disposition": "new"},
            },
            "annotation_status": "pending_review",
        })

    # ---------------- na ----------------
    keep_map = {}
    for r in mmap:
        if r["disposition"] == "keep+strategy":
            keep_map[r["v2_case_key"]] = r["old_case_key"]
    for v2, old in sorted(keep_map.items()):
        oc = v1[old]
        draw = authored.NA_KEEP_DRAW[v2]
        strategy = "strict_not_found" if "strict" in next(
            r["note"] for r in mmap if r["v2_case_key"] == v2) else "related_recommendation_allowed"
        strategy = getattr(authored, "NA_STRATEGY_OVERRIDE", {}).get(v2, strategy)
        rows.append({
            "case_key": v2, "schema_version": 2, "query": oc["query"],
            "query_language": oc["query_language"],
            "viewer_context": {"principal_key": "anon"},
            "relevant_refs": [], "expected_citation_keys": [], "acceptable_keys": [],
            "forbidden_keys": [k for k, _ in draw],
            "forbidden_reasons": {k: reason for k, reason in draw},
            "answer_rubric": {
                "judge_criteria": (["明确说明库内无对应作品；禁止编造"] if strategy == "strict_not_found"
                                   else ["明确「没有完全对应」；只允许推荐真实存在的相似作品，且不得谎称为精确对应"]),
                "deterministic_assertions": [], "must_not_claim": [],
                "ideal_answer": "honest refusal/recommendation, no fabricated claims"},
            "classification": {
                "primary_layer": "no_answer", "query_form": "semantic",
                "ip_scope": False, "ip_name": None,
                "language": oc["query_language"], "temperature_band": "cold",
                "corpus_visibility": None,
                "query_register": "colloquial", "content_category": None,
                "no_answer_strategy": strategy,
                "provenance": provenance("no_answer"),
                "migration": {"old_case_key": old, "disposition": "keep+strategy"},
            },
            "annotation_status": "pending_review",
        })
    for v2 in sorted(authored.NA_NEW):
        a = authored.NA_NEW[v2]
        evd = authored.NA_NEW_EVIDENCE[v2]
        forbidden = []
        for k, reason in getattr(authored, "NA_EXTRA_FORBIDDEN", {}).get(v2, []):
            forbidden.append((k, reason))
        if evd["closest"]:
            forbidden.append((evd["closest"], "最接近的语料条目（相关可推荐），不得被当作精确对应"))
        seen_f = set()
        forbidden = [(k, r) for k, r in forbidden if not (k in seen_f or seen_f.add(k))]
        rows.append({
            "case_key": v2, "schema_version": 2, "query": a["query"],
            "query_language": a["language"],
            "viewer_context": {"principal_key": "anon"},
            "relevant_refs": [], "expected_citation_keys": [], "acceptable_keys": [],
            "forbidden_keys": [k for k, _ in forbidden],
            "forbidden_reasons": {k: reason for k, reason in forbidden},
            "answer_rubric": {
                "judge_criteria": ["明确「没有完全对应」；只允许推荐真实存在的相似作品，且不得谎称为精确对应"],
                "deterministic_assertions": [], "must_not_claim": [a["plausible_theme_note"] + "（语料中不存在）"],
                "ideal_answer": "honest related recommendation grounded in real corpus items only"},
            "classification": {
                "primary_layer": "no_answer", "query_form": "semantic",
                "ip_scope": False, "ip_name": a["ip"],
                "language": a["language"], "temperature_band": "cold",
                "corpus_visibility": None,
                "query_register": "colloquial", "content_category": None,
                "no_answer_strategy": a["strategy"],
                "provenance": provenance("no_answer"),
                "migration": {"old_case_key": "-", "disposition": "new"},
                "na_evidence": evd,
            },
            "annotation_status": "pending_review",
        })

    # ---------------- vi ----------------
    n = 0
    for r in mmap:
        if r["v2_layer"] != "visibility":
            continue
        n += 1
        v2 = "vi-%04d" % n
        old = v1[r["old_case_key"]]
        tgt = old["forbidden_keys"][0]
        rows.append({
            "case_key": v2, "schema_version": 2, "query": old["query"],
            "query_language": old["query_language"],
            "viewer_context": {"principal_key": "anon"},
            "relevant_refs": [], "expected_citation_keys": [], "acceptable_keys": [],
            "forbidden_keys": [tgt],
            "forbidden_reasons": {tgt: "受限文档（%s）：四泄漏面（检索/引用/回答提及/工具事件）任一出现即失败" % idx[tgt]["visibility"]},
            "answer_rubric": {
                "judge_criteria": ["anon 与 fixture:viewer-anon 双身份下，受限作品的标题/摘要/ID 不得出现在结果、引用、回答正文与工具步骤事件"],
                "deterministic_assertions": [], "must_not_claim": [],
                "ideal_answer": "results restricted to public works only; zero leak on four surfaces"},
            "classification": {
                "primary_layer": "visibility", "query_form": "exact",
                "ip_scope": False, "ip_name": idx[tgt]["ip"],
                "language": idx[tgt]["language"], "temperature_band": idx[tgt]["temperature"],
                "corpus_visibility": idx[tgt]["visibility"],
                "query_register": "neutral", "content_category": idx[tgt]["category"],
                "provenance": provenance("visibility"),
                "migration": {"old_case_key": r["old_case_key"], "disposition": "keep+principal"},
            },
            "annotation_status": "pending_review",
        })

    never = dict(never_retrieved(ev, sel))
    for r in rows:
        v2 = r["case_key"]
        if v2 in never and r["classification"]["primary_layer"] in ("semantic_discovery", "hard_neighbor"):
            r["classification"]["provenance"] = dict(r["classification"]["provenance"],
                                                     extreme_hard_case=True, diagnostic_note=NEVER_NOTE)

    order = {l: i for i, l in enumerate(LAYER_ORDER)}
    rows.sort(key=lambda r: (order[r["classification"]["primary_layer"]], r["case_key"]))
    return rows, idx, mapping, v1, mmap, ev, sel, bodies


def validate(rows, idx, mapping, v1, mmap, ev, sel, bodies):
    import importlib.util
    spec = importlib.util.spec_from_file_location("gsa", os.path.join(REPO, "scripts", "corpus", "golden_set_audit.py"))
    gsa = importlib.util.module_from_spec(spec)
    import io, contextlib
    with contextlib.redirect_stdout(io.StringIO()):
        spec.loader.exec_module(gsa)

    report = {}
    problems = []
    by_key = {r["case_key"]: r for r in rows}
    # V1 quotas + keys
    layers = Counter(r["classification"]["primary_layer"] for r in rows)
    quota = {"known_item_exact": 64, "semantic_discovery": 48, "body_evidence": 24,
             "hard_neighbor": 16, "no_answer": 20, "visibility": 24}
    if dict(layers) != quota:
        problems.append(("V1", "quota mismatch", dict(layers)))
    if len(set(by_key)) != len(rows):
        problems.append(("V1", "duplicate case_key"))
    pat = re.compile(r"^(ke|sd|be|hn|na|vi)-\d{4}$")
    if any(not pat.match(k) for k in by_key):
        problems.append(("V1", "case_key namespace violation"))
    # V2 migration map bidirectional
    mmap_expect = {r["v2_case_key"]: r["old_case_key"] for r in mmap
                   if r["old_case_key"] != "-" and not r["disposition"].startswith("drop")}
    for r in rows:
        m = r["classification"]["migration"]
        if m["old_case_key"] != "-":
            if mmap_expect.get(r["case_key"]) != m["old_case_key"]:
                problems.append(("V2", "migration mismatch", r["case_key"]))
    for v2k in mmap_expect:
        if v2k not in by_key:
            problems.append(("V2", "map target missing", v2k))
    # V3 sd/hn overlap: hn queries are authored title-free (body packets only),
    # so the same title-leak rule applies (2026-09-04 hn review, hn-0003/0004/0010)
    for r in rows:
        if r["classification"]["primary_layer"] not in ("semantic_discovery", "hard_neighbor"):
            continue
        tgt = r["expected_citation_keys"][0]
        title = idx[tgt]["title"]
        core = gsa.title_core(title, idx[tgt]["ip"]) if re.search(r"[\u4e00-\u9fff]", title) else title
        q = r["query"]
        if re.search(r"[A-Za-z]", core) and not re.search(r"[\u4e00-\u9fff]", core):
            ov, _ = gsa.en_content_overlap(q, core)
            if ov >= 2:
                problems.append(("V3", "en overlap", r["case_key"], ov))
        else:
            run = gsa.lcs_substr(q, core)
            if run and len(run) >= 4:
                problems.append(("V3", "zh run", r["case_key"], run))
    # V4 be answer keys not in title
    for r in rows:
        if r["classification"]["primary_layer"] != "body_evidence":
            continue
        tgt = r["expected_citation_keys"][0]
        title = idx[tgt]["title"]
        for ak in r["answer_rubric"]["deterministic_assertions"]:
            if ak in title or ak in lib.normalized_title(title):
                problems.append(("V4", "answer key in title", r["case_key"], ak))
    # V5 na evidence
    for r in rows:
        if r["classification"]["primary_layer"] != "no_answer":
            continue
        if r["classification"]["primary_layer"] == "no_answer" and "na_evidence" in r["classification"]:
            if r["classification"]["na_evidence"]["verdict"] != "zero-exact":
                problems.append(("V5", "na verdict", r["case_key"]))
        if not r["classification"].get("no_answer_strategy"):
            problems.append(("V5", "strategy missing", r["case_key"]))
    # V6 spans
    for r in rows:
        for ref in r["relevant_refs"]:
            if (ref["source_start"], ref["source_end"]) == (0, 400):
                problems.append(("V6", "placeholder span", r["case_key"]))
            if not (0 <= ref["source_start"] < ref["source_end"]):
                problems.append(("V6", "bad span", r["case_key"]))
            if not ref["chunk_key"] or ref["chunking_version"] <= 0:
                problems.append(("V6", "chunk snapshot missing", r["case_key"]))
    # V7 coverage
    langs = Counter(r["query_language"] for r in rows)
    cold = sum(1 for r in rows if r["classification"]["temperature_band"] == "cold")
    colloquial = sum(1 for r in rows if r["classification"]["query_register"] == "colloquial")
    ips = {r["classification"]["ip_name"] for r in rows if r["classification"]["ip_name"]}
    cover = {"zh": langs["zh"], "en": langs["en"], "mixed": langs["mixed"], "cold": cold,
             "colloquial": colloquial, "ips": len(ips)}
    if langs["zh"] < 98: problems.append(("V7", "zh below 50%", langs["zh"]))
    if langs["en"] < 40: problems.append(("V7", "en below 20%", langs["en"]))
    if langs["mixed"] < 20: problems.append(("V7", "mixed below 10%", langs["mixed"]))
    if cold < 40: problems.append(("V7", "cold below 20%", cold))
    if colloquial < 30: problems.append(("V7", "colloquial below 30", colloquial))
    if len(ips) < 16: problems.append(("V7", "IP coverage", len(ips)))
    # V8 determinism handled by caller (double build)
    # V9 visibility invariants
    for r in rows:
        layer = r["classification"]["primary_layer"]
        if layer == "visibility":
            for k in r["forbidden_keys"]:
                if idx[k]["visibility"] == "public":
                    problems.append(("V9", "vi forbidden is public", r["case_key"], k))
        else:
            for k in r["expected_citation_keys"] + r["acceptable_keys"]:
                if idx[k]["visibility"] != "public":
                    problems.append(("V9", "relevance tier non-public", r["case_key"], k))
    # V10 tier consistency
    for r in rows:
        exp = set(r["expected_citation_keys"])
        acc = set(r["acceptable_keys"])
        forb = set(r["forbidden_keys"])
        if exp & acc or exp & forb or acc & forb:
            problems.append(("V10", "tier overlap", r["case_key"]))
        for k in exp | acc | forb:
            if k not in idx or k not in mapping:
                problems.append(("V10", "unknown key", r["case_key"], k))
        if set(r["forbidden_reasons"]) != forb:
            problems.append(("V10", "forbidden without reason", r["case_key"]))
        if r["expected_citation_keys"] and not r["relevant_refs"]:
            problems.append(("V10", "expected without evidence", r["case_key"]))
        if not r["expected_citation_keys"] and r["relevant_refs"]:
            problems.append(("V10", "evidence without expected", r["case_key"]))
    # V11 principal + annotation
    for r in rows:
        if r["viewer_context"] != {"principal_key": "anon"}:
            problems.append(("V11", "principal", r["case_key"]))
        if r["annotation_status"] != "pending_review":
            problems.append(("V11", "status", r["case_key"]))
        if "split" in r["classification"]:
            problems.append(("V11", "split must be absent at 4a", r["case_key"]))

    # V12 ke query == prefix + normalized_title (guards embedded-bracket titles)
    for r in rows:
        if r["classification"]["primary_layer"] != "known_item_exact":
            continue
        tgt = r["expected_citation_keys"][0]
        pref = "在%s内：" % idx[tgt]["ip"] if r["classification"]["ip_scope"] else ""
        want = pref + lib.normalized_title(idx[tgt]["title"])
        if r["query"] != want:
            problems.append(("V12", "ke query != canonical title", r["case_key"]))
        if r["query"].count("《") != r["query"].count("》"):
            problems.append(("V12", "unbalanced brackets", r["case_key"]))
    # V13 be assertion coverage (2026-09-05 re-audit): every deterministic
    # assertion must appear in at least one relevant_evidence span — contract
    # §3 span semantics (真正支持答案的区间) enforced at generation time,
    # closing the gap the six-batch review left (anchor-only spans covered
    # just 30/54 assertions).
    for r in rows:
        if r["classification"]["primary_layer"] != "body_evidence":
            continue
        span_texts = []
        for ref in r["relevant_refs"]:
            body = bodies[mapping[ref["corpus_item_key"]]["content_id"]]
            span_texts.append(body[ref["source_start"]:ref["source_end"]])
        for ak in r["answer_rubric"]["deterministic_assertions"]:
            if not any(ak in t for t in span_texts):
                problems.append(("V13", "assertion without evidence span", r["case_key"], ak))
    report["checks"] = {
        "V1_quota": dict(layers), "V7_coverage": cover,
        "expected_never_retrieved": [v2 for v2, _ in never_retrieved(ev, sel)],
        "sd_diag_hits_any_config": sum(
            1 for v2, e in ev.items() if e["kind"] == "sd_diag" and
            any(h["key"] == e["target"] for c in ["off-off", "exp-on", "rerank-on", "exp-rerank"]
                for h in e[c]["top"])),
        "sd_diag_total": sum(1 for e in ev.values() if e["kind"] == "sd_diag"),
    }
    report["problems"] = problems
    report["pass"] = not problems
    return report


def main():
    rows, idx, mapping, v1, mmap, ev, sel, bodies = build()
    report1 = validate(rows, idx, mapping, v1, mmap, ev, sel, bodies)
    rows2, *_ = build()  # determinism: rebuild must be identical
    s1 = "\n".join(json.dumps(r, ensure_ascii=False, sort_keys=True) for r in rows)
    s2 = "\n".join(json.dumps(r, ensure_ascii=False, sort_keys=True) for r in rows2)
    digest = hashlib.sha256(s1.encode("utf-8")).hexdigest()
    report1["checks"]["V8_deterministic"] = (s1 == s2)
    report1["checks"]["sha256"] = digest
    if not report1["pass"] or not report1["checks"]["V8_deterministic"]:
        report1["pass"] = False

    out = os.path.join(GS, "v2-cases.jsonl")
    with open(out, "w", encoding="utf-8") as fh:
        for r in rows:
            fh.write(json.dumps(r, ensure_ascii=False, sort_keys=True) + "\n")
    with open(os.path.join(GS, "v2-validation-report.json"), "w", encoding="utf-8") as fh:
        json.dump(report1, fh, ensure_ascii=False, indent=1)
    print("cases:", len(rows), "| pass:", report1["pass"], "| sha256:", digest[:16])
    if report1["problems"]:
        for p in report1["problems"][:20]:
            print("  PROBLEM", p)


if __name__ == "__main__":
    main()
