# OmniCraft 设计审查报告(合并版)

> **审查日期**:2026-06-24
> **合并来源**:
> - 文档 A:`docs/2026-06-24-omnicraft-documentation-review.md`(文档一致性与配置审查,30+ 问题)
> - 文档 B:`docs/design-review-2026-06-24.md`(架构设计与 UX 审查,17 个问题)
> **审查范围**:`architecture.md`、`AGENTS.md`、`design/ui-spec.md`、`design/design-system.md`、`design/UI Design.md`、`design/homepage-v0.html`、`task.json`、`backend/config.yaml`、Beta 路线图、前后端实际实现
> **整合原则**:保留两份文档所有问题,合并重叠项,标注来源,不遗漏

---

## 概述

本合并报告共整合 **47 个问题**,分为 10 大类。每个问题标注来源:

- `[A]` = 仅文档 A 发现
- `[B]` = 仅文档 B 发现
- `[A+B]` = 两份文档均发现(可能角度不同)

| 类别 | 问题数 | 严重程度 | 来源分布 |
|------|--------|---------|---------|
| A. 文档一致性与维护 | 6 | 🔴 高 | 全部 [A] |
| B. 架构设计 | 3 | 🔴 高 | 全部 [B] |
| C. 配置与实现不一致 | 6 | 🔴 高 | 全部 [A] |
| D. 路由系统与页面迁移 | 4 | 🔴 高 | 全部 [A] |
| E. 功能缺失 | 7 | 🟠 中高 | [A]×4 + [B]×3 |
| F. UI/UX 交互 | 5 | 🟡 中 | [A]×1 + [B]×4 |
| G. 多端/跨平台 | 6 | 🟢 中 | [A]×3 + [B]×2 + [A+B]×1 |
| H. 安全与合规 | 4 | 🔵 中高 | [A]×2 + [B]×2 |
| I. 性能与可扩展性 | 4 | 🟡 中 | [A]×3 + [B]×1 |
| J. 数据库与迁移 | 2 | 🟡 中 | 全部 [A] |

---

## 类别 A:文档一致性与维护问题

### A1. 设计系统三重冲突 `[A]` → 决策:DEC-011(统一 design-system.md,归档 homepage-v0.html)

**发现位置**:`architecture.md §10.4`、`design/design-system.md`、`design/homepage-v0.html`

**问题描述**:

项目中存在**三套互不一致的设计系统**,极易误导开发:

| 文档 | 主色调 | 背景 | Header 高度 | 字体 |
|------|--------|------|------------|------|
| `architecture.md §10.4` | GitHub blue `#0969da` | `#ffffff` | h-16 (64px) | 系统字体 |
| `design/design-system.md` | Indigo `#4F46E5` | `#FFFFFF` | 52px | 系统字体 |
| `design/homepage-v0.html` | Muted blue-gray `#526B8C` | 暖白 `#FAFAF8` | 56px | **Sora** 字体 |

**实际实现**(`frontend/app/globals.css` + `frontend/components/layout/Header.tsx`):使用 Indigo `#4f46e5`、52px header — 匹配 `design-system.md`,但 `architecture.md §10.4` 和 `homepage-v0.html` 均过时。

`homepage-v0.html` 是一个未集成的全新设计概念(暖色调 + Sora 字体 + 无边框卡片),与现有实现完全不同,却没有任何文档说明其状态(已废弃?还是待实施?)。

**建议方案**:
- 以 `design-system.md` + 实际实现为唯一权威
- 归档或删除 `architecture.md §10.4` 的过时色值表
- 明确 `homepage-v0.html` 状态:废弃则移至 `docs/archive/`,待实施则补充实施计划

---

### A2. 侧边栏宽度数值矛盾 `[A]` → 决策:DEC-011(统一为 228px/48px)

**发现位置**:`architecture.md §13.2`、`design/design-system.md`

**问题描述**:

- `architecture.md §13.2`:展开 224px (w-56) / 收起 64px (w-16)
- `design/design-system.md`:展开 228px / 折叠 48px

两份文档数值不一致,实际实现需确认以哪个为准。

**建议方案**:统一为 `design-system.md` 的数值(228px/48px),更新 `architecture.md`。

---

### A3. 原创区导航结构文档矛盾 `[A]` → 决策:DEC-012(由 DEC-013 归档解决)

**发现位置**:`architecture.md §10.6`、`design/UI Design.md P06`、`task.json Task 64`

**问题描述**:

- `architecture.md §10.6`:明确"**无二级内容类型筛选**",采用单层分类 Tab
- `design/UI Design.md P06`:描述有"**二级内容类型子筛选**"(全部/图片/视频/音频/文字/效率模板/模型与设计/其他)
- `task.json Task 64`:标注"已被 Task 90 覆盖"
- **实际实现**(`frontend/app/(public)/original/page.tsx`):单层分类 Tab,符合 `architecture.md`

`UI Design.md` 严重过时,仍描述旧的两级导航,会误导开发者按错误规格实现。

**建议方案**:更新 `UI Design.md`,移除二级筛选描述,与 Task 90 设计对齐。

---

### A4. UI Design.md 严重过时 `[A]` → 决策:DEC-013(归档,同步引用文档)

**发现位置**:`design/UI Design.md`

**问题描述**:

- 仍描述 `/publish` 和 `/dashboard/*` 路由(未提及 `/studio/*`)
- 仍描述原创区"二级内容类型子筛选"(已被 Task 90 移除)
- 新开发者按此文档实现会引入错误

**建议方案**:全文更新,同步 `/studio/*` 路由和 Task 90 设计变更,或直接归档并以 `ui-spec.md` 为唯一权威。

---

### A5. 审查报告未整合 `[A]` → 决策:DEC-021(归档,先确认问题已解决)

**发现位置**:`docs/review/`

**问题描述**:

`docs/review/` 下有 20+ 份审查报告,但:

- 无统一的问题追踪表
- 无法确认哪些问题已修复
- 审查发现可能重复或遗漏

**建议方案**:建立统一问题追踪表(可复用 GitHub Issues),标记每个审查发现的修复状态。

---

### A6. task.json 与 Beta 路线图并行 `[A]` → 决策:DEC-022(保留作为历史记录)

**发现位置**:`task.json`、`docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md`

**问题描述**:

- `task.json` 有 145+ 历史任务
- Beta 路线图有 28 个任务(F-01 到 R-02)
- 两套任务跟踪系统,容易混淆优先级

