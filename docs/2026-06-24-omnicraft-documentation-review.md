# OmniCraft 项目文档审查报告

> **审查日期**:2026-06-24
> **审查范围**:architecture.md、design/ui-spec.md、design/design-system.md、design/UI Design.md、design/homepage-v0.html、task.json、config.yaml、Beta 路线图、前端/后端实际实现
> **审查目的**:全面发现文档中的设计缺陷、功能问题、多端使用问题及文档间不一致

---

## 一、严重:文档间系统性矛盾(三套设计系统并存)

### 1.1 设计系统三重冲突

项目中存在**三套互不一致的设计系统**,极易误导开发:

| 文档 | 主色调 | 背景 | Header 高度 | 字体 |
|------|--------|------|------------|------|
| `architecture.md §10.4` | GitHub blue `#0969da` | `#ffffff` | h-16 (64px) | 系统字体 |
| `design/design-system.md` | Indigo `#4F46E5` | `#FFFFFF` | 52px | 系统字体 |
| `design/homepage-v0.html` | Muted blue-gray `#526B8C` | 暖白 `#FAFAF8` | 56px | **Sora** 字体 |

**实际实现**(`frontend/app/globals.css` + `frontend/components/layout/Header.tsx`):使用 Indigo `#4f46e5`、52px header — 匹配 `design-system.md`,但 `architecture.md §10.4` 和 `homepage-v0.html` 均过时。

**`homepage-v0.html` 是一个未集成的全新设计概念**(暖色调 + Sora 字体 + 无边框卡片),与现有实现完全不同,却没有任何文档说明其状态(是已废弃?还是待实施?)。

### 1.2 侧边栏宽度矛盾

- `architecture.md §13.2`:展开 224px (w-56) / 收起 64px (w-16)
- `design/design-system.md`:展开 228px / 折叠 48px

两份文档数值不一致,实际实现需确认。

### 1.3 原创区导航结构直接矛盾

- `architecture.md §10.6`:明确"**无二级内容类型筛选**",采用单层分类 Tab
- `design/UI Design.md P06`:描述有"**二级内容类型子筛选**"(全部/图片/视频/音频/文字/效率模板/模型与设计/其他)
- `task.json Task 64`:标注"已被 Task 90 覆盖"
- **实际实现**(`frontend/app/(public)/original/page.tsx`):单层分类 Tab,符合 `architecture.md`

**问题**:`UI Design.md` 严重过时,仍描述旧的两级导航,会误导开发者按错误规格实现。

---

## 二、严重:Studio 迁移严重不完整(空存根页面)

`architecture.md §3.2` 声称系统已迁移至 `/studio/*`,旧 `/dashboard/*` 保留重定向。但实际情况:

### 2.1 前端 Studio 页面为空存根

以下 Studio 页面**只有 EmptyState 占位,无任何 API 调用**:

- `frontend/app/(protected)/studio/contributors/page.tsx` — 空存根
- `frontend/app/(protected)/studio/pr-requests/page.tsx` — 空存根
- `frontend/app/(protected)/studio/tag-suggestions/page.tsx` — 空存根

而真正功能完整的页面在旧路由:

- `frontend/app/(protected)/dashboard/contributors/page.tsx` — 调用 `/api/v1/dashboard/contributors` API

### 2.2 后端 API 路由未迁移

`backend/internal/handler/routes.go` 中:

- ✅ 存在:`/dashboard/contributors/:userId/block`
- ❌ 不存在:`/studio/contributors/:userId/block`

但 `architecture.md §3.2` 明确写了:

```
POST /api/v1/studio/contributors/:userId/block    # 作者拉黑贡献者
```

**影响**:用户从 StudioSidebar 点击进入协作管理,看到的是空页面,无法管理 PR、贡献者、标签建议。这是核心功能缺失。

---

## 三、严重:信誉分配置完全缺失(违反硬编码禁令)

### 3.1 config.yaml 缺失所有信誉分加减分值

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

### 3.2 代码中硬编码加分值

`backend/internal/service/reputation_service.go`:

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

### 3.3 阈值差异巨大

