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
  python3 scripts/evals/grid_ablation.py --split dev              # E4 单轴网格（rerank=off 形态）
  python3 scripts/evals/grid_ablation.py --grid sp24-r2           # SP-24 R2 多轴网格（rerank=on 形态）
  python3 scripts/evals/grid_ablation.py --grid sp24-r2 --resume  # 断点续跑
  python3 scripts/evals/grid_ablation.py --grid sp24-r2 --max-cases 4   # 冒烟
  python3 scripts/evals/grid_ablation.py --grid e4 --compare-only       # 只重汇表
产物目录：artifacts/evals/grid/<split>[/‑r2]/
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

# 生产形态开关（config.yaml 默认）：hybrid 已回写 on（A-04 终局 C1）；rerank
# 自 SP-24 R1（PR #607）起默认 on。网格里每格都显式钉死这三个开关，保证配置间
# 只有数值轴不同、比较可归因；E4 跑批时 rerank 默认还是 off，故 e4 网格钉
# rerank=off、rerank 轴单独一格（g7）打开它，sp24-r2 网格整体钉 rerank=on。
BASE_SWITCHES = {
    "rag_hybrid_enabled": True,
    "rag_query_expansion_enabled": False,
    "rag_rerank_enabled": False,
}

R2_BASE_SWITCHES = dict(BASE_SWITCHES, rag_rerank_enabled=True)

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

# SP-24 R2 网格（#573）：rerank=on 为新生产形态（R1 已翻默认），在此基线上扫
# 票面三轴 rrf_k / bm25+vector 候选数 / final K，外加与 final K 天然耦合的
# rerank 池深轴（rag.rerank.input_topk，final K 只能从重排后的池里截断）与
# 多轴组合格。r2-base = config.yaml 原值（rrf=60 / cand=200/200 / final=10 /
# rin=20），与 E4 g7 同格复刻作锚点。
GRID_R2 = {
    "r2-base":      {},
    "r2-rrf20":     {"hybrid": {"rrf_k": 20}},
    "r2-rrf120":    {"hybrid": {"rrf_k": 120}},
    "r2-cand100":   {"hybrid": {"bm25_topk": 100, "vector_topk": 100}},
    "r2-cand400":   {"hybrid": {"bm25_topk": 400, "vector_topk": 400}},
    "r2-final6":    {"hybrid": {"final_topk": 6}},
    "r2-final12":   {"hybrid": {"final_topk": 12}},
    "r2-final16":   {"hybrid": {"final_topk": 16}},
    "r2-rin40":     {"rerank": {"input_topk": 40}},
    "r2-rin40-f16": {"hybrid": {"final_topk": 16}, "rerank": {"input_topk": 40}},
}

# 网格档案：--grid 名 -> (网格定义, 钉死的基线开关, 产物子目录后缀)
GRIDS = {
    "e4":      (GRID, BASE_SWITCHES, ""),
    "sp24-r2": (GRID_R2, R2_BASE_SWITCHES, "-r2"),
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


def write_override(label, spec, path, base_switches):
    switches = dict(base_switches)
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
    rerank = spec.get("rerank", {})
    if rerank:
        # LoadOverride 按键合并：override 里只写 input_topk 时，provider/model
        # 等其余 rerank 键保持 config.yaml 原值。
        if not hybrid:
            lines.append("rag:")
        lines.append("  rerank:")
        for key in sorted(rerank):
            lines.append("    %s: %s" % (key, rerank[key]))
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(lines) + "\n")


def short_switches(spec, base_switches):
    switches = dict(base_switches)
    switches.update(spec.get("switches", {}))
    return {"hybrid": switches["rag_hybrid_enabled"],
            "query_expansion": switches["rag_query_expansion_enabled"],
            "rerank": switches["rag_rerank_enabled"]}


def run_config(label, spec, split, outdir, env, resume, max_cases, base_switches):
    outjsonl = os.path.join(outdir, "grid-%s.jsonl" % label)
    summary = os.path.join(outdir, "grid-%s.summary.json" % label)
    over = os.path.join(outdir, "override-%s.yaml" % label)
    write_override(label, spec, over, base_switches)

    cmd = ["go", "run", "./cmd/rag-eval", "-label", label, "-split", split,
           "-out", outjsonl, "-summary", summary,
           "-skip-generation", "-record", "-run-key-prefix", "grid"]
    if resume:
        cmd.append("-resume")
    if max_cases:
        cmd += ["-max-cases", str(max_cases)]
    run_env = dict(env)
    run_env["CONFIG_OVERRIDE_PATH"] = over
    print(">> running", label, short_switches(spec, base_switches),
          spec.get("hybrid", {}), spec.get("rerank", {}), flush=True)
    proc = subprocess.run(cmd, cwd=os.path.join(REPO, "backend"), env=run_env)
    if proc.returncode != 0:
        raise SystemExit("rag-eval failed for %s (exit %d)" % (label, proc.returncode))

    with open(summary, encoding="utf-8") as fh:
        s = json.load(fh)
    # fail-closed 身份断言（与 a04_ablation 同口径；防止 override 漂移说谎）
    if s.get("switches") != short_switches(spec, base_switches):
        raise SystemExit("switch drift for %s: summary=%s intended=%s"
                         % (label, s.get("switches"), short_switches(spec, base_switches)))
    # rerank=on 的格子必须有 rerank 运行时链（provider/model），否则说明
    # override 没生效、跑成了静默降级（R1 首轮 /tmp override 不可见的同族风险）。
    if base_switches.get("rag_rerank_enabled") and not s.get("runtime", {}).get("rerank"):
        raise SystemExit("rerank chain missing in %s runtime: %r" % (label, s.get("runtime")))
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


