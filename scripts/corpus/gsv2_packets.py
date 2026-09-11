#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Authoring packets for Golden Set v2 (#291 step 4).

Splits the authoring inputs by discipline (contract §5):
- sd/be/hn QUERY packets carry NO title (title-hidden authoring; the
  generator enforces the overlap checks afterwards);
- a separate judge file per sd/hn case (target title + same-batch siblings)
  feeds the three-tier judgment pass, which is allowed to see titles;
- na authoring gets per-IP theme lists (no mask required: the zero-
  correspondence verification is the discipline);
- be packets carry the DB chunk map (title prefix stripped) so spans anchor
  into the real chunking at generation time.

Everything here is read-only evidence for authoring; the frozen artifact is
produced by golden_set_v2_gen.py from gsv2_cases.py literals.
"""
from __future__ import annotations

import glob
import json
import os
import re
import subprocess
import sys
from collections import defaultdict

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import corpus_lib as lib  # noqa: E402

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
BASE = os.path.join(REPO, "artifacts", "corpus-v2")
GS = os.path.join(BASE, "golden-set")
PKT = os.path.join(GS, "v2-packets")
PSQL = ["docker", "compose", "exec", "-T", "postgres", "psql", "-U", "omnicraft", "-d", "omnicraft"]


def psql_query(sql: str):
    res = subprocess.run(PSQL + ["-t", "-A", "-c", sql], capture_output=True, text=True, cwd=REPO)
    if res.returncode != 0:
        raise RuntimeError(res.stderr)
    return [l for l in res.stdout.split("\n") if l != ""]


def load_all():
    idx = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(f"{BASE}/index.jsonl", encoding="utf-8"))}
    mapping = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(f"{BASE}/injection/mapping.jsonl", encoding="utf-8"))}
    sel = json.load(open(f"{GS}/v2-selection.json", encoding="utf-8"))
    body = {}
    for path in sorted(glob.glob(f"{BASE}/corpus-batch-*.jsonl")):
        for line in open(path, encoding="utf-8"):
            r = json.loads(line)
            body[r["corpus_item_key"]] = r["body_md"]
    return idx, mapping, sel, body


def batch_of(key): return re.match(r"c2-ip\d+-(b\d+)-\d+$", key).group(1)


def headings_of(body_md):
    out = []
    for line in body_md.split("\n"):
        if line.startswith("#"):
            out.append(line.strip())
    return out


def excerpts(body_md, head_n=500, mid_ns=(400, 400)):
    n = len(body_md)
    mid1 = body_md[n // 3:n // 3 + mid_ns[0]]
    mid2 = body_md[2 * n // 3:2 * n // 3 + mid_ns[1]]
    return body_md[:head_n], mid1, mid2


def strip_title_prefix(text, title):
    first, sep, rest = text.partition("\n")
    t = lib.normalized_title(title)
    if first.strip() == title.strip() or first.strip() == t or first.strip().lstrip("《").rstrip("》") == t:
        return rest if sep else ""
    return text


def main():
    idx, mapping, sel, body = load_all()
    os.makedirs(PKT, exist_ok=True)
    mmap = [json.loads(l) for l in open(f"{GS}/migration-map-v2.jsonl", encoding="utf-8")]
    cases = {c["case_key"]: c for c in (json.loads(l) for l in open(f"{GS}/golden-cases-draft.jsonl", encoding="utf-8"))}

    # target per sd v2 key (keep or swap)
    sd_swap = {e["v2_key"]: e for e in sel["sd"]}
    sd_targets = {}
    for r in mmap:
        if r["v2_layer"] != "semantic_discovery":
            continue
        if r["disposition"] == "target_swap+rewrite":
            sd_targets[r["v2_case_key"]] = sd_swap[r["v2_case_key"]]["target"]
        else:
            sd_targets[r["v2_case_key"]] = cases[r["old_case_key"]]["expected_citation_keys"][0]

    # ---------------- sd packets ----------------
    by_batch = defaultdict(list)
    for k, v in idx.items():
        by_batch[(v["ip"], batch_of(k))].append(k)
    for v2, tgt in sorted(sd_targets.items()):
        meta = idx[tgt]
        head, m1, m2 = excerpts(body[tgt])
        pk = {
            "v2_key": v2, "target": tgt,  # target key kept for the generator; title NOT included
            "ip": meta["ip"], "language": meta["language"], "temperature": meta["temperature"],
            "category": meta["category"], "word_count": meta["word_count"],
            "headings": headings_of(body[tgt]), "body_head": head, "body_mid1": m1, "body_mid2": m2,
        }
        with open(f"{PKT}/sd-{v2[3:]}.json", "w", encoding="utf-8") as fh:
            json.dump(pk, fh, ensure_ascii=False, indent=1)
        # judge packet: same-batch siblings with titles (post-authoring pass)
        sibs = [k for k in sorted(by_batch[(meta["ip"], batch_of(tgt))]) if k != tgt][:12]
        judge = {
            "v2_key": v2, "target_title": meta["title"], "target_language": meta["language"],
            "siblings": [
                {"key": k, "title": idx[k]["title"], "category": idx[k]["category"],
                 "language": idx[k]["language"], "visibility": idx[k]["visibility"],
                 "word_count": idx[k]["word_count"], "body_head": body[k][:140]}
                for k in sibs],
        }
        with open(f"{PKT}/sd-{v2[3:]}.judge.json", "w", encoding="utf-8") as fh:
            json.dump(judge, fh, ensure_ascii=False, indent=1)

    # ---------------- be packets (DB chunks, title prefix stripped) ----------------
    for e in sel["be"]:
        tgt = e["target"]
        cid = mapping[tgt]["content_id"]
        title = idx[tgt]["title"]
        rows = psql_query(
            "SELECT rc.chunk_index || chr(31) || rc.heading || chr(31) || rc.source_start || chr(31) || "
            "rc.source_end || chr(31) || rc.content_version || chr(31) || rc.chunking_version || chr(31) || "
            "replace(rc.text, chr(10), '\\n') FROM rag_chunks rc "
            "JOIN index_projection_status ips ON ips.content_id = rc.content_id AND ips.index_version = rc.index_version "
            "WHERE rc.content_id = %d AND ips.is_current = TRUE AND ips.state = 'ready' "
            "ORDER BY rc.source_start" % cid)
        chunks = []
        for row in rows:
            ci, heading, ss, se, cv, chunkv, text = row.split(chr(31), 6)
            text = text.replace("\\n", "\n")
            chunks.append({
                "i": int(ci), "heading": heading, "source_start": int(ss), "source_end": int(se),
                "content_version": int(cv), "chunking_version": int(chunkv),
                "text": strip_title_prefix(text, title),
            })
        pk = {"v2_key": e["v2_key"], "target": tgt, "ip": e["ip"], "language": e["language"],
              "category": e["category"], "chunks": chunks}
        with open(f"{PKT}/be-{e['v2_key'][3:]}.json", "w", encoding="utf-8") as fh:
            json.dump(pk, fh, ensure_ascii=False, indent=1)

    # ---------------- hn packets ----------------
    for e in sel["hn"]:
        tgt = e["target"]
        meta = idx[tgt]
        head, m1, _ = excerpts(body[tgt], head_n=600, mid_ns=(300, 0))
        sibs = [k for k in sorted(by_batch[(e["ip"], e["batch"])]) if k != tgt][:10]
        pk = {
            "v2_key": e["v2_key"], "target": tgt, "ip": e["ip"], "batch": e["batch"],
            "language": e["language"], "cluster_size": e["cluster_size"],
            "body_head": head, "body_mid1": m1, "headings": headings_of(body[tgt]),
            "siblings": [
                {"key": k, "title": idx[k]["title"], "category": idx[k]["category"],
                 "language": idx[k]["language"], "visibility": idx[k]["visibility"],
                 "body_head": body[k][:140]}
                for k in sibs],
        }
        with open(f"{PKT}/hn-{e['v2_key'][3:]}.json", "w", encoding="utf-8") as fh:
            json.dump(pk, fh, ensure_ascii=False, indent=1)

    # ---------------- na authoring aid: per-IP theme list ----------------
    themes = {}
    for ip in sorted({v["ip"] for v in idx.values()}):
        themes[ip] = [
            {"key": k, "title": idx[k]["title"], "category": idx[k]["category"],
             "batch": batch_of(k), "language": idx[k]["language"]}
            for k, v in sorted(idx.items()) if v["ip"] == ip]
    with open(f"{PKT}/ip-themes.json", "w", encoding="utf-8") as fh:
        json.dump(themes, fh, ensure_ascii=False, indent=1)

    print("packets written to", PKT)
    print("sd:", len(sd_targets), "be:", len(sel["be"]), "hn:", len(sel["hn"]))


if __name__ == "__main__":
    main()
