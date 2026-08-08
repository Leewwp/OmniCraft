# OmniCraft UI 权威与文件归属矩阵（#65 交付物）

> 创建日期：2026-08-08 | **预计失效日期**: 2026-10-08
> 范围：#64（Web 体验修复 14 项问题）、#80（媒体体验设计）、已归档 #1（P-01 UI Polish）、当前活计划注册表（source-linkage #96 / collaboration-invites #97 / Production Readiness）的重叠与归属
> 依据：`docs/superpowers/specs/2026-08-07-omnicraft-web-experience-corrections-design.md`、`docs/superpowers/specs/2026-08-08-omnicraft-media-experience-design.md`、`design/ui-spec.md`、AGENTS.md 活计划注册表
> 权威关系：本矩阵是执行协调材料，不替代 `design/ui-spec.md`（唯一视觉权威）与各设计规格；冲突时以「文档权威源」一节（AGENTS.md）的优先级为准。

## 1. 目的

后续票据 #66~#76 与媒体 #81~#90 只实现和验证，不得自行解释冲突或重写同一权威章节。本矩阵回答两个问题：

1. **重叠范围归谁**：同一问题出现在多个输入（audit 14 项 / 媒体规格 / 已归档 P-01 / 活计划）时的保留/排除结论。
2. **共享面归谁**：共享文件（组件、i18n、路由、repo）的精确 owner 与串行顺序。

## 2. #64 十四项问题 → 归属 ticket → ui-spec 章节

来源：`docs/working/2026-08-07-ui-issues-audit.md` §2 问题总览（14 项），经 #64 规格确认。

| # | 问题 | 归属 ticket | ui-spec 权威章节 | 保留/排除结论 |
|---|------|-------------|------------------|---------------|
| 1 | 侧边栏背景与页面背景不同 | #66 统一导航壳 | `Header` / page-shell 契约 | 保留：统一 page-shell 背景契约 |
| 2 | 侧边栏与品牌入口不对齐 | #66 | `Header` / page-shell 契约 | 保留：统一宽度与 gutter 契约 |
| 3 | 品牌入口应跳推荐流 | #66 | `Header` | 保留：桌面/移动品牌入口均跳 `/recommend` |
| 4 | 浮窗缺少共享元素转场 | #67（原型）→ #68（接入） | `ContentDetailOverlay` | 保留：FLIP 几何为核心，View Transition 渐进增强 |
| 5 | 系列目录应留在浮层内 | #69 | `SeriesNav` + `ContentDetailOverlay` | 保留：目录改为浮层内可滚动选择器；独立系列页仍可用（#64 决策 15） |
| 6 | “下一章”按钮溢出 | #69 | `SeriesNav` | 保留：系列操作行可 reflow，不越界 |
| 7 | 最近访问 IP 未绑定用户 | #73 IP 访问历史 | `IPCard` / IP 详情页 | 保留：匿名 localStorage + 登录幂等合并；独立 IP 访问历史模型 |
| 8 | IP 详情讨论区无发帖入口 | #70 | `/ip/[ipId]` IP 详情页 | 保留：compact 讨论区补发起讨论入口 + 空态 CTA |
| 9 | 封面与正文边框不对齐 | #68 | `ContentDetail` / `ContentDetailOverlay` | 保留：封面与正文共享同一水平框架 |
| 10 | 关注按钮状态样式反了 | #70 | `FollowButton` | 保留：未关注 primary / 已关注 outline + hover 取消关注 |
| 11 | 点赞/点踩状态刷新丢失 | #71 | `ReactionBar` | 保留：`viewer_reaction` 与公开聚合分离 |
| 12 | 筛选选中态三处不统一 | #66（pill 契约） | `/ips`、`/original`、`/` 筛选章节 | 保留：统一 IP 库彩色药丸 + 语义属性 |
| 13 | 排序下拉覆盖本体 | #72 | `SortSelect` | 保留：共享自定义 listbox（trigger + 定位菜单） |
| 14 | 已收藏状态未显示 | #74（收藏成员关系 cutover 系列） | `ContentDetail` / `CollectionPicker` / `ReactionBar` | 保留：“已收藏”= 收藏成员关系唯一事实源；退役旧 favorites 语义 |

## 3. #80 媒体体验 → 归属 ticket → ui-spec 章节

来源：`docs/superpowers/specs/2026-08-08-omnicraft-media-experience-design.md`。