| 字段 | architecture.md | config.yaml |
|------|-----------------|-------------|
| quality_content_threshold | 50 | 10 |
| quality_comment_threshold | 20 | 5 |

差异 5 倍,将严重影响信誉分计算。

### 3.4 配置字段命名不统一

| architecture.md | config.yaml |
|-----------------|-------------|
| `video_max_size_mb` | `video_max_mb` |
| `mod_archive_max_size_mb` | `mod_max_mb` |
| `min_score_for_publish` | `min_score_for_interaction` |

### 3.5 features 字段缺失

`architecture.md §7` 定义 `features.agent_enabled`、`features.judge_enabled`、`features.ad_enabled`,但 `config.yaml` 完全没有这些字段。无法通过 feature flag 控制这些功能。

---

## 四、严重:路由系统混乱(重复路由 + 冗余页面)

### 4.1 新旧路由并存

前端同时存在:

- `/publish` 和 `/studio/publish/original` + `/studio/publish/fanwork`
- `/dashboard/contents` 和 `/studio/contents`
- `/dashboard/contributors` 和 `/studio/contributors`
- `/dashboard/pr-requests` 和 `/studio/pr-requests`
- `/dashboard/tag-suggestions` 和 `/studio/tag-suggestions`

**问题**:无 301 重定向实现,SEO 重复内容,用户困惑,维护负担倍增。

### 4.2 首页路由冗余

- `frontend/app/(public)/page.tsx` — 首页
- `frontend/app/(public)/home/page.tsx` — 另一个首页?
- `frontend/app/(public)/ips/page.tsx` — IP 列表页(architecture.md 未定义此独立路由)

`architecture.md §3.1` 说首页是 `/`,但 `/home` 路由的存在造成混淆。

---

## 五、功能缺陷

### 5.1 OriginalSidebar 导航链接错误

`frontend/components/original/OriginalSidebar.tsx`:

```tsx
{ icon: Heart, label: t("nav.favorites"), href: "/studio/contents" },
{ icon: FileText, label: t("nav.myOriginal"), href: "/studio/contents" },
```

"收藏"和"我的原创"两个不同功能入口指向**同一 URL** `/studio/contents`。

### 5.2 桌面端一键部署完全禁用

- `features.desktop_deploy_enabled: false`
- D-02 至 D-05 任务未完成
- **影响**:PRD 核心卖点"Agent 一键部署"在 Web Beta 完全不可用,内容详情页的"一键部署"按钮被隐藏。

### 5.3 客户端下载禁用

- `client.download_enabled: false`
- **影响**:用户无法下载 PC 客户端,`/client` 页面显示不可用状态。

### 5.4 法律文本完全缺失

```yaml
legal:
  current_terms_version: ""    # 空
  current_privacy_version: ""  # 空
```

**影响**:用户协议和隐私政策为空,存在重大法律合规风险,阻塞 R-01 验收。

### 5.5 验证码和邮件为开发模式

```yaml
captcha:
  provider: "bypass"    # 开发绕过模式
smtp:
  mode: "logger"        # 仅日志不发送
```

**影响**:邮箱验证、密码重置在生产环境不可用。

---

## 六、多端使用问题

### 6.1 移动端适配缺陷

- OriginalSidebar 在移动端的折叠/抽屉行为未明确实现
- Header 搜索框在移动端的展开/折叠交互
- 表单类页面(发布、设置)在移动端的体验

### 6.2 通知系统实时性差

- AGENTS.md 规定 5 分钟轮询 `/api/v1/auth/me` 和 `/notifications/unread-count`
- **问题**:用户可能 5 分钟后才看到新通知,私信和审核结果延迟严重

### 6.3 私信系统无实时推送

- MVP 使用 SSE
- **问题**:私信体验不如 WebSocket 实时,且 SSE 连接管理在移动端浏览器不稳定

### 6.4 Tauri 客户端与 Web 端账号同步

- `architecture.md` 提到"账号互通:与 Web 端实时登录同步"
- 但桌面端禁用后,这个功能无法验证
- URL Scheme 唤醒机制在桌面端禁用时无意义

---

## 七、数据库与迁移问题

### 7.1 迁移编号超出预留

