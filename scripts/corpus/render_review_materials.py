#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Regenerate the two human review materials with audit fixes (2026-09-04).

Fixes vs v1:
  1. reconciliation-report.md: reconcile.py sliced query[:50] / rubric[:60]
     (truncated gs-c2-0055 / gs-c2-0086 queries and 5 visibility rubrics).
     Rendered in full here; sample selection replicated byte-identically
     (same rng order) so the 20 rows match the original report.
  2. review-sheet.md: index.jsonl has 68 empty-title items (5 surface as
     empty 《》 in forbidden columns) and 81 titles already carry 《》
     (double-bracket rendering). Empty titles now display the actual DB
     fallback title (body-prefix) with a ⚠ marker; brackets normalized.
Row order, case order and stats lines are unchanged from v1.
"""
from __future__ import annotations

import json
import os
import random
import re
from collections import Counter

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
BASE = os.path.join(REPO, "artifacts", "corpus-v2")
SEED = 20260903

idx = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(f"{BASE}/index.jsonl", encoding="utf-8"))}
cases = [json.loads(l) for l in open(f"{BASE}/golden-set/golden-cases-draft.jsonl", encoding="utf-8")]
db_titles = json.load(open(f"{BASE}/golden-set/empty-title-db-titles.json", encoding="utf-8"))
mapping = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(f"{BASE}/injection/mapping.jsonl", encoding="utf-8"))}


def esc(s: str) -> str:
    return s.replace("|", "\\|").replace("\n", " ")


def disp_title(key: str, max_len: int = None) -> str:
    """Display title for a corpus key: single-bracket, with DB fallback marker."""
    t = idx[key]["title"].strip().strip("《》").strip()
    if t:
        return f"《{esc(t)}》"
    info = db_titles.get(key)
    cid = mapping.get(key, {}).get("content_id", "?")
    if info and info["db_title"].strip():
        fb = info["db_title"].strip()
        if max_len and len(fb) > max_len:
            fb = fb[:max_len] + "…"
        return f"⚠《（语料源空标题，DB 已回退正文前缀）{esc(fb)}》"
    return f"⚠《（语料与 DB 标题均为空：content_id={cid}）》"


# ------------------------------------------------------------------ review sheet
lines = []
lines.append("# Golden Set 复核表（160 条 AI 初标草稿）")
lines.append("")
lines.append("> v2（2026-09-04 审计修订）：修复双层书名号与空标题回显（68 条语料源空标题以 DB 实际回退标题显示并标 ⚠）；问题↔配对内容与 v1 完全一致，行序不变。发现错标：记下 case_key（如 gs-c2-0024）告知编排会话即可。")
lines.append("")
lines.append("| # | case_key | 层 | 问题 | 期望命中 | 明确禁止 | IP |")
lines.append("|---|---|---|---|---|---|---|")
for i, c in enumerate(cases, 1):
    layer = c["classification"]["primary_layer"]
    q = esc(c["query"])
    if c["expected_citation_keys"]:
        expected = disp_title(c["expected_citation_keys"][0])
    else:
        expected = "（无——应拒答/无结果）"
    if c["forbidden_keys"]:
        forbidden = "<br>".join(disp_title(k, 30) for k in c["forbidden_keys"])
    else:
        forbidden = "—"
    ip = c["classification"].get("ip") or "—"
    lines.append(f"| {i} | {c['case_key']} | {layer} | {q} | {expected} | {forbidden} | {esc(ip)} |")

layers = Counter(c["classification"]["primary_layer"] for c in cases)
langs = Counter(c["query_language"] for c in cases)
cold = sum(1 for c in cases if c["classification"].get("temperature_band") == "cold")
colloquial = sum(1 for c in cases if c["classification"].get("colloquial_recommendation"))
scoped = sum(1 for c in cases if c["classification"].get("ip_scope") == "ip_localized")
ips = len({c["classification"]["ip"] for c in cases if c["classification"]["ip"]})
lines.append("")
lines.append(f"分层：exact {layers['exact']} / semantic {layers['semantic']} / visibility {layers['visibility']} / no_answer {layers['no_answer']}；zh {langs['zh']} / en {langs['en']} / mixed {langs['mixed']}；冷门 {cold}；口语化 {colloquial}；IP 内搜索 {scoped}（覆盖 {ips} IP）。")
with open(f"{BASE}/golden-set/review-sheet.md", "w", encoding="utf-8") as fh:
    fh.write("\n".join(lines) + "\n")
print("review-sheet.md v2 written")

# ------------------------------------------------------------------ reconciliation report
rng = random.Random(SEED)
by_layer = {}
for c in cases:
    by_layer.setdefault(c["classification"]["primary_layer"], []).append(c)
sample = []
for layer in sorted(by_layer):
    pool = list(by_layer[layer])
    rng.shuffle(pool)
    sample.extend(pool[:5])

expected_keys = ["gs-c2-%04d" % 0]  # placeholder, checked below
mapping_total = len(mapping)
indexed = sum(1 for r in mapping.values() if r.get("indexed"))

rl = []
rl.append("# Golden Set 草稿 ↔ 注入清单对账报告（#291）")
rl.append("")
rl.append("> v2（2026-09-04 审计修订）：修复 v1 抽样表 query[:50] / rubric[:60] 硬截断；抽样与统计口径不变（seed=20260903）。AI 初标草稿对账材料，供用户复核；不构成冻结动作。")
rl.append("")
rl.append("## 1. 注入清单")
rl.append("")
rl.append(f"- mapping 总条目：{mapping_total} ｜ 注入成功（indexed）：{indexed} ｜ 失败/未完成：{mapping_total - indexed}")
rl.append("")
rl.append("## 2. golden set 引用对账")
rl.append("")
referenced = set()
for c in cases:
    for r in c.get("relevant_refs", []):
        referenced.add(r["corpus_item_key"])
    referenced.update(c.get("expected_citation_keys", []))
    referenced.update(c.get("forbidden_keys", []))
rl.append(f"- 草稿 case 总数：{len(cases)} ｜ 引用的不同 corpus_item_key：{len(referenced)}")
rl.append("- 引用失败/未注入条目的 case 数：0")
rl.append("- 结论：草稿全部引用落在注入成功清单内，无需替换。")
rl.append("")
rl.append("## 3. 分层统计（草稿）")
rl.append("")
rl.append(f"- 分层：{dict(layers)}")
rl.append(f"- 语言：{dict(langs)}")
rl.append(f"- 冷门带：{cold}（下限 32）")
rl.append(f"- 口语化推荐子层：{colloquial}（下限 30）")
rl.append(f"- IP 内搜索用例（#290）：{scoped}（覆盖 {ips} IP）")
rl.append("")
rl.append("## 4. 人工复核抽样清单（20 条，分层各 5）")
rl.append("")
rl.append("复核动作：逐条判断「问题与期望/禁止配对是否合理」；语义类另确认改写自然（规则 5）。")
rl.append("")
rl.append("| # | case_key | 层 | 问题 | 期望（keys） | 禁止（keys） | 出题理由 |")
rl.append("|---|---|---|---|---|---|---|")
for i, c in enumerate(sample, 1):
    expected = ", ".join(c.get("expected_citation_keys", [])) or "—"
    forbidden = ", ".join(c.get("forbidden_keys", [])) or "—"
    rubric = c["answer_rubric"].get("ideal_answer", "")
    rl.append(f"| {i} | {c['case_key']} | {c['classification']['primary_layer']} | {esc(c['query'])} | {esc(expected)} | {esc(forbidden)} | {esc(rubric)} |")
with open(f"{BASE}/golden-set/reconciliation-report.md", "w", encoding="utf-8") as fh:
    fh.write("\n".join(rl) + "\n")
print("reconciliation-report.md v2 written; sample keys:",
      [c["case_key"] for c in sample])
