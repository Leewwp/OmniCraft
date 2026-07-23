# 创作者工作室（/studio）

> 本文档由 2026-07-23 文档瘦身从 `architecture.md` §13 抽取，章节号保持原编号以便深链兼容。

## 13. 创作者工作室（/studio）

> **定位**：整合「内容发布 + 数据看板 + 协作管理」的统一创作者工作空间，参考 B 站创作中心和小红书创作者后台的设计思路。旧的 `/publish` 和 `/dashboard/*` 路由迁移至此，原路由保留重定向。

### 13.1 页面布局

```
┌──────────────────────────────────────────────────────┐
│ Header（h-13/52px，全宽，底边框）                     │
├──────────┬───────────────────────────────────────────┤
│ Sidebar  │ 主内容区                                   │
│ (228px   │                                            │
│  展开)   │  ┌─ 内容类型选择网格 ──────────────────┐  │
│          │  │ [图文] [纯文字] [视频] [音频] ...     │  │
│          │  │（按发布频率高→低，左→右排列）        │  │
│          │  └────────────────────────────────────┘  │
│  📝 发布 │                                            │
│   发布原创│  ┌─ 或：发布表单 / 数据看板 / 协作管理 ─┐  │
│   发布二创│  │  (根据侧边栏选中项动态切换)          │  │
│          │  └────────────────────────────────────┘  │
│  📊 看板 │                                            │
│   概览    │                                            │
│   内容管理│                                            │
│   粉丝分析│                                            │
│   收益    │                                            │
│          │                                            │
│  🤝 协作 │                                            │
│   PR申请 │                                            │
│   贡献者 │                                            │
│   标签建议│                                            │
│          │                                            │
│ (收起时  │                                            │
│  仅图标) │                                            │
└──────────┴───────────────────────────────────────────┘
```

### 13.2 侧边栏（StudioSidebar）

#### 结构

| 分组 | 项 | 图标 | 目标路由 | 说明 |
|------|-----|------|---------|------|
| 内容发布 | 发布原创 | ✏️ | `/studio/publish/original` | 原创区内容类型选择 → 发布表单 |
| | 发布二创 | 🎨 | `/studio/publish/fanwork` | 二创区内容类型选择 → 发布表单 |
| 数据看板 | 概览 | 📊 | `/studio/overview` | 访问量/互动量/内容总数概览卡片 + 趋势图 |
| | 内容管理 | 📋 | `/studio/contents` | 我的内容列表（筛选/排序/编辑/删除） |
| | 收藏集 | ⭐ | `/studio/favorites` | 收藏集列表、公开/私有设置和默认收藏集管理 |
| | 粉丝分析 | 👥 | `/studio/followers` | 粉丝增长趋势、粉丝画像（新增） |
| | 内容系列 | 🧭 | `/studio/series` | 内容系列创建、排序和条目管理 |
| | 收益数据 | 💰 | `/studio/revenue` | P1 预留，显示「即将开放」占位 |
| 协作管理 | PR 申请 | 🔀 | `/studio/pr-requests` | 收到的 PR 申请列表（作者视角） |
| | 贡献者 | 👤 | `/studio/contributors` | 贡献者管理 + 黑名单 |
| | 标签建议 | 🏷️ | `/studio/tag-suggestions` | 待审核的标签建议 |

#### 折叠行为

- **展开态**（默认，宽度 228px）：显示图标 + 标题文字，分组标题可见
- **收起态**（宽度 48px）：仅显示图标，分组标题隐藏，分组之间用分隔线区分
- **切换按钮**：侧边栏顶部/底部放置 `<` / `>` 箭头按钮
- **Hover Tooltip**：收起态下鼠标悬停图标 → 在图标右侧弹出 tooltip（`absolute left-full ml-2`），显示该项的标题文字，延迟 300ms 出现
- **键盘**：支持 `Tab` 在图标间切换，`Enter` 导航

#### 视觉规范

- 背景：`bg-canvas.subtle`，右侧 1px border `border-border.muted`
- 选中项：`bg-accent.emphasis/10 text-accent.emphasis font-medium`，左侧 3px accent 色竖线指示器
- 未选中项：`text-fg.muted hover:bg-canvas.default hover:text-fg.default`
- 分组标题：`text-xs text-fg.muted uppercase tracking-wider px-3 pt-4 pb-1`（展开态可见）
- 图标大小：20px × 20px（`w-5 h-5`）
- 展开态项高度：40px（`h-10`），收起态项高度：48px（`h-12`，仅图标居中）

### 13.3 内容发布流程

#### 步骤 1：选择内容类型

进入 `/studio/publish/original` 或 `/studio/publish/fanwork` 后，主区域显示内容类型选择网格。

**卡片网格布局**（ContentTypeGrid）：

```
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│  🖼️      │ │  📝      │ │  🎬      │ │  🎵      │
│  图文     │ │  纯文字   │ │  视频     │ │  音频     │
│  照片+文字 │ │  长文/短文 │ │  短视频    │ │  音乐/播客 │
│  最常用    │ │  自由创作  │ │  记录分享  │ │  声音创作  │
└──────────┘ └──────────┘ └──────────┘ └──────────┘
┌──────────┐ ┌──────────┐ ┌──────────┐
│  📋      │ │  🎼      │ │  📦      │
│  效率模板  │ │  乐谱     │ │  其他     │
│  工具分享  │ │  音乐创作  │ │  更多格式  │
└──────────┘ └──────────┘ └──────────┘
```

