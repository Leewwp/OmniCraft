# OmniCraft（万象工坊）技术架构设计文档

**版本**：1.0 | **对应 PRD**：V0.3 正式版 | **技术栈**：Next.js + Go + PostgreSQL + Tauri

---

> **章节映射（2026-07-23 文档瘦身）**：本文件现仅保留常绿概述章节（§1-3、§6、§8-9，编号保持原样）。
> 其余章节已迁出，深链请按下表访问新位置：
>
> | 原章节 | 新位置 |
> |--------|--------|
> | §4 数据库 Schema | `docs/reference/schema.md` |
> | §5 API 接口详细说明 | `docs/reference/api.md` |
> | §7 配置化管理 | `docs/reference/config.md` |
> | §10 需求补充（V0.3.1） | `docs/specs/v0.3.1-supplement.md` |
> | §11 网页版 Agent（V0.4 MVP） | `docs/specs/web-agent-v0.4-mvp.md` |
> | §12 推荐页面 | `docs/specs/recommendation-page.md` |
> | §13 创作者工作室（/studio） | `docs/specs/studio.md` |
> | §14-15 安全加固/架构优化实现笔记 | `docs/reference/implementation-notes.md` |

## 1. 系统总览

### 1.1 产品定位

OmniCraft 是一个以 **IP 二创内容聚合** 为核心流量底座、**Agent 自动化**为增值能力、**GitHub 式 PR 协同**为社区护城河的全民创意分享平台。

### 1.2 整体架构图（文字版）

```
┌─────────────────────────────────────────────────────────┐
│                      用户端                              │
│   Browser (Next.js SSR)        Tauri PC 客户端           │
│   Web 主站                     Agent 引擎                │
└──────────────┬──────────────────────────┬───────────────┘
               │ HTTPS REST API            │ URL Scheme / HTTPS
               ▼                           ▼
┌──────────────────────────────────────────────────────────┐
│                    Nginx 反向代理                         │
│            (SSL 终止 / 静态资源 / 限流)                   │
└──────────────┬───────────────────────────────────────────┘
               │
       ┌───────┴────────┐
       ▼                ▼
┌─────────────┐  ┌─────────────────────────────────────────┐
│  Next.js    │  │          Go API Server（单体 → 模块化）  │
│  前端服务   │  │  /auth /users /ips /contents /reviews   │
│  (SSR+BFF)  │  │  /versions /social /admin /agent        │
└─────────────┘  └──────────┬──────────────────────────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
       ┌─────────┐    ┌──────────┐   ┌──────────┐
       │PostgreSQL│   │  Redis   │   │ 阿里云OSS │
       │(主数据库)│   │(缓存/限流)│   │(文件存储) │
       └─────────┘    └──────────┘   └──────────┘
              │
       ┌──────┴───────┐
       ▼              ▼
┌──────────┐   ┌──────────────────┐
│PgBouncer │   │  阿里云内容安全   │
│(连接池)  │   │  (AI 审核接口)   │
└──────────┘   └──────────────────┘
```

### 1.3 流量路径

1. **浏览内容**：Browser → Nginx → Next.js（SSR 渲染）→ Go API → PostgreSQL/Redis
2. **上传内容**：Browser → Go API → OSS 直传（客户端获取预签名 URL 后直传 OSS，Go 仅记录元数据）
3. **AI 审核**：Go API 上传完成后异步调用阿里云内容安全，结果回写 PostgreSQL
4. **Agent 部署**：Tauri 客户端通过 URL Scheme 唤醒 → 调用 Go API 获取动作脚本 → 本地执行白名单动作

---

---

## 2. 可扩展性策略

### 2.1 阶段划分

| 阶段 | 用户规模 | 部署方式 | 数据库 |
|------|---------|---------|-------|
| P0 初期 | < 1,000 人 | Docker Compose 单机 | PostgreSQL 单主库 + PgBouncer |
| P1 成长 | 1,000–10,000 人 | Docker Compose 多节点 / 简单负载均衡 | 增加 PostgreSQL 读从库（GORM dbresolver 自动切换） |
| P2 扩张 | > 10,000 人 | 迁移 K8s（阿里云 ACK）| 迁移云托管数据库（RDS PostgreSQL）或自建 PG 集群 |

### 2.2 代码层扩容预留

- **Go 后端**：启动时通过环境变量 `DB_READ_DSN` 注入从库 DSN；GORM `dbresolver` 插件在启动时判断是否配置从库，无从库时全走主库，**无需改代码**
- **连接池**：PgBouncer 作为独立容器，Go 连接 PgBouncer 而非直连 PostgreSQL，支持平滑迁移
- **OSS**：所有文件路径存相对 key，存储域名通过环境变量配置，切换 CDN 或存储桶无需改代码
- **视频/文件限制**：所有上传限制写入配置文件 `config.yaml`，热更新无需重启

### 2.3 Docker Compose → K8s 迁移路线

```
1. 所有服务已容器化，镜像复用
2. docker-compose.yml 对应 K8s Deployment + Service（保留 k8s/ 目录作为预留配置）
3. 数据库迁移：pg_dump 导出 → 云托管 RDS 导入（停机窗口 < 30 分钟）
4. 域名/DNS 切换，Nginx 替换为 Ingress Controller
```

### 2.4 发布门（Ops-08）

生产发布受 `scripts/release/preflight.sh`（有效生产配置契约：占位符/默认值/非 HTTPS/不安全 flags/数据库 TLS 策略/trusted-proxy 拓扑/frontend-API DNS 一致性）与 `scripts/release/staging-drill.sh`（真实 staging 的 preflight → deploy candidate digest → 验证 → schema 兼容回滚 → 重部署）约束。镜像以不可变 sha256 digest 引用（`release/deployment-manifest.schema.json`），回滚绝不执行破坏性 down SQL；真实 staging/OSS/off-site 输入缺失时 drill 阻塞（exit 3），不以模拟证据替代。`.github/workflows/release.yml` 仅接受手动触发，部署 job 绑定 GitHub Environment `production` 保护。

---

---

## 3. 模块划分

### 3.1 Web 前端（Next.js App Router）

#### 页面路由清单

