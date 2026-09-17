# Generation-layer eval harness (SP-22 E1, #561)

Ragas 口径四指标（faithfulness / answer relevancy / context precision /
noise sensitivity）离线跑批，judge = DeepSeek（OpenAI 兼容端点），embedding =
中文友好端点（默认 dashscope text-embedding-v4）。

## 输入

`cmd/rag-eval` 的 generation JSONL（`-out runs.jsonl` 产物）。harness 只读
question / answer / status / citations / retrieved_ids 五个字段；上下文文本
按「标题 + 前 240 rune 分片文本」（与生产工具结果面 `retrievalSummaries`
一致）从 PostgreSQL 回查重建：被引用候选用引用的 chunk_key 精确定位，未
引用候选取与 query 字符二元组重合度最高的分片（已记录的近似）。

## 运行

```bash
# 依赖：openai SDK（venv 即可，系统 3.9 可用）
python3 -m venv /tmp/evals-venv && /tmp/evals-venv/bin/pip install openai

# 密钥只从环境注入（不入脚本与文档）：
export EVALS_JUDGE_API_KEY=...      # DeepSeek
export EVALS_EMBED_API_KEY=...      # 默认 dashscope compatible-mode
# 可选覆盖：EVALS_JUDGE_API_BASE / EVALS_JUDGE_MODEL /
#          EVALS_EMBED_API_BASE / EVALS_EMBED_MODEL
# 服务器侧容器名不同：export EVALS_PG_CONTAINER=omnicraft-postgres-1

/tmp/evals-venv/bin/python scripts/evals/ragas_harness.py \
  --runs artifacts/.../runs.jsonl \
  --label sp22-e1-full \
  --baseline docs/working/<旧label>.ragas.json \
  --resume --sleep-ms 300
```

产物：`docs/working/<label>.ragas.json`（逐 case 指标 + 分桶均值 + judge
用量）与 `<label>.ragas.md`（均值表 / 基线逐指标 diff / 回归 case 清单，
回归阈值 0.03）。断点续跑：中断后同命令加 `--resume`（partial jsonl 逐
case 落盘）。

## 指标口径（Ragas 定义，直实现）

| 指标 | 计算 | 方向 |
|---|---|---|
| faithfulness | 两段式：抽取原子声明 → 对照上下文逐条裁决；支持数/总数 | 越高越好 |
| answer_relevancy | 判官由答案反向生成 3 问 → 与原问题 embedding 余弦均值 | 越高越好 |
| context_precision | 判官对排名序分片逐一相关性裁决 → MAP 式加权精确率 | 越高越好 |
| noise_sensitivity | 原支持声明加入 2 条无关分片后复裁，翻转比例 | 越低越好 |

拒答 case（no_evidence 等）跳过生成层指标，只进分桶计数。

## 为什么是直实现而不是 ragas 库

系统 Python 3.9 上 ragas 依赖树无法解析（2026-09-17 实测：resolver 15 分钟
零包落地）。指标公式按 Ragas 文档逐条对齐；判官 prompt 为可控双语模板，
为 E2 judge 校准（中英翻转偏差 10.7–14.4% 证据）预留全部改写空间。若后续
运行环境升到 Python ≥3.10，可平移回 ragas 库（输入/输出契约不变）。

## 成本锚点

4 指标 × ~5 次判官调用/case × 196 case ≈ 1000 次调用、约 1.1M judge
tokens/轮 —— DeepSeek 计价 ≈ ¥2–10/全量轮（与调查报告 §2.2 锚点一致）。
冒烟用 `--max-cases 5` 先行。

## SP-22 E2/E4 追加工具（2026-09-18）

- `judge_calibration.py`（E2）：标注真值 × DeepSeek judge 3 次重跑 + 中文
  prompt 翻转探针，产出一致率/方差/分层偏差（报告见
  docs/working/2026-09-18-sp22-e2-judge-calibration-report.md）。真值文件
  `artifacts/evals/e2-human-labels.json`（模型标注口径，抽检前按该语义解读）。
- `grid_ablation.py`（E4）：混合权重网格驱动器（rrf_k / bm25+vector 候选数 /
  final_topk / rerank 开关；refusal_threshold 与 chunk_strategy 为 R3/R4 预留
  轴）。逐配置写 override yaml → `cmd/rag-eval -skip-generation -record` →
  身份断言 → `artifacts/evals/grid/<split>/comparison.md` + eval_runs 落行。
  首扫 8 配置结论：rerank 全指标最优（SP-24 R1 证据面），rrf/候选池在当前
  语料为零灵敏度轴。
- `cmd/rag-eval -record -run-key-prefix grid`：检索测量落 eval_runs；
  `rag_eval/grid_headline.go` 的 headline 指标与 PR 门禁同公式。
- 坑：eval_runs.dataset_checksum 是 varchar(64)（剥 sha256: 前缀入库）；
  make_pr_gate_snapshot.py 不得对 retrieved_ids 排序（E4 抓出的排名序伪影，
  MRR 曾被腰斩到 0.46，真实 0.91）。
