#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""SP-22 E4 归因网格 runner（#563）：混合检索权重网格扫描 + ablation 对比表。

逐配置启动 cmd/rag-eval -skip-generation -record（单进程单配置，
CONFIG_OVERRIDE_PATH 切换 rag.hybrid / features 轴），每配置零 LLM 成本只测
检索层 ID 指标（context recall / hit-rate / MRR / over-refusal 代理），结果经
-record 落 eval_runs（run_key = grid-<label>-<split>，供 E5 趋势页渲染），
最后把各配置 summary 汇成对比表（配置 × 指标矩阵 + 每指标最优格标注）。

网格轴（设计输入 §2.2 / §4.D，票 #563）：
  rrf_k / bm25_topk / vector_topk / final_topk / rerank 开关
  （refusal_threshold 与 chunk_strategy 为预留轴：分别待 SP-24 R3/R4 变成
   可扫描的真实开关，run 行的 environment.axes 已带 null 占位。）

与 A-04 冻结消融（scripts/corpus/a04_ablation.py）的关系：A-04 扫的是三个
feature 开关（hybrid/expansion/rerank），产物冻结在
artifacts/corpus-v2/golden-set/a04-ablation/；本 runner 只扫权重数值轴并固定
开关为生产形态（hybrid=on / expansion=off），产物落 artifacts/evals/grid/，
互不覆盖。golden set 只读不重生成（dataset_checksum 随 header 断言一致）。

用法（仓库根目录；密钥只从根 .env / 环境注入，不入脚本）：
  python3 scripts/evals/grid_ablation.py --split dev              # 全网格
  python3 scripts/evals/grid_ablation.py --split dev --resume     # 断点续跑
  python3 scripts/evals/grid_ablation.py --split dev --max-cases 4   # 冒烟
  python3 scripts/evals/grid_ablation.py --compare-only           # 只重汇表
产物目录：artifacts/evals/grid/<split>/
  grid-<label>.jsonl / grid-<label>.summary.json / override-<label>.yaml / comparison.md
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
OUT_ROOT = os.path.join(REPO, "artifacts", "evals", "grid")
ENV_FILE = os.path.join(REPO, ".env")

# 生产形态开关（config.yaml 默认）：hybrid 已回写 on（A-04 终局 C1），expansion
# 与 rerank 默认 off。网格里每格都显式钉死这三个开关，保证配置间只有数值轴
# 不同、比较可归因；rerank 轴单独一格（g7）打开它。
BASE_SWITCHES = {
    "rag_hybrid_enabled": True,
    "rag_query_expansion_enabled": False,
    "rag_rerank_enabled": False,
}

# 网格定义：label -> rag.hybrid 数值轴 + 开关覆盖。g0 为生产基线（config.yaml
# 原值：rrf_k=60 / topk=200 / final=10），其余每格只动一两个轴。
GRID = {
    "g0-baseline":  {},
    "g1-rrf20":     {"hybrid": {"rrf_k": 20}},
    "g2-rrf120":    {"hybrid": {"rrf_k": 120}},
    "g3-cand100":   {"hybrid": {"bm25_topk": 100, "vector_topk": 100}},
    "g4-cand400":   {"hybrid": {"bm25_topk": 400, "vector_topk": 400}},
    "g5-final6":    {"hybrid": {"final_topk": 6}},
    "g6-final16":   {"hybrid": {"final_topk": 16}},
    "g7-rerank":    {"switches": {"rag_rerank_enabled": True}},
}

# 对比表主指标：(标题, summary.retrieval_headline 字段, 方向 +1 越大越好)
METRICS = [
    ("recall@10", "context_recall_at_10", +1),
    ("hit@5", "hit_rate_at_5", +1),
    ("hit@10", "hit_rate_at_10", +1),
    ("MRR", "mrr", +1),
    ("nDCG@5", "ndcg_at_5", +1),
    ("over-refusal 代理", "over_refusal_proxy_rate", -1),
    ("平均延迟 ms", "mean_latency_ms", -1),
]

SANCTIONED_CHAT = {("minimax", "MiniMax-M3"), ("openai_compat", "qwen-plus")}
EMBED_MODEL = "text-embedding-v4"


