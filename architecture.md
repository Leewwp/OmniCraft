# OmniCraft（万象工坊）技术架构设计文档

**版本**：1.0 | **对应 PRD**：V0.3 正式版 | **技术栈**：Next.js + Go + PostgreSQL + Tauri

---

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
│   │   ├── logger.go                # 请求日志
│   │   └── cors.go
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

<!-- AUTO-GENERATED: §3.2 API 路由清单 | source: backend/internal/handler/routes.go | DO NOT EDIT MANUALLY -->

| 方法 | 路径 | 处理器 |
|------|------|--------|
| `DELETE` | `/api/v1/admin/categories/:id` | catHandler.AdminDeleteCategory |
| `DELETE` | `/api/v1/admin/llm-configs/:id` | adminHandler.DeleteLLMConfig |
| `DELETE` | `/api/v1/contents/:id` | contentHandler.DeleteContent |
| `DELETE` | `/api/v1/dashboard/contributors/:userId/block` | prHandler.UnblockContributor |
| `DELETE` | `/api/v1/favorites/:contentId` | favHandler.RemoveFavorite |
| `DELETE` | `/api/v1/ips/:id/follow` | followHandler.UnfollowIP |
| `DELETE` | `/api/v1/messages/:id` | msgHandler.DeleteMessage |
| `DELETE` | `/api/v1/messages/conversations/:id` | msgHandler.LeaveConversation |
| `DELETE` | `/api/v1/social/comments/:id` | socialHandler.DeleteComment |
| `DELETE` | `/api/v1/users/:id/follow` | followHandler.UnfollowUser |
| `DELETE` | `/api/v1/users/me` | userHandler.DeleteAccount |
| `DELETE` | `/api/v1/users/me/history` | histHandler.ClearHistory |
| `DELETE` | `/api/v1/users/me/saved-searches/:id` | tagHandler.DeleteSavedSearch |
| `DELETE` | `/api/v1/users/me/tag-groups/:id` | tagHandler.DeleteTagGroup |
| `GET` | `/api/v1/admin/appeals` | adminHandler.ListAppeals |
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
| `GET` | `/api/v1/config/public` | publicConfigHandler.GetPublicConfig |
| `GET` | `/api/v1/contents` | contentHandler.ListContents |
| `GET` | `/api/v1/contents/:id` | contentHandler.GetContent |
| `GET` | `/api/v1/contents/:id/download` | contentHandler.DownloadContent |
| `GET` | `/api/v1/contents/:id/prs` | NewPRHandler(...).ListPRs |
| `GET` | `/api/v1/contents/:id/related-fanworks` | contentHandler.ListRelatedFanworks |
| `GET` | `/api/v1/contents/:id/versions` | NewVersionHandler(...).ListVersions |
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
| `GET` | `/api/v1/users/:id/favorites` | favHandler.ListUserFavorites |
| `GET` | `/api/v1/users/:id/followers` | followHandler.GetFollowers |
| `GET` | `/api/v1/users/:id/following` | followHandler.GetFollowing |
| `GET` | `/api/v1/users/:id/reputation` | userHandler.GetReputation |
| `GET` | `/api/v1/users/me/contents` | userHandler.GetMyContents |
| `GET` | `/api/v1/users/me/followers/stats` | followHandler.GetFollowerStats |
| `GET` | `/api/v1/users/me/history` | histHandler.GetHistory |
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
| `POST` | `/api/v1/admin/categories` | catHandler.AdminCreateCategory |
| `POST` | `/api/v1/admin/contents/:id/ban` | adminHandler.BanContent |
| `POST` | `/api/v1/admin/feedback/:id/replies` | adminFeedbackHandler.ReplyFeedback |
| `POST` | `/api/v1/admin/ips/:id/approve` | adminHandler.ApproveIP |
| `POST` | `/api/v1/admin/ips/:id/reject` | adminHandler.RejectIP |
| `POST` | `/api/v1/admin/judge/questions` | judgeHandler.CreateQuestions |
| `POST` | `/api/v1/admin/llm-configs` | adminHandler.CreateLLMConfig |
| `POST` | `/api/v1/admin/llm-configs/:id/activate` | adminHandler.ActivateLLMConfig |
| `POST` | `/api/v1/admin/llm-configs/:id/test` | adminHandler.TestLLMConfig |
| `POST` | `/api/v1/admin/users/:id/ban` | adminHandler.BanUser |
| `POST` | `/api/v1/admin/users/:id/unban` | adminHandler.UnbanUser |
| `POST` | `/api/v1/agent/chat/stream` | agentHandler.ChatStream |
| `POST` | `/api/v1/agent/compliance-check` | agentHandler.ComplianceCheck |
| `POST` | `/api/v1/agent/moderate/:id` | agentHandler.Moderate |
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
| `POST` | `/api/v1/contents` | contentHandler.CreateContent |
| `POST` | `/api/v1/contents/:id/report` | socialHandler.ReportContent |
| `POST` | `/api/v1/contents/:id/tags/suggest` | tagHandler.SuggestTag |
| `POST` | `/api/v1/contents/oss-token` | contentHandler.GenerateOSSToken |
| `POST` | `/api/v1/dashboard/contributors/:userId/block` | prHandler.BlockContributor |
| `POST` | `/api/v1/deploy-grants` | inline handler |
| `POST` | `/api/v1/discussions/:id/comments` | discHandler.ReplyToDiscussion |
| `POST` | `/api/v1/favorites` | favHandler.AddFavorite |
| `POST` | `/api/v1/feedback` | feedbackHandler.SubmitTicket |
| `POST` | `/api/v1/feedback/attachments/presign` | feedbackHandler.PresignUpload |
| `POST` | `/api/v1/internal/ai-callback` | internalHandler.AICallback |
| `POST` | `/api/v1/ips` | ipHandler.CreateIP |
| `POST` | `/api/v1/ips/:id/discussions` | discHandler.CreateDiscussion |
| `POST` | `/api/v1/ips/:id/follow` | followHandler.FollowIP |
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
| `POST` | `/api/v1/social/comments` | socialHandler.PostComment |
| `POST` | `/api/v1/social/comments/:id/report` | socialHandler.ReportComment |
| `POST` | `/api/v1/social/discussions` | socialHandler.PostDiscussion |
| `POST` | `/api/v1/social/reactions` | socialHandler.React |
| `POST` | `/api/v1/users/:id/follow` | followHandler.FollowUser |
| `POST` | `/api/v1/users/me/history` | histHandler.RecordView |
| `POST` | `/api/v1/users/me/saved-searches` | tagHandler.CreateSavedSearch |
| `POST` | `/api/v1/users/me/tag-groups` | tagHandler.CreateTagGroup |
| `PUT` | `/api/v1/admin/categories/reorder` | catHandler.AdminReorderCategories |

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

## 4. 数据库 Schema（PostgreSQL DDL）

<!-- AUTO-GENERATED: §4 数据库 Schema | source: backend/migrations/ | DO NOT EDIT MANUALLY -->

### admin_audit_logs

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `admin_user_id` | `BIGINT` | NOT NULL -> users.id | admin_user_id |
| `action` | `VARCHAR(96)` | NOT NULL | action |
| `target_type` | `VARCHAR(48)` | NOT NULL | target_type |
| `target_id` | `VARCHAR(96)` | - | target_id |
| `trace_id` | `VARCHAR(96)` | - | trace_id |
| `metadata` | `JSONB` | NOT NULL DEFAULT '{}' | metadata |
| `result` | `VARCHAR(24)` | NOT NULL | result |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### agent_conversations

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `context_type` | `VARCHAR(50)` | NOT NULL DEFAULT '' | context_type |
| `context_id` | `BIGINT` | - | context_id |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

### agent_messages

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `conversation_id` | `BIGINT` | NOT NULL -> agent_conversations.id | conversation_id |
| `role` | `VARCHAR(20)` | NOT NULL | role |
| `content` | `TEXT` | - | content |
| `tool_calls` | `JSONB` | - | tool_calls |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### ai_review_records

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `target_type` | `VARCHAR(20)` | NOT NULL | target_type |
| `target_id` | `BIGINT` | NOT NULL | target_id |
| `provider` | `VARCHAR(50)` | NOT NULL DEFAULT 'aliyun' | provider |
| `result` | `VARCHAR(20)` | NOT NULL | result |
| `raw_response` | `JSONB` | - | raw_response |
| `scanned_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | scanned_at |

### appeals

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `target_type` | `VARCHAR(20)` | NOT NULL | target_type |
| `target_id` | `BIGINT` | NOT NULL | target_id |
| `reason` | `TEXT` | NOT NULL | reason |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'pending' | status |
| `admin_response` | `TEXT` | - | admin_response |
| `resolved_by` | `BIGINT` | -> users.id | resolved_by |
| `resolved_at` | `TIMESTAMPTZ` | - | resolved_at |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### author_blocklist

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `author_id` | `BIGINT` | NOT NULL -> users.id | author_id |
| `blocked_id` | `BIGINT` | NOT NULL -> users.id | blocked_id |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### browse_history

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL UNIQUE -> users.id | user_id |
| `content_item_id` | `BIGINT` | NOT NULL UNIQUE -> content_items.id | content_item_id |
| `viewed_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | viewed_at |

### categories

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `zone` | `VARCHAR(20)` | NOT NULL | zone |
| `level` | `VARCHAR(20)` | NOT NULL | level |
| `parent_id` | `BIGINT` | -> categories.id | parent_id |
| `name_i18n` | `JSONB` | NOT NULL DEFAULT '{}' | name_i18n |
| `slug` | `VARCHAR(100)` | NOT NULL UNIQUE | slug |
| `sort_order` | `INT` | NOT NULL DEFAULT 0 | sort_order |
| `is_active` | `BOOLEAN` | NOT NULL DEFAULT TRUE | is_active |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

