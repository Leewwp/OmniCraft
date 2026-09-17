"""Report assembly for the generation-layer eval harness (SP-22 E1).

Produces <label>.ragas.json (per-case metrics, bucket means, run header,
judge usage) and <label>.ragas.md (human summary: metric means, buckets,
per-metric diff against a baseline run, regression case list).
"""

from __future__ import annotations

import datetime
import json

METRICS = ["faithfulness", "answer_relevancy", "context_precision", "noise_sensitivity"]
LOWER_IS_BETTER = {"noise_sensitivity"}


def _expiry() -> str:
    """Default +2 months, matching the working-doc convention."""
    today = datetime.date.today()
    month = today.month + 2
    year = today.year + (month - 1) // 12
    month = (month - 1) % 12 + 1
    return f"{year:04d}-{month:02d}-{today.day:02d}"


def aggregate(results: list[dict], buckets: dict) -> dict:
    out = {"overall": {}, "answered": {}, "refused": {}}
    for metric in METRICS:
        vals = [r.get(metric) for r in results if r.get(metric) is not None]
        out["overall"][metric] = _mean(vals)
        answered = [r.get(metric) for r in results
                    if r.get(metric) is not None and not r.get("refused")]
        out["answered"][metric] = _mean(answered)
        refused = [r.get(metric) for r in results
                   if r.get(metric) is not None and r.get("refused")]
        out["refused"][metric] = _mean(refused)
    out["counts"] = buckets
    return out


def _mean(vals):
    vals = [v for v in vals if v is not None]
    if not vals:
        return None
    return round(sum(vals) / len(vals), 4)


def regressions(results: list[dict], baseline_results: dict, threshold: float = 0.03) -> list[dict]:
    """Cases whose faithfulness dropped by more than `threshold` vs baseline."""
    out = []
    for r in results:
        base = baseline_results.get(r["case_key"])
        if not base:
            continue
        for metric in METRICS:
            cur, old = r.get(metric), base.get(metric)
            if cur is None or old is None:
                continue
            delta = cur - old
            worse = delta < -threshold if metric not in LOWER_IS_BETTER else delta > threshold
            if worse:
                out.append({
                    "case_key": r["case_key"],
                    "metric": metric,
                    "baseline": old,
                    "current": cur,
                    "delta": round(delta, 4),
                })
    return out


def write_json(path: str, label: str, header: dict, results: list[dict],
               buckets: dict, usage: dict) -> dict:
    doc = {
        "kind": "ragas-gen-eval",
        "label": label,
        "source_header": header,
        "aggregate": aggregate(results, buckets),
        "cases": results,
        "judge_usage": usage,
    }
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(doc, fh, ensure_ascii=False, indent=2)
    return doc


def _fmt(v):
    return "—" if v is None else f"{v:.4f}"


def write_md(path: str, label: str, doc: dict, baseline_doc: dict | None,
             baseline_label: str | None) -> None:
    agg = doc["aggregate"]
    lines = [f"# Ragas 生成层评测报告：{label}", ""]
    # docs/working/ documents must declare an expiry (doc-validator release gate).
    lines.append("**预计失效日期**: " + _expiry())
    lines.append("")
    lines.append(f"- 用例数：{agg['counts'].get('total')}（应答 {agg['counts'].get('answered')} / 拒答 {agg['counts'].get('refused')}）")
    lines.append(f"- 判官调用：{doc['judge_usage'].get('calls', 0)} 次，judge tokens ≈ {doc['judge_usage'].get('judge_tokens', 0)}")
    lines.append("")
    lines.append("## 指标均值")
    lines.append("")
    lines.append("| 指标 | 应答桶 | 全体 | 口径 |")
    lines.append("|---|---|---|---|")
    notes = {
        "faithfulness": "答案声明被上下文支持比例（越高越好）",
        "answer_relevancy": "反向问题与原问题 embedding 余弦（越高越好）",
        "context_precision": "检索排名 MAP 式（越高越好）",
        "noise_sensitivity": "加噪后声明翻转比例（越低越好）",
    }
    for metric in METRICS:
        lines.append(f"| {metric} | {_fmt(agg['answered'].get(metric))} | {_fmt(agg['overall'].get(metric))} | {notes[metric]} |")
    lines.append("")

    if baseline_doc:
        base = baseline_doc["aggregate"]
        lines.append(f"## 基线对比：{baseline_label}")
        lines.append("")
        lines.append("| 指标 | 基线（应答桶） | 本轮（应答桶） | Δ |")
        lines.append("|---|---|---|---|")
        for metric in METRICS:
            old, cur = base["answered"].get(metric), agg["answered"].get(metric)
            delta = None if old is None or cur is None else round(cur - old, 4)
            lines.append(f"| {metric} | {_fmt(old)} | {_fmt(cur)} | {_fmt(delta)} |")
        lines.append("")
        regs = doc.get("regressions") or []
        lines.append(f"## 回归 case（阈值 0.03）：{len(regs)} 项")
        lines.append("")
        if regs:
            lines.append("| case | 指标 | 基线 | 本轮 | Δ |")
            lines.append("|---|---|---|---|---|")
            for r in regs[:50]:
                lines.append(f"| {r['case_key']} | {r['metric']} | {_fmt(r['baseline'])} | {_fmt(r['current'])} | {_fmt(r['delta'])} |")
        lines.append("")

    lines.append("## 分桶")
    lines.append("")
    lines.append(f"- 按语言：{json.dumps(agg['counts'].get('by_language', {}), ensure_ascii=False)}")
    lines.append(f"- 按回答类型：{json.dumps(agg['counts'].get('by_kind', {}), ensure_ascii=False)}")
    lines.append("")
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(lines))
