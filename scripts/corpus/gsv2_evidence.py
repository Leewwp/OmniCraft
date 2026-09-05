#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Retrieval evidence for Golden Set v2 (#291 step 4, contract §5-3/§5-4).

2026-09-05 五配置化（canonical runtime 裁决，权威 docs/reference/agent-runtime-matrix.md）：
- 消融矩阵 = A-04 五配置（SP-13 §7 修订）：C0 v4 纯向量 → C1 单开 hybrid →
  C2 单开查询扩展 → C3 单开 rerank → C4 三开关全开。C1–C3 单开隔离单变量增益
  （三开关在 C3 累加式下已全开、C4 将与 C3 重复，故取单开口径），C4 测交互。
- 运行身份全部读自根目录 .env（config.Load 同一入口、真实环境变量优先），
  禁止在脚本内硬编码模型；启动前按矩阵断言（canonical minimax/MiniMax-M3 或
  评测变体 openai_compat/qwen-plus，embedding text-embedding-v4@DashScope
  compatible-mode 无 /v1 尾，rerank key 在位），不符即 fail-closed 拒跑。
- 证据落 v2-retrieval-evidence/a04-five-config/（不动 4a 的 union.json——
  golden_set_v2_gen.py 复现 4a sha 依赖它）；union.json 首键 _meta 记录运行
  身份、每配置开关意图、split checksum，证明证据↔切分绑定。
- backend/cmd/rag-probe header 回显 chat/embedding/rerank 身份、词法主路、
  三开关有效态与 embedding 维度；python 侧复核 header 与意图一致（含 1536 维）。

Drives backend/cmd/rag-probe for every authoring query:
- hn probe queries (multi-config candidate union mining),
- na domain-internal queries (zero-exact-correspondence verification),
- na keep queries (distractor re-draw: what actually surfaces),
- sd queries (retrieval sanity diagnostic, not a gate).