```
app/
├── (public)/                        # 无需登录
│   ├── page.tsx                     # 首页（二创区内容流）
│   ├── ip/[ipId]/page.tsx           # IP 详情页
│   ├── ip/[ipId]/[category]/page.tsx # IP 下某类目内容列表
│   ├── content/[contentId]/page.tsx # 内容详情页
│   ├── original/page.tsx            # 原创区首页（小红书式推荐流 + 分类 Tab）
│   ├── original/[contentId]/page.tsx# 原创内容详情页
│   ├── original/[contentId]/fanworks/page.tsx # 原创相关二创列表
│   ├── collections/[id]/page.tsx    # 收藏集详情（公开收藏集可浏览）
│   ├── series/[id]/page.tsx         # 内容系列详情（公开可浏览）
│   ├── user/[userId]/page.tsx       # 用户主页（公开可浏览）
│   ├── user/[userId]/collections/page.tsx # 用户收藏集列表（公开可浏览）
│   ├── search/page.tsx              # 搜索页
│   ├── login/page.tsx               # 登录页
│   ├── register/page.tsx            # 注册页
│   ├── forgot-password/page.tsx     # 忘记密码
│   ├── reset-password/page.tsx      # 重置密码
│   ├── verify-email/page.tsx        # 邮箱验证
│   ├── terms/page.tsx               # 用户协议
│   ├── privacy/page.tsx             # 隐私政策
│   ├── help/page.tsx                # 帮助中心
│   ├── client/page.tsx              # 桌面客户端下载
│   └── home/page.tsx                # 旧首页（重定向）
├── (protected)/                     # 需要登录
│   ├── settings/page.tsx            # 账号设置
│   │   └── tag-groups/page.tsx      # 标签组管理
│   ├── studio/                      # 创作者工作室（统一发布 + 数据看板 + 协作管理）
│   │   ├── layout.tsx               # 共享布局：可折叠侧边栏 + 主内容区
│   │   ├── page.tsx                 # 默认重定向 → publish/original
│   │   ├── publish/
│   │   │   ├── original/page.tsx    # 发布原创（内容类型选择 → 发布表单）
│   │   │   └── fanwork/page.tsx     # 发布二创（内容类型选择 → 发布表单）
│   │   ├── overview/page.tsx        # 数据概览
│   │   ├── contents/page.tsx        # 内容管理
│   │   ├── favorites/page.tsx       # 收藏集管理
│   │   ├── followers/page.tsx       # 粉丝分析（新增）
│   │   ├── series/page.tsx          # 内容系列管理
│   │   ├── revenue/page.tsx         # 收益数据（P1 预留）
│   │   ├── pr-requests/page.tsx     # 协同申请管理
│   │   ├── contributors/page.tsx    # 贡献者管理
│   │   └── tag-suggestions/page.tsx # 标签建议审核
│   ├── publish/page.tsx             # ⚠️ 已废弃 → 重定向 /studio/publish/original
│   ├── dashboard/                   # ⚠️ 已废弃 → 重定向 /studio/overview
│   │   ├── page.tsx                 #   保留重定向，内容迁移至 /studio/*
│   │   ├── contents/page.tsx
│   │   ├── pr-requests/page.tsx
│   │   ├── contributors/page.tsx
│   │   └── tag-suggestions/page.tsx
│   ├── judge/                       # 赛博判官
│   │   ├── exam/page.tsx            # 资质考核
│   │   └── queue/page.tsx           # 待审内容队列
│   ├── history/page.tsx             # 浏览历史
│   ├── appeals/page.tsx             # 我的申诉
│   ├── messages/page.tsx            # 消息中心（通知 + 私信）
│   ├── agent/page.tsx               # 独立全页 Agent 工作台（feature gate + 登录保护）
│   ├── rehab/page.tsx               # 素质建设课程
│   └── feedback/page.tsx            # 用户反馈
└── admin/                           # 管理员后台
    ├── ips/page.tsx                 # IP 库管理
    ├── contents/page.tsx            # 内容终审
    ├── users/page.tsx               # 用户管理
    ├── appeal/page.tsx              # 申诉处理
    ├── config/page.tsx              # 系统配置
    ├── categories/page.tsx          # 分类与标签管理
    ├── agent-config/page.tsx        # Agent / LLM 配置管理
    ├── dashboard/page.tsx           # 管理后台首页
    ├── reports/page.tsx             # 举报管理
    ├── feedback/page.tsx            # 反馈工单管理
    ├── audit-logs/page.tsx          # 审计日志
    └── queue/page.tsx               # 异步任务队列
```

#### 关键组件树

```
components/
├── layout/
│   ├── Header.tsx                   # 顶部导航（登录状态、搜索、最近IP）
│   ├── Footer.tsx
│   └── Sidebar.tsx                  # 标签筛选侧边栏
├── studio/                          # 创作者工作室组件（新增）
│   ├── StudioLayout.tsx             # 工作室共享布局：可折叠侧边栏 + 主内容区
│   ├── StudioSidebar.tsx            # 可折叠侧边栏（展开/收起 + hover tooltip）
│   ├── ContentTypeGrid.tsx          # 内容类型选择卡片网格（按频率排序）
│   ├── FollowerTrendChart.tsx       # 粉丝增长趋势图（折线图）
│   └── FollowerSourceChart.tsx      # 粉丝来源分布图（饼图）
├── ip/
│   ├── IPCard.tsx                   # IP 卡片（首页列表）
│   ├── IPDetail.tsx                 # IP 详情布局
│   └── IPCategoryTabs.tsx           # 内容类目 Tab（参考 Steam）
├── content/
│   ├── ContentCard.tsx              # 内容卡片（通用）
│   ├── ContentDetail.tsx            # 内容详情页
│   ├── ContentDetailOverlay.tsx     # 推荐流与 Agent 引用共用的内容详情浮层
│   ├── MarkdownRenderer.tsx         # MD 渲染（react-markdown）
│   ├── MarkdownEditor.tsx           # MD 编辑器（@uiw/react-md-editor）
│   ├── FileUploader.tsx             # 文件上传（OSS 直传）
│   └── VersionHistory.tsx           # 版本历史时间轴
├── pr/
│   ├── PRCard.tsx                   # PR 申请卡片
│   ├── DiffViewer.tsx               # Diff 高亮渲染（三栏对比）
│   └── MergeEditor.tsx              # 手动合并编辑器
├── social/
│   ├── CommentSection.tsx           # 评论区
│   ├── DiscussionBoard.tsx          # 讨论区（贴吧式）
│   ├── ReactionBar.tsx              # 点赞/点踩/举报
│   ├── FollowButton.tsx             # 关注/取消关注（用户 + IP）
│   ├── NotificationDropdown.tsx     # Header 消息下拉菜单
│   ├── NotificationList.tsx         # 通知列表（按频道分 Tab）
│   ├── ConversationList.tsx         # 私信对话列表
│   └── ChatWindow.tsx               # 私信对话窗口
├── agent/
│   ├── AgentWorkspace.tsx           # /agent 全页会话工作台
│   ├── AgentCitationList.tsx        # 服务端校验后的站内引用列表
│   └── AgentToolStatus.tsx          # 不暴露内部推理的工具状态摘要
├── search/
│   └── GlobalSearchInput.tsx        # 全站关键词搜索；不提供 Agent 模式
├── judge/
│   ├── ExamQuestion.tsx             # 判官考题
│   └── ReviewCard.tsx               # 众裁内容卡
├── rehab/
│   ├── CourseCard.tsx               # 课程卡片
│   ├── CourseContent.tsx            # 课程教学内容渲染（Markdown）
│   └── ReputationDetail.tsx         # 信誉分详情面板（加减分规则说明）
└── ui/                              # 通用 UI 组件
    ├── Button.tsx
    ├── Modal.tsx
    ├── Toast.tsx
    ├── Skeleton.tsx
    └── Badge.tsx
```

