#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""A-04 五配置消融驱动（#286 W3）：逐配置启动 cmd/rag-eval（单进程单配置，
CONFIG_OVERRIDE_PATH 切换三开关），断言 header 身份/开关无漂移，最后把五个
summary.json 汇成对比表（分层原始分子/分母 + Wilson 95% CI，不给综合准确率；
sd-0027/sd-0040 病理单列不进常规解读）。

用法（在仓库根目录）：
  python3 scripts/corpus/a04_ablation.py --split dev            # 全量五配置长跑
  python3 scripts/corpus/a04_ablation.py --split dev --resume   # 断点续跑
  python3 scripts/corpus/a04_ablation.py --split dev --max-cases 4   # 冒烟
产物目录：artifacts/corpus-v2/golden-set/a04-ablation/<split>/
  <label>.jsonl（逐 case checkpoint，可续跑）/ <label>.summary.json / comparison.md
"""

import argparse
import json
import os
import subprocess
import sys

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
OUT_ROOT = os.path.join(REPO, "artifacts", "corpus-v2", "golden-set", "a04-ablation")
ENV_FILE = os.path.join(REPO, ".env")

SWITCHES = ("rag_hybrid_enabled", "rag_query_expansion_enabled", "rag_rerank_enabled")
CONFIGS = {
    "C0-v4-only": {},
    "C1-hybrid": {"rag_hybrid_enabled": True},
    "C2-expansion": {"rag_query_expansion_enabled": True},
    "C3-rerank": {"rag_rerank_enabled": True},
    "C4-all-on": {"rag_hybrid_enabled": True, "rag_query_expansion_enabled": True,
                  "rag_rerank_enabled": True},
}
SANCTIONED_CHAT = {("minimax", "MiniMax-M3"), ("openai_compat", "qwen-plus")}
EMBED_MODEL = "text-embedding-v4"
EMBED_DIMS = 1536
PATHOLOGICAL = ("sd-0027", "sd-0040")


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


def short_switches(feats):
    return {"hybrid": "rag_hybrid_enabled" in feats,
            "query_expansion": "rag_query_expansion_enabled" in feats,
            "rerank": "rag_rerank_enabled" in feats}


def run_config(label, feats, split, outdir, env, resume, max_cases, confirm_test, skip_generation):
    outjsonl = os.path.join(outdir, label + ".jsonl")
    summary = os.path.join(outdir, label + ".summary.json")
    over = os.path.join(outdir, "override-%s.yaml" % label)
    with open(over, "w", encoding="utf-8") as fh:
        fh.write("features:\n")
        for s in SWITCHES:
            fh.write("  %s: %s\n" % (s, "true" if s in feats else "false"))
        # chat generation needs the workspace agent switch on (repo baseline
        # keeps it off; the eval measures the enabled production behaviour)
        fh.write("agent:\n  web_agent_enabled: true\n")

    cmd = ["go", "run", "./cmd/rag-eval", "-label", label, "-split", split,
           "-out", outjsonl, "-summary", summary]
    if resume:
        cmd.append("-resume")
    if max_cases:
        cmd += ["-max-cases", str(max_cases)]
    if confirm_test:
        cmd.append("-confirm-test-run")
    if skip_generation:
        cmd.append("-skip-generation")
    run_env = dict(env)
    run_env["CONFIG_OVERRIDE_PATH"] = over
    print(">> running", label, short_switches(feats), flush=True)
    proc = subprocess.run(cmd, cwd=os.path.join(REPO, "backend"), env=run_env)
    if proc.returncode != 0:
        raise SystemExit("rag-eval failed for %s (exit %d)" % (label, proc.returncode))

    # fail-closed header assertions（续跑时 header 可能不在文件头，改查 summary）
    with open(summary, encoding="utf-8") as fh:
        s = json.load(fh)
    if s.get("switches") != short_switches(feats):
        raise SystemExit("switch drift for %s: summary=%s intended=%s" % (label, s.get("switches"), short_switches(feats)))
    rt = s.get("runtime", {})
    if (rt.get("chat", {}).get("provider"), rt.get("chat", {}).get("model")) not in SANCTIONED_CHAT:
        raise SystemExit("unsanctioned chat identity in %s: %r" % (label, rt.get("chat")))
    if rt.get("embedding", {}).get("model") not in (None, EMBED_MODEL):
        raise SystemExit("embedding model drift in %s: %r" % (label, rt.get("embedding", {}).get("model")))
    if rt.get("keyword_source") not in (None, "postgres"):
        raise SystemExit("keyword_source drift in %s: %r" % (label, rt.get("keyword_source")))
    return s


def fmt_rate(r):
    if not r:
        return "—"
    if not r.get("denominator"):
        return "0/0"
    return "%d/%d = %.3f [%.3f, %.3f]" % (round(r["numerator"]), r["denominator"],
                                          r["numerator"] / r["denominator"],
                                          r.get("ci95_low", 0.0), r.get("ci95_high", 0.0))


def layer_key(g):
    key = g.get("group_key", "?")
    return key + ("@" + g["split"] if g.get("split") else "")


def build_comparison(summaries, split, outdir):
    lines = []
    lines.append("# A-04 五配置消融对比（split=%s）" % split)
    lines.append("")
    lines.append("> 口径：C0 v4 纯向量 → C1 单开 hybrid → C2 单开查询扩展 → C3 单开 rerank → C4 全开；"
                 "分层原始分子/分母 + Wilson 95%% CI，无综合准确率；"
                 "sd-0027 / sd-0040 为 expected_never_retrieved 病理样本，单列不进常规解读。")
    lines.append("")

    # retrieval main metrics per layer per config
    for metric, field in (("Recall@5", "recall_at_5"), ("nDCG@5", "ndcg_at_5")):
        lines.append("## %s（检索主指标）" % metric)
        lines.append("")
        layers = []
        table = {}
        for label, s in summaries.items():
            for g in s.get("retrieval_groups", []):
                k = layer_key(g)
                layers.append(k)
                table.setdefault(k, {})[label] = g.get(field)
        layers = sorted(set(layers))
        header = "| layer | " + " | ".join(summaries.keys()) + " |"
        lines.append(header)
        lines.append("|" + "---|" * (len(summaries) + 1))
        for k in layers:
            row = [k]
            for label in summaries:
                row.append(fmt_rate(table.get(k, {}).get(label)))
            lines.append("| " + " | ".join(row) + " |")
        lines.append("")

    lines.append("## Citation precision（引用精确率，answered 有引用案例）")
    lines.append("")
    labels = sorted({l for s in summaries.values() for l in (s.get("citation_precision") or {})})
    lines.append("| layer | " + " | ".join(summaries.keys()) + " |")
    lines.append("|" + "---|" * (len(summaries) + 1))
    for layer in labels:
        row = [layer]
        for label, s in summaries.items():
            row.append(fmt_rate((s.get("citation_precision") or {}).get(layer)))
        lines.append("| " + " | ".join(row) + " |")
    lines.append("")

    lines.append("## deterministic 判官 / no-answer / visibility 硬门")
    lines.append("")
    lines.append("| config | det pass(有断言层) | na pass | na hard fail | vi leak-free | answered | no_evidence | degraded | provider_error |")
    lines.append("|---|---|---|---|---|---|---|---|---|")
    for label, s in summaries.items():
        det = s.get("deterministic_pass") or {}
        det_txt = ", ".join("%s %s" % (k, fmt_rate(v)) for k, v in sorted(det.items())) or "—"
        gl = s.get("generation_layers") or {}
        na = gl.get("no_answer") or next((v for k, v in gl.items() if k.startswith("no_answer/")), {})
        vi = next((v for k, v in gl.items() if k.startswith("visibility")), {})
        gen_counts = {}
        for v in gl.values():
            for f in ("answered", "no_evidence", "degraded", "provider_errors"):
                gen_counts[f] = gen_counts.get(f, 0) + v.get(f, 0)
        lines.append("| %s | %s | %s | %s | %s | %d | %d | %d | %d |" % (
            label, det_txt, fmt_rate(na.get("no_answer_pass")), fmt_rate(na.get("no_answer_hard_fail")),
            fmt_rate(vi.get("leak_free_rate")),
            gen_counts.get("answered", 0), gen_counts.get("no_evidence", 0),
            gen_counts.get("degraded", 0), gen_counts.get("provider_errors", 0)))
    lines.append("")

    lines.append("## 病理单列（不进层均分常规解读）")
    lines.append("")
    lines.append("| case | 说明 |")
    lines.append("|---|---|")
    lines.append("| sd-0027 | expected_never_retrieved（冻结报告 §4：极限难例，dev 侧） |")
    lines.append("| sd-0040 | expected_never_retrieved（冻结报告 §4：canonical 五配置 top-10 均不可检索，dev 侧） |")
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
    ap.add_argument("--confirm-test-run", action="store_true")
    ap.add_argument("--skip-generation", action="store_true")
    ap.add_argument("--rejudge", action="store_true", help="re-apply judges to stored rows (no provider calls), then rebuild the comparison")
    ap.add_argument("--compare-only", action="store_true", help="rebuild the comparison from existing summaries")
    ap.add_argument("--configs", default=",".join(CONFIGS), help="comma-separated config labels subset")
    args = ap.parse_args()
    if args.compare_only:
        summaries = {}
        for label in args.configs.split(","):
            if not label:
                continue
            sp = os.path.join(OUT_ROOT, args.split, label + ".summary.json")
            with open(sp, encoding="utf-8") as fh:
                summaries[label] = json.load(fh)
        build_comparison(summaries, args.split, os.path.join(OUT_ROOT, args.split))
        return
    if args.split == "test" and not args.confirm_test_run:
        raise SystemExit("test split 须 --confirm-test-run（冻结后单次运行）")

    outdir = os.path.join(OUT_ROOT, args.split)
    os.makedirs(outdir, exist_ok=True)
    env = load_env()
    wanted = [c for c in args.configs.split(",") if c]
    for c in wanted:
        if c not in CONFIGS:
            raise SystemExit("unknown config " + c)

    summaries = {}
    for label in wanted:
        if args.rejudge:
            outjsonl = os.path.join(outdir, label + ".jsonl")
            summary = os.path.join(outdir, label + ".summary.json")
            over = os.path.join(outdir, "override-%s.yaml" % label)
            if not os.path.exists(over):
                with open(over, "w", encoding="utf-8") as fh:
                    fh.write("features:\n")
                    for sw in SWITCHES:
                        fh.write("  %s: %s\n" % (sw, "true" if sw in CONFIGS[label] else "false"))
            cmd = ["go", "run", "./cmd/rag-eval", "-label", label, "-split", args.split,
                   "-out", outjsonl, "-summary", summary, "-rejudge"]
            run_env = dict(env)
            run_env["CONFIG_OVERRIDE_PATH"] = over
            print(">> rejudging", label, flush=True)
            proc = subprocess.run(cmd, cwd=os.path.join(REPO, "backend"), env=run_env)
            if proc.returncode != 0:
                raise SystemExit("rejudge failed for " + label)
            with open(summary, encoding="utf-8") as fh:
                summaries[label] = json.load(fh)
            continue
        summaries[label] = run_config(label, CONFIGS[label], args.split, outdir, env,
                                      args.resume, args.max_cases, args.confirm_test_run,
                                      args.skip_generation)
    build_comparison(summaries, args.split, outdir)


if __name__ == "__main__":
    main()
