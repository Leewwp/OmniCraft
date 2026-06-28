# OmniCraft 审查与原创/二创联动设计计划

> ⚠️ **历史文档**: 本文档描述的计划（Task 99-145）已全部完成（task.json 中 passes: true）。保留此文档仅作历史记录和设计决策参考。活跃开发请参阅 `docs/superpowers/plans/`。

## Summary

本轮审查结论：项目方向成立，但当前实现还处在"可编译但未稳定可用"的阶段。优先级最高的问题不是新增按钮，而是先稳定内容 API 契约、原创区页面、缓存策略和数据关系，否则"相关二创"会建立在不可靠的数据流上。

已验证：`frontend npm run lint` 通过；`backend go test ./...`、`go vet ./...`、`go build ./...` 通过。浏览器验证使用 `http://localhost:3000`，测试了首页筛选、原创区导航、原创卡片点击、主题切换、搜索输入、移动端视口；截图包括 `omnicraft-home-viewport.png`、`omnicraft-original-review.png`、`omnicraft-home-mobile.png`。后端 `8080` 未监听，页面出现 API 连接错误；Next 旧 fetch cache 仍渲染出旧数据，导致原创卡片跳到 `/content/undefined` 并 404。

## Review Findings

- P0: 内容列表存在旧 API 缓存/字段契约风险。[original/page.tsx](/c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/app/(public)/original/page.tsx:23) 使用 `revalidate: 30`，Next fetch cache 中还残留旧版大写字段，前端 [ContentCard.tsx](/c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/content/ContentCard.tsx:83) 只读取 `id/title/zone`，实测原创卡片链接变成 `/content/undefined`。
- P0: 原创区实现与任务/架构不一致。`task.json` Task 25 写"复用 ContentDetail"，但 [original/[contentId]/page.tsx](/c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/app/(public)/original/[contentId]/page.tsx:62) 是手写详情；Task 64 的两级导航仍未完成，和 [architecture.md](/c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md:1627) 不一致。
- P1: 迁移脚本有重复建表和 schema 漂移风险。[011_social.sql](/c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/migrations/011_social.sql:1) 已建 `discussions/browse_history/follows/appeals`，后续 [032_browse_history.sql](/c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/migrations/032_browse_history.sql:1) 到 `035_discussions.sql` 又用不同约束重建同名表。
- P1: 信誉分规则实现不符合 `CLAUDE.md`。[social_service.go](/c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/social_service.go:18) 把评论门槛硬编码为 `0`，而 [CLAUDE.md](/c:/Users/16278/Desktop/file/code/project/OmniCraft/CLAUDE.md:238) 要求低于 3 分禁止评论、发布、众裁、PR、点赞点踩。
- P1: 发布页 Markdown 编辑器未进入后端数据模型。[publish/page.tsx](/c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/app/(protected)/publish/page.tsx:55) 保存了 `markdown`，但 [content_service.go](/c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/internal/service/content_service.go:34) 的 `PublishContentInput` 没有 `description/body`。
- P2: Header 移动端隐藏导航和搜索但没有替代入口；主题按钮存在水合不一致。[Header.tsx](/c:/Users/16278/Desktop/file/code/project/OmniCraft/frontend/components/layout/Header.tsx:20) 直接用 `theme` 渲染图标，浏览器控制台出现 hydration mismatch。

## Key Changes

- 先稳定内容数据契约：后端公开接口统一返回 snake_case DTO；前端增加 `normalizeContentItem` 兼容旧缓存的大写字段，并过滤缺少有效 `id/title/zone` 的卡片；公共浏览页在 MVP 阶段改用 `cache: "no-store"` 或明确 cache tag 失效策略，避免旧 fetch cache 继续污染页面。
- 补齐原创区基础体验：实现 `/original` 两级导航，一级分类来自 `GET /categories?zone=original&level=primary`，二级筛选按架构映射为 `content_type`/`tag` 查询；原创详情复用公共详情展示能力，但隐藏 PR 和版本历史。
- 修复移动端 Header：提供菜单入口，包含二创区、原创区、搜索、登录/注册或用户菜单；主题图标在 mounted 后渲染，避免 SSR/客户端图标不一致。
- 清理后端一致性：把 `Category` 从 `notification.go` 拆到独立模型文件；迁移脚本做一次基线审查，保留一个权威表定义，后续脚本只做 `ALTER TABLE`/补索引；信誉分门槛改从配置读取。
- 文档同步：`task.json` 增加"原创/二创来源联动"任务；修正 Task 25、64、78 的实际状态与文件边界；`design/ui-spec.md` 增加"相关二创"按钮、列表页、空态；`architecture.md` 增加 schema/API；`CLAUDE.md` 增加 zone 与来源关系规则。