### 3.2 Go 后端

#### 项目结构

```
backend/
├── cmd/server/main.go               # 入口
├── config/config.go                 # 配置加载（yaml + 环境变量）
├── internal/
│   ├── middleware/
│   │   ├── auth.go                  # JWT 验证中间件
│   │   ├── ratelimit.go             # 限流（Redis 令牌桶）
│   │   ├── logger.go                # 请求日志（JSON、脱敏、HMAC IP）
│   │   ├── metrics.go               # 低基数请求指标
│   │   ├── tracing.go               # OTel provider、OTLP export、W3C context
│   │   ├── panic_recovery.go        # panic 恢复（class 级日志 + 计数）
│   │   └── cors.go
│   ├── observability/               # 可观测性（生产基线）
│   │   ├── logger.go                # JSON slog + client_ip HMAC 哈希/轮换
│   │   ├── metrics.go               # Prometheus 指标与 DB/Redis 收集器
│   │   ├── server.go                # 内部 :9091（/metrics /healthz /readyz）
│   │   ├── tracing.go               # OTel provider、OTLP export、采样与 trace ID
│   │   └── logger.go                # 迁移/备份 textfile 指标写入
│   ├── handler/                     # HTTP 处理器（按模块）
│   │   ├── auth.go
│   │   ├── user.go
│   │   ├── ip.go
│   │   ├── content.go
│   │   ├── version.go
│   │   ├── social.go
│   │   ├── review.go
│   │   ├── judge.go
│   │   ├── agent.go
│   │   ├── follow.go
│   │   ├── appeal.go
│   │   ├── notification.go
│   │   ├── message.go
│   │   ├── rehab.go
│   │   ├── tag.go
│   │   ├── category.go
│   │   └── admin.go
│   ├── service/                     # 业务逻辑层
│   │   ├── auth_service.go
│   │   ├── ip_service.go
│   │   ├── content_service.go
│   │   ├── version_service.go       # 版本管理引擎
│   │   ├── review_service.go        # 审核链路
│   │   ├── judge_service.go         # 赛博判官
│   │   ├── reputation_service.go    # 信誉分计算
│   │   ├── follow_service.go        # 关注系统
│   │   ├── notification_service.go  # 通知系统
│   │   ├── message_service.go       # 私信系统
│   │   ├── rehab_service.go         # 素质建设课程
│   │   ├── social_service.go        # 评论/讨论/点赞
│   │   ├── appeal_service.go        # 申诉系统
│   │   ├── tag_service.go           # 标签体系
│   │   ├── category_service.go      # 分类管理
│   │   ├── agent_service.go         # 网页端 Agent
│   │   ├── llm_config_service.go    # LLM 配置管理
│   │   └── oss_service.go           # OSS 操作
│   ├── repository/                  # 数据访问层（GORM）
│   │   ├── user_repo.go
│   │   ├── ip_repo.go
│   │   ├── content_repo.go
│   │   ├── version_repo.go
│   │   ├── social_repo.go
│   │   ├── review_repo.go
│   │   ├── follow_repo.go
│   │   ├── notification_repo.go
│   │   ├── message_repo.go
│   │   ├── rehab_repo.go
│   │   ├── appeal_repo.go
│   │   ├── judge_repo.go
│   │   ├── tag_repo.go
│   │   ├── category_repo.go
│   │   ├── embedding_repo.go
│   │   └── llm_config_repo.go
│   ├── model/                       # GORM 模型（对应数据库表）
│   ├── container/                    # 依赖注入容器（统一管理服务实例）
│   ├── worker/                       # 后台异步任务 Worker
│   ├── testutil/                     # 测试工具（Mock DB/Redis/OSS）
│   └── pkg/
│       ├── aliyun/                  # 阿里云 SDK 封装（OSS、内容安全）
│       ├── captcha/                 # 阿里云验证码 2.0 SDK 封装
│       ├── database/                # 数据库连接管理（GORM + dbresolver）
│       ├── diffengine/              # diff-match-patch 封装
│       ├── jwt/                     # JWT 工具
│       ├── llm/                     # LLM Provider 抽象层（Qwen/OpenAI 兼容）
│       ├── logger/                  # 结构化日志（slog JSON）
│       ├── mail/                    # SMTP 邮件发送
│       ├── queue/                   # 异步任务队列
│       ├── recovery/                # Goroutine panic recovery 包装
│       ├── redis/                   # Redis 客户端管理
│       ├── response/                # 统一错误响应信封 + 脱敏
│       └── scheduler/               # 定时任务调度
├── migrations/                      # SQL 迁移文件
├── config.yaml                      # 默认配置
└── docker-compose.yml
```

#### 完整 API 路由清单

<!-- AUTO-GENERATED: §3.2 API 路由清单 | source: backend/internal/router/routes.go | DO NOT EDIT MANUALLY -->