### comments

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `content_item_id` | `BIGINT` | -> content_items.id | content_item_id |
| `discussion_id` | `BIGINT` | -> discussions.id | discussion_id |
| `parent_id` | `BIGINT` | -> comments.id | parent_id |
| `author_id` | `BIGINT` | NOT NULL -> users.id | author_id |
| `body` | `TEXT` | NOT NULL | body |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'published' | status |
| `like_count` | `INT` | NOT NULL DEFAULT 0 | like_count |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### content_attachments

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `file_type` | `VARCHAR(30)` | NOT NULL | file_type |
| `oss_key` | `TEXT` | NOT NULL | oss_key |
| `file_size` | `BIGINT` | - | file_size |
| `mime_type` | `VARCHAR(100)` | - | mime_type |
| `duration_sec` | `INT` | - | duration_sec |
| `width` | `INT` | - | width |
| `height` | `INT` | - | height |
| `is_primary` | `BOOLEAN` | DEFAULT TRUE | is_primary |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### content_contributors

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `pr_count` | `INT` | NOT NULL DEFAULT 1 | pr_count |
| `first_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | first_at |

### content_embeddings

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `content_item_id` | `BIGINT` | PK -> content_items.id | content_item_id |
| `embedding` | `vector(1536)` | NOT NULL | embedding |
| `embedded_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | embedded_at |

### content_items

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `title` | `VARCHAR(500)` | NOT NULL | title |
| `author_id` | `BIGINT` | NOT NULL -> users.id | author_id |
| `zone` | `VARCHAR(10)` | NOT NULL | zone |
| `ip_id` | `BIGINT` | -> ips.id | ip_id |
| `category` | `VARCHAR(50)` | - | category |
| `content_type` | `VARCHAR(20)` | NOT NULL | content_type |
| `cover_image_url` | `TEXT` | - | cover_image_url |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'pending' | status |
| `view_count` | `BIGINT` | NOT NULL DEFAULT 0 | view_count |
| `like_count` | `INT` | NOT NULL DEFAULT 0 | like_count |
| `dislike_count` | `INT` | NOT NULL DEFAULT 0 | dislike_count |
| `is_public` | `BOOLEAN` | NOT NULL DEFAULT TRUE | is_public |
| `allow_copy` | `BOOLEAN` | NOT NULL DEFAULT TRUE | allow_copy |
| `agent_enabled` | `BOOLEAN` | NOT NULL DEFAULT FALSE | agent_enabled |
| `is_paid` | `BOOLEAN` | NOT NULL DEFAULT FALSE | is_paid |
| `price` | `NUMERIC(10,2)` | DEFAULT 0 | price |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

### content_tags

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `tag` | `VARCHAR(50)` | NOT NULL | tag |

### content_versions

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `content_item_id` | `BIGINT` | NOT NULL UNIQUE -> content_items.id | content_item_id |
| `parent_version_id` | `BIGINT` | -> content_versions.id | parent_version_id |
| `author_id` | `BIGINT` | NOT NULL -> users.id | author_id |
| `version_number` | `INT` | NOT NULL UNIQUE | version_number |
| `storage_type` | `VARCHAR(10)` | NOT NULL | storage_type |
| `storage_key` | `TEXT` | - | storage_key |
| `diff_summary` | `TEXT` | - | diff_summary |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'active' | status |
| `is_latest` | `BOOLEAN` | NOT NULL DEFAULT FALSE | is_latest |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### conversation_participants

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `conversation_id` | `BIGINT` | NOT NULL -> conversations.id | conversation_id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `last_read_at` | `TIMESTAMPTZ` | - | last_read_at |

### conversations

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

### discussions

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `ip_id` | `BIGINT` | -> ips.id | ip_id |
| `content_item_id` | `BIGINT` | -> content_items.id | content_item_id |
| `author_id` | `BIGINT` | NOT NULL -> users.id | author_id |
| `title` | `VARCHAR(500)` | NOT NULL | title |
| `body` | `TEXT` | - | body |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'published' | status |
| `view_count` | `BIGINT` | NOT NULL DEFAULT 0 | view_count |
| `reply_count` | `INT` | NOT NULL DEFAULT 0 | reply_count |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

### favorites

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### feedback_attachments

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `ticket_id` | `BIGINT` | NOT NULL -> feedback_tickets.id | ticket_id |
| `oss_key` | `TEXT` | NOT NULL | oss_key |
| `file_type` | `VARCHAR(32)` | NOT NULL | file_type |
| `mime_type` | `VARCHAR(100)` | NOT NULL | mime_type |
| `size_bytes` | `BIGINT` | NOT NULL | size_bytes |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### feedback_replies

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `ticket_id` | `BIGINT` | NOT NULL -> feedback_tickets.id | ticket_id |
| `author_user_id` | `BIGINT` | -> users.id | author_user_id |
| `author_admin_id` | `BIGINT` | -> users.id | author_admin_id |
| `body` | `TEXT` | NOT NULL | body |
| `is_internal_note` | `BOOLEAN` | NOT NULL DEFAULT FALSE | is_internal_note |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### feedback_tickets

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | -> users.id | user_id |
| `contact_email` | `VARCHAR(255)` | - | contact_email |
| `category` | `VARCHAR(32)` | NOT NULL | category |
| `title` | `VARCHAR(160)` | NOT NULL | title |
| `description` | `TEXT` | NOT NULL | description |
| `diagnostic_summary` | `JSONB` | NOT NULL DEFAULT '{}' | diagnostic_summary |
| `status` | `VARCHAR(24)` | NOT NULL DEFAULT 'open' | status |
| `priority` | `VARCHAR(24)` | NOT NULL DEFAULT 'normal' | priority |
| `assignee_admin_id` | `BIGINT` | -> users.id | assignee_admin_id |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |
| `resolved_at` | `TIMESTAMPTZ` | - | resolved_at |

### follows

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `follower_id` | `BIGINT` | NOT NULL UNIQUE -> users.id | follower_id |
| `target_type` | `VARCHAR(20)` | NOT NULL UNIQUE | target_type |
| `target_id` | `BIGINT` | NOT NULL UNIQUE | target_id |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### ip_review_logs

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `ip_id` | `BIGINT` | NOT NULL -> ips.id | ip_id |
| `reviewer_id` | `BIGINT` | -> users.id | reviewer_id |
| `action` | `VARCHAR(20)` | NOT NULL | action |
| `reason` | `TEXT` | - | reason |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### ip_tags

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `ip_id` | `BIGINT` | NOT NULL -> ips.id | ip_id |
| `tag` | `VARCHAR(50)` | NOT NULL | tag |

### ips

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `name` | `VARCHAR(255)` | NOT NULL | name |
| `slug` | `VARCHAR(255)` | NOT NULL UNIQUE | slug |
| `description` | `TEXT` | - | description |
| `cover_url` | `TEXT` | - | cover_url |
| `category` | `VARCHAR(50)` | - | category |
| `creator_id` | `BIGINT` | -> users.id | creator_id |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'pending' | status |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

### judge_cases

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `target_type` | `VARCHAR(20)` | NOT NULL | target_type |
| `target_id` | `BIGINT` | NOT NULL | target_id |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'open' | status |
| `vote_approve` | `INT` | NOT NULL DEFAULT 0 | vote_approve |
| `vote_reject` | `INT` | NOT NULL DEFAULT 0 | vote_reject |
| `min_votes` | `INT` | NOT NULL DEFAULT 20 | min_votes |
| `closed_at` | `TIMESTAMPTZ` | - | closed_at |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### judge_exam_records

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `content_type` | `VARCHAR(50)` | NOT NULL | content_type |
| `score` | `INT` | NOT NULL | score |
| `total` | `INT` | NOT NULL | total |
| `passed` | `BOOLEAN` | NOT NULL | passed |
| `taken_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | taken_at |

### judge_qualifications

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL UNIQUE -> users.id | user_id |
| `content_type` | `VARCHAR(50)` | NOT NULL UNIQUE | content_type |
| `qualified_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | qualified_at |
| `revoked_at` | `TIMESTAMPTZ` | - | revoked_at |
| `is_active` | `BOOLEAN` | NOT NULL DEFAULT TRUE | is_active |

### judge_questions

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `content_type` | `VARCHAR(50)` | NOT NULL | content_type |
| `source_case_id` | `BIGINT` | - | source_case_id |
| `question_data` | `JSONB` | NOT NULL | question_data |
| `is_active` | `BOOLEAN` | NOT NULL DEFAULT TRUE | is_active |
| `created_by` | `VARCHAR(20)` | NOT NULL DEFAULT 'admin' | created_by |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### judge_reason_votes

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `reason_owner_vote_id` | `BIGINT` | NOT NULL UNIQUE -> judge_votes.id | reason_owner_vote_id |
| `voter_id` | `BIGINT` | NOT NULL UNIQUE -> users.id | voter_id |
| `vote_type` | `VARCHAR(10)` | NOT NULL | vote_type |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### judge_votes

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `case_id` | `BIGINT` | NOT NULL UNIQUE -> judge_cases.id | case_id |
| `judge_id` | `BIGINT` | NOT NULL UNIQUE -> users.id | judge_id |
| `vote` | `VARCHAR(10)` | NOT NULL | vote |
| `reason` | `TEXT` | - | reason |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### llm_configs

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `config_name` | `VARCHAR(100)` | NOT NULL | config_name |
| `provider_type` | `VARCHAR(50)` | NOT NULL | provider_type |
| `api_base` | `VARCHAR(500)` | - | api_base |
| `model` | `VARCHAR(100)` | NOT NULL | model |
| `api_key_enc` | `TEXT` | - | api_key_enc |
| `is_active` | `BOOLEAN` | NOT NULL DEFAULT FALSE | is_active |
| `extra_params` | `JSONB` | NOT NULL DEFAULT '{}' | extra_params |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

