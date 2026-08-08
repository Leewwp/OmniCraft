# 架构修复 Backlog（2026-08-08 审查裁决）

**创建日期**: 2026-08-08 · **预计失效日期**: 2026-10-08（所有条目落地或重新评估后归档）

> 创建日期：2026-08-08 · 预计失效日期：2026-10-08（所有条目落地或重新评估后归档）
> 来源：2026-08-08 架构审查（improve-codebase-architecture 流程）→ 用户逐项裁决。
> 审查报告原件在临时目录（不持久），本文件是裁决的持久事实源。

## 背景

2026-08-08 对仓库做架构审查，产出 8 个候选（C1~C8）与次要项。用户审查后逐项裁决归属与时机。C1 已作为 P1 bug 立即修复（issue #78 / PR #79，已合并）。本文件记录其余条目的处置，防止未来会话重复建议。

## 各条目裁决

| ID | 候选 | 真实性裁决 | 处置 | 窗口/前置 |
|----|------|-----------|------|-----------|
| C1 | 发布冻结键分叉 | 确认（enforcement bug） | **已完成**：#78/#79 已合并，共享 key builder `internal/pkg/rediskeys.PublishFreezeKey` + 跨 seam 测试 | — |
| C2 | ContentDetail 家族收敛（双侧栏/死代码/历史记录器/类型） | 基本确认 | **并入 #68 验收**（不单独拆 issue）：删 ContentDetailClient、类型去重、`IP: ` i18n、历史记录器与侧栏收敛先在 #68 内完成，再接正式转场 | #68 内、正式转场实现之前 |
| C3 | 可见性策略重复 | 部分确认（collection/series/agent 已复用共享 scope，非四处全手写） | 写入 source-linkage 计划实现约束与测试项；**不新增**「来源被封禁即隐藏」全局规则，只按计划在来源不可见时隐藏摘要 | 随 source-linkage（064） |
| C4 | 服务工厂/DB 逃生门 | 确认（架构债务） | 后续架构 issue，不作为 collaboration-invites 硬前置（避免扩大 feature 票范围） | source-linkage 后、invites 前重新评估 |
| C5 | Agent god object/双流式客户端 | 部分确认（前端循环为 type-only；useSSE 仍被 UsageGuidePanel 使用，不能直接删） | 单独 Agent 后续架构 issue，真实 Provider smoke 解阻后再排期；不并入 #64 | Task 6 之前 |
| C6 | Overlay 接线复制 ×4 | 确认 | 与 C2 一并写入 #68：抽取共享 overlay entry hook，四个入口只保留来源参数/触发 ref 差异 | #68 内、正式转场实现之前 |
| C7 | 迁移账本（061 缺口/fixture/元数据） | 部分确认（061 不可回填；metadata.json 只登记特殊事务迁移，非全量快照；当前 fixture baseline=050） | **规划纠正**：不再故意先应用 067 再补 063~066；未创建迁移按真实合并序重排为媒体 063→source 064→invites 065→IP 066→favorites 067。仍不补 061、不改历史迁移 | 已在本轮规划审计收口 |
| C8 | 路由组合根/AST 门 | 属实但非当前故障（invites 计划 routeOwner 已兼容实际路径） | 后续架构 issue，暂不阻塞 invites | invites 前重新评估 |

## 次要项裁决（不建 issue）

- safe_error.go 的 init() 为空**不代表**错误表是死的——表已有 5 个基础错误在用；不建 issue。
- ContentDetail.tsx:180 硬编码 `IP: ` 是真实 i18n 违规，随 C2 修复。
- `source_fanwork_id` 后端 0 命中 = source-linkage 尚未实现，非独立漂移 bug；api.md 属 spec-ahead 文档，随 source-linkage 闭合。

## CI gate 修复（发布门全红）

- 三个 gate 在 main 上 pre-existing 全红（Ops alerting / SBOM / Security），阻断一切正常 PR 合并（branch protection `project-gate`）。
- 已建 heavy 父 issue **#92** 与独立子票 #93 Ops alerting、#94 SBOM、#95 Security，并登记为活计划优先级 0；修复前不得把 admin-merge 当正常交付路径（#79/#91 为历史例外）。

## 执行顺序（2026-08-08 确认，已写入 AGENTS.md 注册表）

1. #92/#93~#95 恢复正常合并门；#65 对齐 #64/#80 与 ui-spec 权威。
2. #66/#67/#70/#82 与媒体 heavy T03 (#83) 按文件预约并行；#81 在 #66 后，媒体 UI T04 (#84) 在 #83/#65 后。
3. #68 内先完成 C2+C6，再接正式浮窗转场；媒体 T05~T09 在 #68 与数据合同后执行。
4. source-linkage（064，C3 随行）→ 媒体 T10（复用最终 RelatedFanworks 合同）→ collaboration-invites（065，C8 前置评估）。
5. #64 T07/T05/T08 无业务硬依赖但共享文件预约串行；三票均完成后 → T09（066）→ T10/T11/T12（067 + 云端人工门）→ Web Agent Task 6（C5 前置评估；等真实 llm_api_key）。

## 备注

- 审查建议过但被裁决暂缓/否决的项（C4/C5/C8 的"单独 issue"、C7 的"元数据生成器"）未创建 issue，未来审查如再次建议，先读本文件。