| 方法 | 路径 | 处理器 |
|------|------|--------|
| `DELETE` | `/api/v1/admin/categories/:id` | catHandler.AdminDeleteCategory |
| `DELETE` | `/api/v1/admin/llm-configs/:id` | adminHandler.DeleteLLMConfig |
| `DELETE` | `/api/v1/agent/conversations/:id` | agentHandler.DeleteConversation |
| `DELETE` | `/api/v1/collections/:id` | collectionHandler.DeleteCollection |
| `DELETE` | `/api/v1/collections/:id/items/:itemId` | collectionHandler.RemoveItem |
| `DELETE` | `/api/v1/contents/:id` | contentHandler.DeleteContent |
| `DELETE` | `/api/v1/dashboard/contributors/:userId/block` | prHandler.UnblockContributor |
| `DELETE` | `/api/v1/ips/:id/follow` | followHandler.UnfollowIP |
| `DELETE` | `/api/v1/messages/:id` | msgHandler.DeleteMessage |
| `DELETE` | `/api/v1/messages/conversations/:id` | msgHandler.LeaveConversation |
| `DELETE` | `/api/v1/series/:id` | seriesHandler.DeleteSeries |
| `DELETE` | `/api/v1/series/:id/items/:itemId` | seriesHandler.RemoveItem |
| `DELETE` | `/api/v1/social/comments/:id` | socialHandler.DeleteComment |
| `DELETE` | `/api/v1/users/:id/follow` | followHandler.UnfollowUser |
| `DELETE` | `/api/v1/users/me` | userHandler.DeleteAccount |
| `DELETE` | `/api/v1/users/me/history` | histHandler.ClearHistory |
| `DELETE` | `/api/v1/users/me/saved-searches/:id` | tagHandler.DeleteSavedSearch |
| `DELETE` | `/api/v1/users/me/tag-groups/:id` | tagHandler.DeleteTagGroup |
| `GET` | `/api/v1/admin/appeals` | adminHandler.ListAppeals |
| `GET` | `/api/v1/admin/archive-scan-jobs/:id` | adminArchiveScanHandler.GetJob |
| `GET` | `/api/v1/admin/audit-logs` | adminAuditHandler.ListAuditLogs |
| `GET` | `/api/v1/admin/config` | adminHandler.GetConfig |
| `GET` | `/api/v1/admin/contents` | adminHandler.ListUnderReviewContents |
| `GET` | `/api/v1/admin/contents/trash` | adminHandler.ListTrashedContents |
| `GET` | `/api/v1/admin/feedback` | adminFeedbackHandler.ListFeedback |
| `GET` | `/api/v1/admin/feedback/:id` | adminFeedbackHandler.GetFeedback |
| `GET` | `/api/v1/admin/ips` | adminHandler.ListPendingIPs |
| `GET` | `/api/v1/admin/llm-configs` | adminHandler.ListLLMConfigs |
| `GET` | `/api/v1/admin/queue/dlq` | adminHandler.GetDLQEntries |
| `GET` | `/api/v1/admin/queue/stats` | adminHandler.GetQueueStats |
| `GET` | `/api/v1/admin/reports` | adminHandler.ListReports |
| `GET` | `/api/v1/admin/reports/stats` | adminHandler.GetReportStats |
| `GET` | `/api/v1/admin/users` | adminHandler.ListUsers |
| `GET` | `/api/v1/agent/conversations` | agentHandler.ListConversations |
| `GET` | `/api/v1/agent/conversations/:id` | agentHandler.GetConversationMessages |
| `GET` | `/api/v1/agent/usage-guide/:id` | agentHandler.UsageGuide |
| `GET` | `/api/v1/appeals/me` | appealHandler.GetMyAppeals |
| `GET` | `/api/v1/auth/csrf` | authHandler.CSRFToken |
| `GET` | `/api/v1/auth/me` | authHandler.Me |
| `GET` | `/api/v1/categories` | catHandler.ListCategories |
| `GET` | `/api/v1/collections` | collectionHandler.ListCollections |
| `GET` | `/api/v1/collections/:id` | collectionHandler.GetCollection |
| `GET` | `/api/v1/config/public` | publicConfigHandler.GetPublicConfig |
| `GET` | `/api/v1/contents` | contentHandler.ListContents |
| `GET` | `/api/v1/contents/:id` | contentHandler.GetContent |
| `GET` | `/api/v1/contents/:id/download` | contentHandler.DownloadContent |
| `GET` | `/api/v1/contents/:id/prs` | prHandler.ListPRs |
| `GET` | `/api/v1/contents/:id/related-fanworks` | contentHandler.ListRelatedFanworks |
| `GET` | `/api/v1/contents/:id/versions` | handler.NewVersionHandler(...).ListVersions |
| `GET` | `/api/v1/contents/search` | searchHandler.SearchContents |
| `GET` | `/api/v1/dashboard/tag-suggestions` | tagHandler.ListTagSuggestions |
| `GET` | `/api/v1/discussions/:id` | discHandler.GetDiscussion |
| `GET` | `/api/v1/feedback/:id` | feedbackHandler.GetTicket |
| `GET` | `/api/v1/feedback/me` | feedbackHandler.ListMyTickets |
| `GET` | `/api/v1/ips` | ipHandler.ListIPs |
| `GET` | `/api/v1/ips/:id` | ipHandler.GetIP |
| `GET` | `/api/v1/ips/:id/contents` | ipHandler.GetIPContents |
| `GET` | `/api/v1/ips/:id/discussions` | discHandler.ListDiscussions |
| `GET` | `/api/v1/ips/:id/discussions/search` | discHandler.SearchDiscussions |
| `GET` | `/api/v1/ips/:id/proposals` | proposalHandler.ListProposals |
| `GET` | `/api/v1/ips/:id/proposals/:proposalId` | proposalHandler.GetProposal |
| `GET` | `/api/v1/ips/:id/versions` | proposalHandler.ListVersions |
| `GET` | `/api/v1/ips/stats/category_counts` | ipStatsHandler.GetCategoryCounts |
| `GET` | `/api/v1/judge/cases/:id/verdict` | judgeHandler.GetVerdictDetail |
| `GET` | `/api/v1/judge/exam/:category` | judgeHandler.GetExam |
| `GET` | `/api/v1/judge/queue` | judgeHandler.GetQueue |
| `GET` | `/api/v1/messages` | msgHandler.ListConversations |
| `GET` | `/api/v1/messages/:id` | msgHandler.ListMessages |
| `GET` | `/api/v1/notifications` | notifHandler.ListNotifications |
| `GET` | `/api/v1/notifications/unread-count` | notifHandler.UnreadCount |
| `GET` | `/api/v1/pr/:id` | prHandler.GetPR |
| `GET` | `/api/v1/rehab/courses` | rehabHandler.ListCourses |
| `GET` | `/api/v1/rehab/courses/:id` | rehabHandler.GetCourse |
| `GET` | `/api/v1/rehab/my-progress` | rehabHandler.GetMyProgress |
| `GET` | `/api/v1/reputation-logs/me` | repHandler.GetMyReputationLogs |
| `GET` | `/api/v1/search/suggestions` | searchHandler.Suggestions |
| `GET` | `/api/v1/search/trending` | searchHandler.Trending |
| `GET` | `/api/v1/series` | seriesHandler.ListSeries |
| `GET` | `/api/v1/series/:id` | seriesHandler.GetSeries |
| `GET` | `/api/v1/series/candidates` | seriesHandler.ListCandidates |
| `GET` | `/api/v1/social/comments` | socialHandler.ListComments |
| `GET` | `/api/v1/social/discussions` | socialHandler.ListDiscussions |
| `GET` | `/api/v1/social/discussions/:id` | socialHandler.GetDiscussion |
| `GET` | `/api/v1/social/reactions` | socialHandler.ListReactions |
| `GET` | `/api/v1/stats/summary` | statsHandler.GetSummary |
| `GET` | `/api/v1/tags/faceted` | tagHandler.GetFacetedTags |
| `GET` | `/api/v1/tags/search` | tagHandler.SearchTags |
| `GET` | `/api/v1/users/:id` | userHandler.GetUser |
| `GET` | `/api/v1/users/:id/contents` | userHandler.GetUserContents |
| `GET` | `/api/v1/users/:id/discussions` | discHandler.ListByUser |
| `GET` | `/api/v1/users/:id/followers` | followHandler.GetFollowers |
| `GET` | `/api/v1/users/:id/following` | followHandler.GetFollowing |
| `GET` | `/api/v1/users/:id/reputation` | userHandler.GetReputation |
| `GET` | `/api/v1/users/me/contents` | userHandler.GetMyContents |
| `GET` | `/api/v1/users/me/followers/stats` | followHandler.GetFollowerStats |
| `GET` | `/api/v1/users/me/history` | histHandler.GetHistory |
| `GET` | `/api/v1/users/me/ip-visits` | ipVisitHistoryHandler.ListRecent |
| `GET` | `/api/v1/users/me/saved-searches` | tagHandler.ListSavedSearches |
| `GET` | `/api/v1/users/me/tag-groups` | tagHandler.ListTagGroups |
| `GET` | `/api/v1/users/search` | searchHandler.SearchUsers |
| `GET` | `/api/v1/versions/:id` | versionHandler.GetVersion |
| `PATCH` | `/api/v1/admin/categories/:id` | catHandler.AdminUpdateCategory |
| `PATCH` | `/api/v1/admin/config` | adminHandler.PatchConfig |
| `PATCH` | `/api/v1/admin/contents/:id/restore` | adminHandler.RestoreContent |
| `PATCH` | `/api/v1/admin/feedback/:id` | adminFeedbackHandler.PatchFeedback |
| `PATCH` | `/api/v1/admin/llm-configs/:id` | adminHandler.UpdateLLMConfig |
| `PATCH` | `/api/v1/admin/reports/:id` | adminHandler.ResolveReport |
| `PATCH` | `/api/v1/contents/:id` | contentHandler.UpdateContent |
| `PATCH` | `/api/v1/dashboard/tag-suggestions/:id` | tagHandler.UpdateTagSuggestion |
| `PATCH` | `/api/v1/discussions/:id/pin` | discHandler.PinDiscussion |
| `PATCH` | `/api/v1/notifications/:id/read` | notifHandler.MarkRead |
| `PATCH` | `/api/v1/social/comments/:id` | socialHandler.EditComment |
| `PATCH` | `/api/v1/users/:id` | userHandler.UpdateUser |
| `PATCH` | `/api/v1/users/me/password` | userHandler.ChangePassword |
| `PATCH` | `/api/v1/users/me/support-info` | userHandler.UpdateSupportInfo |
| `PATCH` | `/api/v1/users/me/tag-groups/:id` | tagHandler.UpdateTagGroup |
| `POST` | `/api/v1/admin/appeals/:id` | adminHandler.ResolveAppeal |
| `POST` | `/api/v1/admin/archive-scan-jobs/:id/manual-review` | adminArchiveScanHandler.StartManualReview |
| `POST` | `/api/v1/admin/archive-scan-jobs/:id/resolve` | adminArchiveScanHandler.ResolveManualReview |
| `POST` | `/api/v1/admin/archive-scan-jobs/:id/retry` | adminArchiveScanHandler.Retry |
| `POST` | `/api/v1/admin/categories` | catHandler.AdminCreateCategory |
| `POST` | `/api/v1/admin/contents/:id/ban` | adminHandler.BanContent |
| `POST` | `/api/v1/admin/feedback/:id/replies` | adminFeedbackHandler.ReplyFeedback |
| `POST` | `/api/v1/admin/ips/:id/approve` | adminHandler.ApproveIP |
| `POST` | `/api/v1/admin/ips/:id/reject` | adminHandler.RejectIP |
| `POST` | `/api/v1/admin/judge/questions` | judgeHandler.CreateQuestions |
| `POST` | `/api/v1/admin/llm-configs` | adminHandler.CreateLLMConfig |
| `POST` | `/api/v1/admin/llm-configs/:id/activate` | adminHandler.ActivateLLMConfig |
| `POST` | `/api/v1/admin/llm-configs/:id/test` | adminHandler.TestLLMConfig |
| `POST` | `/api/v1/admin/notifications/broadcast` | adminHandler.BroadcastNotification |
| `POST` | `/api/v1/admin/queue/dlq/:id/replay` | adminHandler.ReplayDLQEntry |
| `POST` | `/api/v1/admin/rag/rebuild` | adminRAGHandler.Rebuild |
| `POST` | `/api/v1/admin/users/:id/ban` | adminHandler.BanUser |
| `POST` | `/api/v1/admin/users/:id/unban` | adminHandler.UnbanUser |
| `POST` | `/api/v1/agent/chat/stream` | agentHandler.ChatStream |
| `POST` | `/api/v1/agent/compliance-check` | agentHandler.ComplianceCheck |
| `POST` | `/api/v1/agent/search` | agentHandler.NLSearch |
| `POST` | `/api/v1/agent/upload-assist` | agentHandler.UploadAssist |
| `POST` | `/api/v1/appeals` | appealHandler.SubmitAppeal |
| `POST` | `/api/v1/auth/forgot-password` | authHandler.ForgotPassword |
| `POST` | `/api/v1/auth/login` | authHandler.Login |
| `POST` | `/api/v1/auth/logout` | authHandler.Logout |
| `POST` | `/api/v1/auth/refresh` | authHandler.Refresh |
| `POST` | `/api/v1/auth/register` | authHandler.Register |
| `POST` | `/api/v1/auth/resend-verification` | authHandler.ResendVerification |
| `POST` | `/api/v1/auth/reset-password` | authHandler.ResetPassword |
| `POST` | `/api/v1/auth/verify-email` | authHandler.VerifyEmail |
| `POST` | `/api/v1/captcha/verify` | captchaHandler.Verify |
| `POST` | `/api/v1/collab-invites/:id/accept` | collabInviteHandler.AcceptInvite |
| `POST` | `/api/v1/collab-invites/:id/decline` | collabInviteHandler.DeclineInvite |
| `POST` | `/api/v1/collections` | collectionHandler.CreateCollection |
| `POST` | `/api/v1/collections/:id/items` | collectionHandler.AddItem |
| `POST` | `/api/v1/contents` | contentHandler.CreateContent |
| `POST` | `/api/v1/contents/:id/collab-invites` | collabInviteHandler.SendInvite |
| `POST` | `/api/v1/contents/:id/report` | socialHandler.ReportContent |
| `POST` | `/api/v1/contents/:id/tags/suggest` | tagHandler.SuggestTag |
| `POST` | `/api/v1/contents/oss-token` | contentHandler.GenerateOSSToken |
| `POST` | `/api/v1/dashboard/contributors/:userId/block` | prHandler.BlockContributor |
| `POST` | `/api/v1/deploy-grants` | inline handler |
| `POST` | `/api/v1/discussions/:id/comments` | discHandler.ReplyToDiscussion |
| `POST` | `/api/v1/feedback` | feedbackHandler.SubmitTicket |
| `POST` | `/api/v1/feedback/attachments/presign` | feedbackHandler.PresignUpload |
| `POST` | `/api/v1/internal/ai-callback` | internalHandler.AICallback |
| `POST` | `/api/v1/ips` | ipHandler.CreateIP |
| `POST` | `/api/v1/ips/:id/discussions` | discHandler.CreateDiscussion |
| `POST` | `/api/v1/ips/:id/follow` | followHandler.FollowIP |
| `POST` | `/api/v1/ips/:id/proposals` | proposalHandler.CreateProposal |
| `POST` | `/api/v1/ips/:id/proposals/:proposalId/vote` | proposalHandler.SubmitVote |
| `POST` | `/api/v1/judge/exam/submit` | judgeHandler.SubmitExam |
| `POST` | `/api/v1/judge/reasons/:id/vote` | judgeHandler.VoteReason |
| `POST` | `/api/v1/judge/vote` | judgeHandler.SubmitVote |
| `POST` | `/api/v1/messages` | msgHandler.SendMessage |
| `POST` | `/api/v1/notifications/read-all` | notifHandler.MarkAllRead |
| `POST` | `/api/v1/pr` | prHandler.SubmitPR |
| `POST` | `/api/v1/pr/:id/accept` | prHandler.AcceptPR |
| `POST` | `/api/v1/pr/:id/merge` | prHandler.ManualMerge |
| `POST` | `/api/v1/pr/:id/reject` | prHandler.RejectPR |
| `POST` | `/api/v1/rehab/courses/:id/complete` | rehabHandler.CompleteCourse |
| `POST` | `/api/v1/rehab/courses/:id/start` | rehabHandler.StartCourse |
| `POST` | `/api/v1/series` | seriesHandler.CreateSeries |
| `POST` | `/api/v1/series/:id/items` | seriesHandler.AddItem |
| `POST` | `/api/v1/social/comments` | socialHandler.PostComment |
| `POST` | `/api/v1/social/comments/:id/report` | socialHandler.ReportComment |
| `POST` | `/api/v1/social/discussions` | socialHandler.PostDiscussion |
| `POST` | `/api/v1/social/reactions` | socialHandler.React |
| `POST` | `/api/v1/users/:id/follow` | followHandler.FollowUser |
| `POST` | `/api/v1/users/me/history` | histHandler.RecordView |
| `POST` | `/api/v1/users/me/ip-visits/merge` | ipVisitHistoryHandler.MergeVisits |
| `POST` | `/api/v1/users/me/saved-searches` | tagHandler.CreateSavedSearch |
| `POST` | `/api/v1/users/me/tag-groups` | tagHandler.CreateTagGroup |
| `PUT` | `/api/v1/admin/categories/reorder` | catHandler.AdminReorderCategories |
| `PUT` | `/api/v1/collections/:id` | collectionHandler.UpdateCollection |
| `PUT` | `/api/v1/collections/:id/items/:itemId` | collectionHandler.UpdateItem |
| `PUT` | `/api/v1/series/:id` | seriesHandler.UpdateSeries |
| `PUT` | `/api/v1/series/:id/items/reorder` | seriesHandler.ReorderItems |
| `PUT` | `/api/v1/users/me/ip-visits/:ipId` | ipVisitHistoryHandler.RecordVisit |

