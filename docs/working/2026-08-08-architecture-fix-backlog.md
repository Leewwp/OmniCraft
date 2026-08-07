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
| C2 | ContentDetail 家族收敛（双侧栏/死代码/历史记录器/类型） | 基本确认 | 并入 #64 前置（不单独拆 issue）；低风险子项（删 ContentDetailClient、类型去重、"IP: " 硬编码、历史记录器收敛）可立即做；侧栏合并须在 #64 T03/T04 与媒体 T05 之前 | 先于 #64 T03/T04 与媒体 T05 |
| C3 | 可见性策略重复 | 部分确认（collection/series/agent 已复用共享 scope，非四处全手写） | 写入 source-linkage 计划实现约束与测试项；**不新增**「来源被封禁即隐藏」全局规则，只按计划在来源不可见时隐藏摘要 | 随 source-linkage（066） |
| C4 | 服务工厂/DB 逃生门 | 确认（架构债务） | 后续架构 issue，不作为 collaboration-invites 硬前置（避免扩大 feature 票范围） | source-linkage 后、invites 前重新评估 |
| C5 | Agent god object/双流式客户端 | 部分确认（前端循环为 type-only；useSSE 仍被 UsageGuidePanel 使用，不能直接删） | 单独 Agent 后续架构 issue，真实 Provider smoke 解阻后再排期；不并入 #64 | Task 6 之前 |
| C6 | Overlay 接线复制 ×4 | 确认 | 与 C2 合并为一个「共享 overlay hook」前置项 | 同 C2：先于 #64 T03/T04 与媒体 T09 |
| C7 | 迁移账本（061 缺口/fixture/元数据） | 部分确认（061 不可回填；metadata.json 只登记特殊事务迁移，非全量快照；当前 fixture baseline=050） | **验证门前移**：在媒体 T04（#84，067 迁移）中增加非连续 migration fixture 验证门；不建独立 issue；不补 061、不改历史迁移 | 媒体 T04 之前/之中 |
| C8 | 路由组合根/AST 门 | 属实但非当前故障（invites 计划 routeOwner 已兼容实际路径） | 后续架构 issue，暂不阻塞 invites | invites 前重新评估 |

## 次要项裁决（不建 issue）

- safe_error.go 的 init() 为空**不代表**错误表是死的——表已有 5 个基础错误在用；不建 issue。
- ContentDetail.tsx:180 硬编码 `IP: ` 是真实 i18n 违规，随 C2 修复。
- `source_fanwork_id` 后端 0 命中 = source-linkage 尚未实现，非独立漂移 bug；api.md 属 spec-ahead 文档，随 source-linkage 闭合。

## CI gate 修复（发布门全红）

- 三个 gate 在 main 上 pre-existing 全红（Ops alerting / SBOM / Security），阻断一切正常 PR 合并（branch protection `project-gate`）。
- 已建 heavy 车道 issue **#92** 修复；修复前合并需 admin-merge（#79/#91 已按此处理）。

## 执行顺序（2026-08-08 确认，已写入 AGENTS.md 注册表）

1. #64 T01/T02/T06（light）∥ 媒体 T01/T02/T03（无阻塞）
2. **C2+C6 批**（侧栏收敛 + overlay hook；先于 #64 T03/T04 与媒体 T05/T09）
3. #64 T03/T04（浮窗转场/媒体加载）→ 媒体 T04（067 + C7 验证门）→ 媒体 T05~T10
4. source-linkage（066，C3 随行）→ #64 T07/T05/T08 → T09（064）→ T10~T12（065）
5. collaboration-invites（063，C8 前置评估）→ Web Agent Task 6（C5 前置评估；等真实 llm_api_key）

## 备注

- 审查建议过但被裁决暂缓/否决的项（C4/C5/C8 的"单独 issue"、C7 的"元数据生成器"）未创建 issue，未来审查如再次建议，先读本文件。