| 媒体规格内容 | 归属 ticket | ui-spec 权威章节 | 保留/排除结论 |
|--------------|-------------|------------------|---------------|
| 原创区筛选不生效 | #81 | `/original` 分类 Tab + `SortSelect` | 保留：无分类 → recommended；有分类 → hot 降级 |
| 收藏集弹窗关闭行为/通知 | #82 | `CollectionPicker` | 保留：背板/Esc 关闭、busy 阻止、弹窗内通知 |
| 媒体集数据/发布合同（063 迁移） | #83（heavy） | `FileUploader` / `PublishForm` 媒体编排 | 保留：宽高 + `sort_order` 持久化，读取确定性排序 |
| 发布端媒体编排 UI | #84 | `FileUploader` / 发布端媒体编排章节 | 保留：多选/九宫格/拖拽/移除/宽高采集/poster |
| 详情/浮层媒体区（contain + 几何稳定） | #85 | `MediaGallery`（新增章节） | 保留：contain 不裁切、指示点、滑动/按钮翻页、超高限高 |
| 全屏媒体查看器 | #86 | `MediaViewer`（新增章节） | 保留：层级（叠加浮层最上）、缩放、翻页、三种退出 |
| 列表卡片自然比例瀑布流 | #87 | `ContentCard` / `MasonryGrid` | 保留：`cover_width/height` 数据驱动，极端比例限高，3:4 默认 |
| 桌面双栏详情浮层 | #88 | `ContentDetailOverlay` | 保留：左媒体右信息，媒体高 ≤ 视口可用，信息独立滚动；文本型单栏 |
| 移动端连续浏览 | #89 | `ContentDetailOverlay` | 保留：上下文列表 + 索引入栈，上滑前进，到底提示 |
| 相关内容块与到底提示 | #90 | `RelatedContents`（新增章节） | 保留：复用列表 API filtered-hot，消费 source-linkage #96 的最终 related-fanworks 合同 |

**排除结论（#80 Out of Scope，不进入 ui-spec 权威正文）**：Tauri 桌面壳、服务端自动截帧、图文混排媒体集、“上一篇”导航、桌面自动加载下一篇、老数据回填、推荐算法本身。

## 4. 已归档 #1（P-01）与本轮的关系

- #1 已 CLOSED 并归档，仅作为**已完成的历史边界输入**，不再是可领取的执行来源（#65 票据原文）。
- 与 #64 重叠的 P-01 产物按「已接受视觉基线」处理：`/`、`/original`、`/ip/[ipId]` 视觉基线保留；`/ips` 方案 B（Indigo 精修）保留；Header/page-shell 的 U-03 输出由 #66 收口（修复审计发现的 1440/1280 宽度漂移）。
- P-01 已批准的原语视觉（U-01/U-02B）、治理门（U-12，doc-validator token 检查）在本轮保持生效，不重复改写。
- **保留**：P-01 原型期“共享浮窗无硬上限”已被 2026-08-04 三缺口执行规划改为栈深 ≤ 5；本轮以 ≤ 5 为准。

## 5. 术语权威（不得引入含义冲突的替代名称）

| 权威术语 | 含义（GLOSSARY 为准） | 禁止替代 |
|----------|----------------------|----------|
| 收藏成员关系 | 内容被纳入用户至少一个活动收藏集的关系 | “收藏夹”、“favorites 语义” |
| 查看者反应 | 当前登录用户对单个目标的唯一点赞/点踩，与公开聚合分离 | “my reaction”列、“viewer_reaction 数据库列” |
| 媒体集 | 内容内可顺序浏览的媒体序列（图片/视频） | 把 image/video 多文件称为“附件” |
| 附件 | 面向下载的素材文件（mod/乐谱/音频/模板等） | 把媒体文件称为附件 |
| 媒体查看器 / 连续浏览 / 相关内容 / 首图即封面 | 见 GLOSSARY 第 71~74 行 | “图片预览弹窗”、“下一篇推荐”、“三创” |

## 6. 文件 owner / 串行规则（精确到文件）

> 与 source-linkage #96、media #80、collaboration-invites #97、Production Readiness、Web Agent Task 6 的共享面。规则与本文件不一致时停止并先修正，不自行选择较宽松的一方。