<!-- END AUTO-GENERATED: §3.2 -->

### 3.3 Tauri PC 客户端

#### 架构

```
tauri-client/
├── src-tauri/
│   ├── src/
│   │   ├── main.rs                  # Tauri 入口
│   │   ├── commands/
│   │   │   ├── file_ops.rs          # 白名单文件操作
│   │   │   ├── env_detect.rs        # 游戏/应用路径检测
│   │   │   └── security.rs          # 签名验证
│   │   └── url_scheme.rs            # URL Scheme 处理
│   └── tauri.conf.json              # 权限配置（仅允许白名单目录）
└── src/                             # WebView 前端（复用 Web 组件）
    └── App.tsx                      # 部署流程 UI
```

#### Agent 白名单动作（永久固定，不可扩展）

| 动作 ID | 说明 | 限制 |
|---------|------|------|
| `download_file` | 从 OSS 下载文件到指定目录 | 仅白名单目录 |
| `extract_archive` | 解压 zip/tar 到指定目录 | 仅白名单目录 |
| `move_file` | 移动文件 | 仅白名单目录内 |
| `create_dir` | 创建目录 | 仅白名单目录内 |
| `read_config` | 读取配置文件 | 仅指定扩展名 |
| `write_config` | 写入配置文件 | 仅指定扩展名，大小限制 |
| `backup_file` | 备份原文件（自动，不可跳过） | `.omnicraft_backup/` |

