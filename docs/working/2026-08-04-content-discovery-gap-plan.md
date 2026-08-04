# 内容发现闭环三缺口执行规划（推荐页 + 浮窗浏览 + Agent 工作台）

> 创建日期：2026-08-04 | 预计失效日期：2026-10-04（三缺口全部合并后归档）
> 本文件登记 to-spec/to-tickets 产出；追踪状态以 GitHub issues 为准。
> **预计失效日期**: 2026-10-04

## 背景

P-01 原型（`opencode/p01-ui-prototype@09cf059`，远端备份 `origin/opencode/p01-ui-prototype`）验证了完整"上下文保留的内容发现闭环"，批准记录在 `docs/working/2026-07-25-ui-prototype-review.md`（A1.5 验收标准）。生产端已上线：二创 `/`、原创 `/original`、IP 库 `/ips`（IP 库改造已完成，U-09 回写 P-01 方案 B）。以下三个生产缺口此前未落入任何计划。

## 三缺口

1. **推荐页 `/recommend`**：独立顶级浏览面（区别于二创/原创/Agent 工作台），单一"为你推荐"内容流（瀑布流卡片，无分区标签），数据源复用 `GET /api/v1/contents`（ListContents），排序不足则扩展参数。
2. **共享内容详情浮窗（浮窗浏览）**：全站内容卡片入口统一打开同一详情浮层；逐层返回导航栈（栈深度 ≤ 5）+ 全退 + 每层滚动记忆 + 退出恢复触发点；除直接 URL 访问详情页外，其余场景访问内容一律走浮窗。
3. **Agent 工作台 `/agent`**：受保护路由，工作台外壳（会话列表 + 主对话区），复用 AgentChatWidget 会话能力；新对话/清空历史遵循既有生命周期契约；工作台内内容访问经浮窗。

## GitHub 追踪

| 类型 | Issue | 阻塞 | 状态 |
|------|-------|------|------|
| Spec | [#14](https://github.com/Leewwp/OmniCraft/issues/14) | — | ready-for-agent |
| Ticket 01 ui-spec 三章 | [#15](https://github.com/Leewwp/OmniCraft/issues/15) | 无 | frontier |
| Ticket 02 内容浮窗底座 | [#16](https://github.com/Leewwp/OmniCraft/issues/16) | #15 | frontier |
| Ticket 03 推荐页 | [#17](https://github.com/Leewwp/OmniCraft/issues/17) | #15 #16 | blocked |
| Ticket 04 全站入口接浮窗 | [#18](https://github.com/Leewwp/OmniCraft/issues/18) | #15 #16 | blocked |
| Ticket 05 Agent 工作台 | [#19](https://github.com/Leewwp/OmniCraft/issues/19) | #15 #16 | blocked |

执行顺序：**#15 → #16 → #17 / #18 / #19**（后三张共享 `zh.json`/`en.json`，须串行编辑；每票一个独立会话 implement，tdd + code-review）。

## 总执行顺序（用户确认）

1. Production Readiness Ops-02~08（Web 发布就绪）
2. Community source-linkage → collaboration-invites（共享文件串行）
3. wayfinder T1（规格 §4.2 L2 工具集修订，light，须在 Web Agent Productization Task 6 前）
4. **本三缺口（#15~#19）**
5. Web Agent Productization（Task 4 与工作台衔接：Widget 重构、Agent 引用浮层接线）
6. multiplatform（wayfinder T2~T7，桌面/移动范围恢复后）

## 协调点

- AGENTS.md 注册表"生产 `/agent` 归 Web Agent Productization Task 4 独占"声明 → 修改为"本计划先行落地工作台、Task 4 衔接"（三缺口启动时执行）。
- 共享文件：`frontend/messages/zh.json`、`en.json`（三缺口与 web-agent 串行）；`design/ui-spec.md` 三章先行（#15）。
- 后端改动面：仅推荐排序参数扩展（只读，不动审核/上传/互动链路）；无新迁移。
- 桌面/移动端不在本计划范围（multiplatform 长线）。