Every returned content_id is mapped back to the corpus via
injection/mapping.jsonl; the ~247 non-corpus dev items already in the index
are dropped from the evidence and counted separately. NOTE (freeze prereq,
2026-09-05): the 4a evidence ran on the OpenSearch lexical backend; the
authoritative re-run must happen after the PG (pg_jieba) lexical unification,
with a rank diff against the 4a union before #291 freezes.
"""
from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
GS = os.path.join(REPO, "artifacts", "corpus-v2", "golden-set")
EVID = os.path.join(GS, "v2-retrieval-evidence", "a04-five-config")
ENV_FILE = os.path.join(REPO, ".env")
MATRIX_REF = "docs/reference/agent-runtime-matrix.md (PR #352)"

# A-04 five-config ablation matrix: each switch isolated on C0, C4 = interaction.
SWITCHES = ("rag_hybrid_enabled", "rag_query_expansion_enabled", "rag_rerank_enabled")
CONFIGS = {
    "C0-v4-only": {},
    "C1-hybrid": {"rag_hybrid_enabled": True},
    "C2-expansion": {"rag_query_expansion_enabled": True},
    "C3-rerank": {"rag_rerank_enabled": True},
    "C4-all-on": {"rag_hybrid_enabled": True, "rag_query_expansion_enabled": True,
                  "rag_rerank_enabled": True},
}

# Sanctioned runtime identities (runtime-matrix.md): canonical chat first,
# DashScope qwen-plus kept as the single-key eval variant. Never hardcode
# these as operational values — they are assertion targets only.
SANCTIONED_CHAT = {("minimax", "MiniMax-M3"), ("openai_compat", "qwen-plus")}
EMBED_MODEL = "text-embedding-v4"
EMBED_BASE = "https://dashscope.aliyuncs.com/compatible-mode"
EMBED_DIMS = 1536


def load_env():
    """Mirror config.Load: root .env, real environment wins."""
    env = dict(os.environ)
    if os.path.exists(ENV_FILE):
        for line in open(ENV_FILE, encoding="utf-8"):
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, _, v = line.partition("=")
            k, v = k.strip(), v.strip().strip('"').strip("'")
            if k and k not in env:
                env[k] = v
    return env


def assert_runtime_identity(env):
    """Fail-closed matrix assertions; returns the echoable identity (no keys)."""
    problems = []
    provider = env.get("AGENT_LLM_PROVIDER", "").strip()
    model = env.get("AGENT_LLM_MODEL", "").strip()
    base = env.get("AGENT_LLM_API_BASE", "").strip().rstrip("/")
    if (provider, model) not in SANCTIONED_CHAT:
        problems.append("chat identity (%s, %s) 不在矩阵集合 %s" % (provider, model, sorted(SANCTIONED_CHAT)))
    if not env.get("AGENT_LLM_API_KEY", "").strip():
        problems.append("AGENT_LLM_API_KEY 为空")
    if base.endswith("/v1"):
        problems.append("AGENT_LLM_API_BASE 不得带 /v1 尾（%s）" % base)

    emb_model = env.get("AGENT_EMBEDDING_MODEL", "").strip() or EMBED_MODEL
    emb_base = (env.get("AGENT_EMBEDDING_API_BASE", "").strip() or EMBED_BASE).rstrip("/")
    emb_key = env.get("AGENT_EMBEDDING_API_KEY", "").strip() or env.get("DASHSCOPE_API_KEY", "").strip()
    if emb_model != EMBED_MODEL:
        problems.append("embedding model %s != 矩阵 %s" % (emb_model, EMBED_MODEL))
    if emb_base != EMBED_BASE:
        problems.append("embedding api_base %s != 矩阵 %s" % (emb_base, EMBED_BASE))
    if not emb_key:
        problems.append("embedding key 为空（AGENT_EMBEDDING_API_KEY / DASHSCOPE_API_KEY）")

    rerank_key = env.get("RAG_RERANK_API_KEY", "").strip() or env.get("DASHSCOPE_API_KEY", "").strip()
    if not rerank_key:
        problems.append("rerank key 为空（RAG_RERANK_API_KEY / DASHSCOPE_API_KEY 回落）")

    if problems:
        raise SystemExit("运行身份断言失败（矩阵权威 %s）：\n  - %s\n"
                         "先修正根目录 .env 再重跑；不得以改断言的方式放行。"
                         % (MATRIX_REF, "\n  - ".join(problems)))
    return {
        "chat": {"provider": provider, "model": model, "api_base": base},
        "embedding": {"model": emb_model, "api_base": emb_base,
                      "provider": env.get("AGENT_EMBEDDING_PROVIDER", "").strip()},
        "rerank": {"key_source": "RAG_RERANK_API_KEY" if env.get("RAG_RERANK_API_KEY", "").strip()
                   else "DASHSCOPE_API_KEY(fallback)"},
        "matrix_ref": MATRIX_REF,
    }


def split_checksum():
    path = os.path.join(GS, "v2-cases-split.jsonl")
    if not os.path.exists(path):
        raise SystemExit("缺少 %s —— 证据必须绑定 group-aware 切分（先跑 gsv2_split.py）" % path)
    return hashlib.sha256(open(path, "rb").read()).hexdigest()[:16]


def load_queries():
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    import gsv2_cases as cases
    cases_v1 = {c["case_key"]: c for c in (json.loads(l) for l in open(f"{GS}/golden-cases-draft.jsonl", encoding="utf-8"))}
    na_keep = ["gs-c2-0137", "gs-c2-0138", "gs-c2-0139", "gs-c2-0140", "gs-c2-0142",
               "gs-c2-0143", "gs-c2-0144", "gs-c2-0145", "gs-c2-0148", "gs-c2-0149",
               "gs-c2-0150", "gs-c2-0155"]
    sel = json.load(open(f"{GS}/v2-selection.json", encoding="utf-8"))
    swap = {e["v2_key"]: e["target"] for e in sel["sd"]}
    mmap = [json.loads(l) for l in open(f"{GS}/migration-map-v2.jsonl", encoding="utf-8")]
    sd_targets = {r["v2_case_key"]: (swap.get(r["v2_case_key"]) or
                  cases_v1[r["old_case_key"]]["expected_citation_keys"][0])
                  for r in mmap if r["v2_layer"] == "semantic_discovery"}
    q = {}
    for v2, c in sorted(cases.HN.items()):
        q[v2] = {"kind": "hn", "query": c["query"], "target": None}
    for v2, c in sorted(cases.NA_NEW.items()):
        q[v2] = {"kind": "na_new", "query": c["query"], "target": None}
    for old, v2 in zip(na_keep, [f"na-{i:04d}" for i in range(1, 13)]):
        q[v2] = {"kind": "na_keep", "query": cases_v1[old]["query"], "target": None}
    for v2, c in sorted(cases.SD.items()):
        q["sd-diag-" + v2] = {"kind": "sd_diag", "query": c["query"], "target": sd_targets[v2]}
    return q


def main():
    os.makedirs(EVID, exist_ok=True)
    env = load_env()
    identity = assert_runtime_identity(env)
    split_sha = split_checksum()
    queries = load_queries()
    print("runtime identity:", json.dumps(identity, ensure_ascii=False))
    print("split checksum (v2-cases-split.jsonl):", split_sha)

    mapping = {r["content_id"]: r["corpus_item_key"] for r in
               (json.loads(l) for l in open(os.path.join(REPO, "artifacts", "corpus-v2", "injection", "mapping.jsonl"), encoding="utf-8"))}
    idx = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(os.path.join(REPO, "artifacts", "corpus-v2", "index.jsonl"), encoding="utf-8"))}

    inpath = "/tmp/gsv2/probe-input.json"
    os.makedirs("/tmp/gsv2", exist_ok=True)
    with open(inpath, "w", encoding="utf-8") as fh:
        json.dump({"queries": [v["query"] for v in queries.values()], "top_k": 10, "viewer_id": 0}, fh, ensure_ascii=False)

    results = {}  # v2_key -> per-config rows
    for label, feats in CONFIGS.items():
        intended = {s: (s in feats) for s in SWITCHES}
        over = "/tmp/gsv2/override-%s.yaml" % label
        with open(over, "w", encoding="utf-8") as fh:
            fh.write("features:\n")
            for s in SWITCHES:
                fh.write("  %s: %s\n" % (s, "true" if intended[s] else "false"))
        cmd = ["go", "run", "./cmd/rag-probe", "-in", inpath,
               "-out", "/tmp/gsv2/probe-%s.jsonl" % label, "-label", label]
        print("running config", label, json.dumps(intended), flush=True)
        proc = subprocess.run(cmd, capture_output=True, text=True,
                              cwd=os.path.join(REPO, "backend"), env=dict(env))
        if not os.path.exists("/tmp/gsv2/probe-%s.jsonl" % label):
            print(proc.stdout[-2000:], proc.stderr[-2000:])
            raise SystemExit("probe failed for " + label)

        with open("/tmp/gsv2/probe-%s.jsonl" % label, encoding="utf-8") as fh:
            header = json.loads(fh.readline())
        if header.get("kind") != "rag-probe":
            raise SystemExit("unexpected probe header for " + label)
        if header.get("switches") != intended:
            raise SystemExit("switch drift for %s: header=%s intended=%s（config.yaml 基线或覆盖未生效）"
                             % (label, header.get("switches"), intended))
        rt = header.get("runtime", {})
        if rt.get("embedding", {}).get("model") not in (None, EMBED_MODEL):
            raise SystemExit("probe embedding model %s != %s" % (rt["embedding"].get("model"), EMBED_MODEL))
        if header.get("embedding_dims") not in (None, EMBED_DIMS):
            raise SystemExit("embedding dims %s != %s（索引世代不匹配，禁止出证据）"
                             % (header.get("embedding_dims"), EMBED_DIMS))

        rows = []
        dropped = 0
        with open("/tmp/gsv2/probe-%s.jsonl" % label, encoding="utf-8") as fh:
            next(fh)  # header
            for line in fh:
                r = json.loads(line)
                kept = []
                for c in r.get("candidates", []):
                    key = mapping.get(c["content_id"])
                    if key is None:
                        dropped += 1
                        continue
                    kept.append({"rank": c["rank"], "key": key, "title": c["title"],
                                 "ip": idx[key]["ip"], "scoreless": True})
                rows.append({"query": r["query"], "degraded": r.get("degraded", ""),
                             "expanded": r.get("expanded_queries", []), "hits": kept})
        with open(os.path.join(EVID, label + ".jsonl"), "w", encoding="utf-8") as fh:
            for row in rows:
                fh.write(json.dumps(row, ensure_ascii=False) + "\n")
        for (v2, meta), row in zip(queries.items(), rows):
            assert row["query"] == meta["query"], (v2, row["query"])
            results.setdefault(v2, {"kind": meta["kind"], "target": meta["target"], "query": meta["query"]})[label] = {
                "degraded": row["degraded"], "expanded_n": len(row["expanded"]),
                "top": [{"rank": h["rank"], "key": h["key"]} for h in row["hits"]]}
        print("  config %s done, non-corpus hits dropped: %d" % (label, dropped), flush=True)

    # union over configs per query, annotated with titles for judgment reading
    for v2, r in results.items():
        union = {}
        for label in CONFIGS:
            for h in r.get(label, {}).get("top", []):
                union.setdefault(h["key"], min(union.get(h["key"], 99), h["rank"]))
        r["union"] = [{"key": k, "best_rank": v} for k, v in sorted(union.items(), key=lambda kv: kv[1])]

    out = {"_meta": {"generated_by": "scripts/corpus/gsv2_evidence.py (five-config A-04 matrix)",
                     "matrix_ref": MATRIX_REF, "runtime_identity": identity,
                     "configs": {k: {s: (s in v) for s in SWITCHES} for k, v in CONFIGS.items()},
                     "split_sha256_16": split_sha,
                     "lexical_note": "词法主路以 probe header runtime.keyword_source 为准；"
                                     "4a 证据跑在 OpenSearch 词法上，冻结前须在 PG 词法下重跑并 diff 排名"}}
    out.update(results)
    with open(os.path.join(EVID, "union.json"), "w", encoding="utf-8") as fh:
        json.dump(out, fh, ensure_ascii=False, indent=1)
    print("evidence written:", os.path.join(EVID, "union.json"))


if __name__ == "__main__":
    main()