#### URL Scheme 协议

```
omnicraft://deploy?content_id=xxx&token=yyy

计划升级（D-03 后）：omnicraft://deploy?grant=<opaque-token>
```

流程：Web 点击「一键部署」→ 浏览器唤醒 Tauri → Rust 层解析 deploy args 提取 content_id + token → WebView 根据 content_id 调用 Go API 获取签名动作脚本 → 执行白名单动作 → 桌面通知结果

### 3.4 AI/审核子系统

#### 三级审核链路

```
内容上传完成
    │
    ▼
[第一级] 阿里云内容安全 异步扫描
    ├── 高危（黄赌毒/木马）→ 直接下架 + 永久封禁用户 → 结束
    ├── 违规 → 下架 + 扣信誉分 → 可申诉
    └── 争议 → 进入 [第二级]
         │
         ▼
    [第二级] 赛博判官众裁
         ├── 分发给对应类型判官（文章/图片/提示词/评论）
         ├── 「不违规」比例 ≥ 60% → 恢复展示
         └── 「不违规」比例 < 60% → 不予展示（管理员可手动恢复）→ 可申诉
              │
              ▼（用户申诉）
         [第三级] 管理员终审
              ├── 恢复上架
              └── 维持下架
```

#### 阿里云内容安全接入

- 接口：`imageModeration` / `textModeration` / `videoAsyncScan`
- 调用方式：Go 后端异步调用，上传完成后投入消息队列（初期用 Go channel + goroutine，后期替换为消息队列）
- 回调：阿里云回调 Go API，更新内容审核状态

