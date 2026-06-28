# OmniCraft 当前任务交接摘要

任务标识：`019dfb5e-1360-7050-b2a4-f8900ada98e4`  
生成时间：2026-05-06  
项目目录：`C:\Users\16278\Desktop\file\code\project\OmniCraft`

## 1. 当前任务定位

本轮任务围绕 OmniCraft 的“原创区稳定化”和“原创/二创来源联动”展开。核心目标不是单独新增一个按钮，而是先把内容 API 契约、前端内容数据规范化、原创区导航、原创详情复用、发布链路和后端数据关系打通，确保后续“相关二创”功能建立在可靠的数据流上。

当前仓库处于大量未提交修改状态，根目录还有其它项目和资料文件改动。下次会话应优先进入 `project/OmniCraft` 工作，不要处理根目录下无关改动。

## 2. 关键决策

- 原创/二创关系采用“单原创来源”模型：每个二创最多绑定一个原创内容，不做多来源关联表。
- 数据库在 `content_items` 上新增 nullable 字段 `source_original_id`，引用同表 `content_items(id)`，删除源原创时使用 `ON DELETE SET NULL`。
- 只有 `zone='fanwork'` 的内容允许填写 `source_original_id`；被引用内容必须是 `zone='original'` 且 `status='published'`。
- 相关二创列表第一版走 `GET /api/v1/contents/:id/related-fanworks`，返回 `source_original_id,total,page,page_size,contents`。
- 前端内容数据统一经过 `frontend/lib/content.ts` 的 normalize 层，兼容后端 snake_case 和旧缓存中的 PascalCase 字段，过滤缺失有效 `id/title/zone` 的内容。
- 公共浏览页暂时使用 `cache: "no-store"` 避免 Next fetch cache 残留旧字段，尤其防止原创卡片跳转到 `/content/undefined`。
- 原创详情页应复用公共 `ContentDetail` 展示能力，并在原创语境下隐藏 PR 和版本历史等不适合原创区首版展示的模块。
- 相关二创按钮只在后端返回 `total > 0` 时显示；没有相关内容时不显示按钮，避免用户进入空页。
- 信誉分交互门槛使用配置项，产品默认值为 3，不再在 service 中硬编码为 0。

## 3. 已完成部分

### 后端

- 新增 `source_original_id` 迁移：`backend/migrations/036_content_source_original.sql`。
- `ContentItem` 模型已增加 `Description`、`Body`、`SourceOriginalID` 等内容字段方向的支持。
- `PublishContentInput` 已增加 `description` 和 `source_original_id`。
- 发布内容时加入 `validateSourceOriginalLink` 校验：
  - 原创不能引用其它原创；
  - 二创可以不绑定来源；
  - 二创只能绑定已发布原创；
  - 二创不能绑定二创或未发布原创。
- 内容列表查询支持 `source_original_id` 过滤。
- 新增相关二创 handler/service/repository 路径，用于 `/contents/:id/related-fanworks`。
- 后端公开接口方向调整为更稳定的 JSON 契约，减少直接泄露 GORM 大写字段的问题。
- `Category` 已从 `notification.go` 拆出到 `backend/internal/model/category.go`。
- `social_service.go` 的低信誉交互门槛改为从配置读取，新增 `minScoreForInteraction` 测试。
- `032_browse_history.sql` 到 `035_discussions.sql` 已朝“不要重复建表、只做补充 ALTER/索引”的方向修正。

### 前端

- 新增 `frontend/lib/content.ts`，集中处理内容 DTO 规范化。
- `/original` 页面已引入一级分类和内容类型筛选，分类从 `GET /categories?zone=original&level=primary` 获取，并保留本地 fallback。
- `/original` 页面内容列表改为 `cache: "no-store"`，并使用 `normalizeContentList`。
- `/original/[contentId]` 原创详情页改为复用公共内容详情能力，并增加相关二创入口逻辑。
- 新增 `/original/[contentId]/fanworks` 相关二创列表页，支持返回原创详情、排序筛选和类型筛选，复用 `MasonryGrid/ContentCard`。
- 发布页支持：
  - `zone=fanwork|original` 预填；
  - `source_original_id` 预填；
  - 二创选择来源原创；
  - 原创分类字段；
  - Markdown 正文通过 `description` 随发布提交。