### messages

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `conversation_id` | `BIGINT` | NOT NULL -> conversations.id | conversation_id |
| `sender_id` | `BIGINT` | NOT NULL -> users.id | sender_id |
| `body` | `TEXT` | NOT NULL | body |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### notifications

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `type` | `VARCHAR(50)` | NOT NULL | type |
| `channel` | `VARCHAR(20)` | NOT NULL | channel |
| `title` | `VARCHAR(500)` | - | title |
| `body` | `TEXT` | - | body |
| `target_type` | `VARCHAR(50)` | - | target_type |
| `target_id` | `BIGINT` | - | target_id |
| `sender_id` | `BIGINT` | -> users.id | sender_id |
| `is_read` | `BOOLEAN` | NOT NULL DEFAULT FALSE | is_read |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### oauth_accounts

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `provider` | `VARCHAR(20)` | NOT NULL UNIQUE | provider |
| `provider_uid` | `VARCHAR(255)` | NOT NULL UNIQUE | provider_uid |
| `access_token` | `TEXT` | - | access_token |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### password_reset_tokens

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `token` | `VARCHAR(255)` | NOT NULL UNIQUE | token |
| `expires_at` | `TIMESTAMPTZ` | NOT NULL | expires_at |
| `used_at` | `TIMESTAMPTZ` | - | used_at |
| `created_at` | `TIMESTAMPTZ` | DEFAULT NOW() | created_at |

### pull_requests

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `submitter_id` | `BIGINT` | NOT NULL -> users.id | submitter_id |
| `base_version_id` | `BIGINT` | NOT NULL -> content_versions.id | base_version_id |
| `proposed_version_id` | `BIGINT` | -> content_versions.id | proposed_version_id |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'open' | status |
| `message` | `TEXT` | - | message |
| `reject_reason` | `TEXT` | - | reject_reason |
| `resolved_at` | `TIMESTAMPTZ` | - | resolved_at |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### reactions

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL UNIQUE -> users.id | user_id |
| `target_type` | `VARCHAR(20)` | NOT NULL UNIQUE | target_type |
| `target_id` | `BIGINT` | NOT NULL UNIQUE | target_id |
| `reaction` | `VARCHAR(10)` | NOT NULL | reaction |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### rehab_completions

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL UNIQUE -> users.id | user_id |
| `course_id` | `BIGINT` | NOT NULL UNIQUE -> rehab_courses.id | course_id |
| `completed_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | completed_at |

### rehab_courses

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `violation_type` | `VARCHAR(100)` | NOT NULL UNIQUE | violation_type |
| `content_i18n` | `JSONB` | NOT NULL DEFAULT '{}' | content_i18n |
| `min_reading_sec` | `INT` | NOT NULL DEFAULT 60 | min_reading_sec |
| `reward_points` | `INT` | NOT NULL DEFAULT 0 | reward_points |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### reports

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `reporter_id` | `BIGINT` | NOT NULL -> users.id | reporter_id |
| `target_type` | `VARCHAR(20)` | NOT NULL | target_type |
| `target_id` | `BIGINT` | NOT NULL | target_id |
| `reason` | `VARCHAR(100)` | NOT NULL | reason |
| `detail` | `TEXT` | - | detail |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'pending' | status |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### reputation_logs

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `delta` | `INT` | NOT NULL | delta |
| `reason` | `VARCHAR(100)` | NOT NULL | reason |
| `related_id` | `BIGINT` | - | related_id |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### saved_searches

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `name` | `VARCHAR(200)` | NOT NULL | name |
| `config` | `JSONB` | NOT NULL DEFAULT '{}' | config |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### tag_groups

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `name` | `VARCHAR(100)` | NOT NULL | name |
| `tags` | `TEXT[]` | NOT NULL DEFAULT '{}' | tags |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

### tag_suggestions

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `content_item_id` | `BIGINT` | NOT NULL UNIQUE -> content_items.id | content_item_id |
| `user_id` | `BIGINT` | NOT NULL UNIQUE -> users.id | user_id |
| `tag` | `VARCHAR(100)` | NOT NULL UNIQUE | tag |
| `action` | `VARCHAR(10)` | NOT NULL UNIQUE | action |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'pending' | status |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |

### tags

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `name` | `VARCHAR(100)` | NOT NULL UNIQUE | name |
| `category` | `VARCHAR(50)` | NOT NULL DEFAULT '' | category |
| `usage_count` | `INT` | NOT NULL DEFAULT 0 | usage_count |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

### users

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `email` | `VARCHAR(255)` | NOT NULL UNIQUE | email |
| `password_hash` | `VARCHAR(255)` | NOT NULL | password_hash |
| `username` | `VARCHAR(64)` | NOT NULL UNIQUE | username |
| `avatar_url` | `TEXT` | - | avatar_url |
| `bio` | `TEXT` | - | bio |
| `reputation` | `INT` | NOT NULL DEFAULT 10 | reputation |
| `preferred_locale` | `VARCHAR(10)` | NOT NULL DEFAULT 'zh-CN' | preferred_locale |
| `role` | `VARCHAR(20)` | NOT NULL DEFAULT 'user' | role |
| `is_banned` | `BOOLEAN` | NOT NULL DEFAULT FALSE | is_banned |
| `ban_reason` | `TEXT` | - | ban_reason |
| `support_info` | `JSONB` | DEFAULT '{}' | support_info |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |


<!-- END AUTO-GENERATED: §4 -->

---

## 5. API 设计详细说明

### 5.1 通用规范

```
Base URL: /api/v1
Content-Type: application/json
认证: Authorization: Bearer <JWT>
错误格式: { "code": "ERROR_CODE", "message": "描述", "details": {} }
分页: ?page=1&page_size=20  →  { "data": [], "total": 100, "page": 1, "page_size": 20 }
```

### 5.2 关键接口示例

#### 内容详情（GET /api/v1/contents/:id）

```json
// Response 200
{
  "id": 123,
  "title": "某同人小说标题",
  "author": { "id": 1, "username": "creator", "avatar_url": "..." },
  "zone": "fanwork",
  "ip": { "id": 5, "name": "原神", "slug": "genshin-impact" },
  "source_original_id": null,
  "content_type": "article",
  "status": "published",
  "current_version_id": 7,
  "view_count": 1024,
  "like_count": 88,
  "attachments": [
    { "file_type": "markdown", "oss_url": "https://..." }
  ],
  "tags": ["同人文", "中篇"],
  "permissions": {
    "is_public": true,
    "allow_copy": true,
    "agent_enabled": false
  },
  "created_at": "2025-01-01T00:00:00Z"
}
```

#### 发布内容（POST /api/v1/contents）

`source_original_id` 为可选字段，只用于二创绑定来源原创。原创内容携带该字段返回 400；二创绑定不存在、非原创或未发布内容也返回 400。

```json
// Request
{
  "title": "基于某原创世界观的二创短篇",
  "description": "Markdown 正文或简介",
  "zone": "fanwork",
  "ip_id": 5,
  "source_original_id": 123,
  "content_type": "article",
  "tags": ["短篇"]
}
```

#### 相关二创（GET /api/v1/contents/:id/related-fanworks）

查询某个原创内容下已发布的二创内容。支持 `page`、`page_size`、`sort`、`content_type` 筛选；`content_type` 可支持逗号分隔多值，例如 `article,prompt` 或 `audio,sheet_music`。

```json
// Response 200
{
  "source_original_id": 123,
  "total": 8,
  "page": 1,
  "page_size": 24,
  "contents": []
}
```

#### 提交 PR（POST /api/v1/pr）

```json
// Request
{
  "content_item_id": 123,
  "base_version_id": 7,
  "storage_type": "diff",         // 'diff' | 'full'
  "oss_key": "pr/456/patch.diff", // 已上传到 OSS 的 patch 文件
  "message": "修正第三段的措辞错误"
}

// Response 201
{
  "pr_id": 456,
  "proposed_version_id": 8,
  "status": "open"
}
```

#### 获取 OSS 预签名上传 URL（POST /api/v1/contents/oss-token）

```json
// Request
{
  "content_type": "video",
  "file_name": "demo.mp4",
  "file_size": 104857600
}