---

---

## 6. 安全与合规

### 6.1 认证与授权

- **JWT**：Access Token 有效期 2 小时，Refresh Token 7 天，Redis 存 Refresh Token 支持强制失效
- **角色权限**：`user` / `creator` / `admin`，中间件层拦截；判官权限通过 `judge_qualifications` 表判定
- **接口限流**：Redis 令牌桶，注册/登录接口 5 次/分钟，内容发布 10 次/小时

### 6.2 OSS 安全

- 所有文件上传通过预签名 URL（STS 临时凭证），前端直传 OSS，后端不经手文件流
- OSS Bucket 设为私有，访问通过带时效的签名 URL（有效期 1 小时）
- 生命周期策略：临时 PR 文件 30 天自动清理，已删除内容文件 90 天清理

### 6.3 Agent 安全

- **当前真相（功能关闭）**：Tauri 原型仍使用 HMAC-SHA256，且 WebView invoke handler 仍直接暴露文件操作命令；因此 `features.desktop_deploy_enabled` 必须保持 `false`，不得宣称 Desktop Agent/一键部署已达到发布条件。
- **发布目标（D-02～D-05）**：Go 仅用 Ed25519 私钥签发 canonical script bytes；Tauri 只嵌入公钥，先验签再严格解析，通过一次性内存 handle 调用单一受控执行入口。
- **文件边界**：路径使用固定 allowlisted root 下的逻辑相对路径；WebView 不得直接调用 download/move/extract/read/write 原语；写配置和移动必须经过原生二次确认并自动备份。
- **发布门禁**：D-02～D-05 与 R-02 未全部通过前，仓库和生产配置均不得开启 Desktop Agent 本地执行能力。

### 6.4 内容安全

- 上传完成 → Go 异步调用阿里云内容安全
- 视频异步扫描（回调模式）
- 文本/图片同步扫描（< 200ms），扫描失败默认 pending，人工补审

---

---

## 7. 可观测性（日志、指标、就绪）

日志使用结构化 JSON（稳定字段 `time/level/msg/service/environment/version/trace_id/request_id/route/method/status/duration_ms/client_ip/error_class`）。`trace_id` 是 OTel 128-bit trace，`request_id` 仍是独立的 8-byte hex 请求关联 ID；SSE `trace_id` 沿用当前 OTel context。`client_ip` 只保存 `LOG_IP_HASH_SECRET` 的 HMAC-SHA256 前 128 bit（32 位小写十六进制）+ 非敏感 `client_ip_key_id`；日志永不出现原始 IP、token、cookie、授权头、验证码票据、签名 URL 查询串或消息正文。前一把哈希密钥只在显式轮换窗口内可用（`observability.ip_key_rotation`）。release 模式缺少哈希密钥时 fail-closed 拒绝启动。

指标低基数：请求量/错误率/延迟（route 模板 + method + status_class 标签）、panic、DB pool、Redis pool、队列积压、worker 失败、迁移状态，以及 OSS/Green/CAPTCHA/SMTP/LLM 外部依赖按依赖名+结果聚合的成功/失败/延迟。`/healthz` 仅进程存活；`/readyz` 依赖感知（DB+Redis 超时探测）且不泄露连接细节；`/metrics` 只在内网 `:9091` 暴露。

Tracing 使用 head-based ratio sampling，经 OTLP/gRPC 只发往 `observability.tracing.endpoint`；full-infra 由 OTel Collector 转发到 Jaeger，Collector 离线只丢弃遥测并告警，不改变业务路径。HTTP、Redis Streams、GORM 和 LLM span 共享 W3C context；GenAI span 只记录 provider/model、temperature 和 token usage，不记录 prompt 或 embedding 正文。

参考栈：应用 JSON stdout → Docker `json-file` 轮转（10MB×5）→ Grafana Alloy（只读日志挂载，无 Docker 控制权）→ Loki（命名卷、30 天 retention）；Prometheus 内网抓取、30 天+磁盘上限双 retention；操作员经 `loki-gate`（127.0.0.1 绑定、token 认证、查询审计落盘）或 SSH 隧道访问。warning/error 审计摘要每日加密归档（异地目标凭证为 Ops-08 输入）。迁移器、`backup-db.sh`、`recovery-drill.sh` 向 `METRICS_TEXTFILE_DIR` 输出 success/failure/last-success 文本指标。

