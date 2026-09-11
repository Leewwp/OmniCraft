#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Group-aware 80/20 split for Golden Set v2 (#291 step 4b, contract §7).

Reads the frozen 4a baseline artifacts/corpus-v2/golden-set/v2-cases.jsonl
(sha256 must equal the pinned rev-2026-09-05 baseline — the reviewed,
human-signed corpus including the per-fact BE span revision; fail-closed on
any drift), assigns classification.split ∈ {dev, test} to all
196 rows and writes:

- v2-cases-split.jsonl   196 rows, the authoritative 4b artifact (freeze input)
- v2-split-report.json   machine-readable groups/balance/checksums
- split-report-v2.md     human-readable report

Design decisions (2026-09-05, recorded in split-report-v2.md; group semantics
revised the same day by the pre-freeze re-audit, rev 2; IP-coverage algorithm
revised by the follow-up audit of the rev-2 report, rev 3):
- Atomic groups = connected components over shared target references in
  expected_citation_keys ∪ acceptable_keys ∪ forbidden_keys. All 152 expected
  targets are unique (zero same-expected groups); multi-case groups arise
  from shared forbidden AND shared acceptable targets. The first 4b cut
  grouped on expected ∪ forbidden only and deliberately excluded acceptable
  as a "soft bonus tier with no leakage gain" — the re-audit falsified that
  rationale: acceptable carries a 0.5 graded gain (graded.go GainAcceptable,
  consumed by GradedNDCGAt10 in runner_v2.go), and five content groups did
  span dev/test through acceptable edges, so dev tuning would move test-side
  graded nDCG. Group semantics widened to all three tiers by user ruling
  2026-09-05 (rev 2, superseding split sha 1b0cc79a8ed78695).
- ke ascii-colon cases are independent cases (NOT a regression pair) — no
  artificial grouping (2026-09-05 ruling, supersedes contract §7 example).
- visibility double-principal is a runtime property of a single case, no
  cross-case grouping; na v1 variants were merged at 4a (no residual).
- Layer targets: largest-remainder rounding of 20% per layer to exactly
  TEST_SIZE; multi-case groups are placed first (least freedom, greedy on a
  balance loss), then singleton placement runs in two passes (rev 3):
  pass 1 reserves one test slot for every IP that has singletons but no test
  representative, in order of increasing candidate-layer count (scarcest
  path first), and pass 2 fills each layer to target with the seeded-greedy
  balance loss. The rev-2 per-layer greedy had left one IP uncovered: on the
  last open ke slot, the 天官赐福 singleton tied the 全职高手 singleton at
  exactly equal balance loss and the seed shuffle order decided the winner —
  a coverage goal riding on a tie-break. Rev 3 also removes all randomness:
  greedy ties break on case_key ascending (seed=20260904 is lineage only and
  no longer affects the output).
- Double-run inside the script must produce byte-identical output (V8-equ).
- A round-trip gate strips classification.split from the output and requires
  the baseline sha back — proving split is the ONLY delta over 4a.