**建议方案**:`AGENTS.md` 已规定模式选择优先级,但建议在 `task.json` 顶部增加醒目标注"历史账本,新工作请参考 Beta 路线图"。

---

## 类别 B:架构设计问题

### B1. 推荐引擎缓存策略存在 Redis 内存膨胀风险 `[B]` → ✅ 已修复关闭(核查确认:有 2h TTL)

**发现位置**:`architecture.md §12.2`

**问题描述**:

当前设计以 `rec:original:{user_id}:{page}` 为 key 缓存每个用户每页的推荐结果,TTL 为 2 小时。

| 用户规模 | 每用户浏览页数 | Redis Key 数量 | 估算内存占用 |
|---------|-------------|---------------|-------------|
| 1,000 | 10 | 10,000 | ~50 MB |
| 10,000 | 10 | 100,000 | ~500 MB |
| 100,000 | 10 | 1,000,000 | ~5 GB |

更根本的问题是:推荐结果本质上是用户相关的,不应该以全量缓存最终结果的方式实现。正确的做法应该是缓存中间结果(用户画像向量 + 热门排行),而不是缓存最终推荐列表。

**建议方案**:
- 只缓存用户画像向量(`user_profile:{user_id}`,TTL 1h)和热门排行(`rank:hot:contents`)
- 推荐结果按需实时计算,利用 pgvector 的 IVFFlat 索引(< 10ms 延迟)
- 如果确实需要缓存最终结果,只缓存第一页(大部分用户只看第一页),后续页实时计算

---

### B2. Go 后端"单体→模块化"演进路径缺乏领域边界定义 `[B]` → 决策:DEC-014(DDD 完整重构)

**发现位置**:`architecture.md §1.2`、`§3.2`

**问题描述**:

架构图描述为"单体 → 模块化",代码结构按技术层次组织而非按业务领域:

```
internal/
├── handler/     # 所有业务的 HTTP 处理器在一个目录
├── service/     # 所有业务的逻辑在一个目录
├── repository/  # 所有数据访问在一个目录
└── model/       # 所有模型在一个目录
```

这导致以下问题:
- 当团队扩大时,多人同时修改不同业务会频繁冲突
- 没有明确的领域边界,无法独立部署或拆分
- 跨领域的业务逻辑(如"发布内容后更新信誉分+发通知+写向量")散落在多个 service 的隐式调用中
- 新增一个业务模块需要同时修改 4 个目录

**建议方案**:

不拆分微服务,仅优化代码组织——按业务领域重新组织包结构:

```
internal/
├── domain/
│   ├── auth/       # handler + service + repository + model
│   ├── content/    # handler + service + repository + model
│   ├── social/     # handler + service + repository + model
│   ├── judge/      # handler + service + repository + model
│   └── ...
├── middleware/      # 跨领域:auth、ratelimit、cors、logger
├── pkg/             # 跨领域工具:jwt、aliyun、diffengine、llm
└── container/       # 依赖注入容器
```

跨领域协调通过明确的事件接口(如 `ContentPublished` 事件),而非 service 间的直接调用。

---

### B3. 内容全文搜索对中文的处理方案不够可靠 `[B]` → 决策:DEC-001

**发现位置**:`architecture.md §15.4` + Beta 设计 §5.1-F