## 8. 环境变量与外部依赖清单

### 8.1 环境变量（`.env` / Docker Compose）

```env
# 数据库
DB_DSN=postgres://user:pass@pgbouncer:5432/omnicraft
DB_READ_DSN=                      # 留空则全走主库，P1 填从库 DSN

# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=

# 可观测性（生产必填）
LOG_IP_HASH_SECRET=               # client_ip 哈希密钥（release 必填）
LOG_IP_KEY_ID=key-2026-08         # 非敏感密钥标识
METRICS_TEXTFILE_DIR=             # 迁移/备份文本指标目录（node_exporter textfile）

# JWT
JWT_SECRET=<随机 64 字节>
JWT_ACCESS_EXPIRE=7200            # 秒
JWT_REFRESH_EXPIRE=604800         # 秒

# 阿里云 OSS
OSS_ACCESS_KEY_ID=
OSS_ACCESS_KEY_SECRET=
OSS_BUCKET_NAME=omnicraft-prod
OSS_ENDPOINT=https://oss-cn-hangzhou.aliyuncs.com
OSS_DOMAIN=                        # OSS Bucket 域名（或 CDN 加速域名）

# 阿里云内容安全
GREEN_ACCESS_KEY_ID=
GREEN_ACCESS_KEY_SECRET=
GREEN_REGION=cn-shanghai
GREEN_CALLBACK_URL=

# Desktop deploy 签名（D-03 完成前保持功能关闭）
AGENT_HMAC_SECRET=                # legacy disabled prototype only；生产不得配置为可用链路
DEPLOY_ED25519_PRIVATE_KEY_B64=   # 后端签名私钥，D-03/R-02 发布输入
DEPLOY_ED25519_KEY_ID=            # 当前签名 key id
# Tauri release build 另需 DEPLOY_PUBLIC_KEY_ID / DEPLOY_PUBLIC_KEY_B64，见 Beta D-03/D-04 计划

# 应用
APP_ENV=production                # development | production
APP_PORT=8080
FRONTEND_URL=https://app.leeppp.online     # 当前生产环境实际域名
```

### 8.2 Docker Compose 部署档

服务器部署以 `docs/deploy/docker-compose.single-server.yml` 和
`docs/deploy/single-server-beta-runbook.md` 为权威；根目录
`docker-compose.yml` 主要用于本地集成，不能按服务数量判断两者是否等价。

| 分组 | 服务 | 生命周期 | 说明 |
|------|------|----------|------|
| Web 核心 | `frontend`、`backend`、`postgres`、`pgbouncer`、`redis`、`nginx` | 常驻 | `nginx` 是唯一公网入口；其余服务仅在 Compose 内网通信 |
| 发布门 | `migrate` | 每次发布一次性运行 | 迁移成功后 backend 才能启动；完成后退出，不计入常驻内存 |
| 3.6 GiB 面试观测 | `prometheus` | 常驻 | 仅抓取 backend 的低基数应用指标，使用精简 scrape 配置，不加载依赖完整观测栈的 targets/rules |
| 完整生产观测 | `alertmanager`、`postgres-exporter`、`redis-exporter`、`cadvisor`、`blackbox`、`node-exporter`、`loki`、`alloy`、`loki-gate` | 资源充足或迁往独立监控节点后常驻 | 提供主机/依赖/容器指标、外部探测、告警投递和集中日志查询 |

3.6 GiB 面试服务器采用“6 个 Web 核心常驻服务 + 一次性 `migrate` +
精简 `prometheus`”。这不是完整生产观测档：结构化 JSON 日志、Docker
日志轮转、`/healthz`、内网 `/readyz` 和 `/metrics`、备份/恢复脚本仍须
保留，但完整日志链（Alloy → Loki → loki-gate）和告警链在该主机暂缓。

当前单服务器模板的常驻内存上限合计约 5.7 GiB；仅 Web 核心与现有
Prometheus 上限合计也约 3.9 GiB。`deploy.resources.limits` 是上限而非
预留，但 3.6 GiB 主机仍必须基于 `docker stats` 的实测峰值重新设定精简
档上限，并至少给 Ubuntu、Docker 和页缓存保留约 1 GiB。不要在该主机上
并行构建前后端镜像；优先使用 CI 构建的不可变镜像。

完整 `ops/observability/prometheus.yml` 会抓取 Alertmanager、数据库/Redis
exporter、cAdvisor、Blackbox 和 node-exporter。精简档不能仅停掉这些容器后
继续宣称监控全绿；服务器切换前必须提供并校验只抓取 `backend:9091` 的
独立 Prometheus 配置。完整生产发布仍使用 17 服务模板，不受面试精简档
影响。

---

---

## 9. 扩容路线图

### 9.1 数据库扩容方案

```
阶段 P0（< 1,000 人）
  └── PostgreSQL 单主 + PgBouncer（Docker Compose）

阶段 P1（1,000–10,000 人）
  ├── 增加 PostgreSQL 读从库（流复制）
  ├── Go 环境变量填写 DB_READ_DSN，GORM dbresolver 自动分流
  └── Redis 可升级为 Sentinel 模式

阶段 P2（> 10,000 人）
  ├── 迁移到阿里云 RDS PostgreSQL（高可用版）
  │     迁移步骤：
  │     1. 开启 RDS 实例，配置主从
  │     2. pg_dump 全量导出 → psql 导入 RDS
  │     3. 开启逻辑复制追追量数据（停机窗口 < 30 分钟）
  │     4. 修改 DB_DSN 指向 RDS，重启服务
  ├── 迁移到 K8s（k8s/ 目录预留 Deployment 配置）
  └── CDN 加速 OSS（填写 OSS_DOMAIN 环境变量即可）
```

### 9.2 K8s 预留目录结构

```
k8s/
├── namespace.yaml
├── frontend-deployment.yaml
├── backend-deployment.yaml
├── redis-statefulset.yaml
├── ingress.yaml               # 替代 Nginx
├── hpa.yaml                   # 自动水平扩缩容
└── configmap.yaml             # 对应 config.yaml
```