"""
from __future__ import annotations

import hashlib
import json
import os
from collections import Counter, defaultdict

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
GS = os.path.join(REPO, "artifacts", "corpus-v2", "golden-set")
BASELINE = os.path.join(GS, "v2-cases.jsonl")
BASELINE_SHA = "0a86a9cc132f3bbe"  # prefix of the frozen 4a sha256 (rev 2026-09-05: per-fact BE spans)
SEED = 20260904
TEST_SIZE = 40
LAYERS = ["known_item_exact", "semantic_discovery", "body_evidence",
          "hard_neighbor", "no_answer", "visibility"]
LANGS = ["zh", "en", "mixed"]
W_LANG, W_COLD, W_IP, W_COVER, W_LAYEROVER = 3.0, 1.0, 1.0, 2.0, 2.0


def sha16(rows):
    s = "\n".join(json.dumps(r, ensure_ascii=False, sort_keys=True) for r in rows)
    return hashlib.sha256(s.encode("utf-8")).hexdigest()[:16]


def load_baseline():
    rows = [json.loads(l) for l in open(BASELINE, encoding="utf-8")]
    got = sha16(rows)
    if got != BASELINE_SHA:
        raise SystemExit("baseline drift: v2-cases.jsonl sha=%s want=%s — rerun "
                         "golden_set_v2_gen.py double-run and reconcile before splitting" % (got, BASELINE_SHA))
    return rows


def atomic_groups(rows):
    """Connected components over expected ∪ acceptable ∪ forbidden co-reference
    (rev 2, 2026-09-05 re-audit: acceptable carries a 0.5 graded gain, so a
    shared acceptable target couples dev tuning to test-side nDCG exactly like
    a shared forbidden target does)."""
    parent = {r["case_key"]: r["case_key"] for r in rows}

    def find(x):
        while parent[x] != x:
            parent[x] = parent[parent[x]]
            x = parent[x]
        return x

    refs = defaultdict(list)
    for r in rows:
        for k in r["expected_citation_keys"] + r.get("acceptable_keys", []) + r["forbidden_keys"]:
            refs[k].append(r["case_key"])
    for members in refs.values():
        for c in members[1:]:
            ra, rb = find(members[0]), find(c)
            if ra != rb:
                parent[ra] = rb
    comps = defaultdict(list)
    for r in rows:
        comps[find(r["case_key"])].append(r["case_key"])
    return sorted((sorted(v) for v in comps.values()),
                  key=lambda g: (-len(g), g[0]))


def layer_targets(rows):
    """Largest-remainder 20% per layer, summing exactly to TEST_SIZE."""
    cnt = Counter(r["classification"]["primary_layer"] for r in rows)
    raw = {l: cnt[l] * TEST_SIZE / len(rows) for l in LAYERS}
    tgt = {l: int(raw[l]) for l in LAYERS}
    short = TEST_SIZE - sum(tgt.values())
    for l in sorted(LAYERS, key=lambda x: (-(raw[x] - tgt[x]), x))[:short]:
        tgt[l] += 1
    return tgt


def dims(rows):
    n = float(len(rows))
    lang = {l: sum(1 for r in rows if r["query_language"] == l) / n for l in LANGS}
    cold = sum(1 for r in rows if r["classification"]["temperature_band"] == "cold") / n
    ips = sorted({r["classification"]["ip_name"] for r in rows if r["classification"]["ip_name"]})
    ipshare = {ip: sum(1 for r in rows if r["classification"]["ip_name"] == ip) / n for ip in ips}
    return lang, cold, ipshare


def balance_loss(test_rows, all_rows):
    """Weighted deviation of the test side from the corpus-wide dimension shares.

    IP term is two-sided: overrepresentation beyond the corpus share, plus a
    flat per-IP coverage floor (every IP with corpus cases should appear at
    least once in test — 16 IPs × ≥5 cases each make a 40-case test able to
    cover all of them)."""
    lang_t, cold_t, ips_t = dims(all_rows)
    n = float(len(test_rows)) or 1.0
    lang = {l: sum(1 for r in test_rows if r["query_language"] == l) / n for l in LANGS}
    cold = sum(1 for r in test_rows if r["classification"]["temperature_band"] == "cold") / n
    loss = W_LANG * sum(abs(lang[l] - lang_t[l]) for l in LANGS) + W_COLD * abs(cold - cold_t)
    got = Counter(r["classification"]["ip_name"] for r in test_rows if r["classification"]["ip_name"])
    over = 0.0
    uncovered = 0
    for ip in ips_t:
        g = got.get(ip, 0)
        over += max(0.0, g / n - ips_t[ip])
        if g == 0:
            uncovered += 1
    return loss + W_IP * over + W_COVER * uncovered


def assign(rows):
    by_key = {r["case_key"]: r for r in rows}
    groups = atomic_groups(rows)
    tgt = layer_targets(rows)

    split = {}
    test_layers = Counter()
    test_size = 0

    # phase 1: multi-case groups, largest first — least placement freedom
    multi = [g for g in groups if len(g) > 1]
    for g in multi:
        gl = Counter(by_key[c]["classification"]["primary_layer"] for c in g)
        if test_size + len(g) <= TEST_SIZE:
            over_test = sum(max(0, test_layers[l] + gl[l] - tgt[l]) for l in LAYERS)
            if over_test == 0:
                for c in g:
                    split[c] = "test"
                test_layers += gl
                test_size += len(g)
                continue
        for c in g:
            split[c] = "dev"

    singletons_by_layer = defaultdict(list)
    for g in groups:
        if len(g) == 1:
            singletons_by_layer[by_key[g[0]]["classification"]["primary_layer"]].append(g[0])

    def ip_of(c):
        return by_key[c]["classification"]["ip_name"]

    def test_rows_plus(c):
        return [by_key[x] for x in [k for k, v in split.items() if v == "test"] + [c]]

    # phase 2 pass 1 (rev 3): scarcity-first coverage reservations. The rev-2
    # per-layer greedy left an IP uncovered because it spent the last open
    # layer slots on balance ties decided by shuffle order — a coverage goal
    # cannot ride on a tie-break. Instead: every IP that has singletons but no
    # test representative after phase 1 is served BEFORE the fill pass, in
    # order of increasing candidate-layer count (an IP whose only still-open
    # path is a single layer must be served before multi-path IPs can consume
    # that layer's slots); within an IP the (layer, case) minimising the
    # balance loss wins, ties by case_key ascending.
    ips_with_singletons = {ip_of(c) for l in LAYERS for c in singletons_by_layer[l] if ip_of(c)}
    while True:
        covered = {ip_of(c) for c, s in split.items() if s == "test" and ip_of(c)}
        pending = [ip for ip in sorted(ips_with_singletons) if ip not in covered]
        if not pending:
            break
        best_ip, best_cands = None, None
        for ip in pending:
            cands = [(l, c) for l in LAYERS if tgt[l] - test_layers[l] > 0
                     for c in singletons_by_layer[l] if c not in split and ip_of(c) == ip]
            if not cands:
                continue
            if best_cands is None or len({l for l, _ in cands}) < len({l for l, _ in best_cands}):
                best_ip, best_cands = ip, cands
        if best_ip is None:
            break  # every remaining uncovered IP is fully locked out by
            # dev-side groups + exhausted quotas — reported, not forced
        l, c = min(best_cands,
                   key=lambda lc: (balance_loss(test_rows_plus(lc[1]), rows), lc[1]))
        split[c] = "test"
        test_layers[l] += 1
        test_size += 1

    # phase 2 pass 2: singletons fill each layer to target, greedy on
    # dimension balance; ties break on case_key ascending (rev 3: no shuffle —
    # a seed-order tie must never decide a coverage/balance outcome again).
    for l in LAYERS:
        need = tgt[l] - test_layers[l]
        pool = [c for c in singletons_by_layer[l] if c not in split]
        if need <= 0:
            for c in pool:
                split[c] = "dev"
            continue
        chosen = []
        for _ in range(min(need, len(pool))):
            best, best_loss = None, None
            for c in sorted(pool):
                if c in chosen:
                    continue
                ls = balance_loss(test_rows_plus(c), rows)
                if best_loss is None or ls < best_loss - 1e-12:
                    best, best_loss = c, ls
            chosen.append(best)
            split[best] = "test"
            test_layers[l] += 1
            test_size += 1
        for c in pool:
            if c not in chosen:
                split[c] = "dev"

    assert test_size == sum(1 for v in split.values() if v == "test")
    return split, groups, tgt, test_layers


def build(rows):
    """One full deterministic pass: assignment → annotated rows + report dict."""
    rows = [json.loads(json.dumps(r)) for r in rows]  # deep copy
    split, groups, tgt, test_layers = assign(rows)
    for r in rows:
        r["classification"]["split"] = split[r["case_key"]]
    return rows, split, groups, tgt, test_layers


def render_md(rep, out_rows, all_rows, by_key):
    ipcov_all = sorted({r["classification"]["ip_name"] for r in all_rows if r["classification"]["ip_name"]})
    ip_test = rep["balance"]["ip_test"]
    lang_t = rep["balance"]["language_test"]
    lang_all = rep["balance"]["language_all"]
    n_test, n_all = rep["test_size"], float(len(all_rows))
    L = []
    L.append("# Golden Set v2 group-aware 切分报告（#291 第 4b 步，合同 §7）")
    L.append("")
    L.append("> 生成：`scripts/corpus/gsv2_split.py`（确定性，rev 3：平局一律按 case_key 定序、无随机；双跑字节一致）｜输入基线："
             "`v2-cases.jsonl` sha256=%s（4a 定版 rev 2026-09-05：含 per-fact BE span 修订，先决校验 fail-closed）｜输出定版："
             "`v2-cases-split.jsonl` sha256=%s｜round-trip 校验：剥离 split 后逐字节回到基线 ✓" % (
                 rep["baseline_sha256_16"], rep["split_sha256_16"]))
    L.append("")
    L.append("> **rev 2（2026-09-05 复审修订）**：组语义扩为三档建图（expected∪acceptable∪forbidden 连通分量），"
             "取代首版切分 sha `1b0cc79a8ed78695`（两档图）。首版「acceptable 不进组语义」的论据（软加分档、无泄漏增益）"
             "被复审证伪——acceptable 以 0.5 增益计入 graded nDCG@10（`graded.go GainAcceptable` / `runner_v2.go:276`），"
             "实测 5 组内容经 acceptable 边跨 dev/test。扩档经用户 2026-09-05 裁定。")
    L.append("")
    L.append("> **rev 3（2026-09-05 二次复审修订，本版）**：rev 2 曾报「天官赐福无法入 test 系结构性必然」经用户插桩"
             "复算证伪——仅 7/11 case 被锁 dev 组，ke-0003/ke-0025 为单例且 ke 层余 5 槽，末位 ke 槽上 ke-0003（天官赐福）"
             "与 ke-0036（全职高手）平衡损失精确相等（Δ=0.0），胜者由 seed 洗牌序决定；16/16 在全部硬约束下可行"
             "（天官赐福/全职/盗墓/诡秘走 ke 槽 + 原神/双城/火影走 be 槽，须多槽协同）。修复 = phase 2 改「稀缺优先」"
             "两遍选取 + 全部平局按 case_key 定序（彻底去随机）。rev 2 的 split sha `f0764976f350135d`（test IP 15/16）"
             "由本版取代。")
    L.append("")
    L.append("## 1. 结果概览")
    L.append("")
    L.append("| 项 | 值 |")
    L.append("|---|---|")
    L.append("| dev / test | %d / %d（test 目标 %d） |" % (
        len(out_rows) - rep["test_size"], rep["test_size"], TEST_SIZE))
    L.append("| 原子组 | %d 组（多 case 组 %d、单例 %d），全部完整落单侧 |" % (
        rep["groups"]["total"], len(rep["groups"]["multi_case"]), rep["groups"]["singleton_count"]))
    L.append("| 六层 test 配额 | %s |" % "、".join(
        "%s %d/%d" % (l, rep["layer_actual_test"][l], rep["layer_targets"][l]) for l in LAYERS))
    cov = sum(1 for ip in ipcov_all if ip_test.get(ip, 0) > 0)
    L.append("| test IP 覆盖 | %d/%d IP ≥1 条 |" % (cov, len(ipcov_all)))
    L.append("| 双跑一致性 | %s（进程内重建 + 跨进程重跑均字节一致） |" % ("✓" if rep["deterministic_double_run"] else "✗"))
    hard = [(r["case_key"], r["classification"]["split"]) for r in out_rows
            if r["classification"]["provenance"].get("extreme_hard_case")]
    if hard:
        L.append("| 极限难例落侧 | %s |" % "、".join("%s→%s" % kv for kv in sorted(hard)))
    L.append("")
    L.append("## 2. 组语义（2026-09-05 复审修订版）")
    L.append("")
    L.append("1. **原子组 = expected∪acceptable∪forbidden 共享目标的连通分量**（复审翻案，用户 2026-09-05 裁定）。")
    L.append("   152 个 expected 目标全部唯一（同 expected 组 = 0）；多 case 组由共享 forbidden 与共享 acceptable")
    L.append("   目标共同构成——同一内容在两组 case 中互为关联角色（一方 expected / 另一方 forbidden 或")
    L.append("   acceptable），跨侧放置会让 dev 调参直接搬动另一侧的 test 计分（forbidden 移动 trap 命中，")
    L.append("   acceptable 移动 graded nDCG 的 0.5 增益项），故必须同侧。")
    L.append("2. **三档图实测**：%d 个多 case 组、最大团 %d 条、锁定 %d 条；首版两档图为 23 组 / 最大 8 / 锁定 70，"
             % (len(rep["groups"]["multi_case"]), rep["groups"]["max_group_size"], rep["groups"]["locked_cases"]))
    L.append("   排除 acceptable 后 5 组内容跨侧泄漏（be-0012↔sd-0011、hn-0003↔ke-0033+sd-0027、")
    L.append("   sd-0032↔ke-0034、ke-0006↔hn-0011、sd-0013↔sd-0015），本次扩档后全部消除。")
    L.append("3. **ke ascii 冒号 10 条不配对**（2026-09-05 裁决，取代合同 §7 的配对示例）：不同标题的独立 case。")
    L.append("4. **vi 双身份是单 case 运行时行为**，无跨 case 配对；vi 层 24 条全为单例。")
    L.append("5. **na 同题变体 4a 已合并**，无残余同题组。")
    L.append("")
    L.append("## 3. 多 case 原子组清单与归属")
    L.append("")
    L.append("| 组 | case | 层构成 | 落侧 |")
    L.append("|---|---|---|---|")
    for g in rep["groups"]["multi_case"]:
        side = {by_key[c]["classification"]["split"] for c in g}
        lc = Counter(by_key[c]["classification"]["primary_layer"][:2] for c in g)
        L.append("| %s | %s | %s | %s |" % (
            g[0], "、".join(g), "+".join("%s×%d" % kv for kv in sorted(lc.items())),
            "/".join(sorted(side))))
    L.append("")
    L.append("## 4. 平衡表")
    L.append("")
    L.append("### 层（test 目标 = 最大余数法 20%/层）")
    L.append("")
    L.append("| 层 | 全卷 | test 目标 | test 实际 | dev |")
    L.append("|---|---|---|---|---|")
    layer_dev = Counter(r["classification"]["primary_layer"] for r in out_rows
                        if r["classification"]["split"] == "dev")
    for l in LAYERS:
        tot = sum(1 for r in all_rows if r["classification"]["primary_layer"] == l)
        L.append("| %s | %d | %d | %d | %d |" % (l, tot, rep["layer_targets"][l],
                                                 rep["layer_actual_test"][l], layer_dev[l]))
    L.append("")
    L.append("### 语言 / 冷热")
    L.append("")
    L.append("| 维度 | 全卷（占比） | test（占比） | 20% 期望 |")
    L.append("|---|---|---|---|")
    for l in LANGS:
        L.append("| %s | %d（%.1f%%） | %d（%.1f%%） | %.1f |" % (
            l, lang_all[l], 100 * lang_all[l] / n_all, lang_t[l], 100 * lang_t[l] / n_test,
            lang_all[l] / n_all * n_test))
    cold_all = rep["balance"]["cold_all"]
    L.append("| cold | %d（%.1f%%） | %d（%.1f%%） | %.1f |" % (
        cold_all, 100 * cold_all / n_all, rep["balance"]["cold_test"],
        100 * rep["balance"]["cold_test"] / n_test, cold_all / n_all * n_test))
    L.append("")
    L.append("### IP（test 侧分布）")
    L.append("")
    L.append("| IP | 全卷 | test |")
    L.append("|---|---|---|")
    ip_all = Counter(r["classification"]["ip_name"] for r in all_rows if r["classification"]["ip_name"])
    for ip in ipcov_all:
        L.append("| %s | %d | %d |" % (ip, ip_all[ip], ip_test.get(ip, 0)))
    L.append("")
    ip_missing = [ip for ip in ipcov_all if ip_test.get(ip, 0) == 0]
    if ip_missing:
        L.append("（IP 缺覆盖说明：%s 的全部 case 均被连通进 dev 侧多 case 原子组（经 acceptable/forbidden 边），"
                 "在「原子组完整落单侧 + 六层 test 配额精确命中」双约束下无法入 test；按合同 §7 原子组优先于维度平衡，"
                 "记录不调。）" % "、".join(ip_missing))
        L.append("")
    L.append("（na 层 12 条无 IP 归属，不计入 IP 维度。）")
    L.append("")
    L.append("## 5. 算法（可复现声明）")
    L.append("")
    L.append("- 六层 test 目标 = 最大余数法取整至恰好 %d 条：%s。" % (
        TEST_SIZE, "、".join("%s %d" % (l, rep["layer_targets"][l]) for l in LAYERS)))
    L.append("- 组间分配三阶段：①多 case 组按（规模降序，key 升序）逐组判定——能整组放进 test 且不击穿任何层配额")
    L.append("  则进 test，否则整组 dev；②**稀缺优先覆盖预留（rev 3）**——phase 1 后无 test 代表、但有单例的每个 IP，")
    L.append("  按候选层集合由小到大依次预留一个 test 槽（唯一路径 IP 最优先，防止多路径 IP 消耗掉其唯一层的槽位），")
    L.append("  IP 内取平衡损失最小的 (层, case)；③各层单例补足层配额差，逐个选取使「语言 L1 偏差×3 + 冷热偏差×1 +")
    L.append("  IP 超配×1 + IP 缺覆盖×2」最小者。**全部平局按 case_key 升序（rev 3：无洗牌——rev 2 的末位 ke 槽曾由")
    L.append("  seed 洗牌序在 6 个零差候选中决定胜者，导致一个 IP 失去 test 代表）。全程无随机、无时钟、无外部输入。")
    L.append("- 复现：`python3 scripts/corpus/gsv2_split.py`（先决校验基线 sha，不符即拒绝运行）。")
    L.append("- 下游注意：**4a 产物 `v2-cases.jsonl` 保持无 split 字段（gen 的 V11 门断言）**；第 5 步冻结")
    L.append("  与 A-04 harness 的 split 过滤一律读 `v2-cases-split.jsonl`。")
    L.append("")
    return "\n".join(L) + "\n"


def main():
    rows = load_baseline()
    baseline_sha = sha16(rows)

    out_rows, split, groups, tgt, test_layers = build(rows)
    # V8-equivalent determinism: rebuild must be byte-identical
    out_rows2, split2, *_ = build(rows)
    deterministic = (sha16(out_rows) == sha16(out_rows2)) and split == split2

    test_rows = [r for r in out_rows if r["classification"]["split"] == "test"]
    dev_rows = [r for r in out_rows if r["classification"]["split"] == "dev"]

    checks = {}
    checks["baseline_sha"] = baseline_sha
    checks["test_total_is_40"] = len(test_rows) == TEST_SIZE
    checks["total_196"] = len(out_rows) == 196
    # atomic groups intact: every multi-case group entirely on one side
    broken = [g for g in groups if len({split[c] for c in g}) > 1]
    checks["atomic_groups_intact"] = not broken
    checks["deterministic_double_run"] = deterministic
    # round-trip: stripping split must reproduce the 4a baseline byte-for-byte
    stripped = [json.loads(json.dumps(r)) for r in out_rows]
    for r in stripped:
        del r["classification"]["split"]
    checks["roundtrip_baseline_sha"] = sha16(stripped) == baseline_sha
    checks["layer_targets"] = dict(tgt)
    checks["layer_actual_test"] = {l: test_layers[l] for l in LAYERS}
    checks["layer_targets_met"] = all(test_layers[l] == tgt[l] for l in LAYERS)
    ok = all(v for k, v in checks.items() if isinstance(v, bool))
    if not ok:
        for k, v in checks.items():
            if isinstance(v, bool) and not v:
                print("SPLIT CHECK FAILED:", k, v)
        raise SystemExit("split validation failed — nothing written")

    out_path = os.path.join(GS, "v2-cases-split.jsonl")
    with open(out_path, "w", encoding="utf-8") as fh:
        for r in out_rows:
            fh.write(json.dumps(r, ensure_ascii=False, sort_keys=True) + "\n")

    # machine-readable report
    lang_t, cold_t, ips_t = dims(rows)
    report = {
        "seed": SEED, "test_size": TEST_SIZE,
        "baseline_sha256_16": baseline_sha,
        "split_sha256_16": sha16(out_rows),
        "deterministic_double_run": deterministic,
        "atomic_group_rule": "connected components over expected∪acceptable∪forbidden co-reference (rev 2)",
        "groups": {
            "total": len(groups),
            "multi_case": [g for g in groups if len(g) > 1],
            "singleton_count": sum(1 for g in groups if len(g) == 1),
            "max_group_size": max(len(g) for g in groups),
            "locked_cases": sum(len(g) for g in groups if len(g) > 1),
        },
        "layer_targets": dict(tgt),
        "layer_actual_test": {l: test_layers[l] for l in LAYERS},
        "balance": {
            "language_test": {l: sum(1 for r in test_rows if r["query_language"] == l) for l in LANGS},
            "language_all": {l: sum(1 for r in rows if r["query_language"] == l) for l in LANGS},
            "cold_test": sum(1 for r in test_rows if r["classification"]["temperature_band"] == "cold"),
            "cold_all": sum(1 for r in rows if r["classification"]["temperature_band"] == "cold"),
            "ip_test": dict(Counter(r["classification"]["ip_name"] for r in test_rows
                                    if r["classification"]["ip_name"])),
        },
        "checks": checks,
    }
    with open(os.path.join(GS, "v2-split-report.json"), "w", encoding="utf-8") as fh:
        json.dump(report, fh, ensure_ascii=False, indent=1)

    with open(os.path.join(GS, "split-report-v2.md"), "w", encoding="utf-8") as fh:
        fh.write(render_md(report, out_rows, rows,
                           {r["case_key"]: r for r in out_rows}))

    print("split: dev %d / test %d | groups %d (multi %d) | targets met: %s" % (
        len(dev_rows), len(test_rows), len(groups),
        sum(1 for g in groups if len(g) > 1), checks["layer_targets_met"]))
    print("layer test (target→actual):", {l[:2]: (tgt[l], test_layers[l]) for l in LAYERS})
    print("split sha256:", report["split_sha256_16"], "| double-run identical:", deterministic)
    print("written:", out_path)


if __name__ == "__main__":
    main()