## Original/Fanwork Link Design

采用"单原创来源"模型：每个二创最多绑定一个原创内容。

- 数据库：给 `content_items` 增加 nullable `source_original_id BIGINT REFERENCES content_items(id) ON DELETE SET NULL`，只允许 `zone='fanwork'` 使用；应用层校验被引用内容必须是 `zone='original'` 且 `status='published'`；新增索引 `(source_original_id, status, created_at DESC)`。
- 后端模型/API：`PublishContentInput` 增加 `source_original_id`；`GET /api/v1/contents?source_original_id=<id>` 支持过滤；新增 `GET /api/v1/contents/:id/related-fanworks?page=&page_size=&sort=&content_type=` 返回 `{source_original_id,total,page,page_size,contents}`。
- 前端发布：当发布区为二创区时，增加可选"来源原创"选择器；从原创详情进入发布时可预填 `source_original_id`；原创发布时如果带该字段直接拒绝。
- 原创详情：请求第一页相关二创，`total > 0` 时在主操作区显示"相关二创"按钮，点击进入 `/original/[contentId]/fanworks`；无相关内容时不显示按钮。
- 相关二创页：复用 `MasonryGrid/ContentCard`，顶部显示原原创标题、返回原创详情、排序和内容类型筛选；空态显示"暂无相关二创"。

## Test Plan

- Backend unit/integration: 迁移包含 `source_original_id` 字段和索引；发布二创可绑定已发布原创；绑定不存在、非原创、未发布内容返回 400；原创内容带 `source_original_id` 返回 400；相关二创接口只返回 `published fanwork`。
- Frontend type/checks: `npm run lint`；验证 Content DTO 规范化能处理 snake_case 和旧大写字段；缺失 `id` 的数据不渲染可点击卡片。
- Browser verification: 启动后端和前端，访问 `http://localhost:3000/original/[id]`，确认"相关二创"按钮存在；点击跳转 `/original/[id]/fanworks`；切换排序/类型筛选；移动端菜单可访问原创区和相关二创页；控制台无 hydration/key/API 契约错误。
- Regression: 首页二创筛选、原创两级导航、内容详情、发布二创、发布原创、低信誉用户评论/点赞限制都要覆盖。

## Assumptions

- 第一版只支持"一个二创绑定一个原创来源"，不做多来源关联表。
- 旧数据不强制回填；已有二创默认 `source_original_id = null`，不会出现在相关二创列表。
- 相关按钮只在存在相关二创时显示，避免用户点击进入空结果。
- 当前 `8080` 后端未运行造成的页面 API 错误不是前端完成状态，后续实现验收必须启动完整后端依赖。

---

## 优化计划（Task 99–145）

以下优化任务按 5 个阶段执行，前置依赖关系：Phase 1（安全）→ Phase 2（UX + 通知）→ Phase 3（搜索 + 下载）→ Phase 4（性能）→ Phase 5（验证 + 架构）

### Phase 1: 安全加固（Task 99–105）

| Task | 标题 | 风险级别 |
|------|------|----------|
| 99 | Admin Config 泄露防护 | P0 - 密钥泄露 |
| 100 | CORS 策略收紧 | P0 - 跨域攻击 |
| 101 | Auth 实时状态检查 | P0 - 封禁绕过 |
| 102 | 错误消息脱敏 | P1 - 内部信息泄露 |
| 103 | 账号删除与 OSS 路径隔离 | P1 - 数据安全 |
| 104 | Goroutine Panic Recovery | P1 - 服务稳定性 |
| 105 | 受保护路由守卫 | P1 - 认证绕过 |

**验收标准**：
- Admin config API 返回的所有敏感字段为 `***REDACTED***`
- 非 allowlist 域名请求 CORS 被 reject
- 封禁用户请求立即被拦截（无需重新登录）
- 前后端无 `err.Error()` 直接暴露给用户
- 删除账号后内容作者显示为"已注销用户"
- Goroutine panic 不导致进程崩溃
- 未登录访问 `/studio/*` 等路由重定向到 `/login`

### Phase 2: UX 修复 + 通知系统（Task 106–118）