def load_env():
    """Mirror config.Load: root .env, real environment wins."""
    env = dict(os.environ)
    if os.path.exists(ENV_FILE):
        for line in open(ENV_FILE, encoding="utf-8"):
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, _, v = line.partition("=")
            env.setdefault(k.strip(), v.strip().strip('"').strip("'"))
    return env


def write_override(label, spec, path):
    switches = dict(BASE_SWITCHES)
    switches.update(spec.get("switches", {}))
    lines = ["features:"]
    for key in sorted(switches):
        lines.append("  %s: %s" % (key, "true" if switches[key] else "false"))
    lines.append("agent:")
    lines.append("  web_agent_enabled: true")
    hybrid = spec.get("hybrid", {})
    if hybrid:
        lines.append("rag:")
        lines.append("  hybrid:")
        for key in sorted(hybrid):
            lines.append("    %s: %s" % (key, hybrid[key]))
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(lines) + "\n")


def short_switches(spec):
    switches = dict(BASE_SWITCHES)
    switches.update(spec.get("switches", {}))
    return {"hybrid": switches["rag_hybrid_enabled"],
            "query_expansion": switches["rag_query_expansion_enabled"],
            "rerank": switches["rag_rerank_enabled"]}


def run_config(label, spec, split, outdir, env, resume, max_cases):
    outjsonl = os.path.join(outdir, "grid-%s.jsonl" % label)
    summary = os.path.join(outdir, "grid-%s.summary.json" % label)
    over = os.path.join(outdir, "override-%s.yaml" % label)
    write_override(label, spec, over)

    cmd = ["go", "run", "./cmd/rag-eval", "-label", label, "-split", split,
           "-out", outjsonl, "-summary", summary,
           "-skip-generation", "-record", "-run-key-prefix", "grid"]
    if resume:
        cmd.append("-resume")
    if max_cases:
        cmd += ["-max-cases", str(max_cases)]
    run_env = dict(env)
    run_env["CONFIG_OVERRIDE_PATH"] = over
    print(">> running", label, short_switches(spec), spec.get("hybrid", {}), flush=True)
    proc = subprocess.run(cmd, cwd=os.path.join(REPO, "backend"), env=run_env)
    if proc.returncode != 0:
        raise SystemExit("rag-eval failed for %s (exit %d)" % (label, proc.returncode))

    with open(summary, encoding="utf-8") as fh:
        s = json.load(fh)
    # fail-closed 身份断言（与 a04_ablation 同口径；防止 override 漂移说谎）
    if s.get("switches") != short_switches(spec):
        raise SystemExit("switch drift for %s: summary=%s intended=%s"
                         % (label, s.get("switches"), short_switches(spec)))
    rt = s.get("runtime", {})
    if (rt.get("chat", {}).get("provider"), rt.get("chat", {}).get("model")) not in SANCTIONED_CHAT:
        raise SystemExit("unsanctioned chat identity in %s: %r" % (label, rt.get("chat")))
    if rt.get("embedding", {}).get("model") not in (None, EMBED_MODEL):
        raise SystemExit("embedding model drift in %s: %r" % (label, rt.get("embedding", {}).get("model")))
    if rt.get("keyword_source") not in (None, "postgres"):
        raise SystemExit("keyword_source drift in %s: %r" % (label, rt.get("keyword_source")))
    headline = s.get("retrieval_headline")
    if not headline or not headline.get("retrieval_evaluated"):
        raise SystemExit("no retrieval headline in %s summary" % label)
    return s


def axes_of(spec):
    hybrid = spec.get("hybrid", {})
    switches = short_switches(spec)
    parts = ["rrf=%s" % hybrid.get("rrf_k", 60),
             "cand=%s/%s" % (hybrid.get("bm25_topk", 200), hybrid.get("vector_topk", 200)),
             "final=%s" % hybrid.get("final_topk", 10),
             "rerank=%s" % ("on" if switches["rerank"] else "off")]
    return " ".join(parts)