// Response 200
{
  "upload_url": "https://bucket.oss-cn-hangzhou.aliyuncs.com/...",
  "oss_key": "contents/2025/01/uuid.mp4",
  "expires_at": "2025-01-01T01:00:00Z"
}
```

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

- Tauri 端所有本地操作通过 Rust 层白名单校验，WebView 无法直接调用系统 API
- Go 下发的动作脚本当前使用 HMAC-SHA256 签名（D-03 完成后改为 Ed25519），Tauri 验签后执行
- 白名单目录通过 `tauri.conf.json` 写死，运行时不可修改

### 6.4 内容安全

- 上传完成 → Go 异步调用阿里云内容安全
- 视频异步扫描（回调模式）
- 文本/图片同步扫描（< 200ms），扫描失败默认 pending，人工补审

---

## 7. 配置化开关与参数（config.yaml）

所有可动态调整的参数集中在 `config.yaml`，通过管理员 API 热更新。
字段名与 `config/config.go` 中结构体的 `mapstructure` tag 一一对应。

<!-- AUTO-GENERATED: §7 配置字段注册表 | source: backend/config/config.go | DO NOT EDIT MANUALLY -->

| 配置路径 | 类型 | 说明 |
|----------|------|------|
| `agent.chat_max_context_messages` | `int` | ChatMaxContextMsgs |
| `agent.embedding_dimensions` | `int` | EmbeddingDimensions |
| `agent.embedding_model` | `string` | EmbeddingModel |
| `agent.hmac_secret` | `string` | HMACSecret |
| `agent.llm_api_base` | `string` | LLMAPIBase |
| `agent.llm_api_key` | `string` | LLMAPIKey |
| `agent.llm_model` | `string` | LLMModel |
| `agent.llm_provider` | `string` | LLMProvider |
| `agent.max_user_message_chars` | `int` | MaxUserMessageChars |
| `agent.rate_limit_per_day` | `int` | RateLimitPerDay |
| `agent.upload_assist_max_file_mb` | `int` | UploadAssistMaxFileMB |
| `agent.web_agent_enabled` | `bool` | WebAgentEnabled |
| `cache.content_detail_ttl` | `int` | ContentDetailTTL |
| `cache.content_list_ttl` | `int` | ContentListTTL |
| `cache.email_verify_ttl` | `int` | EmailVerifyTTL |
| `cache.hot_rank_zset_ttl` | `int` | HotRankZSetTTL |
| `cache.ip_detail_ttl` | `int` | IPDetailTTL |
| `cache.ip_list_ttl` | `int` | IPListTTL |
| `cache.password_reset_ttl` | `int` | PasswordResetTTL |
| `cache.publish_freeze_ttl` | `int` | PublishFreezeTTL |
| `cache.tag_cache_ttl` | `int` | TagCacheTTL |
| `cache.user_status_ttl` | `int` | UserStatusTTL |
| `cache.view_count_flush_interval` | `int` | ViewCountFlushInterval |
| `captcha.access_key_id` | `string` | AccessKeyID |
| `captcha.access_key_secret` | `string` | AccessKeySecret |
| `captcha.prefix` | `string` | Prefix |
| `captcha.provider` | `string` | Provider |
| `captcha.region` | `string` | Region |
| `captcha.scene_id` | `string` | SceneID |
| `captcha.ticket_ttl_sec` | `int` | TicketTTLSec |
| `client.download_enabled` | `bool` | DownloadEnabled |
| `client.download_url` | `string` | DownloadURL |
| `client.latest_version` | `string` | LatestVersion |
| `database.dsn` | `string` | DSN |
| `database.read_dsn` | `string` | ReadDSN |
| `features.creator_support_enabled` | `bool` | CreatorSupportEnabled |
| `features.desktop_deploy_enabled` | `bool` | DesktopDeployEnabled |
| `features.payment_enabled` | `bool` | PaymentEnabled |
| `feedback.upload_grant_ttl_sec` | `int` | UploadGrantTTLSec |
| `green.access_key_id` | `string` | AccessKeyID |
| `green.access_key_secret` | `string` | AccessKeySecret |
| `green.callback_allowed_ips` | `[]string` | CallbackAllowedIPs |
| `green.callback_url` | `string` | CallbackURL |
| `green.region` | `string` | Region |
| `judge.error_rate_revoke` | `float64` | ErrorRateRevoke |
| `judge.error_rate_window` | `int` | ErrorRateWindow |
| `judge.exam_pass_rate` | `float64` | ExamPassRate |
| `judge.min_votes_required` | `int` | MinVotesRequired |
| `judge.pass_threshold` | `float64` | PassThreshold |
| `jwt.access_token_ttl` | `int` | AccessTokenTTL |
| `jwt.refresh_token_ttl` | `int` | RefreshTokenTTL |
| `jwt.secret` | `string` | Secret |
| `legal.current_privacy_version` | `string` | CurrentPrivacyVersion |
| `legal.current_terms_version` | `string` | CurrentTermsVersion |
| `limits.image_max_mb` | `int` | ImageMaxMB |
| `limits.mod_max_mb` | `int` | ModMaxMB |
| `limits.sheet_music_max_mb` | `int` | SheetMusicMaxMB |
| `limits.text_max_mb` | `int` | TextMaxMB |
| `limits.video_max_mb` | `int` | VideoMaxMB |
| `limits.video_max_sec` | `int` | VideoMaxSec |
| `oss.access_key_id` | `string` | AccessKeyID |
| `oss.access_key_secret` | `string` | AccessKeySecret |
| `oss.bucket_name` | `string` | BucketName |
| `oss.domain` | `string` | Domain |
| `oss.download_url_ttl_sec` | `int` | DownloadURLTTL |
| `oss.endpoint` | `string` | Endpoint |
| `publish.freeze_on_violation` | `bool` | FreezeOnViolation |
| `publish.max_daily_posts` | `int` | MaxDailyPosts |
| `publish.require_review` | `bool` | RequireReview |
| `publish.type_order_fanwork` | `[]string` | TypeOrderFanwork |
| `publish.type_order_original` | `[]string` | TypeOrderOriginal |
| `queue` | `queue.QueueConfig` | Queue |
| `rate_limit.agent_window_sec` | `int` | AgentWindowSec |
| `rate_limit.credential_per_minute` | `int` | CredentialPerMinute |
| `rate_limit.enabled` | `bool` | Enabled |
| `rate_limit.max_json_body_bytes` | `int64` | MaxJSONBodyBytes |
| `rate_limit.max_query_chars` | `int` | MaxQueryChars |
| `rate_limit.max_search_limit` | `int` | MaxSearchLimit |
| `rate_limit.max_search_page` | `int` | MaxSearchPage |
| `rate_limit.normal_per_minute` | `int` | NormalPerMinute |
| `rate_limit.normal_window_sec` | `int` | NormalWindowSec |
| `rate_limit.search_per_minute` | `int` | SearchPerMinute |
| `rate_limit.upload_per_hour` | `int` | UploadPerHour |
| `rate_limit.upload_window_sec` | `int` | UploadWindowSec |
| `recommendation.embedding_multiplier` | `int` | EmbeddingMultiplier |
| `recommendation.embedding_topk` | `int` | EmbeddingTopk |
| `recommendation.enabled` | `bool` | Enabled |
| `recommendation.hot_decay_hours` | `float64` | HotDecayHours |
| `recommendation.min_interaction_for_personalize` | `int` | MinInteractionForPersonalize |
| `recommendation.personalization_weight` | `float64` | PersonalizationWeight |
| `recommendation.rank_interval_min` | `int` | RankIntervalMin |
| `recommendation.refresh_interval_h` | `int` | RefreshIntervalH |
| `recommendation.trending_window_days` | `int` | TrendingWindowDays |
| `redis.addr` | `string` | Addr |
| `redis.db` | `int` | DB |
| `redis.password` | `string` | Password |
| `reputation.min_score_for_interaction` | `int` | MinScoreForInteraction |
| `reputation.quality_comment_threshold` | `int` | QualityCommentThreshold |
| `reputation.quality_content_threshold` | `int` | QualityContentThreshold |
| `reputation.repeat_violation_extra_penalty` | `int` | RepeatViolationExtraPenalty |
| `reputation.repeat_violation_threshold` | `int` | RepeatViolationThreshold |
| `reputation.repeat_violation_window_days` | `int` | RepeatViolationWindowDays |
| `reputation.score_judge_accuracy` | `int` | ScoreJudgeAccuracy |
| `reputation.score_judge_error` | `int` | ScoreJudgeError |
| `reputation.score_malicious_comment` | `int` | ScoreMaliciousComment |
| `reputation.score_malicious_content` | `int` | ScoreMaliciousContent |
| `reputation.score_malicious_pr` | `int` | ScoreMaliciousPR |
| `reputation.score_malicious_report` | `int` | ScoreMaliciousReport |
| `reputation.score_malicious_tag_report` | `int` | ScoreMaliciousTagReport |
| `reputation.score_pr_merged` | `int` | ScorePRMerged |
| `reputation.score_quality_comment` | `int` | ScoreQualityComment |
| `reputation.score_quality_content` | `int` | Score values (positive = award, negative = penalty).
Zero means "use the hardcoded default in reputation_service.go". |
| `reputation.score_rehab_course` | `int` | ScoreRehabCourse |
| `reputation.score_tag_recognized` | `int` | ScoreTagRecognized |
| `reputation.score_valid_report` | `int` | ScoreValidReport |
| `security.allowed_origins` | `[]string` | AllowedOrigins |
| `security.trusted_proxies` | `[]string` | TrustedProxies |
| `server.mode` | `string` | Mode |
| `server.port` | `string` | Port |
| `server.shutdown_timeout` | `int` | ShutdownTimeout |
| `smtp.from_address` | `string` | FromAddress |
| `smtp.host` | `string` | Host |
| `smtp.mode` | `string` | Mode |
| `smtp.password` | `string` | Password |
| `smtp.port` | `int` | Port |
| `smtp.user` | `string` | User |
| `social.comment_fold_threshold` | `float64` | CommentFoldThreshold |
| `social.report_auto_hide_rate` | `float64` | ReportAutoHideRate |
| `upload.sheet_music_extensions` | `[]string` | SheetMusicExtensions |
| `verification.email_ttl_sec` | `int` | EmailTTLSec |
| `verification.login_captcha_threshold` | `int` | LoginCaptchaThreshold |
| `verification.password_min_length` | `int` | PasswordMinLength |
| `verification.register_pending_ttl_sec` | `int` | RegisterPendingTTLSec |
| `verification.resend_cooldown_sec` | `int` | ResendCooldownSec |
| `verification.reset_ttl_sec` | `int` | ResetTTLSec |
| `web.public_base_url` | `string` | PublicBaseURL |

<!-- END AUTO-GENERATED: §7 -->

---

## 8. 环境变量与外部依赖清单

### 8.1 环境变量（`.env` / Docker Compose）

```env
# 数据库
DB_DSN=postgres://user:pass@pgbouncer:5432/omnicraft
DB_READ_DSN=                      # 留空则全走主库，P1 填从库 DSN

# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=

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

# Agent 签名
AGENT_HMAC_SECRET=<随机 32 字节>

# 应用
APP_ENV=production                # development | production
APP_PORT=8080
FRONTEND_URL=https://app.leeppp.online     # 当前生产环境实际域名
```

### 8.2 Docker Compose 服务清单

```yaml
version: "3.9"
services:
  frontend:        # Next.js（端口 3000）
  backend:         # Go API（端口 8080）
  postgres:        # PostgreSQL 16（端口 5432，内网）
  pgbouncer:       # PgBouncer（端口 5432，暴露给 backend）
  redis:           # Redis 7（端口 6379，内网）
  nginx:           # Nginx（端口 80/443，唯一对外）