- 多个公共内容入口页面已切换到 normalize 层，减少 API 字段变化造成的渲染风险。
- `Header` 移动端导航和主题水合问题已做修复方向的改动，包含移动端菜单入口和 mounted 后主题图标渲染。
- 根目录已有本轮浏览器截图：
  - `C:\Users\16278\Desktop\file\code\omnicraft-home-viewport.png`
  - `C:\Users\16278\Desktop\file\code\omnicraft-original-review.png`
  - `C:\Users\16278\Desktop\file\code\omnicraft-home-mobile.png`
  - `C:\Users\16278\Desktop\file\code\omnicraft-home-dark.png`
  - `C:\Users\16278\Desktop\file\code\omnicraft-home-dark-selected.png`
  - `C:\Users\16278\Desktop\file\code\omnicraft-original-undefined-detail.png`

### 测试与验证记录

根据 `docs/PLAN.md` 当前记录，已验证：

- `frontend npm run lint` 通过。
- `backend go test ./...` 通过。
- `backend go vet ./...` 通过。
- `backend go build ./...` 通过。
- 浏览器访问 `http://localhost:3000`，测试过首页筛选、原创区导航、原创卡片点击、主题切换、搜索输入、移动端视口。

已知验证限制：

- 当时后端 `8080` 未监听，页面存在 API 连接错误。
- Next fetch cache 曾残留旧数据，导致原创卡片链接变成 `/content/undefined` 并 404。
- 下次完成前必须重新启动完整前后端并按 AGENTS.md 做真实浏览器验证。

## 4. 待办事项

### 必须优先完成

- 重新跑完整验证：
  - 在 `backend` 下执行 `go test ./...`、`go vet ./...`、`go build ./...`。
  - 在 `frontend` 下执行 `npm run lint`。
  - 启动后端、数据库、Redis 和前端，确认 `8080` 与 `3000` 都可访问。
- 浏览器验证原创/二创联动：
  - 访问 `http://localhost:3000/original/[id]`。
  - 确认原创详情页正常加载。
  - 当存在相关二创时，确认“相关二创”按钮出现。
  - 点击进入 `/original/[id]/fanworks`。
  - 测试排序与类型筛选。
  - 移动端视口测试 Header 菜单、原创区和相关二创页入口。
  - 检查控制台是否仍有 hydration、key、API 契约错误。
- 确认后端 route 注册顺序正确，`/contents/:id/related-fanworks` 不被 `/contents/:id` 抢占。
- 确认 `ContentItem` 中 `Description/Body` 与数据库实际 schema 一致；如果旧迁移没有 `body` 字段，不要让前端/后端假设它已经存在。
- 确认 `source_original_id` 对原创内容的限制是否需要数据库 CHECK 约束。目前主要是应用层校验。
- 确认 `GET /api/v1/contents?content_type=` 是否支持逗号分隔；前端相关二创页目前会传 `text,article,prompt` 这类组合值。
- 清理或忽略 `tauri-client` 下大量未跟踪构建产物、`node_modules` 和 `target`，避免误提交。

### 功能完善

- 发布页“来源原创”选择器目前偏基础，应补搜索/分页，否则原创内容多时不可用。
- 原创分类输入目前是手填 slug，应改成分类 API 驱动的选择器。
- 相关二创页空态可用，但需要确认真实后端空结果和错误状态表现。
- 内容详情中的“发布相关二创”入口应能携带 `zone=fanwork&source_original_id=<id>` 跳转到发布页。
- 统一 `description` 与 `body/markdown` 的产品语义，避免 Markdown 正文长期塞在 `description` 里。
- 文档中提到 `design/ui-spec.md`、`architecture.md`、`CLAUDE.md` 需要同步规则；当前是否已实际修改这些文件需再次确认。

## 5. 重要文件修改记录

### 后端核心

- `backend/internal/model/content.go`：内容模型增加来源原创、正文/描述相关字段。
- `backend/internal/model/category.go`：新增独立分类模型。
- `backend/internal/model/notification.go`：移出 `Category`，减少模型职责混杂。
- `backend/internal/service/content_service.go`：发布输入、来源原创校验、发布审查 payload、相关二创服务逻辑。
- `backend/internal/repository/content_repo.go`：内容查询增加 `source_original_id` 过滤和相关二创查询。
- `backend/internal/handler/content.go`：内容列表 query 解析、相关二创响应、发布请求绑定。
- `backend/internal/handler/routes.go`：内容路由注册调整。
- `backend/internal/service/social_service.go`：信誉门槛配置化。
- `backend/internal/handler/social.go`、`backend/internal/repository/social_repo.go`：配合信誉和社交接口调整。
- `backend/config.yaml`、`backend/config/config.go`：新增或调整 reputation 配置。
- `backend/migrations/032_browse_history.sql` 到 `035_discussions.sql`：修正重复建表风险。
- `backend/migrations/036_content_source_original.sql`：新增 `source_original_id` 字段和索引。
- `backend/internal/service/content_source_test.go`：来源原创校验单元测试。
- `backend/internal/service/reputation_gate_test.go`：信誉分门槛单元测试。

