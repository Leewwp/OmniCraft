#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""SP-24 R3（#574）拒答边界扫描：对一条已录制的全量生成跑批做零 API 成本
后处理，模拟 rag.refusal.min_surviving_citations 边界对拒答混淆矩阵的影响。

原理：复验后引用存活数不改变模型生成（检索结果与边界无关），分类阶段是
确定性的——grounded 答案在存活引用数 < N 时翻转为 no_evidence（正文清空）。
conversational 行（零工具/全外部工具轮）按实现豁免，不参与翻转。真值 =
golden 表 classification.primary_layer（no_answer 层 = unanswerable，与
pr_gate.go 同口径）。

输出：每个边界的混淆矩阵（correct answer / over-refusal / hallucination /
correct refusal）+ 相对基线的翻转计数 + markdown 表（--out）。

用法（仓库根目录）：
  python3 scripts/evals/refusal_sweep.py --runs artifacts/evals/sp24/sp24-r3-baseline.jsonl
  python3 scripts/evals/refusal_sweep.py --runs ... --boundaries 1,2,3 --out artifacts/evals/sp24/refusal-sweep.md
产物目录：artifacts/evals/sp24/
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from collections import Counter

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import evals_lib as lib

GOLD_NO_ANSWER_LAYER = "no_answer"


def load_golden_layers() -> dict:
    """case_key -> primary_layer straight from the golden table."""
    sql = (
        "SELECT COALESCE(json_agg(t), '[]'::json) FROM ("
        "SELECT case_key, classification FROM eval_golden_cases "
        "WHERE is_active ORDER BY case_key) t"
    )
    out = {}
    for row in json.loads(lib._psql(sql).strip() or "[]"):
        cls = row.get("classification") or {}
        if isinstance(cls, str):
            cls = json.loads(cls)
        out[row["case_key"]] = cls.get("primary_layer", "")
    return out


def load_generation_rows(path: str) -> list[dict]:
    rows = []
    header = None
    for line in open(path, encoding="utf-8"):
        line = line.strip()
        if not line:
            continue
        r = json.loads(line)
        if r.get("phase") == "generation":
            rows.append(r)
        elif "dataset_checksum" in r:
            header = r
    if not rows:
        raise SystemExit("no generation rows in " + path)
    return rows, header


def refused_row(row: dict) -> bool:
    # pr_gate.go 同口径：status 或 kind 任一为 no_evidence 即拒答。
    return row.get("status") == "no_evidence" or row.get("answer_kind") == "no_evidence"


def simulate(rows: list[dict], layers: dict, boundary: int) -> dict:
    """confusion matrix at min_surviving_citations = boundary."""
    m = Counter()
    flips_answerable = []
    flips_unanswerable = []
    for row in rows:
        answerable = layers.get(row["case_key"]) != GOLD_NO_ANSWER_LAYER
        refused = refused_row(row)
        # conversational 行按实现豁免：边界只作用于严格 grounded 分类。
        exempt = row.get("answer_kind") == "conversational"
        sim_refused = refused or (not exempt and not refused
                                  and len(row.get("citations") or []) < boundary)
        key = "answerable" if answerable else "unanswerable"
        if sim_refused:
            m["refused_" + key] += 1
            if not refused:
                (flips_answerable if answerable else flips_unanswerable).append(row["case_key"])
        else:
            m["answered_" + key] += 1
    return {
        "boundary": boundary,
        "answerable": m["answered_answerable"] + m["refused_answerable"],
        "unanswerable": m["answered_unanswerable"] + m["refused_unanswerable"],
        "answered_answerable": m["answered_answerable"],
        "refused_answerable": m["refused_answerable"],
        "answered_unanswerable": m["answered_unanswerable"],
        "refused_unanswerable": m["refused_unanswerable"],
        "correct_answer_rate": m["answered_answerable"] / max(1, m["answered_answerable"] + m["refused_answerable"]),
        "over_refusal_rate": m["refused_answerable"] / max(1, m["answered_answerable"] + m["refused_answerable"]),
        "hallucination_rate": m["answered_unanswerable"] / max(1, m["answered_unanswerable"] + m["refused_unanswerable"]),
        "correct_refusal_rate": m["refused_unanswerable"] / max(1, m["answered_unanswerable"] + m["refused_unanswerable"]),
        "flips_answerable": flips_answerable,
        "flips_unanswerable": flips_unanswerable,
    }


def render(results: list[dict], runs_label: str, dataset_checksum: str) -> str:
    lines = []
    lines.append("# SP-24 R3 拒答边界扫描（min_surviving_citations → 混淆矩阵）")
    lines.append("")
    lines.append("> 输入 run: `%s`；dataset_checksum: `%s`；零 API 成本后处理模拟" % (runs_label, dataset_checksum))
    lines.append("> 口径与 pr_gate.go 同源（no_answer 层 = unanswerable；conversational 行豁免）；")
    lines.append("> revalidateCitations 硬门不动——扫描只移动答案侧边界。")
    lines.append("")
    lines.append("| 边界 | correct answer | over-refusal | hallucination | correct refusal | 翻转(可答→拒) | 翻转(不可答→拒) |")
    lines.append("|---|---|---|---|---|---|---|")
    for r in results:
        lines.append("| min=%d | %.4f | %.4f | %.4f | %.4f | %d | %d |" % (
            r["boundary"], r["correct_answer_rate"], r["over_refusal_rate"],
            r["hallucination_rate"], r["correct_refusal_rate"],
            len(r["flips_answerable"]), len(r["flips_unanswerable"])))
    lines.append("")
    for r in results:
        if r["flips_answerable"]:
            lines.append("- min=%d 可答层翻转例: %s" % (r["boundary"], ", ".join(r["flips_answerable"])))
        if r["flips_unanswerable"]:
            lines.append("- min=%d 不可答层翻转例（hallucination 下降来源）: %s" % (r["boundary"], ", ".join(r["flips_unanswerable"])))
    lines.append("")
    return "\n".join(lines) + "\n"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--runs", required=True, help="rag-eval full-run jsonl (generation rows)")
    ap.add_argument("--boundaries", default="1,2,3", help="comma-separated min_surviving_citations values")
    ap.add_argument("--out", default="", help="markdown output path (default: alongside the run)")
    args = ap.parse_args()

    rows, header = load_generation_rows(args.runs)
    layers = load_golden_layers()
    checksum = (header or {}).get("dataset_checksum", "")

    results = [simulate(rows, layers, int(b)) for b in args.boundaries.split(",") if b.strip()]
    md = render(results, os.path.basename(args.runs), checksum)
    out = args.out or os.path.join(os.path.dirname(os.path.abspath(args.runs)), "refusal-sweep.md")
    with open(out, "w", encoding="utf-8") as fh:
        fh.write(md)
    print(md)
    print("sweep written:", out)


if __name__ == "__main__":
    main()