**关联决策**:[DEC-001](./2026-06-24-design-review-decisions.md#dec-001中文全文搜索查询侧配置对齐)

**问题描述**:

当前使用 PostgreSQL `tsvector` + `tsquery` 实现中文全文搜索:

```sql
CREATE INDEX idx_content_items_search ON content_items USING GIN(search_vector);
-- search_vector 通过 to_tsvector('simple', title || ' ' || description) 生成
```

问题在于:
- `to_tsvector('simple', ...)` 按空格分词,中文无空格分隔,整段文本被当作一个 token
- 搜索"美食"时,`tsquery` 无法匹配到包含"美食推荐"的内容(除非使用了前缀匹配)
- Beta 设计已指出需要 `pg_trgm` 或明确的分词方案,但未给出具体选型

**建议方案**(按推荐优先级):

1. **引入 jieba 分词 + PostgreSQL 插件**:`pg_jieba` 提供中文分词,`to_tsvector('jiebacfg', text)` 生成正确的 tsvector
2. **Go 层分词**:在 Go 后端用 `gojieba` 分词后,将分词结果拼接存入 `search_vector` 列(不依赖 PG 插件)
3. **pg_trgm 降级**:`CREATE INDEX ON content_items USING GIN(title gin_trgm_ops)`,配合 `LIKE '%keyword%'` 或 `SIMILARITY()` 函数

对于 MVP/Beta,建议方案 3 作为快速修复,方案 2 作为正式方案。

> **⚠️ 事实核查修正(2026-06-24)**:经代码核查,上述问题描述部分不准确。索引侧已使用 `jiebacfg`(迁移 041/042),非 `simple`。实际问题在查询侧:`splitAndNormalize` 只按空格分词,查询用 `to_tsquery('simple', ?)` 与索引的 `jiebacfg` 不匹配。详见 [DEC-001](./2026-06-24-design-review-decisions.md#dec-001中文全文搜索查询侧配置对齐)。

---

## 类别 C:配置与实现不一致

### C1. 信誉分配置完全缺失(违反硬编码禁令) `[A]` → 决策:DEC-004

**发现位置**:`architecture.md §7`、`backend/config.yaml`、`backend/internal/service/reputation_service.go`

**关联决策**:[DEC-004](./2026-06-24-design-review-decisions.md#dec-004信誉分配置补全阈值统一为-10)(同时修复 C2、C3,阈值统一为 10)

**问题描述**:

`architecture.md §7` 定义了完整的 reputation 配置:

```yaml
reputation:
  malicious_contribution: -3
  malicious_comment: -2
  quality_content_bonus: 3
  quality_comment_bonus: 2
  contribution_accepted: 3
  ...
```

但 `backend/config.yaml` 中 reputation 部分只有:

```yaml
reputation:
  min_score_for_interaction: 3
  quality_content_threshold: 10
  quality_comment_threshold: 5
  repeat_violation_window_days: 7
  ...
```

**所有加减分值字段都缺失!**

代码中硬编码加分值:

```go
func (s *ReputationService) AwardQualityContent(...) error {
    ...
    return s.AddReputation(userID, 3, "quality_content", &contentID)  // 硬编码 3
}
func (s *ReputationService) AwardPRMerged(...) error {
    return s.AddReputation(userID, 3, "pr_merged", &prID)  // 硬编码 3
}
```

**违反 AGENTS.md 规则**:"Read config, not hardcode"、"所有限制从 config.yaml 读取"。

**建议方案**:将 `architecture.md §7` 中所有 reputation 字段补入 `config.yaml`,重构 `reputation_service.go` 从配置读取。

---

### C2. 信誉分阈值差异巨大 `[A]` → 决策:DEC-004(统一为 10)

**发现位置**:`architecture.md §7`、`backend/config.yaml`

**问题描述**:

| 字段 | architecture.md | config.yaml |
|------|-----------------|-------------|
| quality_content_threshold | 50 | 10 |
| quality_comment_threshold | 20 | 5 |

差异 5 倍,将严重影响信誉分计算。

**建议方案**:确认正确阈值,统一文档与配置。若 config.yaml 为准,更新 architecture.md;反之更新 config.yaml。

---

### C3. 配置字段命名不统一 `[A]` → 决策:DEC-004(以 config.yaml 为准)

**发现位置**:`architecture.md §7`、`backend/config.yaml`

**问题描述**:

| architecture.md | config.yaml |
|-----------------|-------------|
| `video_max_size_mb` | `video_max_mb` |
| `mod_archive_max_size_mb` | `mod_max_mb` |
| `min_score_for_publish` | `min_score_for_interaction` |

文档与实现字段名不一致,容易导致配置读取失败或开发者困惑。

**建议方案**:统一命名规范(建议保留 config.yaml 现有命名,更新 architecture.md)。

---

### C4. features 字段缺失 `[A]` → 决策:DEC-004(涵盖)

**发现位置**:`architecture.md §7`、`backend/config.yaml`

**问题描述**:

`architecture.md §7` 定义 `features.agent_enabled`、`features.judge_enabled`、`features.ad_enabled`,但 `config.yaml` 完全没有这些字段。无法通过 feature flag 控制这些功能。

**建议方案**:补全 features 字段,默认值与 architecture.md 一致。

---

### C5. Agent LLM 配置混乱 `[A]` → 决策:DEC-020(去除过时注释,填入真实 API Key)

**发现位置**:`backend/config.yaml`

**问题描述**:

```yaml
agent:
  llm_api_key: ""    # 留空,注释说"改用 env var 注入(需修复 config.go)"
```

注释明确说"需修复",但状态不明,可能导致 Agent 功能失效。

**建议方案**:完成 env var 注入实现,移除"需修复"注释,或在 Beta 中明确禁用 Agent 功能。

---

### C6. 内容下载权限逻辑不清 `[A]` → 决策:DEC-024(复用 min_score_for_interaction)

**发现位置**:`AGENTS.md`、`backend/config.yaml`

**问题描述**:

- AGENTS.md 规定"信誉分 < 3 用户禁止下载"
- 但 `config.yaml` 中 `min_score_for_interaction: 3` 是否涵盖下载权限不明确
- 下载权限的阈值字段命名不清晰

**建议方案**:增加明确的 `min_score_for_download` 字段,或文档说明 `min_score_for_interaction` 涵盖下载。

---

## 类别 D:路由系统与页面迁移问题

### D1. Studio 迁移严重不完整(空存根页面) `[A]` → 决策:DEC-005

**发现位置**:`architecture.md §3.2`、前端 Studio 页面

**关联决策**:[DEC-005](./2026-06-24-design-review-decisions.md#dec-005studio-存根页面完整迁移方案-b)(方案 B 完整迁移,同时修复 D2、D3)

**问题描述**:

`architecture.md §3.2` 声称系统已迁移至 `/studio/*`,旧 `/dashboard/*` 保留重定向。但以下 Studio 页面**只有 EmptyState 占位,无任何 API 调用**:

- `frontend/app/(protected)/studio/contributors/page.tsx` — 空存根
- `frontend/app/(protected)/studio/pr-requests/page.tsx` — 空存根
- `frontend/app/(protected)/studio/tag-suggestions/page.tsx` — 空存根

而真正功能完整的页面在旧路由:

- `frontend/app/(protected)/dashboard/contributors/page.tsx` — 调用 `/api/v1/dashboard/contributors` API

**影响**:用户从 StudioSidebar 点击进入协作管理,看到的是空页面,无法管理 PR、贡献者、标签建议。这是核心功能缺失。

**建议方案**:要么实现 Studio 存根页面功能(复用 dashboard 逻辑),要么暂时隐藏 Studio 协作管理入口并重定向到 dashboard。

---

### D2. 后端 API 路由未迁移 `[A]` → 决策:DEC-005(策略 2 解决)

**发现位置**:`backend/internal/handler/routes.go`、`architecture.md §3.2`

**问题描述**:

`routes.go` 中:

- ✅ 存在:`/dashboard/contributors/:userId/block`
- ❌ 不存在:`/studio/contributors/:userId/block`

但 `architecture.md §3.2` 明确写了:

```
POST /api/v1/studio/contributors/:userId/block    # 作者拉黑贡献者
```

**建议方案**:后端增加 `/studio/*` 路由别名(或迁移),与 architecture.md 契约对齐。

---

### D3. 新旧路由并存无重定向 `[A]` → 决策:DEC-005(策略 4 解决)

**发现位置**:前端路由结构

**问题描述**:

前端同时存在:

- `/publish` 和 `/studio/publish/original` + `/studio/publish/fanwork`
- `/dashboard/contents` 和 `/studio/contents`
- `/dashboard/contributors` 和 `/studio/contributors`
- `/dashboard/pr-requests` 和 `/studio/pr-requests`
- `/dashboard/tag-suggestions` 和 `/studio/tag-suggestions`

无 301 重定向实现,SEO 重复内容,用户困惑,维护负担倍增。

**建议方案**:实现 301 重定向(旧→新),或删除冗余页面。

---

### D4. 首页路由冗余 `[A]` → 决策:DEC-023(合并,删除 /home 和 /ips)

**发现位置**:前端路由结构

**问题描述**:

- `frontend/app/(public)/page.tsx` — 首页
- `frontend/app/(public)/home/page.tsx` — 另一个首页?
- `frontend/app/(public)/ips/page.tsx` — IP 列表页(architecture.md 未定义此独立路由)

`architecture.md §3.1` 说首页是 `/`,但 `/home` 路由的存在造成混淆。

**建议方案**:确认权威首页路由,删除冗余或实现重定向;明确 `/ips` 路由的文档定义。

---

## 类别 E:功能缺失

### E1. 缺少内容草稿系统 `[B]` → 决策:DEC-008(完整实现,同时修复 E2)

**发现位置**:整体文档审查

**问题描述**:

`POST /api/v1/contents` 直接发布内容,`status` 默认值为 `pending`(进入审核),没有草稿(draft)状态。

对于创作者平台,草稿是核心体验:
- 用户写了一篇长文,浏览器崩溃 → 全部丢失
- 上传了一个大视频后想先预览再发布 → 不支持
- 用户想保存进度稍后继续编辑 → 不支持

**建议方案**:
- `content_items.status` 增加 `draft` 枚举值
- 草稿仅创建者可见,不触发审核、不公开、不计入统计
- 从草稿发布:`draft → pending`(进入审核链路)
- 前端功能:发布表单增加「保存草稿」按钮 + 自动保存(localStorage 每 30 秒备份 + 服务端每 2 分钟同步)+ Studio 内容管理页增加「草稿」Tab
- API 增加:`GET /api/v1/studio/contents?status=draft`(仅作者)

---

### E2. 缺少内容定时发布功能 `[B]` → 决策:DEC-008(策略 5 一并实现)

**发现位置**:整体文档审查

**问题描述**:

创作者经常希望在特定时间发布内容(节假日、活动日、黄金时段)。当前完全不支持定时发布。这与 E1(草稿系统)相关但独立——草稿解决"未完成时保存",定时发布解决"已完成但延迟公开"。

**建议方案**:
- `content_items` 增加 `scheduled_at TIMESTAMPTZ` 字段(nullable)
- 定时任务每分钟扫描 `status='draft' AND scheduled_at <= NOW()` 的内容,自动将 `draft → pending`
- 发布表单增加「定时发布」选项(日期时间选择器)
- 如果 `scheduled_at` 有值,按钮文案从「发布」变为「定时发布」
- Studio 内容管理页显示定时发布状态和倒计时

---

### E3. 缺少用户数据导出功能(个人信息保护法合规) `[B]` → 决策:DEC-009(完整实现方案 A)

**发现位置**:整体审查

**问题描述**:

《中华人民共和国个人信息保护法》第 45 条规定,个人有权向个人信息处理者查阅、复制其个人信息,并可请求将个人信息转移至其指定的个人信息处理者。

当前系统:
- 账号删除(Task 103)是软删除,但不支持先导出再删除
- 完全没有数据导出 API
- 用户无法获取自己在平台上的数据副本

**建议方案**:
- `POST /api/v1/users/me/export` → 返回 `202 Accepted`
- 后台异步任务打包用户数据:个人信息 + 发布内容(含附件下载链接)+ 评论、收藏、关注列表 + 信誉分变动日志
- 导出格式:JSON + 附件 ZIP 打包
- 生成 OSS 临时下载链接(24h 有效),完成后通过通知系统告知用户
- API 增加限流:每用户每天最多 1 次导出请求
- Admin 审计日志记录每次导出操作

---

### E4. 创作者数据分析能力不足 `[B]` → 决策:DEC-015(基础增强)

**发现位置**:`architecture.md §13.4`

**问题描述**:

Studio 概览(`/studio/overview`)只有 4 个统计卡片 + 访问量折线图。但创作者实际需要:
- **内容类型分析**:图文 vs 视频 vs 文字,哪种类型表现最好?
- **单篇内容漏斗**:曝光 → 点击 → 阅读完成 → 点赞 → 收藏 → 关注
- **发布时间分析**:什么时间段发布的内容获得最多互动?
- **受众分析**:我的粉丝还关注了哪些其他创作者/内容?
- **内容 retention**:用户平均阅读/观看时长(特别是视频内容)

**建议方案**:
- 新增 `GET /api/v1/studio/analytics/overview?days=30` 返回增强统计数据
- 新增 `GET /api/v1/studio/analytics/contents?sort=views&days=30` 返回单篇内容详细指标
- Studio 概览增强:热门内容 Top 10 表格 + 按 content_type 分组的饼图 + 日期范围选择器(7天/30天/90天/自定义)
- Studio 内容管理列表增加每篇内容的快捷统计预览

---

### E5. OriginalSidebar 导航链接错误 `[A]` → 决策:DEC-010(实现收藏集页面)

**发现位置**:`frontend/components/original/OriginalSidebar.tsx`

**问题描述**:

```tsx
{ icon: Heart, label: t("nav.favorites"), href: "/studio/contents" },
{ icon: FileText, label: t("nav.myOriginal"), href: "/studio/contents" },
```

"收藏"和"我的原创"两个不同功能入口指向**同一 URL** `/studio/contents`。

**建议方案**:收藏应指向收藏集页面(`/studio/collections` 或 `/collections`),我的原创应指向 `/studio/contents?zone=original`。

---

### E6. 桌面端一键部署完全禁用 `[A]` → 决策:DEC-032(现在实施 D-02 到 D-05)

**发现位置**:`backend/config.yaml`、Beta 路线图

**问题描述**:

- `features.desktop_deploy_enabled: false`
- D-02 至 D-05 任务未完成
- **影响**:PRD 核心卖点"Agent 一键部署"在 Web Beta 完全不可用,内容详情页的"一键部署"按钮被隐藏。

**建议方案**:完成 D-02 至 D-05 任务,或在 Beta 阶段明确标注"桌面部署为 P1 功能"并提供替代引导。

---

### E7. 客户端下载禁用 `[A]` → 决策:DEC-032(现在实施)

**发现位置**:`backend/config.yaml`

**问题描述**:

- `client.download_enabled: false`
- **影响**:用户无法下载 PC 客户端,`/client` 页面显示不可用状态。

**建议方案**:提供客户端下载渠道(GitHub Releases 或官网),启用 download_enabled。

---

## 类别 F:UI/UX 交互问题

### F1. 瀑布流无限滚动没有 URL 状态同步和滚动位置恢复 `[B]` → 决策:DEC-006

**发现位置**:`architecture.md §10.5–10.6`

**关联决策**:[DEC-006](./2026-06-24-design-review-decisions.md#dec-006瀑布流完整分页体验方案-b)(经核查实际无分页,方案 B 完整实现)

**问题描述**:

原创区使用瀑布流 + 无限滚动,但设计中缺失两个关键体验:

1. **滚动位置丢失**:用户滚到第 5 页 → 点击内容 → 返回 → 回到顶部(需要重新滚 5 页)
2. **URL 不可分享**:用户看到第 3 页的某个内容,分享 URL 给别人 → 对方看到的是第一页

这两个问题在小红书、Pinterest 的 Web 端都已解决。

**建议方案**:
- URL 状态同步:`history.replaceState` 将 `?page=N` 写入 URL(不触发页面重载)
- 从 URL 恢复:`page` 参数存在时,直接加载对应页(跳过前面的内容)
- 滚动位置恢复:使用 `sessionStorage` 记录 `{route, scrollY, page}`,配合 Next.js 的 `scrollRestoration`
- 内容卡片点击使用 `router.push`(保留 history 栈),浏览器返回时自动恢复位置
- 提供「回到顶部」浮动按钮(滚动超过 3 页时显示)

> **⚠️ 事实核查修正(2026-06-24)**:经代码核查,上述问题描述部分不准确。实际不是"无限滚动无状态恢复",而是**根本没有无限滚动** — `original/page.tsx` 只请求 `page_size: "24"`,无 page 参数,无"加载更多"按钮,无无限滚动。用户看不到第 25 条以后的内容。这是功能缺失,比"状态恢复"更严重。详见 [DEC-006](./2026-06-24-design-review-decisions.md#dec-006瀑布流完整分页体验方案-b)。

---

### F2. 侧边栏折叠后功能不可发现 `[B]` → ✅ 已修复关闭(核查确认:折叠时显示图标+tooltip)

**发现位置**:`architecture.md §13.2`

**问题描述**:

StudioSidebar 收起时(`w-16`)仅显示图标。Hover tooltip 延迟 300ms 才出现。用户不会主动 hover 每一个图标来寻找功能。新用户完全不知道每个图标代表什么。

**建议方案**(推荐组合):
- **默认不折叠**,折叠按钮放在侧边栏底部;用户主动选择折叠时,提示"折叠后将仅显示图标"
- **首次进入 Studio 时**侧边栏展开并高亮动画,引导用户认知各入口位置
- **收起时**,hover 任一图标 → 所有图标同时显示 mini 文字标签(右侧滑出)

---

### F3. ContentCard 在两套区域各有一套设计,缺乏抽象 `[B]` → ✅ 已修复关闭(核查确认:仅 1 个实现)

**发现位置**:`architecture.md §10.5`

**问题描述**:

二创区卡片和原创区卡片有完全独立的视觉规范:

| 属性 | 二创区卡片 | 原创区卡片 |
|------|----------|----------|
| 封面比例 | 3:4 固定 | 自适应原始宽高比 |
| 标签 Badge | 显示(最多 3 个) | 不显示 |
| 评论数 | 显示 | 不显示 |
| IP 名称 | 显示 | 不显示 |
| Hover 效果 | 无特殊效果 | scale-105 + 浅色遮罩 |

两套设计各自独立描述为两段文字,没有抽象出共享的基础组件。修改卡片圆角/间距/字体需要同时改两处,新增内容类型两个卡片都要适配。

**建议方案**:

抽象共享组件层次:

```
ContentCardBase          ← 共享框架:卡片容器 + 封面区 + 信息区 + 交互区
├── FanworkContentCard   ← 二创区变体:3:4 封面 + IP 名 + 评论数 + 标签
└── OriginalContentCard  ← 原创区变体:自适应封面 + 仅点赞数 + hover 效果
```

变体差异通过 props 或配置对象驱动:

```ts
interface ContentCardVariant {
  coverRatio: 'fixed' | 'adaptive';
  showTags: boolean;
  showCommentCount: boolean;
  showIPName: boolean;
  hoverEffect: 'none' | 'scale';
}
```

---

### F4. 原创区 12 个分类 Tab 的横向滚动交互体验不佳 `[B]` → ✅ 已修复关闭(核查确认:sticky 定位,滚动时可见)

**发现位置**:`architecture.md §10.6`

**问题描述**:

原创区顶部 `推荐` + 11 个一级分类 = 12 个 Tab 全部横向排列。移动端只能看到 2-3 个,平板端约 5-6 个,其余需要横向滚动才能发现。没有箭头指示器或渐变遮罩提示"可以滚动"。

**建议方案**:
- **PC 端**(>1100px):显示所有 Tab(空间足够)
- **平板端**(700-1100px):显示前 7 个 Tab + 右侧渐变遮罩 + `>` 箭头按钮
- **移动端**(≤700px):固定显示「推荐」+ 前 4 个热门分类 +「更多 ∨」下拉
- 分类按用户偏好动态排序(用户常看的分类自动前移)

---

### F5. 移动端适配缺陷 `[A]` → 决策:DEC-025(参考小红书 APP 实现)

**发现位置**:整体审查

**问题描述**:

- OriginalSidebar 在移动端的折叠/抽屉行为未明确实现
- Header 搜索框在移动端的展开/折叠交互
- 表单类页面(发布、设置)在移动端的体验

**建议方案**:补充移动端适配规格,明确 OriginalSidebar 在移动端转为抽屉、Header 搜索框折叠为图标按钮。

---

## 类别 G:多端/跨平台问题

### G1. Tauri 客户端完全无离线能力 `[B]` → 决策:DEC-016(暂缓,降级 P3)

**发现位置**:`architecture.md §3.3`

**问题描述**:

Tauri 客户端所有操作都需要先调用后端 API(获取脚本 → 下载文件 → 验证状态),全程依赖网络。没有网络时客户端完全不可用——连已下载到本地的内容列表都看不到。

**建议方案**:
- 引入本地 SQLite 数据库(`tauri-plugin-sql`)存储:
  - 已下载内容的元数据缓存(标题、类型、文件路径、下载时间)
  - 客户端配置和偏好
- 客户端主界面增加「已下载」Tab:
  - 离线时默认展示已下载内容列表
  - 允许浏览和打开本地文件(仅白名单目录内)
- 在线时后台同步:检查已下载内容是否有更新版本
- 离线时网络请求失败 → 展示友好提示 + 切换到离线模式

---

### G2. 移动端缺乏 PWA 支持 `[B]` → 决策:DEC-017(暂缓,降级 P3)

**发现位置**:整体审查

**问题描述**:

架构文档提到响应式断点(≤700px 移动端、≤1100px 平板),但没有 PWA manifest、Service Worker、离线缓存策略。对于以内容消费为主的平台,移动端通常是 60%+ 的流量来源。仅有响应式布局不足以提供良好体验。

**建议方案**:

- 添加 `manifest.json`(定义应用名、图标、主题色、display: standalone)
- 使用 Workbox(`next-pwa` 或手动配置):
  - 静态资源预缓存(JS/CSS/字体)
  - OSS 图片/视频:Network First,缓存成功响应(Cache API)
  - API 响应:Network First,离线时展示缓存数据 + "离线模式"提示
- Service Worker 注册在 `layout.tsx` 中
- 不要求离线全功能,但至少首屏和已浏览内容可离线访问

---

### G3. 桌面端"一键部署"深链的浏览器兼容性风险 `[B]` → ✅ 已修复关闭(核查确认:URL Scheme 已实现)

**发现位置**:`architecture.md §3.3` + Beta 设计 §12

**问题描述**:

`omnicraft://deploy?grant=<token>` 深链方案存在风险:
- 用户使用非默认浏览器(Chrome 中点按钮,但系统默认是 Edge)
- Firefox 对自定义 URL Scheme 支持不一致
- 如果 Tauri 未安装,浏览器显示"无法打开此链接"——用户不知道发生了什么
- 没有降级方案:深链失败后用户无法手动完成部署

**建议方案**:
- 在 `/client` 页面提供「手动部署」引导步骤
- Tauri 客户端增加「从剪贴板读取部署链接」功能
- Web 端「一键部署」按钮增加状态检测:深链 3 秒未被处理 → 显示降级提示
- 深链触发使用 `window.location.href` 而非 `window.open`(兼容性更好)

---

### G4. Tauri 客户端与 Web 端账号同步 `[A]` → 决策:DEC-026(⏸️ 暂缓)

**发现位置**:`architecture.md`

**问题描述**:

- `architecture.md` 提到"账号互通:与 Web 端实时登录同步"
- 但桌面端禁用后,这个功能无法验证
- URL Scheme 唤醒机制在桌面端禁用时无意义

**建议方案**:桌面端启用前完成账号同步验证,或文档明确该功能为 P1。

---

### G5. 通知系统实时性差 `[A]` → 决策:DEC-033(SSE 实时推送)

**发现位置**:`AGENTS.md`

**问题描述**:

- AGENTS.md 规定 5 分钟轮询 `/api/v1/auth/me` 和 `/notifications/unread-count`
- **问题**:用户可能 5 分钟后才看到新通知,私信和审核结果延迟严重

**建议方案**:通知系统可保持轮询(降低复杂度),但需缩短至 1-2 分钟,或对关键通知(审核结果、封禁)使用 SSE 推送。

---

### G6. 私信系统无实时推送 `[A+B]` → 决策:DEC-007(SSE 实现方案 A)

**发现位置**:`AGENTS.md` + Beta 设计 §5.4(文档 B 问题 2)

**问题描述**:

当前通知系统和私信系统都依赖 5 分钟轮询。Beta 设计已将 SSE 实时化标记为 P1 延后。

但这里的问题是:轮询对于"通知"勉强可用,对于"私信"是**完全不合格的**。即时通讯是最基本的用户预期:

- 用户 A 发了一条私信,用户 B 最多 5 分钟后才看到
- 如果两个人在对话,来回 10 条消息可能跨越 50 分钟
- Beta 如果开放私信功能,5 分钟延迟会让用户认为产品坏了

**建议方案**:
- 至少为私信系统使用 SSE(Server-Sent Events),实现简单且单向足够
- 通知系统可以继续保持轮询(降低复杂度)
- 新增 `GET /api/v1/messages/stream` SSE 流式端点
- 私信 SSE 连接复用 HTTP/2 多路复用,不额外增加连接开销

---

## 类别 H:安全与合规

### H1. 前端 Token 存储方案存在 XSS 泄露风险 `[B]` → 决策:DEC-002(降级 P3)

**发现位置**:Beta 设计 §7.1–7.2

**关联决策**:[DEC-002](./2026-06-24-design-review-decisions.md#dec-002token-存储方案降级为-p3-长期优化)(经核查实际架构已安全,降级为 P3)

**问题描述**:

Access Token 存储在 JavaScript 可访问的内存/变量中,Refresh Token 计划迁移到 HttpOnly Cookie。但 Access Token 仍通过 JavaScript 读取并附加到 `Authorization: Bearer` 头。XSS 攻击可窃取 Access Token,攻击者在 2 小时内可冒充用户进行任意操作。

**建议方案**(按安全性递增):

- **方案 A(短期)**:Access Token 使用闭包变量存储(不挂 window/global),API 模块通过内部引用获取
- **方案 B(中期/推荐 Beta 实施)**:使用 BFF(Backend For Frontend)模式——Next.js API Routes 作为代理,token 存服务端 session(`iron-session`),前端 Cookie 仅含 session ID(HttpOnly)
- **方案 C(长期)**:Token Binding + BFF,Access Token 绑定客户端 fingerprint(User-Agent + IP 范围),异地/IP 变化时要求重新认证

> **⚠️ 事实核查修正(2026-06-24)**:经代码核查,上述问题描述部分不准确。Refresh Token **已经**在 HttpOnly Cookie 中(通过 `credentials: "include"` 自动携带),非"计划迁移"。Access Token 存在内存变量中(不挂 window)。页面刷新通过 `/auth/refresh` 恢复。当前架构已相当安全,降级为 P3。详见 [DEC-002](./2026-06-24-design-review-decisions.md#dec-002token-存储方案降级为-p3-长期优化)。

---

### H2. 密码重置"模糊响应"与用户体验的平衡问题 `[B]` → ✅ 已修复关闭(核查确认:三种状态提示+密码强度)

**发现位置**:Beta 设计 §7.3

**问题描述**:

Beta 设计要求"忘记密码仍使用模糊响应,避免邮箱枚举"。安全方向正确,但用户体验受影响——用户输错邮箱 → 看到"已发送" → 等 5 分钟没收到 → 困惑 → 可能反复提交触发限流。

**建议方案**(安全优先,改善体验):

- **模糊响应保持不变**(安全不可妥协)
- **前端文案优化**:明确告知"如果该邮箱已注册,将在 5 分钟内发送重置链接。请检查收件箱和垃圾邮件文件夹"
- 增加常见问题链接:"没有收到邮件?"(`/help#reset-password`)
- **后端增加安全措施**:同一邮箱(规范化后)15 分钟内只能请求 1 次重置(冷却期);同一 IP 每小时最多 5 次
- **帮助中心补充说明**:确认注册邮箱、检查垃圾邮件、尝试其他邮箱等引导

---

### H3. 法律文本完全缺失 `[A]` → 决策:DEC-003(⏸️ 搁置)

**发现位置**:`backend/config.yaml`

**关联决策**:[DEC-003](./2026-06-24-design-review-decisions.md#dec-003法律文本暂时搁置用户自行处理)(用户自行处理法律文本,代码修复待定稿后实施)

**问题描述**:

```yaml
legal:
  current_terms_version: ""    # 空
  current_privacy_version: ""  # 空
```

**影响**:用户协议和隐私政策为空,存在重大法律合规风险,阻塞 R-01 验收。

**建议方案**:起草用户协议和隐私政策,填入版本号,前端注册/登录流程强制展示。

---

### H4. 验证码和邮件为开发模式 `[A]` → 决策:DEC-018(切换为生产模式)

**发现位置**:`backend/config.yaml`

**问题描述**:

```yaml
captcha:
  provider: "bypass"    # 开发绕过模式
smtp:
  mode: "logger"        # 仅日志不发送
```

**影响**:邮箱验证、密码重置在生产环境不可用。

**建议方案**:生产环境切换为 `aliyun_v2` 验证码和真实 SMTP,通过环境变量注入密钥。

---

## 类别 I:性能与可扩展性

### I1. 推荐引擎冷启动未解决 `[A]` → 决策:DEC-027(增加兴趣选择,可跳过)

**发现位置**:`task.json Task 122`、`architecture.md §12`

**问题描述**:

- Task 122 提到"注册时选择兴趣类别"优化冷启动
- 但实现状态不明,新用户仍可能看到无关内容

**建议方案**:实现注册时兴趣选择,新用户默认纯热门趋势推荐(已设计),逐步过渡到个性化。

---

### I2. 向量索引升级路径不明 `[A]` → 决策:DEC-028(升级为 HNSW)

**发现位置**:`architecture.md §12`

**问题描述**:

- 当前使用 IVFFlat (适合百万级以内)
- `architecture.md` 提到"P2 阶段可升级为 HNSW"
- **问题**:无具体升级方案和数据迁移策略

**建议方案**:制定 IVFFlat → HNSW 迁移方案,包括数据重建、索引切换、回滚策略。

---

### I3. 热门排行更新频率 `[A]` → 决策:DEC-029(增加配置项)

**发现位置**:`architecture.md §12`

**问题描述**:

- 每 10 分钟更新一次 `rank:hot:contents`
- **问题**:热门内容刷新延迟,用户可能看到过时排序

**建议方案**:缩短至 5 分钟,或对热门 Tab 使用更频繁的增量更新。

---

### I4. 队列系统默认禁用 `[A]` → 决策:DEC-019(启用队列系统)

**发现位置**:`backend/config.yaml`

**问题描述**:

```yaml
queue:
  enabled: false
```

**影响**:AI 审核、通知发送、向量化等异步任务可能同步执行,影响响应时间。

**建议方案**:生产环境启用队列,配置 Redis 或 RabbitMQ 作为 broker。

---

## 类别 J:数据库与迁移问题

### J1. 迁移编号超出预留 `[A]` → 决策:DEC-030(范围更新为 001-055)

**发现位置**:`backend/migrations/`、Beta 路线图

**问题描述**:

- Beta 计划预留 049-052
- 实际已有 `055_web_beta_review_repairs.sql`
- **问题**:编号管理失控,可能影响迁移顺序执行

**建议方案**:规范迁移编号管理,预留区间外的新迁移需更新路线图。

---

### J2. 软删除策略分散 `[A]` → 决策:DEC-031(部分统一,以软删除为主)

**发现位置**:`backend/migrations/`

**问题描述**:

- 053: content_items 软删除
- 054: users 软删除
- **问题**:comments、discussions 等关联表无软删除,可能遗留孤儿数据

**建议方案**:为所有用户生成内容表(comments、discussions、collections 等)增加软删除字段,或文档明确哪些表不需要软删除。

---

## 问题优先级矩阵

> **说明**:优先级为审查时的初始评估。"决策后状态"列反映讨论结论导致的优先级变化或覆盖关系。

| 优先级 | 编号 | 问题 | 来源 | 影响范围 | 修复成本 | 决策后状态 |
|--------|-----|------|------|---------|---------|-----------|
| **P0** | B3 | 中文搜索不可靠 | [B] | 全站搜索功能 | 中 | ✅ DEC-001 |
| **P0** | H1 | Token XSS 泄露风险 | [B] | 全站安全 | 中 | ⬇️ 降级 P3(DEC-002) |
| **P0** | H3 | 法律文本完全缺失 | [A] | Beta 发布阻塞 | 中 | ⏸️ 搁置(DEC-003) |
| **P0** | C1 | 信誉分配置缺失+硬编码 | [A] | 信誉分体系 | 中 | ✅ DEC-004 |
| **P0** | D1 | Studio 空存根页面 | [A] | 创作者核心功能 | 中 | ✅ DEC-005 |
| **P0** | F1 | 瀑布流无状态恢复 | [B] | 原创区核心体验 | 低 | ✅ DEC-006 |
| **P1** | G6 | 私信延迟 5 分钟 | [A+B] | 即时通讯体验 | 中 | ✅ DEC-007 |
| **P1** | E1 | 缺草稿系统 | [B] | 创作者体验 | 中 | ✅ DEC-008 |
| **P1** | E3 | 缺数据导出 | [B] | 法律合规 | 中 | ✅ DEC-009 |
| **P1** | D2 | 后端 API 路由未迁移 | [A] | 契约一致性 | 低 | ✅ DEC-005 涵盖 |
| **P1** | D3 | 新旧路由并存无重定向 | [A] | SEO + 用户体验 | 低 | ✅ DEC-005 涵盖 |
| **P1** | E5 | OriginalSidebar 链接错误 | [A] | 导航功能 | 低 | ✅ DEC-010 |
| **P1** | A1 | 设计系统三重冲突 | [A] | 开发误导 | 低 | ✅ DEC-011 |
| **P1** | A3 | 原创区导航文档矛盾 | [A] | 开发误导 | 低 | ✅ DEC-012 |
| **P1** | A4 | UI Design.md 过时 | [A] | 开发误导 | 低 | ✅ DEC-013 |
| **P2** | B1 | 推荐缓存膨胀 | [B] | 扩展性 | 低 | ✅ 已修复关闭 |
| **P2** | B2 | 代码组织缺乏领域边界 | [B] | 开发效率 | 高 | ✅ DEC-014 |
| **P2** | E2 | 缺定时发布 | [B] | 创作者体验 | 低 | ✅ DEC-008 涵盖 |
| **P2** | E4 | 数据分析不足 | [B] | 创作者留存 | 高 | ✅ DEC-015 |
| **P2** | F2 | 侧边栏不可发现 | [B] | 创作者 Studio | 低 | ✅ 已修复关闭 |
| **P2** | F3 | ContentCard 重复 | [B] | 前端维护成本 | 中 | ✅ 已修复关闭 |
| **P2** | F4 | 分类 Tab 不可见 | [B] | 原创区浏览 | 低 | ✅ 已修复关闭 |
| **P2** | G1 | Tauri 无离线 | [B] | 桌面端体验 | 高 | ⏸️ DEC-016(降级 P3) |
| **P2** | G2 | 缺 PWA | [B] | 移动端体验 | 中 | ⏸️ DEC-017(降级 P3) |
| **P2** | G3 | 深链兼容性 | [B] | 桌面部署 | 低 | ✅ 已修复关闭 |
| **P2** | H2 | 模糊响应困惑 | [B] | 密码重置体验 | 低 | ✅ 已修复关闭 |
| **P2** | C2 | 信誉分阈值差异 | [A] | 信誉分计算 | 低 | ✅ DEC-004 涵盖 |
| **P2** | C3 | 配置字段命名不统一 | [A] | 配置读取 | 低 | ✅ DEC-004 涵盖 |
| **P2** | C4 | features 字段缺失 | [A] | feature flag | 低 | ✅ DEC-004 涵盖 |
| **P2** | H4 | 验证码/邮件开发模式 | [A] | 生产就绪 | 低 | ✅ DEC-018 |
| **P2** | I4 | 队列系统禁用 | [A] | 性能 | 中 | ✅ DEC-019 |
| **P2** | C5 | Agent LLM 配置混乱 | [A] | Agent 功能 | 低 | ✅ DEC-020 |
| **P3** | A2 | 侧边栏宽度矛盾 | [A] | 文档一致性 | 低 | ✅ DEC-011 涵盖 |
| **P3** | A5 | 审查报告未整合 | [A] | 文档维护 | 低 | ✅ DEC-021 |
| **P3** | A6 | task.json 与路线图并行 | [A] | 任务管理 | 低 | ✅ DEC-022 |
| **P3** | C6 | 下载权限逻辑不清 | [A] | 权限控制 | 低 | ✅ DEC-024 |
| **P3** | D4 | 首页路由冗余 | [A] | 路由清晰度 | 低 | ✅ DEC-023 |
| **P3** | E6 | 桌面端部署禁用 | [A] | PRD 卖点 | 高 | ✅ DEC-032 |
| **P3** | E7 | 客户端下载禁用 | [A] | 客户端分发 | 中 | ✅ DEC-032 |
| **P3** | F5 | 移动端适配缺陷 | [A] | 移动端体验 | 中 | ✅ DEC-025 |
| **P3** | G4 | 账号同步未验证 | [A] | 多端体验 | 中 | ⏸️ DEC-026(暂缓) |
| **P3** | G5 | 通知实时性差 | [A] | 通知体验 | 中 | ✅ DEC-033 |
| **P3** | I1 | 推荐冷启动 | [A] | 新用户体验 | 中 | ✅ DEC-027 |
| **P3** | I2 | 向量索引升级路径 | [A] | 长期扩展 | 高 | ✅ DEC-028 |
| **P3** | I3 | 热门排行更新频率 | [A] | 内容新鲜度 | 低 | ✅ DEC-029 |
| **P3** | J1 | 迁移编号超出预留 | [A] | 迁移管理 | 低 | ✅ DEC-030 |
| **P3** | J2 | 软删除策略分散 | [A] | 数据一致性 | 中 | ✅ DEC-031 |

---

## 来源统计

| 来源 | 问题数 | 占比 |
|------|--------|------|
| `[A]` 仅文档 A | 30 | 64% |
| `[B]` 仅文档 B | 16 | 34% |
| `[A+B]` 两份均发现 | 1 | 2% |
| **合计** | **47** | 100% |

**重叠分析**:
- 完全重叠:G6(私信实时性)
- 角度不同但相关:E6(桌面部署禁用)与 G3(深链兼容性)关注同一功能的不同方面
- 互补关系:文档 A 侧重文档一致性与配置实现,文档 B 侧重架构设计与 UX,合并后覆盖更全面

---

## 附录:审查覆盖的文档清单

| 文档 | 路径 | 审查来源 |
|------|------|---------|
| 技术架构设计 | `architecture.md` | A + B |
| Agent 工作指南 | `AGENTS.md` | A |
| UI 设计规格 | `design/ui-spec.md` | A + B |
| 设计系统 | `design/design-system.md` | A |
| UI Design(旧) | `design/UI Design.md` | A |
| 首页设计概念 | `design/homepage-v0.html` | A |
| 历史任务账本 | `task.json` | A + B |
| 后端配置 | `backend/config.yaml` | A |
| 后端路由 | `backend/internal/handler/routes.go` | A |
| 信誉分服务 | `backend/internal/service/reputation_service.go` | A |
| Beta 设计规格 | `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md` | B |
| Beta Roadmap | `docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md` | A + B |
| Beta 实现笔记 | `docs/superpowers/plans/2026-05-30-omnicraft-beta-implementation-notes.md` | A + B |
| 前端实际实现 | `frontend/app/`、`frontend/components/` | A |

---

*合并完成时间:2026-06-24*
*合并来源:文档 A(文档一致性与配置审查)+ 文档 B(架构设计与 UX 审查)*
*整合原则:保留所有问题,合并重叠项,标注来源,不遗漏*
