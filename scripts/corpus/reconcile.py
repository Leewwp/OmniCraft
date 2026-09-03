#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Reconciliation report: golden-set draft vs injected items (#291).

Cross-checks every corpus_item_key referenced by the draft against the
injection mapping; lists any case that references a failed/unindexed item
(these must be regenerated via `golden_set_draft.py --mapping`), and emits the
20-case stratified manual-review sample for the user.
"""
from __future__ import annotations

import argparse
import json
import os
import random
import sys
from collections import Counter
from typing import Any, Dict, List

SEED = 20260903
MANUAL_SAMPLE_PER_LAYER = 5


def load_jsonl(path: str) -> List[Dict[str, Any]]:
    rows = []
    with open(path, "r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    return rows


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus-dir", default=None)
    parser.add_argument("--mapping", default=None)
    parser.add_argument("--golden", default=None)
    args = parser.parse_args()

    repo = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    corpus_dir = args.corpus_dir
    if not corpus_dir:
        cand = os.path.join(repo, "artifacts", "corpus-v2")
        corpus_dir = cand if os.path.exists(os.path.join(cand, "index.jsonl")) else os.path.join(
            os.path.dirname(repo), "OmniCraft", "artifacts", "corpus-v2"
        )
    mapping_path = args.mapping or os.path.join(corpus_dir, "injection", "mapping.jsonl")
    golden_path = args.golden or os.path.join(corpus_dir, "golden-set", "golden-cases-draft.jsonl")

    mapping = {r["corpus_item_key"]: r for r in load_jsonl(mapping_path)}
    cases = load_jsonl(golden_path)

    broken: List[Dict[str, str]] = []
    referenced = set()
    for case in cases:
        keys = [r["corpus_item_key"] for r in case.get("relevant_refs", [])]
        keys += list(case.get("expected_citation_keys", []))
        keys += list(case.get("forbidden_keys", []))
        for key in keys:
            referenced.add(key)
            row = mapping.get(key)
            if row is None or not row.get("indexed"):
                broken.append(
                    {"case_key": case["case_key"], "corpus_item_key": key, "layer": case["classification"]["primary_layer"]}
                )

    # manual review sample: stratified per primary layer
    rng = random.Random(SEED)
    by_layer: Dict[str, List[Dict[str, Any]]] = {}
    for case in cases:
        by_layer.setdefault(case["classification"]["primary_layer"], []).append(case)
    sample: List[Dict[str, Any]] = []
    for layer in sorted(by_layer):
        pool = list(by_layer[layer])
        rng.shuffle(pool)
        sample.extend(pool[:MANUAL_SAMPLE_PER_LAYER])

    injected_ok = sum(1 for r in mapping.values() if r.get("indexed"))
    report_path = os.path.join(corpus_dir, "golden-set", "reconciliation-report.md")
    with open(report_path, "w", encoding="utf-8") as fh:
        fh.write("# Golden Set 草稿 ↔ 注入清单对账报告（#291）\n\n")
        fh.write("> 生成：2026-09-03 ｜ AI 初标草稿对账材料，供用户复核；不构成冻结动作。\n\n")
        fh.write("## 1. 注入清单\n\n")
        fh.write(
            "- mapping 总条目：%d ｜ 注入成功（indexed）：%d ｜ 失败/未完成：%d\n"
            % (len(mapping), injected_ok, len(mapping) - injected_ok)
        )
        failed = [k for k, r in mapping.items() if not r.get("indexed")]
        if failed:
            fh.write("- 失败/未完成 keys（%d）：%s\n" % (len(failed), ", ".join(sorted(failed)[:40])))
        fh.write("\n## 2. golden set 引用对账\n\n")
        fh.write("- 草稿 case 总数：%d ｜ 引用的不同 corpus_item_key：%d\n" % (len(cases), len(referenced)))
        fh.write("- 引用失败/未注入条目的 case 数：%d\n" % len({b["case_key"] for b in broken}))
        if broken:
            fh.write("\n| case_key | 层 | 引用的问题 key |\n|---|---|---|\n")
            for b in broken[:30]:
                fh.write("| %s | %s | %s |\n" % (b["case_key"], b["layer"], b["corpus_item_key"]))
            fh.write(
                "\n**处置**：以上 case 需用 `golden_set_draft.py --mapping <mapping.jsonl>` 重生成"
                "（选样池过滤为成功清单），即自动完成替换。\n"
            )
        else:
            fh.write("- 结论：草稿全部引用落在注入成功清单内，无需替换。\n")
        fh.write("\n## 3. 分层统计（草稿）\n\n")
        layers = Counter(c["classification"]["primary_layer"] for c in cases)
        langs = Counter(c["query_language"] for c in cases)
        cold = sum(1 for c in cases if c["classification"].get("temperature_band") == "cold")
        colloquial = sum(1 for c in cases if c["classification"].get("colloquial_recommendation"))
        scoped = sum(1 for c in cases if c["classification"].get("ip_scope") == "ip_localized")
        fh.write(
            "- 分层：%s\n- 语言：%s\n- 冷门带：%d（下限 32）\n- 口语化推荐子层：%d（下限 30）\n- IP 内搜索用例（#290）：%d（覆盖 %d IP）\n"
            % (
                dict(layers),
                dict(langs),
                cold,
                colloquial,
                scoped,
                len({c["classification"]["ip"] for c in cases if c["classification"]["ip"]}),
            )
        )
        fh.write("\n## 4. 人工复核抽样清单（20 条，分层各 5）\n\n")
        fh.write("复核动作：逐条判断「问题与期望/禁止配对是否合理」；语义类另确认改写自然（规则 5）。\n\n")
        fh.write("| # | case_key | 层 | 问题 | 期望（keys） | 禁止（keys） | 出题理由 |\n|---|---|---|---|---|---|---|\n")
        for i, case in enumerate(sample, 1):
            expected = ", ".join(case.get("expected_citation_keys", [])) or "—"
            forbidden = ", ".join(case.get("forbidden_keys", [])) or "—"
            rubric = case["answer_rubric"].get("ideal_answer", "")
            fh.write(
                "| %d | %s | %s | %s | %s | %s | %s |\n"
                % (i, case["case_key"], case["classification"]["primary_layer"], case["query"][:50], expected, forbidden, rubric[:60])
            )
    print("reconciliation report -> %s (broken refs: %d)" % (report_path, len({b["case_key"] for b in broken})))


if __name__ == "__main__":
    main()