**原创区内容类型排序**（按发布频率高→低）：

| 排序 | content_type | 显示名 | 图标 | 描述 |
|------|-------------|--------|------|------|
| 1 | `image` | 图文 | 🖼️ | 照片 + 文字描述，分享生活与创作 |
| 2 | `article` | 纯文字 | 📝 | 长文/短文，自由表达 |
| 3 | `video` | 视频 | 🎬 | 短视频/长视频，记录与分享 |
| 4 | `audio` | 音频 | 🎵 | 音乐/播客/有声内容 |
| 5 | `template` | 效率模板 | 📋 | Notion/Excel 等工具模板 |
| 6 | `sheet_music` | 乐谱 | 🎼 | MIDI/MusicXML/PDF 乐谱 |
| 7 | `other` | 其他 | 📦 | 其他格式内容 |

**二创区内容类型排序**（按发布频率高→低）：

| 排序 | content_type | 显示名 | 图标 | 描述 |
|------|-------------|--------|------|------|
| 1 | `image` | 图文 | 🖼️ | 二创插画/漫画/Cosplay 照片 |
| 2 | `article` | 纯文字 | 📝 | 同人文/分析/短篇 |
| 3 | `video` | 视频 | 🎬 | 二创剪辑/手书/MAD |
| 4 | `audio` | 音频 | 🎵 | 翻唱/配音/广播剧 |
| 5 | `mod` | Mod | 🎮 | 游戏模组/材质包 |
| 6 | `prompt` | AI 提示词 | 🤖 | Stable Diffusion/ChatGPT 提示词 |
| 7 | `sheet_music` | 乐谱 | 🎼 | 二创编曲/扒谱 |
| 8 | `other` | 其他 | 📦 | 其他二创格式 |

排序从 `config.yaml > publish.type_order_original` / `publish.type_order_fanwork` 读取，支持管理员热更新。

#### 步骤 2：填写发布表单

选择内容类型后，主区域切换为发布表单（zone + content_type 已锁定，不可切换）。表单复用现有 `/publish` 页面逻辑：

- 标题、描述（Markdown）
- 文件上传（FileUploader，白名单校验）
- IP 选择器（仅二创区，必填）
- 来源原创选择器（仅二创区，可选）
- 标签（TagInput）
- 权限开关（is_public / allow_copy / agent_enabled）
- AI 辅助填写按钮（UploadAssistPanel，`web_agent_enabled=true` 时显示）
- 合规检测 Badge（ComplianceCheckBadge）

### 13.4 数据看板

#### 概览（/studio/overview）

参考 B 站创作中心首页，展示：

```
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│ 总内容数   │ │ 总访问量   │ │ 总点赞数   │ │ 粉丝数    │
│   42      │ │  12,580   │ │   1,203   │ │   256     │
│  ↑3 本月   │ │  ↑18% 本周 │ │  ↑5% 本月  │ │  ↑12 本月  │
└──────────┘ └──────────┘ └──────────┘ └──────────┘

┌─────────────────────────────────────────┐
│  访问量趋势（近 30 天折线图）              │
│  📈 recharts LineChart                  │
└─────────────────────────────────────────┘

┌──────────────────────┐ ┌──────────────────┐
│  内容排行（Top 5）     │ │  待处理事项        │
│  1. xxx  1,200👁️     │ │  PR 申请: 3       │
│  2. yyy    890👁️     │ │  标签建议: 5      │
│  ...                  │ │  举报待处理: 2    │
└──────────────────────┘ └──────────────────┘
```

#### 粉丝分析（/studio/followers，新增）

- 粉丝总数 + 新增/流失趋势折线图（近 30 天）
- 粉丝来源分布（来自哪条内容的关注）
- 粉丝活跃时段热力图（P1）
- 后端新增 API：`GET /api/v1/users/:id/followers/stats?days=30`

#### 收益数据（/studio/revenue，P1 预留）

- 显示「收益功能即将开放，敬请期待」EmptyState
- 未来展示：累计收益、收益趋势、提现入口
- 依赖 `features.creator_support_enabled: true`（P1 开启）

### 13.5 路由迁移对照

| 旧路由 | 新路由 | 处理方式 |
|--------|--------|---------|
| `/publish` | `/studio/publish/original` | 301 重定向 |
| `/dashboard` | `/studio/overview` | 301 重定向 |
| `/dashboard/contents` | `/studio/contents` | 301 重定向 |
| `/dashboard/pr-requests` | `/studio/pr-requests` | 301 重定向 |
| `/dashboard/contributors` | `/studio/contributors` | 301 重定向 |
| `/dashboard/tag-suggestions` | `/studio/tag-suggestions` | 301 重定向 |
| — | `/studio/publish/fanwork` | 新增 |
| — | `/studio/followers` | 新增 |
| — | `/studio/series` | 新增 |
| — | `/studio/revenue` | 新增（P1 占位） |

### 13.6 配置扩展（config.yaml）

```yaml
publish:
  type_order_original: ["image", "article", "video", "audio", "template", "sheet_music", "other"]
  type_order_fanwork: ["image", "article", "video", "audio", "mod", "prompt", "sheet_music", "other"]
```

### 13.7 新增 API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/users/:id/followers/stats` | 粉丝统计（总数、新增趋势、来源分布），参数 `?days=30` |

---

*文档完成时间：与 PRD V0.3 对齐，后续变更请同步更新 task.json 对应任务。*

---