```

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

---

## 10. 需求补充（V0.3.1）

> 本章节记录 V0.3.1 阶段新增的设计决策，补充对前述章节的局部修订。

---

### 10.1 标签体系增强

#### 设计目标

参考 **Google Scholar faceted search** 和 **GitHub 搜索**，实现左侧动态标签面板，帮助用户逐步缩小浏览范围。

#### 标签面板交互逻辑

```
① 页面左侧顶部：选择「大类」（学习 / 游戏 / 动漫 / 音乐 / 其他）
② 大类下方：显示该大类内所有标签，按「关联内容数」降序排列
③ 点击标签 A 后：面板更新为「同时含标签 A 的内容中」出现频率最高的其他标签
④ 继续点击标签 B：进一步缩小为「同时含 A+B」的内容中的共现标签
⑤ Advanced 折叠区：内容类型、发布时间范围、排序方式、IP（二创区）等精细筛选
⑥ 「保存此搜索」按钮：将当前大类+标签组合+Advanced 配置持久化为命名搜索
```

#### 共现标签查询 SQL（核心算法）

```sql
-- 给定已选标签 ['高中']，查询共现标签排行
SELECT ct2.tag, COUNT(*) AS co_count
FROM content_tags ct1
JOIN content_tags ct2
  ON ct1.content_item_id = ct2.content_item_id
WHERE ct1.tag = '高中'
  AND ct2.tag != '高中'
GROUP BY ct2.tag
ORDER BY co_count DESC
LIMIT 20;