def axes_of(spec, base_switches):
    hybrid = spec.get("hybrid", {})
    switches = short_switches(spec, base_switches)
    parts = ["rrf=%s" % hybrid.get("rrf_k", 60),
             "cand=%s/%s" % (hybrid.get("bm25_topk", 200), hybrid.get("vector_topk", 200)),
             "final=%s" % hybrid.get("final_topk", 10),
             "rin=%s" % spec.get("rerank", {}).get("input_topk", 20),
             "rerank=%s" % ("on" if switches["rerank"] else "off")]
    return " ".join(parts)


def build_comparison(summaries, split, outdir, dataset_checksum, grid_name, grid_def, base_switches):
    lines = []
    if grid_name == "sp24-r2":
        title = "SP-24 R2 混合权重网格对比（rerank=on 生产形态，多轴组合，split=%s）" % split
        note = ("> 口径：开关钉死 R1 后生产形态（hybrid=on / expansion=off / **rerank=on**），"
                "每格只动数值轴（rrf_k / 候选数 / final K / rerank 池深 rin）；"
                "r2-base = config.yaml 原值，与 E4 g7 同格复刻作锚点。")
    else:
        title = "SP-22 E4 归因网格对比（单轴扫描，split=%s，零 LLM 成本检索层）" % split
        note = "> 口径：开关钉死生产形态（hybrid=on / expansion=off），每格只动数值轴；"
    lines.append("# " + title)
    lines.append("")
    lines.append(note)
    lines.append("> 指标 = 检索层 ID 指标（与 PR 门禁同公式，grid_headline.go）；"
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
        row = ["%s" % label, axes_of(grid_def[label], base_switches)]
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
    if grid_name == "sp24-r2":
        lines.append("- 基线 = `r2-base`（rerank=on 下的 config.yaml 生产原值，= E4 g7 复刻）。"
                     "单轴格与基线的差 = 该轴在 rerank 之上的边际贡献；组合格 rin40-f16 "
                     "验证 rerank 池深 × final K 交互；选优或「无需调」结论回票 #573（SP-24 R2）。")
    else:
        lines.append("- 基线 = `g0-baseline`（config.yaml 生产原值）。其余每格单轴改动，"
                     "与基线的差即该轴的边际贡献；多轴组合留待 SP-24 R2 权重网格选优。")
    lines.append("- 每格已通过 `-record` 落 `eval_runs`（run_key = `grid-<label>-%s`），"
                 "趋势与历史对比在 /admin/evals（E5）。" % split)
    if grid_name == "e4":
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
    ap.add_argument("--grid", default="e4", choices=sorted(GRIDS),
                    help="grid profile: e4 (SP-22 单轴, rerank=off) or sp24-r2 (rerank=on 多轴)")
    ap.add_argument("--resume", action="store_true")
    ap.add_argument("--max-cases", type=int, default=0)
    ap.add_argument("--configs", default="", help="comma-separated subset of grid labels (default: all)")
    ap.add_argument("--compare-only", action="store_true", help="rebuild the comparison from existing summaries")
    args = ap.parse_args()

    grid_def, base_switches, suffix = GRIDS[args.grid]
    outdir = os.path.join(OUT_ROOT, args.split + suffix)
    os.makedirs(outdir, exist_ok=True)
    wanted = args.configs.split(",") if args.configs else list(grid_def)
    wanted = [c for c in wanted if c]
    for c in wanted:
        if c not in grid_def:
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
            summaries[label] = run_config(label, grid_def[label], args.split, outdir, load_env(),
                                          args.resume, args.max_cases, base_switches)
        checksums.add(summaries[label].get("dataset_checksum", ""))
    if len(checksums) > 1:
        raise SystemExit("dataset checksum drift across configs: %s" % sorted(checksums))
    build_comparison(summaries, args.split, outdir, checksums.pop() if checksums else "",
                     args.grid, grid_def, base_switches)


if __name__ == "__main__":
    main()
