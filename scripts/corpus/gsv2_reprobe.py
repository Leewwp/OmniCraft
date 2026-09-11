#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Surgical re-probe for changed hn queries (#291 step 4 hn review round).

2026-09-04 hn review rewrote three queries for the title-free rule
(hn-0003/hn-0004/hn-0010). Re-running all 84 evidence queries would refresh
numbers already reviewed in the vi/na/be rounds, so this script probes ONLY
the changed queries under the four A-04 configs and surgically updates:
  - per-config jsonl rows (rows are keyed by query text; old row replaced),
  - union.json entries for the three cases (per-config blocks + union field),
and writes a provenance artifact hn-reprobe-<date>.json.
"""
from __future__ import annotations

import datetime
import json
import os
import subprocess
import sys

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
GS = os.path.join(REPO, "artifacts", "corpus-v2", "golden-set")
EVID = os.path.join(GS, "v2-retrieval-evidence")
CASES = ["hn-0003", "hn-0004", "hn-0010"]  # 2026-09-04 hn review rewrites
CONFIGS = {
    "off-off": {},
    "exp-on": {"rag_query_expansion_enabled": True},
    "rerank-on": {"rag_rerank_enabled": True},
    "exp-rerank": {"rag_query_expansion_enabled": True, "rag_rerank_enabled": True},
}


def main():
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    import gsv2_cases as cases
    new_queries = {c: cases.HN[c]["query"] for c in CASES}

    union = json.load(open(os.path.join(EVID, "union.json"), encoding="utf-8"))
    old_queries = {c: union[c]["query"] for c in CASES}

    mapping = {r["content_id"]: r["corpus_item_key"] for r in
               (json.loads(l) for l in open(os.path.join(REPO, "artifacts", "corpus-v2", "injection", "mapping.jsonl"), encoding="utf-8"))}
    idx = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(os.path.join(REPO, "artifacts", "corpus-v2", "index.jsonl"), encoding="utf-8"))}

    inpath = "/tmp/gsv2/reprobe-input.json"
    os.makedirs("/tmp/gsv2", exist_ok=True)
    with open(inpath, "w", encoding="utf-8") as fh:
        json.dump({"queries": [new_queries[c] for c in CASES], "top_k": 10, "viewer_id": 0}, fh, ensure_ascii=False)

    env_extra = subprocess.run(["bash", "-lc", "set -a; source .env 2>/dev/null; set +a; "
                               "echo $DASHSCOPE_API_KEY"], capture_output=True, text=True, cwd=REPO).stdout.strip()

    provenance = {"ran_at": datetime.datetime.now().isoformat(timespec="seconds"),
                  "reason": "hn review 2026-09-04: title-free rewrites",
                  "queries": {}}
    for c in CASES:
        provenance["queries"][c] = {"old": old_queries[c], "new": new_queries[c]}

    for label, feats in CONFIGS.items():
        over = "/tmp/gsv2/override-%s.yaml" % label
        with open(over, "w", encoding="utf-8") as fh:
            if feats:
                fh.write("features:\n")
                for k, v in feats.items():
                    fh.write("  %s: %s\n" % (k, "true" if v else "false"))
        cmd = ["go", "run", "./cmd/rag-probe", "-in", inpath,
               "-out", "/tmp/gsv2/reprobe-%s.jsonl" % label, "-label", label]
        env = dict(os.environ)
        env.update({
            "CONFIG_OVERRIDE_PATH": over,
            "AGENT_EMBEDDING_MODEL": "text-embedding-v4",
            "AGENT_EMBEDDING_API_BASE": "https://dashscope.aliyuncs.com/compatible-mode",
            "AGENT_EMBEDDING_API_KEY": env_extra,
            "AGENT_LLM_PROVIDER": "openai_compat",
            "AGENT_LLM_API_BASE": "https://dashscope.aliyuncs.com/compatible-mode",
            "AGENT_LLM_MODEL": "qwen-plus",
            "AGENT_LLM_API_KEY": env_extra,
            "RAG_RERANK_API_KEY": env_extra,
        })
        print("probing config", label, flush=True)
        proc = subprocess.run(cmd, capture_output=True, text=True, cwd=os.path.join(REPO, "backend"), env=env)
        outfile = "/tmp/gsv2/reprobe-%s.jsonl" % label
        if not os.path.exists(outfile):
            print(proc.stdout[-2000:], proc.stderr[-2000:])
            raise SystemExit("probe failed for " + label)
        rows = []
        dropped = 0
        with open(outfile, encoding="utf-8") as fh:
            for line in fh:
                r = json.loads(line)
                if r.get("kind") == "rag-probe":
                    continue
                kept = []
                for cd in r.get("candidates", []):
                    key = mapping.get(cd["content_id"])
                    if key is None:
                        dropped += 1
                        continue
                    kept.append({"rank": cd["rank"], "key": key, "title": cd["title"],
                                 "ip": idx[key]["ip"], "scoreless": True})
                rows.append({"query": r["query"], "degraded": r.get("degraded", ""),
                             "expanded": r.get("expanded_queries", []), "hits": kept})
        assert [r["query"] for r in rows] == [new_queries[c] for c in CASES], label

        # surgical update: per-config jsonl (replace the old-query row)
        cfg_path = os.path.join(EVID, label + ".jsonl")
        out_lines = []
        replaced = 0
        for line in open(cfg_path, encoding="utf-8"):
            row = json.loads(line)
            if row["query"] in old_queries.values():
                c = [k for k in CASES if old_queries[k] == row["query"]][0]
                new_row = [r for r in rows if r["query"] == new_queries[c]][0]
                out_lines.append(json.dumps(new_row, ensure_ascii=False))
                replaced += 1
            else:
                out_lines.append(line.rstrip("\n"))
        assert replaced == len(CASES), (label, replaced)
        with open(cfg_path, "w", encoding="utf-8") as fh:
            fh.write("\n".join(out_lines) + "\n")

        # surgical update: union.json entries
        for c in CASES:
            new_row = [r for r in rows if r["query"] == new_queries[c]][0]
            union[c][label] = {"degraded": new_row["degraded"], "expanded_n": len(new_row["expanded"]),
                               "top": [{"rank": h["rank"], "key": h["key"]} for h in new_row["hits"]]}
        print("  %s done, non-corpus dropped: %d" % (label, dropped), flush=True)

    for c in CASES:
        merged = {}
        for label in CONFIGS:
            for h in union[c].get(label, {}).get("top", []):
                merged.setdefault(h["key"], min(merged.get(h["key"], 99), h["rank"]))
        union[c]["query"] = new_queries[c]
        union[c]["union"] = [{"key": k, "best_rank": v} for k, v in sorted(merged.items(), key=lambda kv: kv[1])]
    with open(os.path.join(EVID, "union.json"), "w", encoding="utf-8") as fh:
        json.dump(union, fh, ensure_ascii=False, indent=1)

    art = os.path.join(EVID, "hn-reprobe-20260904.json")
    with open(art, "w", encoding="utf-8") as fh:
        json.dump(provenance, fh, ensure_ascii=False, indent=1)
    print("union.json + per-config jsonl surgically updated; provenance:", art)


if __name__ == "__main__":
    main()
