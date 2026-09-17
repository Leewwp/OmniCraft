#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""SP-22 E2 judge 校准（#564）：标注真值 × DeepSeek judge 3 次重跑的一致率/方差/偏差分布。

输入：
  --labels artifacts/evals/e2-human-labels.json   标注真值（24 条分层抽样，
      18 条应答桶带 faithfulness/relevancy 真值，6 条拒答桶做覆盖记录）
  --runs  cmd/rag-eval 的 generation JSONL（E1 全量基线即可，同数据同上下文重建）

做什么：
  1. 同批 18 条应答 case，DeepSeek judge（metrics_direct.py 原 prompt，
     temperature=0，同 seed 配置）重跑 N 次（默认 3）——faithfulness 与
     answer relevancy 两指标。
  2. 一致率：judge 均值 vs 标注真值，逐 case |Δ|、同侧率（0.7 阈值两侧
     是否一致）、Pearson 相关。
  3. 方差：N 次重跑的逐 case 极差/标准差、指标级漂移。
  4. 中英翻转探针：judge prompt 现为英文模板，翻转为中文模板（内容不变）
     重测一小批，量化 prompt 语言敏感性（arXiv 2606.14278 的中英翻转偏差
     证据在本站的对应实测）。

产物：docs/working/sp22-e2-calibration.json + .md（阈值建议书另写，
本脚本只出数）。

用法（密钥只从 env）：
  EVALS_JUDGE_API_KEY=... EVALS_EMBED_API_KEY=... \
    python3 scripts/evals/judge_calibration.py --runs artifacts/sp22-e1/runs-all2.jsonl
"""

from __future__ import annotations

import argparse
import json
import os
import statistics
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import evals_lib as lib
from metrics_direct import JudgeBackend, faithfulness, answer_relevancy

# 中文翻转模板：与 metrics_direct.CLAIMS_PROMPT/VERDICTS_PROMPT 语义逐句对齐，
# 只换语言，不换判准 —— 探针测的是 prompt 语言，不是判准漂移。
CLAIMS_PROMPT_ZH = """你在评估一个创作分享平台的 RAG 助手回答。

问题：
{question}

回答：
{answer}

把回答拆成原子声明。声明 = 关于站内内容的可验证陈述（作品、创作者、角色、出处、用法）。只抽这类内容声明。

不要抽：
- 助手自述过程的话（"我先查一下"、"I found"、"已为您找到"）；
- 提议、追问、请求澄清；
- 问候、致谢、致歉、免责；
- 关于回答本身或平台 UI 用法的陈述。

如果回答没有任何内容声明（例如只有过程叙述、致歉或提问），返回空列表。

只返回 JSON：{{"claims": ["...", "..."]}}"""

VERDICTS_PROMPT_ZH = """你在用检索到的上下文段落核验声明（创作分享平台）。

上下文段落：
{contexts}

声明：
{claims}

对每一条声明判断：是否被上述段落直接支持（支持 = 段落含有确认它所需的信息；不得使用你自己的世界知识）。改写算支持；夸大或添加段落没有的细节不算支持。

