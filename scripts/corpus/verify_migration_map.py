#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Deterministic verification of the committed v1->v2 migration map (#291).

Checks (all must pass, exit 1 otherwise):
  V1 rebuild determinism: rebuilding rows from source data equals the committed jsonl
  V2 old coverage: all 160 gs-c2-* keys appear exactly once
  V3 v2 keys: 196 unique, per-layer counts equal the frozen quota
  V4 dispositions: exact keep45/swap19, semantic rewrite38/swap10, na keep12/drop12, vi 24, new 48
  V5 swap pools: recomputed same-IP public candidate pools > 0 for all 29 swap cases,
     and match the numbers embedded in the notes
  V6 drop(merged) rows point at a kept case with the same normalized query
"""
from __future__ import annotations

import json
import os
import re
import sys
from collections import Counter

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import build_migration_map as bmm  # noqa: E402

failures = []


def check(name, ok, detail=""):
    print(("PASS" if ok else "FAIL"), name, detail)
    if not ok:
        failures.append(name)


def main():
    idx, cases = bmm.load()
    rows = bmm.build_rows(idx, cases)
    committed = [json.loads(l) for l in open(f"{BASE}/golden-set/migration-map-v2.jsonl", encoding="utf-8")]

    # V1 rebuild equality
    rebuilt = [{"old_case_key": r[0], "old_layer": r[1], "disposition": r[2],
                "v2_case_key": r[3], "v2_layer": r[4], "note": r[5]} for r in rows]
    check("V1 rebuild == committed jsonl", rebuilt == committed,
          f"({len(rebuilt)} vs {len(committed)} rows)")

    # V2 old coverage
    old_rows = [r for r in committed if r["old_case_key"] != "-"]
    old_counts = Counter(r["old_case_key"] for r in old_rows)
    golden_keys = {c["case_key"] for c in cases}
    check("V2 old coverage 1:1 (160)", len(old_rows) == 160 and set(old_counts) == golden_keys
          and all(v == 1 for v in old_counts.values()))

    # V3 v2 keys & quotas
    v2_keys = {}
    for r in committed:
        v2_keys[r["v2_case_key"]] = r["v2_layer"]
    layer_counts = Counter(v2_keys.values())
    check("V3 v2 keys unique & quota", len(v2_keys) == 196 and layer_counts == Counter(bmm.V2_QUOTA),
          f"({len(v2_keys)} keys, {dict(layer_counts)})")

    # V4 dispositions
    disp = Counter((r["old_layer"], r["disposition"]) for r in committed)
    expected_disp = {("exact", "keep"): 45, ("exact", "target_swap"): 19,
                     ("semantic", "rewrite"): 38, ("semantic", "target_swap+rewrite"): 10,
                     ("visibility", "keep+principal"): 24,
                     ("no_answer", "keep+strategy"): 12, ("no_answer", "drop(merged)"): 12,
                     ("-", "new"): 48}
    check("V4 dispositions", dict(disp) == expected_disp, str(dict(disp)))

    # V5 swap pools recomputed
    used_expected = {k for c in cases for k in c["expected_citation_keys"]}
    bad_pool = []
    for r in committed:
        m = re.search(r"候选池 (\d+) 条", r["note"])
        if not m:
            continue
        old_case = next(c for c in cases if c["case_key"] == r["old_case_key"])
        forms = {"exact"} if r["old_layer"] == "exact" else {"fuzzy", "partial"}
        pool = bmm.swap_pool(idx, used_expected, old_case["classification"]["ip"], forms)
        if pool <= 0 or pool != int(m.group(1)):
            bad_pool.append((r["old_case_key"], pool, m.group(1)))
    n_swaps = sum(1 for r in committed if "候选池" in r["note"])
    check("V5 swap pools >0 and match notes (29 cases)", n_swaps == 29 and not bad_pool,
          f"({n_swaps} swap rows, bad={bad_pool[:3]})")

    # V6 drop twins
    by_key = {c["case_key"]: c for c in cases}
    keep_norm = {bmm.norm_q(by_key[o]["query"]): o for o in bmm.NA_KEEP}
    bad_twin = [r["old_case_key"] for r in committed
                if r["disposition"] == "drop(merged)"
                and keep_norm.get(bmm.norm_q(by_key[r["old_case_key"]]["query"])) is None]
    check("V6 drop(merged) twins valid", not bad_twin, str(bad_twin[:3]))

    print("\n%s (%d failures)" % ("ALL CHECKS PASSED" if not failures else "FAILED: " + ", ".join(failures), len(failures)))
    return 1 if failures else 0


BASE = bmm.BASE

if __name__ == "__main__":
    sys.exit(main())
