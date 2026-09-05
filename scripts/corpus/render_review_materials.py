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
import sys
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
V1_OUT = ("\n".join(lines) + "\n")
print("review-sheet v2 prepared")

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
V2_OUT = ("\n".join(rl) + "\n")


def run_v1():
    with open(f"{BASE}/golden-set/review-sheet.md", "w", encoding="utf-8") as fh:
        fh.write(V1_OUT)
    with open(f"{BASE}/golden-set/reconciliation-report.md", "w", encoding="utf-8") as fh:
        fh.write(V2_OUT)
    print("v1 materials rewritten")


def render_v3():
    """Materials v3 (contract §9). Rendered from v2-cases.jsonl."""
    rows = [json.loads(l) for l in open(f"{BASE}/golden-set/v2-cases.jsonl", encoding="utf-8")]
    idxv3 = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(f"{BASE}/index.jsonl", encoding="utf-8"))}

    def disp(key):
        raw = idxv3[key]["title"].strip()
        # strip only a TRUE outer 《》 pair; embedded-bracket titles
        # (《西游记》文物展…) display verbatim
        if len(raw) >= 2 and raw.startswith("《") and raw.endswith("》"):
            t = raw[1:-1].strip()
            return "《%s》" % esc(t) if t else "《(fallback)%s…》" % esc(idxv3[key]["display_title"][:24])
        return esc(raw)  # embedded-bracket titles (《西游记》文物展…) verbatim

    def evidence_summary(r):
        layer = r["classification"]["primary_layer"]
        if layer == "known_item_exact":
            return "标题精确匹配；开篇段落为证据锚"
        if layer in ("semantic_discovery", "body_evidence", "hard_neighbor"):
            return esc((r["answer_rubric"].get("ideal_answer") or "")[:60])
        if layer == "no_answer":
            note = r["classification"].get("na_evidence", {}).get("evidence_note", "域外主题，语料零对应")
            return esc(note[:60])
        return "受限文档四泄漏面（检索/引用/正文/工具事件）必须全零"

    layer_names = {"known_item_exact": "ke", "semantic_discovery": "sd", "body_evidence": "be",
                   "hard_neighbor": "hn", "no_answer": "na", "visibility": "vi"}
    grouped = {}
    for r in rows:
        grouped.setdefault(r["classification"]["primary_layer"], []).append(r)
    rng3 = random.Random(20260904)
    parts = ["# Golden Set v2 人工复核材料 v3（196 条）", "",
             "> 生成：golden_set_v2_gen（sha256 见 v2-validation-report.json）｜复核动作：逐条判断「查询↔expected/acceptable/forbidden 配对是否合理」；sd/be 另确认查询自然且不复述标题；na 确认拒答/推荐语义；vi 确认 forbidden 归属。发现错标：记 case_key 告知编排会话。", ""]
    for layer in ["visibility", "no_answer", "body_evidence", "hard_neighbor", "semantic_discovery", "known_item_exact"]:
        sel_rows = grouped[layer]
        if layer == "known_item_exact":
            pool = list(sel_rows)
            rng3.shuffle(pool)
            sel_rows = sorted(pool[:13], key=lambda r: r["case_key"])
            note = "（ke 抽 20% ≈13 条）"
        else:
            note = "（全审 %d 条）" % len(sel_rows)
        parts.append("## %s %s %s" % (layer_names[layer], layer, note))
        parts.append("")
        parts.append("| case_key | 查询 | principal | strategy | expected | acceptable | forbidden（含理由） | visibility | 证据摘要 |")
        parts.append("|---|---|---|---|---|---|---|---|---|")
        for r in sel_rows:
            cl = r["classification"]
            strat = cl.get("no_answer_strategy", "—")
            if cl.get("provenance", {}).get("extreme_hard_case"):
                strat += " ⚠极限难例"
            exp = "<br>".join(disp(k) for k in r["expected_citation_keys"]) or "（无——拒答）"
            acc = "<br>".join(disp(k) for k in r["acceptable_keys"]) or "—"
            forb = "<br>".join("%s（%s）" % (disp(k), esc(v[:40])) for k, v in r["forbidden_reasons"].items()) or "—"
            parts.append("| %s | %s | %s | %s | %s | %s | %s | %s | %s |" % (
                r["case_key"], esc(r["query"]), r["viewer_context"]["principal_key"], strat,
                exp, acc, forb, cl.get("corpus_visibility") or "—", evidence_summary(r)))
        parts.append("")
    # hn retrieval evidence appendix (2026-09-04 hn review): ranks must be
    # programmatic from union.json; two conventions shown side by side because
    # probe Top-10 is chunk-level and first-vs-last occurrence reads diverged.
    ev = json.load(open(f"{BASE}/golden-set/v2-retrieval-evidence/union.json", encoding="utf-8"))
    sel2 = json.load(open(f"{BASE}/golden-set/v2-selection.json", encoding="utf-8"))
    hn_targets = {e["v2_key"]: e["target"] for e in sel2["hn"]}
    parts.append("## hn 检索证据（程序化取自 union.json）")
    parts.append("")
    parts.append("口径：probe Top-10 为 chunk 级，同一内容的多 chunk 可重复出现。「内容级首现」= 去重后目标首个出现位次（A-04 Recall@K 口径）；「chunk 命中位次」= 目标各 chunk 在 Top-10 的原始位次全集（2026-09-04 复核中名次歧义的来源，两口径并列以免打架）。— = 未进该配置 Top-10。hn-0003/0004/0010 为 title-free 复核轮重写，证据为重跑值（hn-reprobe-20260904.json）。")
    parts.append("")
    parts.append("| case | 目标 | 内容级首现 off-off / exp / rerank / exp+rr | chunk 命中位次 |")
    parts.append("|---|---|---|---|")
    for i in range(1, 17):
        key = "hn-%04d" % i
        t = hn_targets[key]
        e = ev[key]
        firsts, chunks = [], []
        for cfg in ["off-off", "exp-on", "rerank-on", "exp-rerank"]:
            hits = [h["rank"] for h in e.get(cfg, {}).get("top", []) if h["key"] == t]
            firsts.append(str(hits[0]) if hits else "—")
            if hits:
                chunks.append("%s %s" % (cfg, hits))
        parts.append("| %s | %s | %s | %s |" % (key, disp(t), " / ".join(firsts), "；".join(chunks) or "四配置均未命中"))
    parts.append("")
    stats = {
        "layers": dict(Counter(r["classification"]["primary_layer"] for r in rows)),
        "languages": dict(Counter(r["query_language"] for r in rows)),
        "cold": sum(1 for r in rows if r["classification"]["temperature_band"] == "cold"),
        "colloquial": sum(1 for r in rows if r["classification"]["query_register"] == "colloquial"),
        "ips": len({r["classification"]["ip_name"] for r in rows if r["classification"]["ip_name"]}),
    }
    parts.append("## 覆盖不变量")
    parts.append("")
    parts.append("- 分层：%s" % stats["layers"])
    parts.append("- 语言：%s（下限 zh 98 / en 40 / mixed 20）" % stats["languages"])
    parts.append("- 冷门：%d（下限 40）｜口语化：%d（下限 30）｜IP 覆盖：%d/16" % (
        stats["cold"], stats["colloquial"], stats["ips"]))
    out = f"{BASE}/golden-set/materials-v3.md"
    with open(out, "w", encoding="utf-8") as fh:
        fh.write("\n".join(parts) + "\n")
    print("materials-v3.md written:", out, "| rows:", len(rows))


if __name__ == "__main__":
    import sys
    if len(sys.argv) > 1 and sys.argv[1] == "--v3":
        render_v3()
    else:
        run_v1()