只返回 JSON：{{"verdicts": [true, false, ...]}}，与声明逐条对应。"""


def numbered(contexts):
    return "\n\n".join("[%d] %s" % (i + 1, c) for i, c in enumerate(contexts))


def faithfulness_zh(be, case, contexts):
    data = be._chat_json(CLAIMS_PROMPT_ZH.format(question=case.query, answer=case.answer[:4000]))
    claims = [str(c) for c in (data.get("claims") or [])]
    if not claims:
        return {"claims": [], "score": None}
    data = be._chat_json(VERDICTS_PROMPT_ZH.format(
        contexts=numbered(contexts)[:12000],
        claims="\n".join("%d. %s" % (i + 1, c) for i, c in enumerate(claims)),
    ))
    verdicts = data.get("verdicts") or []
    supported = sum(1 for i in range(len(claims)) if i < len(verdicts) and bool(verdicts[i]))
    return {"claims": claims, "score": supported / len(claims)}


def pearson(xs, ys):
    n = len(xs)
    if n < 2:
        return None
    mx, my = sum(xs) / n, sum(ys) / n
    cov = sum((x - mx) * (y - my) for x, y in zip(xs, ys))
    vx = sum((x - mx) ** 2 for x in xs) ** 0.5
    vy = sum((y - my) ** 2 for y in ys) ** 0.5
    if vx == 0 or vy == 0:
        return None
    return cov / (vx * vy)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--labels", default="artifacts/evals/e2-human-labels.json")
    ap.add_argument("--runs", default="artifacts/sp22-e1/runs-all2.jsonl")
    ap.add_argument("--repeats", type=int, default=3)
    ap.add_argument("--flip-probe", type=int, default=6, help="zh-prompt flip probe on first N answered cases")
    ap.add_argument("--sleep-ms", type=int, default=300)
    ap.add_argument("--out-dir", default="docs/working")
    args = ap.parse_args()
    if not os.environ.get("EVALS_JUDGE_API_KEY"):
        print("missing EVALS_JUDGE_API_KEY", file=sys.stderr)
        return 2

    with open(args.labels, encoding="utf-8") as fh:
        labels_doc = json.load(fh)
    labels = labels_doc["cases"]

    header, cases = lib.build_dataset(args.runs)
    by_key = {c.case_key: c for c in cases}
    wanted = [k for k in labels if not labels[k].get("refused")]
    answered = [by_key[k] for k in wanted if k in by_key]
    missing = [k for k in wanted if k not in by_key]
    if missing:
        raise SystemExit("labels reference missing cases: %s" % missing)
    print("answered=%d refused=%d contexts_ready=%d"
          % (len(answered), sum(1 for v in labels.values() if v.get("refused")),
             sum(1 for c in answered if c.contexts)))

    be = JudgeBackend(sleep_ms=args.sleep_ms)
    runs = []
    for r in range(args.repeats):
        rows = {}
        for case in answered:
            surfaces = [c.surface() for c in case.contexts]
            f = faithfulness(be, case, surfaces)
            ar = answer_relevancy(be, case)
            rows[case.case_key] = {
                "faithfulness": f["score"],
                "answer_relevancy": ar["score"],
                "claims": len(f.get("claims") or []),
            }
            print("  run%d %s f=%s ar=%s" % (r + 1, case.case_key, f["score"], ar["score"]), flush=True)
        runs.append(rows)

    flip = []
    for case in answered[: args.flip_probe]:
        surfaces = [c.surface() for c in case.contexts]
        zh = faithfulness_zh(be, case, surfaces)
        flip.append({"case_key": case.case_key, "faithfulness_zh_prompt": zh["score"],
                     "faithfulness_en_prompt": runs[0][case.case_key]["faithfulness"]})
        print("  flip %s zh=%s en=%s" % (case.case_key, zh["score"], runs[0][case.case_key]["faithfulness"]), flush=True)

    # ---- aggregation
    per_case = {}
    agree_high = agree_total = 0
    f_diffs, ar_diffs = [], []
    for key in [c.case_key for c in answered]:
        truth = labels[key]
        fs = [r[key]["faithfulness"] for r in runs if r[key]["faithfulness"] is not None]
        ars = [r[key]["answer_relevancy"] for r in runs if r[key]["answer_relevancy"] is not None]
        f_mean = statistics.fmean(fs) if fs else None
        ar_mean = statistics.fmean(ars) if ars else None
        f_rng = max(fs) - min(fs) if fs else None
        ar_rng = max(ars) - min(ars) if ars else None
        per_case[key] = {
            "truth_faithfulness": truth["faithfulness"], "truth_relevancy": truth["relevancy"],
            "judge_faithfulness": f_mean, "judge_relevancy": ar_mean,
            "f_range_across_runs": f_rng, "ar_range_across_runs": ar_rng,
            "f_abs_diff": abs(f_mean - truth["faithfulness"]) if f_mean is not None else None,
            "ar_abs_diff": abs(ar_mean - truth["relevancy"]) if ar_mean is not None else None,
            "layer": key.split("-")[0],
            "notes": truth.get("notes", ""),
        }
        if f_mean is not None:
            f_diffs.append(abs(f_mean - truth["faithfulness"]))
            same = (f_mean >= 0.7) == (truth["faithfulness"] >= 0.7)
            agree_high += int(same)
            agree_total += 1

    f_judges = [per_case[k]["judge_faithfulness"] for k in per_case if per_case[k]["judge_faithfulness"] is not None]
    f_truths = [per_case[k]["truth_faithfulness"] for k in per_case if per_case[k]["judge_faithfulness"] is not None]
    ar_judges = [per_case[k]["judge_relevancy"] for k in per_case if per_case[k]["judge_relevancy"] is not None]
    ar_truths = [per_case[k]["truth_relevancy"] for k in per_case if per_case[k]["judge_relevancy"] is not None]

    by_layer = {}
    for key, row in per_case.items():
        if row["f_abs_diff"] is not None:
            by_layer.setdefault(row["layer"], []).append(row["f_abs_diff"])

    flip_diffs = [abs(x["faithfulness_zh_prompt"] - x["faithfulness_en_prompt"])
                  for x in flip if x["faithfulness_zh_prompt"] is not None and x["faithfulness_en_prompt"] is not None]

    def _fmean(vals):
        vals = [v for v in vals if v is not None]
        return statistics.fmean(vals) if vals else None

    doc = {
        "kind": "sp22-e2-judge-calibration",
        "sample": {"answered": len(answered), "refused": sum(1 for v in labels.values() if v.get("refused")),
                   "layers": sorted(by_layer)},
        "repeats": args.repeats,
        "judge_usage": {"calls": be.calls, "judge_tokens": be.judge_tokens,
                        "judge_model": be.judge_model, "embed_model": be.embed_model},
        "agreement": {
            "faithfulness_mean_abs_diff": _fmean(f_diffs),
            "faithfulness_same_side_of_0_7": (agree_high / agree_total) if agree_total else None,
            "faithfulness_pearson": pearson(f_truths, f_judges),
            "relevancy_mean_abs_diff": _fmean([per_case[k]["ar_abs_diff"] for k in per_case]),
            "relevancy_pearson": pearson(ar_truths, ar_judges),
        },
        "variance": {
            "f_max_range": max((per_case[k]["f_range_across_runs"] or 0) for k in per_case),
            "f_mean_range": _fmean([per_case[k]["f_range_across_runs"] for k in per_case]),
            "ar_max_range": max((per_case[k]["ar_range_across_runs"] or 0) for k in per_case),
            "metric_drift": {("run%d" % (i + 1)): _fmean([r[k]["faithfulness"] for k in r]) for i, r in enumerate(runs)},
        },
        "layer_bias": {layer: statistics.fmean(ds) for layer, ds in sorted(by_layer.items())},
        "flip_probe": {"n": len(flip), "mean_abs_diff": _fmean(flip_diffs),
                       "cases": flip},
        "per_case": per_case,
    }

    os.makedirs(args.out_dir, exist_ok=True)
    out_json = os.path.join(args.out_dir, "sp22-e2-calibration.json")
    with open(out_json, "w", encoding="utf-8") as fh:
        json.dump(doc, fh, ensure_ascii=False, indent=1)

    a = doc["agreement"]
    print("\n=== agreement ===")
    print("faithfulness: mean|Δ|=%.3f same-side@0.7=%.1f%% pearson=%.3f"
          % (a["faithfulness_mean_abs_diff"] or -1, (a["faithfulness_same_side_of_0_7"] or 0) * 100,
             a["faithfulness_pearson"] or 0))
    print("relevancy:    mean|Δ|=%.3f pearson=%.3f" % (a["relevancy_mean_abs_diff"] or -1, a["relevancy_pearson"] or 0))
    print("variance: f_max_range=%.3f metric_drift=%s" % (doc["variance"]["f_max_range"], doc["variance"]["metric_drift"]))
    print("flip probe: mean|Δ|=%s" % doc["flip_probe"]["mean_abs_diff"])
    print("wrote", out_json)


if __name__ == "__main__":
    raise SystemExit(main())