- Beta 计划预留 049-052
- 实际已有 `055_web_beta_review_repairs.sql`
- **问题**:编号管理失控,可能影响迁移顺序执行

### 7.2 软删除策略分散

- 053: content_items 软删除
- 054: users 软删除
- **问题**:comments、discussions 等关联表无软删除,可能遗留孤儿数据

---

## 八、性能与可扩展性

### 8.1 推荐引擎冷启动未解决

- Task 122 提到"注册时选择兴趣类别"优化冷启动
- 但实现状态不明,新用户仍可能看到无关内容

### 8.2 向量索引升级路径不明

- 当前使用 IVFFlat (适合百万级以内)
- `architecture.md` 提到"P2 阶段可升级为 HNSW"
- **问题**:无具体升级方案和数据迁移策略

### 8.3 热门排行更新频率

- 每 10 分钟更新一次 `rank:hot:contents`
- **问题**:热门内容刷新延迟,用户可能看到过时排序

---

## 九、文档维护问题

### 9.1 UI Design.md 严重过时

- 仍描述 `/publish` 和 `/dashboard/*` 路由(未提及 `/studio/*`)
- 仍描述原创区"二级内容类型子筛选"(已被 Task 90 移除)
- **影响**:新开发者按此文档实现会引入错误

### 9.2 审查报告未整合

`docs/review/` 下有 20+ 份审查报告,但:

- 无统一的问题追踪表
- 无法确认哪些问题已修复
- 审查发现可能重复或遗漏

### 9.3 task.json 与 Beta 路线图并行

- `task.json` 有 145+ 历史任务
- Beta 路线图有 28 个任务(F-01 到 R-02)
- **问题**:两套任务跟踪系统,容易混淆优先级

---

## 十、其他问题

### 10.1 队列系统默认禁用

```yaml
queue:
  enabled: false
```

**影响**:AI 审核、通知发送、向量化等异步任务可能同步执行,影响响应时间。

### 10.2 Agent LLM 配置混乱

```yaml
agent:
  llm_api_key: ""    # 留空,注释说"改用 env var 注入(需修复 config.go)"
```

**问题**:注释明确说"需修复",但状态不明,可能导致 Agent 功能失效。

### 10.3 内容下载权限逻辑

- AGENTS.md 规定"信誉分 < 3 用户禁止下载"
- 但 `config.yaml` 中 `min_score_for_interaction: 3` 是否涵盖下载权限不明确
- **问题**:下载权限的阈值字段命名不清晰

---

## 总结:优先级排序

### P0 — 阻塞 Beta 发布,必须立即解决

1. **准备法律文本** — 用户协议和隐私政策为空,阻塞 R-01 验收
2. **补全信誉分配置** — 移除代码硬编码,将所有加减分值加入 config.yaml
3. **完成 Studio 迁移或回退** — 实现存根页面功能或暂时隐藏入口

### P1 — 影响核心体验,尽快解决

4. **统一设计系统文档** — 删除或归档过时的 architecture.md §10.4 和 homepage-v0.html
5. **统一配置字段命名** — architecture.md 与 config.yaml 字段名对齐
6. **更新 UI Design.md** — 同步 Task 90 的设计变更
7. **清理重复路由** — 实现 301 重定向或删除冗余页面
8. **修复 OriginalSidebar 链接错误** — 收藏和我的原创指向同一 URL

### P2 — 影响生产就绪,发布前解决

9. **切换验证码和 SMTP 到生产模式** — bypass 和 logger 仅限开发
10. **启用队列系统** — 异步任务不应同步执行
11. **修复 Agent LLM 配置** — env var 注入需完成

### P3 — 长期优化

12. **整合审查报告** — 建立统一问题追踪表
13. **明确 homepage-v0.html 状态** — 废弃或纳入实施计划
14. **规划向量索引升级路径** — IVFFlat → HNSW 迁移方案
15. **优化通知实时性** — 考虑 WebSocket 替代轮询

---

## 待讨论事项

本报告列出的问题需与项目维护者逐一确认解决方案。维护者已自行发现部分问题并准备了方案,本报告作为独立分析的对照参考,待整合后统一讨论。