| Task | 标题 | 类别 |
|------|------|------|
| 106 | Studio 侧边栏暗黑模式 | 暗黑模式修复 |
| 107 | Footer 全局暗黑模式 | 暗黑模式修复 |
| 108 | 全面 i18n 抽取 | 国际化 |
| 109 | 首页统计数据替换 | 数据真实化 |
| 110 | 前端数据验证 | 数据安全 |
| 111 | 发布表单文件上传修复 | 功能修复 |
| 112 | 评论悬浮预览 | UX 改进 |
| 113 | 内容详情页互动增强 | UX 改进 |
| 114 | 通知自动创建后端 | 后端功能 |
| 115 | 通知铃铛 + 未读数 | 前端功能 |
| 116 | 私信 UI 完善 | 前端功能 |
| 117 | 密码重置流程 | 全栈功能 |
| 118 | 浏览历史自动记录 | 后端功能 |

**验收标准**：
- 所有页面暗黑模式无样式错乱
- 所有 UI 字符串通过 `next-intl` 引用
- 首页统计数据从 API 真实获取
- 发布表单文件上传上传流程正常
- 通知铃铛显示未读数，点击进入通知列表
- 私信对话列表与发送功能正常
- 密码重置邮件可发送，token 验证与密码更新正常
- 浏览历史页面按日期分组展示

### Phase 3: 搜索增强 + 内容下载（Task 119–127）

| Task | 标题 | 类别 |
|------|------|------|
| 119 | 全文搜索（PostgreSQL tsvector） | 后端功能 |
| 120 | 搜索建议 + 热搜词 | 全栈功能 |
| 121 | 内容下载功能 | 全栈功能 |
| 122 | 收藏集数据模型 + 后端 API | 后端功能 |
| 123 | 收藏集前端 UI | 前端功能 |
| 124 | 收藏集去重与筛选 | 后端功能 |
| 125 | 搜索结果优化 | 后端优化 |
| 126 | 搜索分面筛选 | 全栈功能 |
| 127 | 内容软删除 | 后端功能 |

**验收标准**：
- 搜索 API 支持中文全文搜索，返回高亮片段
- 搜索框输入时下拉显示建议词和热搜 Top 10
- 内容详情页下载按钮可用，下载计数 +1
- 收藏集 CRUD 和内容添加/移除正常
- 收藏集内内容去重，支持按 content_type 筛选
- 删除内容转为软删除，列表中不再显示

### Phase 4: 性能优化（Task 128–135）

| Task | 标题 | 类别 |
|------|------|------|
| 128 | 热门内容 N+1 查询优化 | 后端优化 |
| 129 | IP 列表 N+1 查询优化 | 后端优化 |
| 130 | 推荐引擎 N+1 查询优化 | 后端优化 |
| 131 | 分页查询统一 | 后端优化 |
| 132 | API 响应字段精简 | 后端优化 |
| 133 | 缓存穿透防护 | 后端优化 |
| 134 | 前端图片懒加载 | 前端优化 |
| 135 | 前端数据缓存 / SWR 策略 | 前端优化 |

**验收标准**：
- 所有列表端点 P95 延迟 ≤ 500ms（< 1000 并发）
- 无 N+1 查询（GORM `Preload` 或 `Joins`）
- 所有列表端点使用游标或 offset+limit 分页
- 前端列表页使用 SWR 缓存，like/bookmark 操作乐观更新
- 图片使用 lazy loading + 占位图

### Phase 5: 架构优化 + 验证（Task 136–145）

| Task | 标题 | 类别 |
|------|------|------|
| 136 | 数据库迁移 038–042 | 数据库 |
| 137 | 新增 API 路由注册 | 后端架构 |
| 138 | Service Container 模式 | 后端架构 |
| 139 | 端到端冒烟测试 | 测试 |
| 140 | 全量 lint/typecheck | 测试 |
| 141 | 结构化日志 | 后端架构 |
| 142 | 优雅关机 | 后端架构 |
| 143 | 健康检查端点 | 后端架构 |
| 144 | 构建优化 | 前端架构 |
| 145 | 文档同步与最终验证 | 文档 |

**验收标准**：
- 所有 5 个迁移文件无冲突执行成功
- Service Container 统一依赖注入
- `go build` / `go vet` / `npm run build` / `npm run lint` 全部零错误
- 结构化日志 JSON 格式输出，含 trace_id
- 优雅关机：关机超时内完成已有请求
- `/health` 端点返回数据库和 Redis 连接状态
- 前端构建产物体积合理，无未使用依赖