| 共享文件 | owner 顺序 | 串行说明 |
|----------|-----------|----------|
| `frontend/components/content/ContentDetail.tsx` | #65（权威）→ #68（收敛 + overlay hook）→ #96（来源归因消费）→ #90（相关内容块）→ #97（邀请卡片） | C2+C6 由 #68 承担；#71/#69/#72 在其后按文件预约串行 |
| `frontend/components/content/ContentDetailOverlay.tsx` + `*Layer.tsx` | #67（原型）→ #68（接入）→ #85/#88/#89（媒体区/双栏/连续浏览）→ #90 | #67 不共享生产文件（原型验证）；#68 提供共享 overlay hook |
| `frontend/components/content/SeriesNav.tsx` | #69（目录选择器 + 响应式修复）→ 后续仅实现 | #69 在 #68 之后串行 |
| `frontend/components/content/ReactionBar.tsx` | #71（viewer_reaction 契约收口）→ 后续仅实现 | #71 在 #68/#69 之后；#74 消费其收藏入口 |
| `frontend/components/content/RelatedFanworks.tsx` | #96（最终合同）→ #90（组装相关内容块） | #96 先交付数据合同，#90 只消费不发明 |
| `frontend/components/layout/Header.tsx` / `Sidebar.tsx` | #66（page-shell/品牌入口）→ Web Agent Task 6 Step 2（浮窗转场后） | #66 先统一契约面 |
| `frontend/components/social/FollowButton.tsx` | #70 | 无共享面冲突 |
| `frontend/components/social/DiscussionBoard.tsx` | #70 | 与 #109（评论/讨论文本审核）共享组件但改动面不同（文案 vs 能力），串行预约 |
| `frontend/components/collections/CollectionPicker.tsx`（或等价路径） | #82（关闭行为/通知）→ #74（成员关系唯一源） | #82 在 #65 权威落地后、#74 前 |
| `frontend/components/uploader/FileUploader.tsx`（或等价路径） | #84（编排 UI）→ #97（CollabUserPicker 扩展字段） | #84 依赖 #83 的发布合同 |
| `frontend/components/content/SortSelect.tsx`（共享排序） | #72（新组件，替换三处原生 select） | #72 在 #71/#69 之后串行（共享 i18n 与 ContentDetail） |
| `frontend/lib/content.ts`（normalizer） | #65（权威）→ #68 → #74（收藏成员关系）→ #90 | 逐级串行，不并发改写 |
| `frontend/messages/zh.json` / `en.json` | #66/#70/#71/#72/#82/#84/#90/#97 全部需要 | 按注册表 DAG 串行；i18n namespace 建议见各规格 |
| `backend/internal/repository/content_repo.go` | #96 → #97 | source-linkage 先合并，collaboration-invites 后执行 |
| `backend/internal/router/routes.go`、`backend/config.yaml`、`backend/internal/config/config.go` | #104/#97/#73/#76 等串行预约 | 改动后跑 doc-validator `--fix` |
| `backend/migrations/*.sql` | #83（063）→ #96 无迁移 → #97（065）→ #73（066）→ #76（067） | 编号 forward-only，历史迁移不可改 |
| `scripts/verify-project.sh` | 各 ticket 按需追加浏览器契约 | 与 Production Readiness / Web Agent 的验证门串行 |
| `design/ui-spec.md` | **#65 是唯一权威更新 owner**（#65→#80 相关权威已在本轮一次性写入） | 后续 #66~#90 只实现和验证，不得重写相同章节；确需修订时先在本矩阵登记 |

## 7. 与相邻计划的边界

| 计划 | 边界 |
|------|------|
| source-linkage #96 | 交付最终 `RelatedFanworks`/related-fanworks 契约；#90 消费；共享 `ContentDetail.tsx`、`content_repo.go`、i18n |
| collaboration-invites #97 | 最后接入 PublishForm/i18n；共享 `content_repo.go`、`PublishForm.tsx`、`zh.json`/`en.json` |
| Production Readiness | Ops-00~08 已收口；Ops-09 桌面范围暂缓；本轮不触碰桌面能力与发布声明 |
| Web Agent Task 6 | 真实 Provider smoke 仍阻塞（无 `agent.llm_api_key`）；建议在 #64 T03/T04 浮窗转场落地后执行 Step 2 |
| content-safety 批次 #103~#109 | 共享 `DiscussionBoard.tsx`、评论/讨论文案面（#109），按文件预约串行；不共享媒体权威章节 |
| moderation 批次 #111~#113 | 头像/私信/反馈附件图片审核，与媒体集上传编排 #84 共享上传链路语义但不共享实现文件（#84 在 #111 前完成 UI 契约） |

## 8. 变更记录

- 2026-08-08：#65 首次建立。矩阵覆盖 #64 14 项、#80 全部 10 票、已归档 #1、活计划注册表；ui-spec 一次性写入 Header/page-shell、/original 分类 Tab、ContentCard/MasonryGrid、FileUploader/发布端媒体编排、MediaGallery、MediaViewer、RelatedContents、ContentDetail、ContentDetailOverlay、SeriesNav、IP 详情讨论、IPCard、ReactionBar、FollowButton、SortSelect、CollectionPicker 权威。