-- 多标签共现（已选 ['高中', '数学']）
SELECT ct_other.tag, COUNT(*) AS co_count
FROM (
  SELECT content_item_id
  FROM content_tags
  WHERE tag IN ('高中', '数学')
  GROUP BY content_item_id
  HAVING COUNT(DISTINCT tag) = 2   -- 必须同时含已选所有标签
) base
JOIN content_tags ct_other ON base.content_item_id = ct_other.content_item_id
WHERE ct_other.tag NOT IN ('高中', '数学')
GROUP BY ct_other.tag
ORDER BY co_count DESC
LIMIT 20;
```

#### 标签体系新增 DB Schema（migrations/015_tags.sql）

```sql
-- 全局标签表（P0 按需创建，P1 定期同步 content_tags 统计）
CREATE TABLE tags (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(100) UNIQUE NOT NULL,
    category      VARCHAR(50),  -- '学习' | '游戏' | '动漫' | '音乐' | '影视' | '其他'
    usage_count   BIGINT NOT NULL DEFAULT 0,  -- 定期更新缓存值
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tags_category_usage ON tags(category, usage_count DESC);
CREATE INDEX idx_tags_name_gin ON tags USING GIN(to_tsvector('simple', name));

-- 标签建议（用户对某内容提议增减标签，作者审批）
CREATE TABLE tag_suggestions (
    id              BIGSERIAL PRIMARY KEY,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tag             VARCHAR(100) NOT NULL,
    action          VARCHAR(10) NOT NULL,   -- 'add' | 'remove'
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- status: 'pending' | 'approved' | 'rejected' | 'reported'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(content_item_id, user_id, tag, action)
);

-- 用户标签组（自建快捷筛选组，如「鸣佐组合」= ['火影忍者','鸣人','佐助']）
CREATE TABLE tag_groups (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    tags        TEXT[] NOT NULL,  -- e.g., ARRAY['火影忍者','鸣人','佐助']
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tag_groups_user ON tag_groups(user_id);

-- 已保存的搜索配置
CREATE TABLE saved_searches (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    config      JSONB NOT NULL,
    -- config: { "category": "学习", "tags": ["高中","数学"], "zone": "original",
    --           "content_type": "article", "sort": "hot", "time_range": "week",
    --           "advanced": { "ip_id": null } }
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### 新增 API 路由（标签体系）

```
GET    /api/v1/tags/faceted               # 获取分面标签列表
                                          # ?category=学习&selected_tags[]=高中&zone=original
GET    /api/v1/tags/search                # 标签名搜索（发布时作者打标签用）

POST   /api/v1/contents/:id/tags/suggest # 用户建议增减标签
GET    /api/v1/dashboard/tag-suggestions  # 作者查看待处理的标签建议
PATCH  /api/v1/tag-suggestions/:id        # 作者 approve/reject 标签建议

GET    /api/v1/users/me/tag-groups        # 获取我的标签组
POST   /api/v1/users/me/tag-groups        # 创建标签组
PATCH  /api/v1/users/me/tag-groups/:id    # 修改标签组
DELETE /api/v1/users/me/tag-groups/:id    # 删除标签组

GET    /api/v1/users/me/saved-searches    # 获取保存的搜索
POST   /api/v1/users/me/saved-searches    # 保存搜索
DELETE /api/v1/users/me/saved-searches/:id
```

#### 内容列表接口扩展（时间范围排序）

```
GET /api/v1/contents
  参数追加：
  - sort: 'hot' | 'new' | 'clicks' | 'best_rated'  （原有 + 新增最高好评率）
  - time_range: 'all' | 'week' | 'month' | 'year'  （新增）
  - tags[]: 多标签 AND 筛选              （新增）
  - tag_group_id: 使用保存的标签组筛选   （新增）
```

---

### 10.2 乐谱内容支持

#### 支持格式

| 格式 | 扩展名 | 说明 | 前端 Viewer |
|------|--------|------|------------|
| MIDI | .mid .midi | 音符序列，可播放 | midi-player-js + soundfont-player |
| MusicXML | .xml .mxl | 结构化乐谱，可渲染五线谱 | OpenSheetMusicDisplay (OSMD) |
| PDF | .pdf | 扫描乐谱或排版输出 | 浏览器原生 `<embed>` / pdf.js |
| MSCZ | .mscz | MuseScore 格式（二进制） | 仅下载，提示用 MuseScore 打开 |
| MSCX | .mscx | MuseScore 格式（XML） | 尝试 OSMD 解析，失败则仅下载 |

#### SheetMusicViewer 组件逻辑

```
components/content/SheetMusicViewer.tsx
├── 根据 attachment.mime_type 选择渲染策略
│   ├── audio/midi → MIDIPlayer（播放进度条 + 钢琴卷帘可选）
│   ├── application/xml / vnd.recordare.musicxml → OSMDRenderer（五线谱渲染）
│   ├── application/pdf → <embed type="application/pdf"> 全宽显示
│   └── MSCZ / MSCX → DownloadPrompt（提示下载并用 MuseScore 打开）
└── 均显示「下载原文件」按钮（allow_copy=true 时）
```

#### MIME 类型映射（Go 后端 OSS Token 接口白名单追加）

```go
"mid":  "audio/midi",
"midi": "audio/midi",
"xml":  "application/vnd.recordare.musicxml+xml",
"mxl":  "application/vnd.recordare.musicxml",
"mscz": "application/x-musescore",
"mscx": "application/x-musescore+xml",
```

---

### 10.3 多语言支持（i18n）

#### 技术方案

- **框架**：`next-intl`（无 URL 前缀模式，通过 context 注入语言）
- **初期语言**：`zh-CN`（默认）、`en-US`
- **语言存储**：`users.preferred_locale` 字段（已加入 schema）+ `localStorage` 客户端缓存
- **翻译文件位置**：`frontend/messages/zh.json`、`frontend/messages/en.json`

#### 前端集成方式

```
1. layout.tsx 包裹 NextIntlClientProvider，从 cookie/localStorage 读取 locale
2. Header 顶部右侧增加语言切换器（ZH / EN 两个按钮）
3. 切换语言 → 写 localStorage + 调用 PATCH /users/:id 更新服务端偏好
4. 所有 UI 字符串通过 useTranslations() hook 引用，禁止硬编码中文
5. 后端错误码（code 字段）由前端翻译，不依赖后端返回语言文字
```

#### 翻译文件结构示例

```json
// messages/zh.json
{
  "common": { "login": "登录", "register": "注册", "publish": "发布" },
  "nav": { "fanwork": "二创区", "original": "原创区" },
  "content": { "like": "点赞", "dislike": "踩", "report": "举报" },
  "tag": { "suggestAdd": "建议添加此标签", "suggestRemove": "建议移除此标签" },
  "judge": { "exam": "资质考核", "queue": "审核队列" }
}
```

---

### 10.4 UI 设计规范

> **权威来源**：UI 设计规范的唯一权威为 [`design/design-system.md`](./design/design-system.md) 和 [`design/ui-spec.md`](./design/ui-spec.md)。
> 本节仅列出设计系统概要，详细色值、字体、间距、组件规范请查阅上述文件。
> `docs/archive/UI Design.md` 和 `docs/archive/homepage-v0.html` 已归档，不再作为设计参考。

#### 设计系统概要

- **主色调**：Indigo `#4F46E5`（详见 `design-system.md`）
- **背景色**：`#FFFFFF`（亮色）/ 暗色模式见 `design-system.md`
- **Header 高度**：52px
- **字体**：系统字体栈（详见 `design-system.md`）
- **主题切换**：用户首选主题存入 `localStorage: theme = 'light' | 'dark' | 'system'`，Header 右上角提供切换按钮，不与语言偏好存入服务端（纯客户端偏好）

#### 组件规范

- **标签 Badge**：低饱和色、圆角 6px、字号 12px
- **按钮**：Solid（主色）/ Outline（border 色）/ Ghost（无边框）三种变体
- **卡片**：1px border，hover 时 border 加深，无 box-shadow（扁平风格）
- **暗色模式**：通过 `class="dark"` on `<html>` 切换（shadcn/ui 内置支持）

> 详细组件规范、色值 Token、间距体系请查阅 [`design/design-system.md`](./design/design-system.md)。

---

### 10.5 内容浏览布局（小红书式瀑布流）

#### 布局方案

- **库**：`react-masonry-css`（轻量，SSR 友好）或 `masonic`（虚拟化，大数据集友好，P1 升级）
- **列数**：移动端 2 列 / 平板 3 列 / PC 4 列
- **瀑布流触发范围**：首页（无筛选状态）+ IP 详情页内容列表 + 原创区

#### ContentCard 规范

**二创区卡片**（zone='fanwork'）：

```
┌─────────────────┐
│                 │  ← 封面图（3:4 比例，object-cover）
│   cover image   │    若 cover_image_url 为空 → 显示内容类型默认占位图
│                 │    （音符 sheet_music / 文档 article / 控制器 mod...）
├─────────────────┤
│ 标题（2行截断） │
│ @作者  · IP名  │
│ ❤ 88  💬 12  │  ← 点赞数 / 评论数
│ [标签][标签]    │  ← 最多 3 个低饱和色标签
└─────────────────┘
```

**原创区卡片**（zone='original'，小红书式瀑布流）：

```
┌─────────────────┐
│                 │  ← 封面图（自适应高度，保持原始宽高比）
│   cover image   │    若 cover_image_url 为空 → 显示内容类型默认占位图
│                 │
│                 │
├─────────────────┤
│ 标题（2行截断） │
│ @作者            │
│ ❤ 88             │  ← 点赞数（小红书式：仅显示点赞数）
└─────────────────┘
```

原创区卡片简化规则：
- **封面图高度自适应**：图片/视频按原始宽高比渲染，不强制 3:4；文字类内容使用固定高度渐变背景 + 标题摘要
- **无 IP 名**：原创区无 IP 概念，不显示 IP 行
- **仅显示点赞数**：不显示评论数，降低信息密度
- **不显示标签 Badge**：原创区卡片不展示标签，保持视觉干净
- **悬停效果**：图片微放大 (scale-105) + 浅色遮罩层，类似小红书 hover 效果

#### 封面图默认占位规则

| content_type | 默认占位 |
|---|---|
| `article` | 随机生成渐变色背景 + 标题前两字 |
| `image` | 使用第一个 image 附件缩略图（含 GIF/表情包，通过标签区分） |
| `video` | 视频第一帧（由上传完成后异步截帧写入 cover_image_url） |
| `sheet_music` | 五线谱图案 SVG 占位 |
| `mod` / `template` | 积木/齿轮图案 SVG 占位 |
| `prompt` | AI 芯片图案 SVG 占位 |

---

### 10.6 浏览逻辑设计

#### 二创区首页 `/`

首页分三个区域（从上到下）：

1. **最近访问 IP**：横向滚动，localStorage 存储最近 5 个 IP
2. **IP 浏览区**：横向滚动 IPCard 列表，提供分类筛选 Tab（全部 / 游戏 / 影视 / 综艺 / 短剧 / 动画 / 漫画 / 小说 / 明星偶像 / 音乐 / 虚拟主播 / 其他）+ 排序方式（最热门 / 最新 / 最多内容），对应 `ips.category` 枚举
3. **二创内容浏览区**：瀑布流 ContentCard，提供内容类型筛选（全部 / 文字 / 图片 / 视频 / 音频 / Mod / AI提示词 / 乐谱 / 其他）+ 排序（最热门 / 最新 / 最多点击 / 最高好评率）+ 时间范围。展示所有 IP 的二创内容，不限特定 IP

#### 原创区首页 `/original`

参考小红书网页端设计，采用「推荐流 + 分类 Tab」单层导航结构，**无二级内容类型筛选**。

**布局结构**（从上到下）：

1. **顶部分类 Tab 栏**：横向滚动的分类 Tab（`推荐` + 11 个一级分类），`推荐` 为默认选中
2. **主内容区**：瀑布流 ContentCard，无限滚动加载（每页 24 条）

**Tab 行为**：

| Tab | 数据源 | 排序策略 |
|-----|-------|---------|
| 推荐（默认） | `GET /contents?zone=original&sort=recommended` | 推荐算法引擎（个性化 + 热门趋势混合，见 §12） |
| 美食 / 影视 / 游戏 / ... | `GET /contents?zone=original&category=xxx&sort=hot` | 该分类下的热门内容（view+like 加权） |

**设计要点**：
- 所有 Tab 下**不再提供二级内容类型筛选**（不区分图文/视频/音频等）
- 分类 Tab 从后端 `/api/v1/categories?zone=original&level=primary` 动态加载
- 用户发布时保留自选分类（`content_items.category`），推荐算法使用该字段作为内容特征
- 卡片采用小红书式简化设计：封面图自适应高度 + 标题（2 行截断） + @作者 + 点赞数（见 §10.5）

**响应式行为**：
- 移动 (≤700px)：2 列瀑布流，Tab 栏横向滚动
- 平板 (≤1100px)：3 列瀑布流
- PC (>1100px)：4 列瀑布流，最大宽度 1280px

#### 内容类型统一说明

**原创区内容分类**（`content_items.category`，落库枚举，按潜在用户量排序）：
影视(`film_tv`) | 游戏(`gaming`) | 文学(`literature`) | 宠物(`pet`) | 美食(`food`) | 美妆穿搭(`beauty_fashion`) | 家居(`home`) | 数码科技(`tech_digital`) | 旅行(`travel`) | 运动(`sports`) | 效率(`productivity`) — 共 11 个枚举值

- **前端分类 Tab**：`推荐`（默认，算法推荐流）+ 上述 11 个分类（从 API 动态加载）
- `推荐` 为**前端伪分类** — 不传 `category` 参数，后端走推荐引擎（see §12）
- 分类体系由后端 `categories` 表动态加载，管理员后台统一管理（增删改排序）
- **已移除二级内容类型筛选**（原"图文/视频/音频/文字/效率模板/模型与设计/其他"不再作为浏览筛选条件）

**`content_items.content_type` 保留用途**：
- 发布时记录内容格式（用于附件渲染策略：MarkdownRenderer / SheetMusicViewer / 视频播放器等）
- 推荐算法中作为辅助特征（非主导信号）
- 不暴露给原创区浏览端作为筛选维度

**IP 分类**（`ips.category`，二创区，按潜在用户量排序）：`gaming` | `film_tv` | `variety` | `short_drama` | `animation` | `comics` | `novel` | `celebrity_idol` | `music` | `vtuber` | `other`

**分类查询规则**：
- 原创区：通过 `content_items.category` 一级分类筛选（zone='original' 时有效）；已移除 content_type 二级筛选
- 二创区：通过 `content_items.ip_id` 关联 IP，IP 通过 `ips.category` 进行分类

---

## 11. 网页端 Agent（V0.4 MVP）

> **范围**：MVP 仅实现 Tier 1 最小可行集（1a 上传自动包装、1b 合规检测、2a 自然语言搜索、
> 2b 内容使用指导、3b 内容初审辅助），架构设计保留 Tier 2/3 扩展路径。
> 当前任务优先级低于现有 Agent（Tauri 客户端）和主站核心功能，本节仅供未来实现参考。

### 11.1 设计原则

- **Provider 抽象**：LLM 供应商通过统一接口隔离，MVP 实现通义千问，可插拔替换 OpenAI / DeepSeek
- **Tool-Call 模式（MVP）**：LLM 从白名单 tool 列表中选择调用，单轮或短链路执行，不使用自主 ReAct 循环
- **SSE 流式传输**：所有 Agent 流式响应使用 Server-Sent Events，不引入 WebSocket
- **Feature Flag 控制**：`agent.web_agent_enabled: false`（默认关闭，独立于 Tauri `features.agent_enabled`；配置位置见 §11.3）
- **零独立基础设施**：向量检索使用 pgvector（已有 PostgreSQL），不引入新服务

### 11.2 LLM Provider 抽象层（Go）

```go
// internal/pkg/llm/provider.go
type ChatMessage struct {
    Role    string `json:"role"`    // "system" | "user" | "assistant" | "tool"
    Content string `json:"content"`
}

type ChatRequest struct {
    Messages    []ChatMessage          `json:"messages"`
    Tools       []ToolDefinition       `json:"tools,omitempty"`
    Stream      bool                   `json:"stream"`
    MaxTokens   int                    `json:"max_tokens,omitempty"`
}

type ChatDelta struct {
    Content   string `json:"content"`
    ToolCalls []ToolCall `json:"tool_calls,omitempty"`
    Done      bool   `json:"done"`
}

type LLMProvider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatMessage, error)
    ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatDelta, error)
}
```

**实现类**：

| 实现 | 说明 |
|---|---|
| `QwenProvider` | 调用阿里云通义千问 API（`dashscope.aliyuncs.com`） |
| `OpenAICompatProvider` | 通用 OpenAI 协议（DeepSeek / 本地 Ollama 均可复用此实现） |

运行时通过 `config.yaml > agent.llm_provider` 选择，工厂函数 `llm.NewProvider(cfg)` 返回接口实例。

### 11.3 配置扩展（config.yaml）

> 完整字段列表以 `config/config.go > AgentConfig` 结构体为准。以下为关键字段摘要：

```yaml
agent:
  web_agent_enabled: false          # 网页端 Agent 总开关（独立于 Tauri agent_enabled）
  llm_provider: qwen                # qwen | openai_compat
  llm_model: qwen-turbo             # 模型名称（随 provider 变）
  llm_api_base: ""                  # 留空则使用 provider 默认 endpoint
  llm_api_key: ""                   # 从 env: AGENT_LLM_API_KEY 注入
  embedding_model: text-embedding-v3
  embedding_dimensions: 1536
  rate_limit_per_day: 50
  upload_assist_max_file_mb: 10
  max_user_message_chars: 4000      # 用户消息最大字符数
  chat_max_context_messages: 10     # 对话上下文最大消息数
  hmac_secret: ""                   # Tauri Agent HMAC 签名密钥
```

**环境变量**（不写入 config.yaml）：

```
AGENT_LLM_API_KEY=sk-xxx
```

**LLM 配置优先级**（DB 优先于 config.yaml）：

```
读取顺序：llm_configs 表 (is_active=TRUE) → 若无激活记录 → 降级到 config.yaml agent.* 配置
管理员通过 /admin/agent-config 可视化界面管理 LLM 配置，支持多配置切换和连接测试。
internal/pkg/llm/factory.go 的 NewProvider(cfg) 需扩展为先查 DB active config，找不到再 fallback 到 config.yaml。
```

### 11.4 数据库 Schema

```sql
-- 启用 pgvector 扩展（一次性执行）
CREATE EXTENSION IF NOT EXISTS vector;

-- 对话历史
CREATE TABLE agent_conversations (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT REFERENCES users(id) ON DELETE CASCADE,
    context_type    VARCHAR(50) NOT NULL,  -- 'upload_assist'|'compliance'|'search'|'usage_guide'|'moderation'|'general'
    context_id      BIGINT,                -- 关联 content_item_id（可为 NULL）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON agent_conversations (user_id);

CREATE TABLE agent_messages (
    id                  BIGSERIAL PRIMARY KEY,
    conversation_id     BIGINT REFERENCES agent_conversations(id) ON DELETE CASCADE,
    role                VARCHAR(20) NOT NULL,  -- 'user'|'assistant'|'tool'
    content             TEXT NOT NULL,
    tool_calls          JSONB,
    -- tool_calls 结构（遵循 OpenAI Chat Completions tool_calls 数组格式）：
    -- [{ "id": "call_xxx", "type": "function",
    --    "function": { "name": "search_contents", "arguments": "{...json...}" } }, ...]
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON agent_messages (conversation_id);

-- 内容向量嵌入（用于语义搜索）
CREATE TABLE content_embeddings (
    content_item_id     BIGINT PRIMARY KEY REFERENCES content_items(id) ON DELETE CASCADE,
    embedding           vector(1536) NOT NULL,
    embedded_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- IVFFlat 索引（适合百万级以内向量；P2 阶段可升级为 HNSW）
CREATE INDEX ON content_embeddings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

### 11.5 API 路由

所有 Agent 接口挂载于 `/api/v1/agent/`，需登录（JWT），受 Redis 限流。

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/agent/upload-assist` | 1a 上传自动包装：解析已上传文件，返回建议标签/分类/标题/简介 |
| `POST` | `/agent/compliance-check` | 1b 合规检测：文件格式 + Aliyun 内容安全 + LLM 侵权分析 |
| `POST` | `/agent/search` | 2a 自然语言搜索：query → embedding → pgvector → 排序返回 |
| `POST` | `/agent/usage-guide` | 2b 使用指导：基于内容 metadata + 附件信息生成安装/使用说明 |
| `POST` | `/agent/moderate` | 3b 内容初审：Aliyun Safety + LLM 综合分析，返回风险等级 + 整改建议 |
| `POST` | `/agent/chat/stream` | 通用流式对话（SSE），供上述功能的流式变体复用 |

**请求/响应约定**：

```jsonc
// POST /agent/upload-assist
{
  "content_item_id": 123,   // 已创建（草稿状态）的内容 ID
  "file_keys": ["oss/xxx/mod.zip"]  // OSS 文件 key 列表
}
// Response（非流式）
{
  "suggested_tags": ["Minecraft", "家具", "1.20+"],
  "suggested_category": "mod",
  "suggested_title": "简约北欧风家具包 v2.0",
  "suggested_description": "...",
  "conversation_id": 456
}

// POST /agent/chat/stream → SSE
data: {"delta": "根据你的文件...", "done": false}
data: {"delta": "建议标签：", "done": false}
data: {"done": true, "conversation_id": 456}
```

### 11.6 MVP Tool 白名单

Agent 在 Tool-Call 模式下只能调用以下内部工具，不可执行任意代码：

| Tool 名称 | 签名 | 说明 |
|---|---|---|
| `search_contents` | `(query string, filters map) → []ContentSummary` | 调用已有内容搜索逻辑 |
| `get_content_detail` | `(id int64) → ContentDetail` | 获取内容详情（含 attachments、tags） |
| `get_tag_list` | `(category string) → []Tag` | 获取标签列表（辅助打标） |
| `check_file_format` | `(file_key string) → FormatReport` | 校验文件扩展名、MIME、基本结构 |
| `call_aliyun_safety` | `(text string) → SafetyResult` | 调用内容安全 API |
| `vector_search` | `(query_embedding []float32, topk int) → []ContentSummary` | pgvector 语义检索 |

新工具须经过评审后加入白名单，**Agent 不可自定义或组合超出白名单的调用**。

### 11.7 向量化 Pipeline

内容发布/更新时触发异步向量化任务（与现有 AI 审核任务同级）：

```
内容发布 → content_service.PublishContent()
  └── 异步 goroutine:
        1. 拼接向量化文本：title + description + tags（joined）
        2. 调用 LLM embedding API（embedding_model）
        3. 写入 content_embeddings（upsert）
```

自然语言搜索流程：

```
用户 query
  → POST /agent/search
  → agent_service.Search():
      1. 调用 embedding API 向量化 query
      2. pgvector cosine 相似度检索（topk=20）
      3. LLM 对结果排序/过滤/摘要
      4. 返回结构化结果列表
```

向量化失败不阻断发布流程，失败时记录日志、稍后重试。

### 11.8 限流实现

```
Redis Key: agent:rl:{user_id}:{YYYY-MM-DD}
操作: INCR → 若结果 > rate_limit → 返回 429
过期: EXPIRE 86400（当日到期自动清零）
```

超限响应：`HTTP 429 { "code": "AGENT_RATE_LIMIT_EXCEEDED", "reset_at": "2026-04-16T00:00:00Z" }`

### 11.9 前端 Agent 组件

| 组件 | 位置 | 说明 |
|---|---|---|
| `AgentChatWidget.tsx` | 全站右下角悬浮 | 通用对话入口，`web_agent_enabled=false` 时不渲染 |
| `UploadAssistPanel.tsx` | 发布页（publish/page.tsx）| 上传完成后显示「AI 自动填写」按钮，调用 `/agent/upload-assist` |
| `ComplianceCheckBadge.tsx` | 发布页确认区 | 展示合规检测结果（通过/风险/违规），支持流式进度 |
| `SearchAgentInput.tsx` | 搜索页（/search） | 自然语言搜索输入框，替换关键词搜索框；降级兼容普通搜索 |
| `UsageGuidePanel.tsx` | 内容详情页侧边 | 「AI 使用指导」折叠卡片，按需加载 |

SSE 流式渲染复用 `lib/useSSE.ts` hook（封装 `EventSource`）。

---

## 12. 推荐引擎（原创区推荐流）

> **定位**：为原创区 `/original` 的「推荐」Tab 提供算法驱动的个性化内容推送，参考小红书推荐流的体验。本模块独立于网页端 Agent（§11），是独立的推荐子系统。

### 12.1 设计目标

- 用户打开原创区默认进入「推荐」Tab，看到个性化内容流
- 新用户（< 10 次互动）看到热门趋势内容
- 有交互历史的用户看到兴趣相关 + 热门内容混合推荐
- 推荐结果每 2 小时刷新缓存，保证内容新鲜度

### 12.2 推荐算法

#### 评分公式

```
final_score = α × sim_score + (1 - α) × hot_score

其中：
  α             = config.recommendation.personalization_weight（默认 0.6）
  sim_score     = cosine_similarity(user_profile_embedding, content_embedding)
  hot_score     = log(1 + view_count + like_count × 3) × time_decay
  time_decay    = 2 ^ (-age_hours / hot_decay_hours)
```

#### 冷启动（新用户 / 低互动用户）

当用户 `browse_history + favorites + reactions` 总条目 < `min_interaction_for_personalize`（默认 10）时：
- 直接使用 `rank:hot:contents` Redis Sorted Set 返回热门内容（同类于二创区首页 hot 排序）
- 不计算个性化相似度

#### 个性化推荐流程

```
用户请求 GET /contents?zone=original&sort=recommended
  │
  ▼
1. 检查用户互动量 → 不足阈值 → 降级为热门推荐
  │ 充足
  ▼
2. 构建用户画像向量（user_profile_embedding）
   - 从 browse_history 取最近 50 条浏览内容的 content_embeddings 取均值
   - 从 favorites 取收藏内容的 embedding 加权（×2）
   - 从 reactions（like）取点赞内容的 embedding 加权（×1.5）
  │
  ▼
3. pgvector 向量检索
   - 在 content_embeddings 表中 ANN 检索 topK（默认 200）候选
   - 过滤：zone='original', status='published', 排除已浏览/已互动内容
  │
  ▼
4. 计算 final_score 并排序
   - sim_score: 候选内容 embedding 与 user_profile_embedding 的余弦相似度
   - hot_score: 从 Redis rank:hot:contents 获取热度分数
   - 按 final_score DESC 排序，取 top 200
  │
  ▼
5. 分页返回（page/page_size 在内存中分页）
   - 写入 Redis 缓存（key: `rec:original:{user_id}:{page}`，TTL = refresh_interval_h）
```

### 12.3 数据依赖

| 数据 | 来源 | 说明 |
|------|------|------|
| `content_embeddings` | pgvector 表（§11.4） | 内容向量，由内容发布/更新时异步生成（复用 §11.7 向量化 Pipeline） |
| `browse_history` | PostgreSQL 表（§4.5） | 用户浏览历史，用于构建用户画像 |
| `favorites` | PostgreSQL 表（§4.5） | 用户收藏，画像构建中加权 ×2 |
| `reactions` | PostgreSQL 表（§4.5） | 用户点赞（like），画像构建中加权 ×1.5 |
| `rank:hot:contents` | Redis Sorted Set | 全站内容热度排行，由定时任务更新 |

### 12.4 API 变更

`GET /api/v1/contents?zone=original&sort=recommended`

- 新增 `sort=recommended` 枚举值
- `sort=recommended` 且 `zone=original` 时走推荐引擎
- `sort=recommended` 且 `zone != original` 时降级为 `sort=hot`
- 无需登录也可使用（未登录用户走纯热门推荐）

### 12.5 定时任务

| 任务 | 频率 | 说明 |
|------|------|------|
| 热门排行更新 | 每 10 分钟 | 计算近 `trending_window_days` 天内容的 hot_score，更新 `rank:hot:contents` |
| 推荐缓存刷新 | 每 `refresh_interval_h` 小时 | 清除过期推荐缓存 |
| 向量化补齐 | 每分钟 | 检查 content_embeddings 中缺失的新内容并补生成（复用 §11.7 Pipeline） |

### 12.6 配置项

所有推荐参数从 `config.yaml > recommendation` 读取（见 §7），支持管理员热更新。具体参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `enabled` | true | 推荐引擎全局开关，关闭时 `sort=recommended` 降级为 `sort=hot` |
| `hot_decay_hours` | 48 | 热门度半衰期 |
| `personalization_weight` | 0.6 | 个性化 vs 热门的混合比例 |
| `min_interaction_for_personalize` | 10 | 启用个性化的最低互动次数 |
| `embedding_topk` | 200 | pgvector ANN 检索候选集大小 |
| `trending_window_days` | 7 | 热门趋势计算窗口 |
| `refresh_interval_h` | 2 | 推荐缓存刷新间隔 |

---

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

## 14. 安全加固（Task 99-105）

> 本章节对应 task.json 任务 99-105，记录安全漏洞修复的架构决策。

### 14.1 Admin Config 泄露防护

- `GET /admin/config` 和 `PATCH /admin/config` 仅返回 `PublicConfig` 结构体（Features、Limits、Reputation、Judge、Social、Recommendation 等安全配置字段）
- 所有敏感字段（JWT.Secret、OSS.AccessKeySecret、Green.AccessKeySecret、Agent.LLMAPIKey、Agent.HMACSecret、Database.DSN）在 Config 结构体中标记 `json:"-"`
- 管理员运行时配置修改持久化到 `data/config_override.yaml`，重启后合并加载

### 14.2 CORS 策略

- CORS 中间件不再使用 `Access-Control-Allow-Origin: *`，改为从 `config.yaml > allowed_origins` 列表动态匹配请求 Origin
- 开发环境允许 `localhost` 全系列；生产环境必须配置具体域名
- 所有 POST/PATCH/DELETE 请求要求 `X-CSRF-Token` 请求头（Task 104）

### 14.3 Auth 中间件实时状态检查

- `AuthRequired` 中间件在 JWT 验证后增加 Redis 用户状态缓存检查：`user:status:{userID}` → `{"is_banned":bool,"role":"string"}`，TTL=5min
- `AdminRequired` 在 AuthRequired 基础上从缓存检查 `role=="admin"`
- 用户被 ban 或角色变更后立即更新缓存，无需等 token 过期
- 登录/注册时写入缓存；BanUser/UnbanUser/角色变更时更新缓存

### 14.4 错误消息脱敏

- 创建 `internal/pkg/response/safe_error.go`：业务已知错误（ErrContentNotFound etc.）返回原始消息，未知错误统一返回"操作失败，请稍后重试"
- 创建 `internal/pkg/response/error_codes.go`：错误码到 HTTP 状态码的标准映射表
- 全局替换 `c.JSON(500, gin.H{"error": err.Error()})` 为 `SafeErrorResponse(c, err)`
- 内部错误仅记录到结构化日志（slog），不返回客户端

### 14.5 账号删除与 OSS 路径隔离

- `DeleteAccount` 将 `password_hash` 设为 `bcrypt.GenerateFromPassword(randomBytes(32), 14)` 而非空字符串
- OSS 预签名 URL 的 key 必须以 `uploads/{userID}/` 为前缀，handler 校验路径前缀与当前用户 ID 匹配

### 14.6 Goroutine Panic Recovery

- 创建 `internal/pkg/recovery.go`：`GoSafe(fn func())` 包装 `defer recover + slog.Error`
- 所有异步 goroutine（submitIPForAIReview、EmbedContentAsync、triggerVideoSnapshot 等）使用 `go recovery.GoSafe(func() { ... })`
- 19 处静默丢弃错误（`_ = h.contentRepo.IncrViewCount(id)` 等）改为 `slog.Error + 优雅降级`

---

## 15. 架构优化（Task 106-145）

### 15.1 前端受保护路由守卫

- 新增 `(protected)/layout.tsx`：统一鉴权守卫，未登录用户重定向到 `/login`，消除了各页面独立 `useAuth()` 检查导致的内容闪烁
- admin 路由在 auth 守卫基础上叠加角色检查

### 15.2 Service Container 模式

- 创建 `internal/container/container.go`：统一初始化所有 repo → service → handler，通过依赖注入避免 service 重复实例化
- `cmd/server/main.go` 中初始化 `ServiceContainer` 并注入所有 handler
- 每个 service 只初始化一次，handler 从 container 获取 service 实例

### 15.3 通知自动创建

- 创建 `internal/service/notification_service.go`：`CreateNotification(userID, channel, type, title, body, targetType, targetID)`
- 在 CommentService、SocialService、FollowService、PRService、AppealService、MessageService 中调用
- 所有通知创建使用异步 goroutine（`go recovery.GoSafe()`），不阻塞主流程
- 数据库索引 `idx_notifications_user_unread ON notifications(user_id, is_read, created_at DESC)`

### 15.4 内容全文搜索

- 新增迁移 `038_content_search.sql`：在 `content_items` 表添加 `search_vector tsvector` 列，创建 GIN 索引；创建触发器在 title/description 变更时自动更新
- 新增 `GET /contents/search?q=&zone=&category=&content_type=&tags=&sort=relevance&page=&limit=20`
- 使用 `to_tsquery + ts_rank` 实现全文检索，`ts_headline` 返回高亮片段
- 应用 `OptionalAuth` 中间件：登录用户可个性化排序，匿名用户纯相关性排序

### 15.5 搜索建议与热门搜索

- `GET /search/suggestions?q=&limit=10`：tags name 前缀匹配 + content_items title 前缀匹配，返回 Top 10
- `GET /search/trending?limit=20`：从 Redis hot_rank 数据提取热门关键词
- 搜索建议按使用频率排序（Redis 计数）

### 15.6 内容下载 API

- `GET /contents/:id/download`：校验登录态 + `AllowCopy=true` 后返回 OSS 签名下载 URL（302 重定向）
- Redis 维护下载计数 ZSET `download_counts`
- `content_items` 新增 `download_count` 字段（或独立 Redis 维护）

### 15.7 收藏去重与分页筛选

- `AddFavorite` 幂等性：已收藏返回 200 而非 500
- `RemoveFavorite` 安全处理：删除不存在记录返回 200
- `GET /users/:id/favorites` 增加 `content_type`、`page`、`limit` 查询参数，支持分页元数据
- `GET /contents/:id` 增加 `is_favorited` 字段（OptionalAuth）

### 15.8 内容软删除

- 新增迁移 `041_content_soft_delete.sql`：在 `content_items` 表添加 `deleted_at TIMESTAMPTZ` 字段（nullable，索引）
- `DeleteContent` 改为 `UPDATE content_items SET deleted_at=NOW() WHERE id=?`
- 所有内容列表查询添加 `WHERE deleted_at IS NULL` 条件（GORM DefaultScope 或全局 scope）
- 新增 `GET /admin/contents/trash`：管理员查看已软删除内容
- 新增 `PATCH /admin/contents/:id/restore`：恢复软删除内容
- 定时任务：软删除 30 天后永久删除（管理后台手动触发或后台 cron）

### 15.9 数据库 Schema 变更汇总（Task 99-145 新增）

> 注：以下为原始计划编号。实际迁移文件编号因并行开发已发生偏移，最新编号见 `backend/migrations/` 目录（001-056）。
> 关键错位：计划 038=全文搜索 → 实际 038=notification_channels, 041=content_search_vector；
> 计划 039=密码重置 → 实际 039=conversation_unread_count；
> 计划 040=P0 修复（含 download_count/ban_reason/email_verified_at）；
> 计划 041=内容软删除 → 实际 053=content_items_soft_delete。

```sql
-- P0 修复（实际迁移 040_p0_fixes.sql）
ALTER TABLE content_items ADD COLUMN ban_reason TEXT;
ALTER TABLE content_items ADD COLUMN download_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;

-- 全文搜索（实际迁移 041_content_search_vector.sql）
ALTER TABLE content_items ADD COLUMN search_vector tsvector;
CREATE INDEX idx_content_items_search ON content_items USING GIN(search_vector);

-- 密码重置（实际迁移 050_verification_and_terms.sql，与其他验证功能合并）
CREATE TABLE password_reset_tokens (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       VARCHAR(255) UNIQUE NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ
);
CREATE INDEX idx_password_reset_token ON password_reset_tokens(token, expires_at);
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;

-- 内容软删除（实际迁移 053_content_items_soft_delete.sql）
ALTER TABLE content_items ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_content_items_deleted ON content_items(deleted_at) WHERE deleted_at IS NOT NULL;

-- User.ID 类型统一（uint → int64）
-- 注意：此迁移需要将 users.id 从 SERIAL 改为 BIGSERIAL（若当前为 INTEGER）
-- 所有引用 users.id 的外键列同步改为 BIGINT
```

### 15.10 新增 API 路由汇总（Task 99-145）

> 路由已合并至 §3.2 API 路由清单。本节仅保留作为 Task 99-145 的历史范围参考。

### 15.11 结构化日志与优雅关机

- 使用 Go 1.21+ 内置 `slog` 包替换 `fmt.Printf`
- 新增 `internal/middleware/request_id.go`：为每个请求生成唯一 `X-Request-ID`（UUID）
- 新增 `internal/middleware/logging.go`：使用 slog.Info 记录请求方法、路径、状态码、耗时、Request-ID
- 服务启动使用 `http.Server` + `srv.Shutdown(ctx)` 支持优雅关机（15秒超时）
- 配置修改持久化到 `data/config_override.yaml`，启动时合并加载

### 15.12 前端数据缓存与暗色模式修复

- 引入 SWR 或 @tanstack/react-query 作为全局数据缓存层，避免每个页面独立 fetch
- AuthContext 使用 SWR 的 mutate 实现登录/登出后缓存失效
- Sidebar.tsx、StudioSidebar.tsx、Footer.tsx 中所有硬编码颜色替换为 Tailwind 语义化类（bg-white → bg-background 等）
- 新增 `(protected)/layout.tsx` 统一鉴权守卫
- 硬编码中文字符串全部提取到 `messages/zh.json` 和 `messages/en.json`
- 首页/原创区统计数字从 API 动态获取（`GET /stats/summary`）
- 注册成功后直接使用返回 token，不再二次调用 login()
- API 层 401 响应自动尝试 refresh token，多并发请求仅触发一次 refresh
