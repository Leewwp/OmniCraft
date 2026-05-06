# OmniCraft 审查与原创/二创联动设计计划

## Summary

本轮审查结论：项目方向成立，但当前实现还处在“可编译但未稳定可用”的阶段。优先级最高的问题不是新增按钮，而是先稳定内容 API 契约、原创区页面、缓存策略和数据关系，否则“相关二创”会建立在不可靠的数据流上。

已验证：`frontend npm run lint` 通过；`backend go test ./...`、`go vet ./...`、`go build ./...` 通过。浏览器验证使用 `http://localhost:3000`，测试了首页筛选、原创区导航、原创卡片点击、主题切换、搜索输入、移动端视口；截图包括 `omnicraft-home-viewport.png`、`omnicraft-original-review.png`、`omnicraft-home-mobile.png`。后端 `8080` 未监听，页面出现 API 连接错误；Next 旧 fetch cache 仍渲染出旧数据，导致原创卡片跳到 `/content/undefined` 并 404。

## Review Findings

- P0: 内容列表存在旧 API 缓存/字段契约风险。[original/page.tsx](/c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/app/(public)/original/page.tsx:23) 使用 `revalidate: 30`，Next fetch cache 中还残留旧版大写字段，前端 [ContentCard.tsx](/c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/content/ContentCard.tsx:83) 只读取 `id/title/zone`，实测原创卡片链接变成 `/content/undefined`。
- P0: 原创区实现与任务/架构不一致。`task.json` Task 25 写“复用 ContentDetail”，但 [original/[contentId]/page.tsx](/c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/app/(public)/original/[contentId]/page.tsx:62) 是手写详情；Task 64 的两级导航仍未完成，和 [architecture.md](/c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md:1627) 不一致。
- P1: 迁移脚本有重复建表和 schema 漂移风险。[011_social.sql](/c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/migrations/011_social.sql:1) 已建 `discussions/browse_history/follows/appeals`，后续 [032_browse_history.sql](/c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/migrations/032_browse_history.sql:1) 到 `035_discussions.sql` 又用不同约束重建同名表。
- P1: 信誉分规则实现不符合 `CLAUDE.md`。[social_service.go](/c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/social_service.go:18) 把评论门槛硬编码为 `0`，而 [CLAUDE.md](/c:/Users/16278/Desktop/file/code/project/OmniCraft/CLAUDE.md:238) 要求低于 3 分禁止评论、发布、众裁、PR、点赞点踩。
- P1: 发布页 Markdown 编辑器未进入后端数据模型。[publish/page.tsx](/c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/app/(protected)/publish/page.tsx:55) 保存了 `markdown`，但 [content_service.go](/c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/content_service.go:34) 的 `PublishContentInput` 没有 `description/body`。
- P2: Header 移动端隐藏导航和搜索但没有替代入口；主题按钮存在水合不一致。[Header.tsx](/c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/layout/Header.tsx:20) 直接用 `theme` 渲染图标，浏览器控制台出现 hydration mismatch。

## Key Changes

- 先稳定内容数据契约：后端公开接口统一返回 snake_case DTO；前端增加 `normalizeContentItem` 兼容旧缓存的大写字段，并过滤缺少有效 `id/title/zone` 的卡片；公共浏览页在 MVP 阶段改用 `cache: "no-store"` 或明确 cache tag 失效策略，避免旧 fetch cache 继续污染页面。
- 补齐原创区基础体验：实现 `/original` 两级导航，一级分类来自 `GET /categories?zone=original&level=primary`，二级筛选按架构映射为 `content_type`/`tag` 查询；原创详情复用公共详情展示能力，但隐藏 PR 和版本历史。
- 修复移动端 Header：提供菜单入口，包含二创区、原创区、搜索、登录/注册或用户菜单；主题图标在 mounted 后渲染，避免 SSR/客户端图标不一致。
- 清理后端一致性：把 `Category` 从 `notification.go` 拆到独立模型文件；迁移脚本做一次基线审查，保留一个权威表定义，后续脚本只做 `ALTER TABLE`/补索引；信誉分门槛改从配置读取。
- 文档同步：`task.json` 增加“原创/二创来源联动”任务；修正 Task 25、64、78 的实际状态与文件边界；`design/ui-spec.md` 增加“相关二创”按钮、列表页、空态；`architecture.md` 增加 schema/API；`CLAUDE.md` 增加 zone 与来源关系规则。

## Original/Fanwork Link Design

采用“单原创来源”模型：每个二创最多绑定一个原创内容。

- 数据库：给 `content_items` 增加 nullable `source_original_id BIGINT REFERENCES content_items(id) ON DELETE SET NULL`，只允许 `zone='fanwork'` 使用；应用层校验被引用内容必须是 `zone='original'` 且 `status='published'`；新增索引 `(source_original_id, status, created_at DESC)`。
- 后端模型/API：`PublishContentInput` 增加 `source_original_id`；`GET /api/v1/contents?source_original_id=<id>` 支持过滤；新增 `GET /api/v1/contents/:id/related-fanworks?page=&page_size=&sort=&content_type=` 返回 `{source_original_id,total,page,page_size,contents}`。
- 前端发布：当发布区为二创区时，增加可选“来源原创”选择器；从原创详情进入发布时可预填 `source_original_id`；原创发布时如果带该字段直接拒绝。
- 原创详情：请求第一页相关二创，`total > 0` 时在主操作区显示“相关二创”按钮，点击进入 `/original/[contentId]/fanworks`；无相关内容时不显示按钮。
- 相关二创页：复用 `MasonryGrid/ContentCard`，顶部显示原原创标题、返回原创详情、排序和内容类型筛选；空态显示“暂无相关二创”。

## Test Plan

- Backend unit/integration: 迁移包含 `source_original_id` 字段和索引；发布二创可绑定已发布原创；绑定不存在、非原创、未发布内容返回 400；原创内容带 `source_original_id` 返回 400；相关二创接口只返回 `published fanwork`。
- Frontend type/checks: `npm run lint`；验证 Content DTO 规范化能处理 snake_case 和旧大写字段；缺失 `id` 的数据不渲染可点击卡片。
- Browser verification: 启动后端和前端，访问 `http://localhost:3000/original/[id]`，确认“相关二创”按钮存在；点击跳转 `/original/[id]/fanworks`；切换排序/类型筛选；移动端菜单可访问原创区和相关二创页；控制台无 hydration/key/API 契约错误。
- Regression: 首页二创筛选、原创两级导航、内容详情、发布二创、发布原创、低信誉用户评论/点赞限制都要覆盖。

## Assumptions

- 第一版只支持“一个二创绑定一个原创来源”，不做多来源关联表。
- 旧数据不强制回填；已有二创默认 `source_original_id = null`，不会出现在相关二创列表。
- 相关按钮只在存在相关二创时显示，避免用户点击进入空结果。
- 当前 `8080` 后端未运行造成的页面 API 错误不是前端完成状态，后续实现验收必须启动完整后端依赖。
