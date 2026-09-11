#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Golden Set v2 target selection (#291 step 4, contract 2026-09-04 §5/§12).

Deterministic, seeded (20260904) selection of the v2 targets:
- ke 19 target_swap picks (same-IP public title_form=exact raw-title pool)
- sd 10 target_swap picks (same-IP public fuzzy/partial raw-title pool)
- be 24 fresh targets (public, raw title, version_count=1, body rich enough)
- hn 16 targets (one per IP, from the IP's densest same-batch cluster)
- na/vi/keep slots mirror the frozen migration map (build_migration_map.py).

Pool exclusion (contract §5 + §6): keys already expected by v1 keep cases,
vi forbidden keys, and every key picked earlier in this run; title_origin must
be raw (68 fallback-title items never enter ke/be pools, contract §6.1).

Output: artifacts/corpus-v2/golden-set/v2-selection.json (selection record +
allocation summary). Query authoring happens separately in gsv2_cases.py; this
file only fixes WHICH items the cases point at.
"""
from __future__ import annotations

import json
import os
import random
import re
from collections import Counter, defaultdict

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
BASE = os.path.join(REPO, "artifacts", "corpus-v2")
GS = os.path.join(BASE, "golden-set")
SEED = 20260904

# language steering per swap slot (old case key -> preferred languages, best
# first; "zh" fallback whenever no preference matches the pool)
KE_SWAP_LANG = {
    # ip_scoped swaps stay zh (「在IP内：」prefix is a zh form)
    "gs-c2-0001": ["zh"], "gs-c2-0010": ["zh"], "gs-c2-0011": ["zh"],
    "gs-c2-0013": ["zh"], "gs-c2-0014": ["zh"], "gs-c2-0015": ["zh"],
    "gs-c2-0016": ["zh"],
    # global swaps carry the en/mixed load
    "gs-c2-0018": ["zh"], "gs-c2-0024": ["zh"], "gs-c2-0025": ["zh"],
    "gs-c2-0029": ["en", "zh"], "gs-c2-0032": ["zh"], "gs-c2-0038": ["mixed", "en", "zh"],
    "gs-c2-0039": ["zh"], "gs-c2-0040": ["zh"], "gs-c2-0041": ["zh"],
    "gs-c2-0045": ["zh", "cold-zh"], "gs-c2-0048": ["zh"], "gs-c2-0052": ["en", "zh"],
}
SD_SWAP_LANG = {
    "gs-c2-0066": ["mixed", "zh"], "gs-c2-0069": ["en", "zh"],
    "gs-c2-0076": ["mixed", "en", "zh"], "gs-c2-0082": ["en", "zh"],
    "gs-c2-0086": ["zh"], "gs-c2-0096": ["zh"], "gs-c2-0103": ["zh"],
    "gs-c2-0105": ["zh"], "gs-c2-0106": ["zh"], "gs-c2-0110": ["en", "zh"],
}
# be/hn allocation targets (free picks)
BE_LANG = {"zh": 15, "en": 6, "mixed": 3}
BE_MIXED_MIN = 3
BE_COLD_MIN = 4
BE_CAT_MIN = {"longform": 10, "settings": 3, "shortform": 4}
HN_LANG = {"zh": 12, "en": 3, "mixed": 1}


def load():
    idx = {r["corpus_item_key"]: r for r in (json.loads(l) for l in open(f"{BASE}/index.jsonl", encoding="utf-8"))}
    cases = {c["case_key"]: c for c in (json.loads(l) for l in open(f"{GS}/golden-cases-draft.jsonl", encoding="utf-8"))}
    mmap = [json.loads(l) for l in open(f"{GS}/migration-map-v2.jsonl", encoding="utf-8")]
    return idx, cases, mmap


def batch_of(key: str) -> str:
    # c2-ipNN-bBB-NNN -> bBB
    m = re.match(r"c2-ip\d+-(b\d+)-\d+$", key)
    return m.group(1) if m else ""


def pick_swap(pool, prefs, rng):
    """Pick one pool item honouring the language preference list."""
    for pref in prefs:
        if pref == "cold-zh":
            cand = [k for k in pool if pool[k]["language"] == "zh" and pool[k]["temperature"] == "cold"]
        else:
            cand = [k for k in pool if pool[k]["language"] == pref]
        if cand:
            return sorted(cand)[rng.randrange(len(cand))]
    cand = sorted(pool)
    return cand[rng.randrange(len(cand))]


def main():
    idx, cases, mmap = load()
    by_v2 = {r["v2_case_key"]: r for r in mmap}

    # exclusions: v1 keep-case expected keys + vi forbidden keys (restricted,
    # already excluded by public filter, kept explicit for the record)
    swap_disp = {"target_swap", "target_swap+rewrite"}
    used = set()
    for r in mmap:
        old = r["old_case_key"]
        if old == "-" or r["disposition"] in swap_disp or r["disposition"].startswith("drop"):
            continue
        c = cases[old]
        used.update(c.get("expected_citation_keys", []))
        used.update(c.get("forbidden_keys", []))

    avail = {k: v for k, v in idx.items()
             if v["visibility"] == "public" and v["title"].strip()
             and v["title_origin"] == "raw" and k not in used}

    out = {"seed": SEED, "ke": [], "sd": [], "be": [], "hn": [], "notes": []}

    # --- ke swaps ---
    for r in mmap:
        if r["old_layer"] != "exact" or r["disposition"] != "target_swap":
            continue
        old = r["old_case_key"]
        c = cases[old]
        ip = c["classification"]["ip"]
        forms = {"exact"} if c["classification"]["query_form"] == "exact" else {"fuzzy", "partial"}
        pool = {k: v for k, v in avail.items() if v["ip"] == ip and v["title_form"] in forms}
        rng = random.Random(f"{SEED}:ke-swap:{old}")
        tgt = pick_swap(pool, KE_SWAP_LANG[old], rng)
        del avail[tgt]
        ip_scoped = c["classification"]["ip_scope"] == "ip_localized"
        out["ke"].append({
            "v2_key": r["v2_case_key"], "old_key": old, "mode": "swap",
            "old_target": c["expected_citation_keys"][0], "target": tgt,
            "ip": ip, "ip_scoped": ip_scoped, "pool_size": len(pool),
            "pool_lang": dict(Counter(v["language"] for v in pool.values())),
            "rng": f"ke-swap:{old}",
        })

    # --- sd swaps ---
    for r in mmap:
        if r["disposition"] != "target_swap+rewrite":
            continue
        old = r["old_case_key"]
        c = cases[old]
        ip = c["classification"]["ip"]
        pool = {k: v for k, v in avail.items()
                if v["ip"] == ip and v["title_form"] in {"fuzzy", "partial"}}
        rng = random.Random(f"{SEED}:sd-swap:{old}")
        tgt = pick_swap(pool, SD_SWAP_LANG[old], rng)
        del avail[tgt]
        out["sd"].append({
            "v2_key": r["v2_case_key"], "old_key": old, "mode": "swap",
            "old_target": c["expected_citation_keys"][0], "target": tgt,
            "ip": ip, "pool_size": len(pool), "rng": f"sd-swap:{old}",
        })

    # --- be targets: structural quality filter + quota-balanced greedy ---
    be_cand = [k for k, v in avail.items()
               if v["version_count"] == 1 and v["word_count"] >= 600
               and v["category"] != "discussion"]
    be_mixed = [k for k, v in avail.items()
                if v["version_count"] == 1 and v["word_count"] >= 400
                and v["category"] != "discussion" and v["language"] == "mixed"]
    rng = random.Random(f"{SEED}:be")
    # deterministic shuffled order, then greedy fill in priority passes:
    # category minimums -> cold minimum -> language quotas -> free rest
    order = sorted(be_cand)
    rng.shuffle(order)
    cat_left = dict(BE_CAT_MIN)
    cold_need = BE_COLD_MIN
    lang_left = dict(BE_LANG)
    ip_count = Counter()
    picked = []

    def take(k):
        picked.append(k)
        lang_left[idx[k]["language"]] = lang_left.get(idx[k]["language"], 0) - 1
        ip_count[idx[k]["ip"]] += 1

    def try_take(k, cond):
        if len(picked) >= 24 or k in picked or not cond(k):
            return
        if ip_count[idx[k]["ip"]] >= 2:
            return
        take(k)

    def ip_ok(k):
        return ip_count[idx[k]["ip"]] < 2

    mixed_taken = 0
    for k in sorted(be_mixed):  # pass 0: mixed floor (relaxed word limit)
        if mixed_taken >= BE_MIXED_MIN or len(picked) >= 24 or k in picked:
            break
        if ip_ok(k):
            mixed_taken += 1
            cat = idx[k]["category"]
            if cat in cat_left and cat_left[cat] > 0:
                cat_left[cat] -= 1
            take(k)
    for k in order:  # pass 1: category minimums
        if k in picked:
            continue
        cat = idx[k]["category"]
        if cat in cat_left and cat_left[cat] > 0 and lang_left.get(idx[k]["language"], 0) > 0 and ip_ok(k):
            cat_left[cat] -= 1
            take(k)
    for k in order:  # pass 2: cold minimum
        if k in picked or len(picked) >= 24:
            continue
        if cold_need > 0 and idx[k]["temperature"] == "cold" and lang_left.get(idx[k]["language"], 0) > 0 and ip_ok(k):
            cold_need -= 1
            take(k)
    for k in order:  # pass 3: language quotas
        try_take(k, lambda k, _k=k: lang_left.get(idx[_k]["language"], 0) > 0)
    for k in order:  # pass 4: free rest (language caps respected loosely)
        try_take(k, lambda k: True)
    assert len(picked) == 24, f"be picks={len(picked)}"
    assert len(set(picked)) == 24, "be picked a duplicate target"
    for k in picked:
        avail.pop(k, None)
    for i, k in enumerate(picked, 1):
        v = idx[k]
        out["be"].append({"v2_key": f"be-{i:04d}", "target": k, "mode": "new",
                          "ip": v["ip"], "language": v["language"],
                          "category": v["category"], "temperature": v["temperature"],
                          "word_count": v["word_count"], "batch": batch_of(k)})

    # --- hn targets: one per IP, densest same-batch cluster, version_count=1 ---
    hn_avail = {k: v for k, v in avail.items() if v["version_count"] == 1}
    clusters = defaultdict(list)
    for k, v in hn_avail.items():
        clusters[(v["ip"], batch_of(k))].append(k)
    out["hn"] = []
    # densest >=5-member cluster per IP, then deterministic language
    # designation: en x3 + mixed x1, assigned to the first IPs (sorted) where
    # a >=5 cluster can supply a hot member of that language (the supplying
    # cluster replaces the densest one for that IP); everything else zh.
    ip_all_clusters = {}
    ip_cluster = {}
    for ip in sorted({v["ip"] for v in hn_avail.values()}):
        cs = sorted([c for c in clusters if c[0] == ip and len(clusters[c]) >= 5],
                    key=lambda c: (-len(clusters[c]), c[1]))
        ip_all_clusters[ip] = cs
        ip_cluster[ip] = cs[0]
    designations = {}
    for want, quota in (("en", HN_LANG["en"]), ("mixed", HN_LANG["mixed"])):
        for ip in sorted(ip_cluster):
            if designations.get(ip) or quota <= 0:
                continue
            hit = [c for c in ip_all_clusters[ip]
                   if any(idx[k]["language"] == want and idx[k]["temperature"] == "hot"
                          for k in clusters[c])]
            if hit:
                ip_cluster[ip] = hit[0]
                designations[ip] = want
                quota -= 1
    for ip in sorted(ip_cluster):
        designations.setdefault(ip, "zh")
    for i, ip in enumerate(sorted(ip_cluster), 1):
        rng = random.Random(f"{SEED}:hn:{ip}")
        cluster_key = ip_cluster[ip]
        members = sorted(clusters[cluster_key])
        want = designations[ip]
        cand = [k for k in members if idx[k]["language"] == want
                and idx[k]["temperature"] == "hot"]
        if not cand:
            cand = [k for k in members if idx[k]["language"] == want] or members
        tgt = sorted(cand)[rng.randrange(len(cand))]
        out["hn"].append({"v2_key": f"hn-{i:04d}", "target": tgt,
                          "mode": "new", "ip": ip, "batch": cluster_key[1],
                          "language": idx[tgt]["language"], "form": idx[tgt]["title_form"],
                          "cluster_size": len(members),
                          "temperature": idx[tgt]["temperature"]})

    # --- allocation summary over ALL 196 (keep targets from v1) ---
    layers = {"ke": [], "sd": [], "na": [], "vi": []}
    layer_prefix = {"known_item_exact": "ke", "semantic_discovery": "sd",
                    "no_answer": "na", "visibility": "vi"}
    for r in mmap:
        old = r["old_case_key"]
        if old == "-" or r["disposition"].startswith("drop"):
            continue
        c = cases[old]
        tgt = None
        if r["v2_layer"] == "known_item_exact":
            tgt = next(e["target"] for e in out["ke"] if e["old_key"] == old) \
                if r["disposition"] == "target_swap" else c["expected_citation_keys"][0]
        elif r["v2_layer"] == "semantic_discovery":
            tgt = next(e["target"] for e in out["sd"] if e["old_key"] == old) \
                if r["disposition"] == "target_swap+rewrite" else c["expected_citation_keys"][0]
        layers[layer_prefix[r["v2_layer"]]].append((r["v2_case_key"], tgt, c))
    lang = Counter()
    cold = 0
    ips = set()
    for v2, tgt, c in layers["ke"] + layers["sd"]:
        lang[idx[tgt]["language"]] += 1
        ips.add(idx[tgt]["ip"])
        cold += idx[tgt]["temperature"] == "cold"
    for v2, tgt, c in layers["vi"]:
        lang[c["query_language"]] += 1
        ips.add(c["classification"]["ip"])
    for v2, tgt, c in layers["na"]:
        lang[c["query_language"]] += 1  # keep 12; na-new 8 authored later
    for e in out["be"] + out["hn"]:
        lang[e["language"]] += 1
        ips.add(e["ip"])
    out["summary"] = {
        "fixed_lang": dict(lang), "fixed_cold_ke_sd": cold, "ips_covered": len(ips),
        "note": "na-new 8 languages are authored in gsv2_cases (plan: zh5/en2/mixed1); "
                "sd-keep 38 query language follows target language.",
    }

    path = os.path.join(GS, "v2-selection.json")
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(out, fh, ensure_ascii=False, indent=1)
    print("selection written:", path)
    print("ke swaps:", len(out["ke"]), "sd swaps:", len(out["sd"]),
          "be:", len(out["be"]), "hn:", len(out["hn"]))
    print("be langs:", dict(Counter(e["language"] for e in out["be"])),
          "be cold:", sum(1 for e in out["be"] if e["temperature"] == "cold"),
          "be cats:", dict(Counter(e["category"] for e in out["be"])))
    print("hn langs:", dict(Counter(e["language"] for e in out["hn"])),
          "hn ips:", len({e["ip"] for e in out["hn"]}))
    print("summary:", out["summary"])


if __name__ == "__main__":
    main()