def build_comparison(summaries, split, outdir, dataset_checksum):
    lines = []
    lines.append("# SP-22 E4 归因网格对比（split=%s，零 LLM 成本检索层扫描）" % split)
    lines.append("")
    lines.append("> 口径：开关钉死生产形态（hybrid=on / expansion=off），每格只动数值轴；"
                 "指标 = 检索层 ID 指标（与 PR 门禁同公式，grid_headline.go）；"
                 "over-refusal 代理 = 可答用例 top-10 无期望命中占比（无生成层，真混淆矩阵在 E3 冻结快照）；"
                 "每列最优 ⭐（延迟列除外——只列参考，不参与最优判定）。")
    lines.append("")
    lines.append("dataset_checksum: `%s`" % dataset_checksum)
    lines.append("")

    header = "| 配置 | 轴 | " + " | ".join(t for t, _, _ in METRICS) + " | recall@5 | evaluated |"
    lines.append(header)
    lines.append("|" + "---|" * (len(METRICS) + 4))

    # best cell per metric (latency excluded)
    best = {}
    for title, field, direction in METRICS:
        if field == "mean_latency_ms":
            continue
        vals = [(s.get("retrieval_headline", {}).get(field), label)
                for label, s in summaries.items()]
        vals = [(v, l) for v, l in vals if v is not None]
        if not vals:
            continue
        if direction > 0:
            best[field] = max(vals)[1]
        else:
            best[field] = min(vals)[1]

    for label, s in summaries.items():
        h = s.get("retrieval_headline", {})
        row = ["%s" % label, axes_of(GRID[label])]
        for title, field, _ in METRICS:
            v = h.get(field)
            cell = "—" if v is None else "%.4f" % v
            if field == "mean_latency_ms" and v is not None:
                cell = "%.0f" % v
            if best.get(field) == label:
                cell = "⭐ " + cell
            row.append(cell)
        row.append("%.4f" % h.get("context_recall_at_5", 0))
        row.append("%d" % h.get("retrieval_evaluated", 0))
        lines.append("| " + " | ".join(row) + " |")
    lines.append("")

    lines.append("## 读法与后续")
    lines.append("")
    lines.append("- 基线 = `g0-baseline`（config.yaml 生产原值）。其余每格单轴改动，"
                 "与基线的差即该轴的边际贡献；多轴组合留待 SP-24 R2 权重网格选优。")
    lines.append("- 每格已通过 `-record` 落 `eval_runs`（run_key = `grid-<label>-%s`），"
                 "趋势与历史对比在 /admin/evals（E5）。" % split)
    lines.append("- rerank 格（g7）依赖 env 的 RAG_RERANK_API_KEY（与生产同源）；"
                 "SP-24 R1 会以本表 + 生成层基线 diff 作为翻默认的证据面。")
    lines.append("")

    path = os.path.join(outdir, "comparison.md")
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(lines) + "\n")
    print("comparison written:", path)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--split", default="dev", choices=["dev", "test"])
    ap.add_argument("--resume", action="store_true")
    ap.add_argument("--max-cases", type=int, default=0)
    ap.add_argument("--configs", default=",".join(GRID), help="comma-separated subset of grid labels")
    ap.add_argument("--compare-only", action="store_true", help="rebuild the comparison from existing summaries")
    args = ap.parse_args()

    outdir = os.path.join(OUT_ROOT, args.split)
    os.makedirs(outdir, exist_ok=True)
    wanted = [c for c in args.configs.split(",") if c]
    for c in wanted:
        if c not in GRID:
            raise SystemExit("unknown grid label " + c)

    summaries = {}
    checksums = set()
    for label in wanted:
        sp = os.path.join(outdir, "grid-%s.summary.json" % label)
        if args.compare_only:
            if not os.path.exists(sp):
                raise SystemExit("missing summary for " + label)
            with open(sp, encoding="utf-8") as fh:
                summaries[label] = json.load(fh)
        else:
            summaries[label] = run_config(label, GRID[label], args.split, outdir, load_env(),
                                          args.resume, args.max_cases)
        checksums.add(summaries[label].get("dataset_checksum", ""))
    if len(checksums) > 1:
        raise SystemExit("dataset checksum drift across configs: %s" % sorted(checksums))
    build_comparison(summaries, args.split, outdir, checksums.pop() if checksums else "")


if __name__ == "__main__":
    main()
