#!/usr/bin/env python3
"""发布前全量门：生成层 advisory 阈值断言（SP-22 E2 采纳值，#611）。

输入一份 ragas_harness 产物（<label>.ragas.json，全量复跑新鲜结果）与
evals/thresholds.yaml 的 generation_layer_advisory 段，按已采纳语义断言：

  faithfulness       < faithfulness_floor  → FAIL（硬地板）
                     < faithfulness_observe → PASS + 观察告警（不拦门）
  context_precision  < context_precision_observe → FAIL（防大滑坡下限）
  noise_sensitivity  > noise_sensitivity_max → FAIL（越低越好）
  answer_relevancy   answer_relevancy_gate=false → 永不拦门，仅随报告输出
                       （与真值 Pearson −0.17，观测指标——E2 校准结论）

结构完整性：应答桶样本量不足（< min_answered）时 FAIL，防空跑全绿。
判定口径 = 应答桶（answered）均值；拒答桶无生成层指标（Ragas 定义）。

用法：
  python3 scripts/evals/generation_gate.py \
      --ragas /path/to/<label>.ragas.json \
      [--thresholds evals/thresholds.yaml]

退出码：0 = PASS（可含观察告警），1 = FAIL（附逐项证据行）。
"""

import argparse
import json
import sys
from pathlib import Path

MIN_ANSWERED_DEFAULT = 150


def load_advisory(path: str) -> dict:
    """解析 thresholds.yaml 的 generation_layer_advisory 平铺段。

    只认该段内的 `key: value` 行（本段自 E2 回填起即为平铺标量，嵌套化
    时应改回 pyyaml——环境约束：evals venv 只装 openai，不引 yaml 依赖）。
    """
    section: dict = {}
    in_section = False
    for raw in Path(path).read_text(encoding="utf-8").splitlines():
        line = raw.rstrip()
        if not line or line.lstrip().startswith("#"):
            continue
        stripped = line.strip()
        if not line.startswith((" ", "\t")) and stripped.endswith(":"):
            in_section = stripped == "generation_layer_advisory:"
            continue
        if in_section and ":" in stripped:
            key, _, value = stripped.partition(":")
            section[key.strip()] = value.strip()
    if not section:
        raise SystemExit(f"no generation_layer_advisory section found in {path}")
    return section


def as_bool(value: str) -> bool:
    return value.strip().lower() in ("true", "yes", "1")


def as_float(value: str) -> float:
    return float(value)


def main() -> int:
    ap = argparse.ArgumentParser(description="release full-gate assertion over a ragas run")
    ap.add_argument("--ragas", required=True, help="ragas_harness <label>.ragas.json (fresh full run)")
    ap.add_argument("--thresholds", default="evals/thresholds.yaml")
    args = ap.parse_args()

    doc = json.loads(Path(args.ragas).read_text(encoding="utf-8"))
    advisory = load_advisory(args.thresholds)

    agg = doc.get("aggregate") or {}
    counts = (agg.get("counts") or {})
    answered = agg.get("answered") or {}
    total = counts.get("total", 0)
    answered_n = counts.get("answered", 0)

    faith = answered.get("faithfulness")
    ar = answered.get("answer_relevancy")
    cp = answered.get("context_precision")
    ns = answered.get("noise_sensitivity")

    min_answered = int(as_float(advisory.get("min_answered", MIN_ANSWERED_DEFAULT)))
    ar_gated = as_bool(advisory.get("answer_relevancy_gate", "false"))
    f_observe = as_float(advisory["faithfulness_observe"])
    f_floor = as_float(advisory["faithfulness_floor"])
    cp_min = as_float(advisory["context_precision_observe"])
    ns_max = as_float(advisory["noise_sensitivity_max"])

    print(f"generation full gate — {args.ragas}")
    print(f"  cases total={total} answered={answered_n} (min {min_answered})")

    failures: list[str] = []
    warnings: list[str] = []

    if answered_n < min_answered:
        failures.append(f"answered bucket {answered_n} < min {min_answered} (empty-run guard)")

    if faith is None:
        failures.append("faithfulness missing on the answered bucket")
    elif faith < f_floor:
        failures.append(f"faithfulness {faith:.4f} < floor {f_floor}")
    elif faith < f_observe:
        warnings.append(f"faithfulness {faith:.4f} < observe {f_observe} (floor {f_floor} holds)")

    if cp is None:
        failures.append("context_precision missing on the answered bucket")
    elif cp < cp_min:
        failures.append(f"context_precision {cp:.4f} < {cp_min}")

    if ns is None:
        failures.append("noise_sensitivity missing on the answered bucket")
    elif ns > ns_max:
        failures.append(f"noise_sensitivity {ns:.4f} > {ns_max}")

    ar_note = "gated" if ar_gated else "observe-only (never gates)"
    print(f"  faithfulness       {faith if faith is None else round(faith, 4)} (observe {f_observe} / floor {f_floor})")
    print(f"  answer_relevancy   {ar if ar is None else round(ar, 4)} — {ar_note}")
    print(f"  context_precision  {cp if cp is None else round(cp, 4)} (min {cp_min})")
    print(f"  noise_sensitivity  {ns if ns is None else round(ns, 4)} (max {ns_max})")

    for w in warnings:
        print(f"  WARN: {w}")
    if failures:
        for f in failures:
            print(f"  FAIL: {f}")
        print("RESULT: FAIL")
        return 1
    print("RESULT: PASS" + (" (with observations)" if warnings else ""))
    return 0


if __name__ == "__main__":
    sys.exit(main())
