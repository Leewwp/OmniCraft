#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Build the old(gs-c2-*) -> v2(ke/sd/be/hn/na/vi-*) migration map (#291).

Deterministic; disposition lists come from the 2026-09-04 audit
(docs/working/2026-09-04-golden-set-audit-report.md, verified twice).
Outputs: artifacts/corpus-v2/golden-set/migration-map-v2.{md,jsonl}.
Contract: docs/working/2026-09-04-golden-set-v2-annotation-spec.md.
Verify with scripts/corpus/verify_migration_map.py.
"""
from __future__ import annotations

import json
import os
import re

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
BASE = os.path.join(REPO, "artifacts", "corpus-v2")
SPEC = "docs/working/2026-09-04-golden-set-v2-annotation-spec.md"

EXACT_SWAP = {"gs-c2-0001", "gs-c2-0010", "gs-c2-0011", "gs-c2-0013", "gs-c2-0014", "gs-c2-0015",
              "gs-c2-0016", "gs-c2-0018", "gs-c2-0024", "gs-c2-0025", "gs-c2-0029", "gs-c2-0032",
              "gs-c2-0038", "gs-c2-0039", "gs-c2-0040", "gs-c2-0041", "gs-c2-0045", "gs-c2-0048",
              "gs-c2-0052"}
SEMANTIC_SWAP = {"gs-c2-0066", "gs-c2-0069", "gs-c2-0076", "gs-c2-0082", "gs-c2-0086",
                 "gs-c2-0096", "gs-c2-0103", "gs-c2-0105", "gs-c2-0106", "gs-c2-0110"}
NA_KEEP = ["gs-c2-0137", "gs-c2-0138", "gs-c2-0139", "gs-c2-0140", "gs-c2-0142", "gs-c2-0143",
           "gs-c2-0144", "gs-c2-0145", "gs-c2-0148", "gs-c2-0149", "gs-c2-0150", "gs-c2-0155"]
NA_STRICT = {"gs-c2-0138", "gs-c2-0149"}  # Excel 修仙 / 量子修仙：how-to 事实型 → strict

V2_QUOTA = {"known_item_exact": 64, "semantic_discovery": 48, "body_evidence": 24,
            "hard_neighbor": 16, "no_answer": 20, "visibility": 24}

norm_q = lambda q: re.sub(r"^anyone knows:\s*", "", q.strip())


def load():
    idx = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(f"{BASE}/index.jsonl", encoding="utf-8"))}
    cases = [json.loads(l) for l in open(f"{BASE}/golden-set/golden-cases-draft.jsonl", encoding="utf-8")]
    return idx, cases


def swap_pool(idx, used_expected, ip, forms):
    return sum(1 for r in idx.values()
               if r["ip"] == ip and r["visibility"] == "public"
               and r["title_form"] in forms and r["title"].strip()
               and r["corpus_item_key"] not in used_expected)


def build_rows(idx, cases):
    used_expected = {k for c in cases for k in c["expected_citation_keys"]}
    by_key = {c["case_key"]: c for c in cases}
    rows = []  # (old_key, old_layer, disposition, v2_key, v2_layer, note)

    n = 0
    for c in [x for x in cases if x["classification"]["primary_layer"] == "exact"]:
        n += 1
        v2 = f"ke-{n:04d}"
        if c["case_key"] in EXACT_SWAP:
            pool = swap_pool(idx, used_expected, c["classification"]["ip"], {"exact"})
            note = f"换同IP public title_form=exact 目标（候选池 {pool} 条），查询随新标题重生成" + \
                   ("；保留「在IP内：」前缀" if c["classification"]["ip_scope"] == "ip_localized" else "")
            rows.append((c["case_key"], "exact", "target_swap", v2, "known_item_exact", note))
        else:
            rows.append((c["case_key"], "exact", "keep", v2, "known_item_exact", "查询与目标原样保留"))
        if ":" in c["query"]:
            rows[-1] = rows[-1][:5] + (rows[-1][5] + "；ASCII冒号（#319 已修，回归观测子集）",)

    n = 0
    for c in [x for x in cases if x["classification"]["primary_layer"] == "semantic"]:
        n += 1
        v2 = f"sd-{n:04d}"
        if c["case_key"] in SEMANTIC_SWAP:
            pool = swap_pool(idx, used_expected, c["classification"]["ip"], {"fuzzy", "partial"})
            rows.append((c["case_key"], "semantic", "target_swap+rewrite", v2, "semantic_discovery",
                         f"先换同IP public fuzzy/partial 目标（候选池 {pool} 条）再藏标题重写+三档标注"))
        else:
            rows.append((c["case_key"], "semantic", "rewrite", v2, "semantic_discovery",
                         "藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档"))

    n = 0
    for c in [x for x in cases if x["classification"]["primary_layer"] == "visibility"]:
        n += 1
        rows.append((c["case_key"], "visibility", "keep+principal", f"vi-{n:04d}", "visibility",
                     "保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零"))

    keep_map = {}
    n = 0
    for old in NA_KEEP:
        n += 1
        v2 = f"na-{n:04d}"
        keep_map[old] = v2
        strategy = "strict_not_found" if old in NA_STRICT else "related_recommendation_allowed"
        rows.append((old, "no_answer", "keep+strategy", v2, "no_answer",
                     f"策略={strategy}；干扰项重抽为真·近邻（人工判）"))
    for c in [x for x in cases if x["classification"]["primary_layer"] == "no_answer"]:
        if c["case_key"] in keep_map:
            continue
        twin = next(k for k in NA_KEEP if norm_q(by_key[k]["query"]) == norm_q(c["query"]))
        rows.append((c["case_key"], "no_answer", "drop(merged)", keep_map[twin], "no_answer",
                     f"与 {keep_map[twin]}（{twin}）同题，合并删除"))
    for i in range(1, 9):
        rows.append(("-", "-", "new", f"na-{12 + i:04d}", "no_answer",
                     f"域内新增 {i}/8：IP 内合理主题、检索验证零精确对应；策略按查询形态定"))

    for i in range(1, 25):
        rows.append(("-", "-", "new", f"be-{i:04d}", "body_evidence",
                     "正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间"))
    for i in range(1, 17):
        rows.append(("-", "-", "new", f"hn-{i:04d}", "hard_neighbor",
                     "同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档"))
    return rows


def main():
    idx, cases = load()
    rows = build_rows(idx, cases)
    assert len([r for r in rows if r[0] != "-"]) == 160, "old cases must map 1:1"

    with open(f"{BASE}/golden-set/migration-map-v2.jsonl", "w", encoding="utf-8") as fh:
        for old, oldl, disp, v2, v2l, note in rows:
            fh.write(json.dumps({"old_case_key": old, "old_layer": oldl, "disposition": disp,
                                 "v2_case_key": v2, "v2_layer": v2l, "note": note},
                                ensure_ascii=False, separators=(",", ":")) + "\n")

    from collections import Counter
    cnt = Counter((r[1], r[2]) for r in rows)
    md = ["# Golden Set v1 → v2 全量迁移映射（196 条方案）", "",
          f"> 生成：2026-09-04 ｜ 由 `scripts/corpus/build_migration_map.py` 确定性生成（处置清单来自已双重复核的审计报告 v1.2）｜ 配套规范：`{SPEC}`（v2.0-draft-r2）｜ 验证：`scripts/corpus/verify_migration_map.py` ｜ 待用户确认规范后执行",
          "", f"处置汇总：{dict(cnt)}", "",
          "| 旧 case_key | 旧层 | 处置 | v2 key | v2 层 | 说明 |", "|---|---|---|---|---|---|"]
    for old, oldl, disp, v2, v2l, note in rows:
        md.append(f"| {old} | {oldl} | {disp} | {v2} | {v2l} | {note} |")
    with open(f"{BASE}/golden-set/migration-map-v2.md", "w", encoding="utf-8") as fh:
        fh.write("\n".join(md) + "\n")
    print("rows:", len(rows), "| dispositions:", dict(cnt))


if __name__ == "__main__":
    main()
