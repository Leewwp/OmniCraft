# RAG Provenance 审计与 Current Corpus v1

> Created: 2026-08-26
> **预计失效日期**: 2026-10-26
> Scope: local interview evidence; not a production release artifact

## 审计结论

本轮检查了当前 Git 历史、所有可见分支/tag、RAG seed、baseline 生成代码、
`eval_runs` 和本地 PostgreSQL 数据库。历史 `253-content` corpus 无法从当前仓库
重建：

- `backend/testdata/rag_eval_seed.sql` 从 `e1364be` 才加入，描述的是 39 个保留
  evaluation content（其中 34 个 published），不是 253 条历史语料快照；
- `backend/testdata/rag_eval_baseline.json` 只保存指标、per-case hits 和环境计数，
  不保存 content body、ID 清单、seed dump 或 corpus checksum；
- 当前 `eval_runs` 只有两条历史本地运行记录，环境中的 `corpus_contents=38`；
- Git 历史、tag、分支和可达/不可达对象中没有可用的 253-content SQL dump、JSON
  manifest 或数据库备份；
- 因此不能创建一个声称“封存历史 253 corpus”的 tag，也不能把当前 169 条语料
  伪装成历史 253 条语料。

这与 [面试证据编排规格](2026-08-22-interview-evidence-orchestration-spec.md)
一致：P0 的结果是“历史 provenance 不可恢复”，P1 必须转入显式命名的新 corpus，
而不是修改旧 baseline。

## Current corpus v1

当前本地数据库形成了可用于下一轮 differential 的明确输入：

| 字段 | 值 |
|---|---|
| corpus id | `current-local-published-v1` |
| repository commit | `f4f9484` |
| source seeds | `backend/testdata/rag_eval_seed.sql` + `scripts/seed_local_rich_data.py` |
| source seed revision | `f4f9484` |
| active golden cases | 63 |
| published, not deleted scope | 169 |
| published + public, not deleted | 163 |
| anonymous visibility after author/IP predicates | 161 |
| content identity checksum | `sha256:240b3bac94916e791f41436abf9ab73fbb39702cd9b25f86cda264990a3765be` |
| golden-set checksum | `sha256:e759d965ae70cb2bd0cb1022735207c307b13b6d23c9946dbc88c01225521b04` |
| chunking | version 1, max 512 tokens, overlap 48 |
| index generation | 1 for the local projection; real MiniMax run is isolated in generation 2 |
| visibility predicate | published, non-deleted, non-banned author, non-banned IP, public or owner |

169 条 scope 由 34 条 RAG evaluation seed content 和 135 条 local rich seed
content 组成。这个 corpus 不是生产数据，也不是历史 253 corpus；它的价值是让
下一轮 MiniMax differential 在同一内容集合、同一 golden set、同一 visibility
predicate 下可复核。

`docs/working/2026-08-26-rag-current-corpus-v1-manifest.json` 保存了这份机器可读
身份记录。评测报告中的 `corpus_content_checksum` 现在基于 content identity
字段（而不是仅 content ID），因此标题、正文、可见性或 latest version 变化都会
导致 checksum 变化。

## P1 执行状态

独立数据库 `omnicraft_test_minimax_20260822` 仍存在，并已验证：169 条 published
scope、169 个 chunk、169 个 `embo-01` embedding，维度为 1536。内容向量不需要
重新生成，可以直接复用这份隔离 generation。

用户已轮换凭证并通过本机 `.env` 完成 MiniMax provider preflight；未在聊天、报告或 Git
中记录密钥。真实 `embo-01` query embedding 已在该隔离库完成 K=10 与 K=20 differential，
结果见同日的 real evidence 报告。历史 baseline 保持不变，真实结果仍只属于 current-v1
本地面试证据，不代表生产质量或 SLA。

报告中的 `projection_generation` 已从 `index_projection_status` 的 ready/current 状态读取；
两份 real 报告均为 generation 2。该字段不再把配置的 `generation_start` 误当成实际运行代际。