### 前端核心

- `frontend/lib/content.ts`：内容 DTO normalize 工具。
- `frontend/components/content/ContentCard.tsx`：卡片对有效内容字段和链接处理增强。
- `frontend/components/content/ContentDetail.tsx`：原创详情复用和上下文隐藏能力。
- `frontend/components/content/MasonryGrid.tsx`：空态和过滤后的内容渲染支持。
- `frontend/app/(public)/original/page.tsx`：原创区两级导航/筛选、no-store、normalize。
- `frontend/app/(public)/original/[contentId]/page.tsx`：原创详情复用公共详情、相关二创入口。
- `frontend/app/(public)/original/[contentId]/fanworks/page.tsx`：新增相关二创列表页。
- `frontend/app/(protected)/publish/page.tsx`：发布页支持原创/二创区、来源原创、原创分类、Markdown 正文提交。
- `frontend/components/layout/Header.tsx`：移动端菜单和主题水合修复。
- `frontend/components/home/HomePageClient.tsx`、`frontend/app/(public)/page.tsx`：首页入口和筛选行为调整。
- `frontend/app/(public)/content/[contentId]/page.tsx`、`frontend/app/(public)/ip/[ipId]/page.tsx`、`frontend/app/(public)/ip/[ipId]/[category]/page.tsx`、`frontend/app/(public)/user/[userId]/UserProfileClient.tsx`：切换到统一内容 normalize 或相关展示契约。

### 文档与任务

- `docs/PLAN.md`：当前审查结论、架构决策、测试计划和待办。
- `progress.txt`：追加本轮进度记录。
- `task.json`：任务状态或任务定义有小幅同步。

### 需谨慎处理的未跟踪内容

- `tauri-client/` 下有大量未跟踪文件，包括 `node_modules`、`dist`、Rust `target` 等构建产物。下次提交前应先检查 `.gitignore` 和实际需要纳入版本控制的源文件，避免误提交依赖和编译产物。

## 6. 整体架构思路

OmniCraft 采用 Go 后端、Next.js 前端、PostgreSQL/Redis 存储、Tauri 客户端预留的分层架构。

- 后端按 handler、service、repository、model 分层：
  - handler 负责 HTTP 参数解析和响应形态；
  - service 负责产品规则，比如来源原创校验、信誉分门槛、发布审查；
  - repository 负责 GORM 查询组合；
  - model 只表达数据库实体和 JSON 契约。
- 内容系统以 `content_items` 为核心表，通过 `zone` 区分 `fanwork` 和 `original`。
- 原创/二创联动不引入新关联表，使用 `content_items.source_original_id` 建立单向来源关系，降低首版复杂度。
- 分类系统通过 `categories` 提供原创区一级分类，前端保留 fallback，保证 API 不可用时页面仍能降级渲染。
- 前端 App Router 页面尽量使用服务端 fetch，但公共内容页在当前阶段使用 `cache: "no-store"`，等 API 契约稳定后再引入细粒度缓存策略。
- 前端内容渲染统一经 `frontend/lib/content.ts` normalize，避免后端 GORM 字段、snake_case DTO、旧 Next cache 混用导致 UI 断裂。
- UI 复用路径是 `ContentDetail`、`ContentCard`、`MasonryGrid`，原创区只做上下文差异控制，不再维护一套手写详情页。

## 7. 下次会话建议启动步骤

1. 进入项目目录：

   ```powershell
   Set-Location 'C:\Users\16278\Desktop\file\code\project\OmniCraft'
   ```

2. 查看当前差异：

   ```powershell
   git status --short
   git diff --stat
   ```

3. 先检查关键文件：

   ```powershell
   Get-Content -LiteralPath 'docs\PLAN.md' -Encoding UTF8
   Get-Content -LiteralPath 'frontend\lib\content.ts' -Encoding UTF8
   Get-Content -LiteralPath 'backend\internal\service\content_service.go' -Encoding UTF8
   ```

4. 优先修复/确认待办事项，再跑测试和浏览器验证。

5. 最后按 AGENTS.md 要求在最终回复中写明测试 URL、浏览器交互、截图和验证结果。
