#!/usr/bin/env python3
"""Generation-layer eval harness entry point (SP-22 E1, ticket #561).

Consumes a `cmd/rag-eval` generation JSONL, reconstructs the context surface
from PostgreSQL, runs the four Ragas-caliber metrics on a DeepSeek judge, and
writes <label>.ragas.json + <label>.ragas.md next to the requested out dir.

Usage:
    python3 ragas_harness.py \
        --runs artifacts/.../C1-hybrid.jsonl \
        --label sp22-e1-smoke \
        [--baseline docs/working/<old>.ragas.json] \
        [--out-dir docs/working] [--max-cases 5] [--sleep-ms 300] [--resume]

Required env (values only from the environment; never hardcode):
    EVALS_JUDGE_API_KEY      DeepSeek API key (judge)
    EVALS_EMBED_API_KEY      embedding endpoint key (default dashscope)
Optional env overrides: EVALS_JUDGE_API_BASE / EVALS_JUDGE_MODEL /
EVALS_EMBED_API_BASE / EVALS_EMBED_MODEL / EVALS_PG_CONTAINER et al.

Run with the bundled venv interpreter if openai is not system-wide:
    /path/to/venv/bin/python ragas_harness.py ...
"""

from __future__ import annotations

import argparse
import json
import os
import random
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import evals_lib as lib
import report
from metrics_direct import JudgeBackend, evaluate_case


def parse_args():
    ap = argparse.ArgumentParser(description="Ragas-caliber generation-layer eval harness")
    ap.add_argument("--runs", required=True, help="rag-eval generation JSONL path")
    ap.add_argument("--label", required=True, help="run label for artifacts")
    ap.add_argument("--baseline", default=None, help="previous <label>.ragas.json for diff")
    ap.add_argument("--out-dir", default="docs/working", help="artifact output directory")
    ap.add_argument("--max-cases", type=int, default=0, help="cap case count (0 = all)")
    ap.add_argument("--sleep-ms", type=int, default=300)
    ap.add_argument("--resume", action="store_true", help="skip cases present in the partial file")
    ap.add_argument("--seed", type=int, default=20260917, help="noise sampling seed")
    return ap.parse_args()


def load_partial(path: str) -> dict:
    if not os.path.exists(path):
        return {}
    done = {}
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                row = json.loads(line)
                done[row["case_key"]] = row
            except (json.JSONDecodeError, KeyError):
                continue
    return done


def build_noise_pool(chunks: dict, exclude_ids) -> list[str]:
    """Sample-able irrelevant chunk surfaces from unrelated contents."""
    pool = []
    for cid, rows in chunks.items():
        if cid in exclude_ids or not rows:
            continue
        row = rows[0]
        pool.append(f"{row['title']}\n{row['text'][:lib.EXCERPT_RUNES]}")
    return pool


def main() -> int:
    args = parse_args()
    if not os.environ.get("EVALS_JUDGE_API_KEY"):
        print("missing EVALS_JUDGE_API_KEY (read-only env contract; see README)", file=sys.stderr)
        return 2
    random.seed(args.seed)

    header, cases = lib.build_dataset(args.runs)
    if args.max_cases > 0:
        cases = cases[: args.max_cases]
    if not cases:
        print("no generation rows found", file=sys.stderr)
        return 2

    os.makedirs(args.out_dir, exist_ok=True)
    partial_path = os.path.join(args.out_dir, f"{args.label}.ragas.partial.jsonl")
    done = load_partial(partial_path) if args.resume else {}
    if args.resume:
        print(f"resume: {len(done)} cases already measured")

    all_ids = [cid for case in cases for cid in case.retrieved_ids]
    case_ids = {int(cid) for case in cases for cid in case.retrieved_ids}
    all_chunks = lib.load_chunks(all_ids)
    noise_pool = lib.load_noise_pool(case_ids)
    print(f"cases={len(cases)} noise_pool={len(noise_pool)} contexts_ready="
          f"{sum(1 for c in cases if c.contexts)}")

    be = JudgeBackend(sleep_ms=args.sleep_ms)
    results = [done[k] for k in sorted(done)]
    with open(partial_path, "a", encoding="utf-8") as partial:
        for i, case in enumerate(cases):
            if case.case_key in done:
                continue
            t0 = time.time()
            try:
                row = evaluate_case(be, case, noise_pool)
            except RuntimeError as err:
                row = {"case_key": case.case_key, "status": case.status,
                       "refused": case.refused, "error": str(err)}
            partial.write(json.dumps(row, ensure_ascii=False) + "\n")
            partial.flush()
            results.append(row)
            done[case.case_key] = row
            print(f"[{i + 1}/{len(cases)}] {case.case_key} "
                  f"f={row.get('faithfulness')} ar={row.get('answer_relevancy')} "
                  f"cp={row.get('context_precision')} ns={row.get('noise_sensitivity')} "
                  f"({time.time() - t0:.1f}s, judge_calls={be.calls})")

    buckets = lib.buckets(cases)
    usage = {"calls": be.calls, "judge_tokens": be.judge_tokens,
             "judge_model": be.judge_model, "embed_model": be.embed_model}

    baseline_doc = None
    baseline_results = {}
    if args.baseline and os.path.exists(args.baseline):
        with open(args.baseline, encoding="utf-8") as fh:
            baseline_doc = json.load(fh)
        baseline_results = {c["case_key"]: c for c in baseline_doc.get("cases", [])}

    doc = report.write_json(
        os.path.join(args.out_dir, f"{args.label}.ragas.json"),
        args.label, header, results, buckets, usage,
    )
    if baseline_results:
        doc["regressions"] = report.regressions(results, baseline_results)
        with open(os.path.join(args.out_dir, f"{args.label}.ragas.json"), "w",
                  encoding="utf-8") as fh:
            json.dump(doc, fh, ensure_ascii=False, indent=2)
    report.write_md(
        os.path.join(args.out_dir, f"{args.label}.ragas.md"),
        args.label, doc, baseline_doc,
        (baseline_doc or {}).get("label"),
    )
    if os.path.exists(partial_path):
        os.remove(partial_path)
    print(f"done: {args.label}.ragas.json / .ragas.md in {args.out_dir} "
          f"(judge calls={be.calls}, tokens≈{be.judge_tokens})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
