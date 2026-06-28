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
│   ├── user/[userId]/page.tsx       # 用户主页（公开可浏览）
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
│   │   ├── followers/page.tsx       # 粉丝分析（新增）
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
│   ├── feedback/page.tsx            # 用户反馈
│   └── studio/favorites/page.tsx    # 我的收藏（收藏集列表）
├── (protected)/                     # 需要登录（续）
│   └── collections/[id]/page.tsx    # 收藏集详情
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

```
POST   /api/v1/auth/register          # 注册
POST   /api/v1/auth/login             # 登录
POST   /api/v1/auth/logout            # 登出
POST   /api/v1/auth/refresh           # 刷新 Token
GET    /api/v1/auth/me                # 当前用户信息
GET    /api/v1/auth/csrf              # 获取 CSRF Token
POST   /api/v1/auth/forgot-password   # 发送密码重置邮件
POST   /api/v1/auth/reset-password    # 密码重置（token 校验）
POST   /api/v1/auth/verify-email      # 邮箱验证
POST   /api/v1/auth/resend-verification # 重新发送验证邮件

GET    /api/v1/users/:id              # 用户主页
PATCH  /api/v1/users/:id              # 更新用户信息
GET    /api/v1/users/:id/contents     # 用户发布的内容
GET    /api/v1/users/:id/favorites    # 用户收藏列表
GET    /api/v1/users/:id/reputation   # 信誉分详情
POST   /api/v1/users/:id/follow       # 关注用户
DELETE /api/v1/users/:id/follow       # 取消关注用户
GET    /api/v1/users/:id/followers    # 粉丝列表
GET    /api/v1/users/:id/followers/stats  # 粉丝统计（总数/趋势/来源，参数 ?days=30）
GET    /api/v1/users/:id/following    # 关注列表
DELETE /api/v1/users/me               # 账号注销（软删除/匿名化，需双重验证）
PATCH  /api/v1/users/me/password      # 修改密码（旧密码验证 + bcrypt 加密）
PATCH  /api/v1/users/me/support-info  # 更新创作者支持信息
GET    /api/v1/users/me/contents      # 我的内容列表
GET    /api/v1/users/me/followers/stats # 我的粉丝统计
POST   /api/v1/users/me/history       # 记录浏览历史
GET    /api/v1/users/me/history       # 浏览历史列表
DELETE /api/v1/users/me/history       # 清空浏览历史

GET    /api/v1/ips                    # IP 列表（搜索+筛选+排序）
POST   /api/v1/ips                    # 创建 IP（提交审核）
GET    /api/v1/ips/:id                # IP 详情
GET    /api/v1/ips/:id/contents       # IP 下的内容列表（分类+排序）
GET    /api/v1/ips/:id/discussions    # IP 讨论区
POST   /api/v1/ips/:id/discussions    # 在 IP 下发帖
GET    /api/v1/ips/:id/discussions/search # 搜索 IP 内讨论
POST   /api/v1/ips/:id/follow         # 关注 IP
DELETE /api/v1/ips/:id/follow         # 取消关注 IP
GET    /api/v1/ips/stats/category_counts # IP 分类统计

GET    /api/v1/contents               # 内容列表（原创区/首页）
                                          # sort: 'hot' | 'new' | 'most_views' | 'best_rated' | 'recommended'（原创区推荐流）
POST   /api/v1/contents               # 发布内容
GET    /api/v1/contents/:id           # 内容详情
PATCH  /api/v1/contents/:id           # 更新内容（仅作者）
DELETE /api/v1/contents/:id           # 删除内容（仅作者）
POST   /api/v1/contents/:id/report    # 举报内容
GET    /api/v1/contents/:id/related-fanworks # 原创内容的相关二创列表

POST   /api/v1/studio/contributors/:userId/block    # 作者拉黑贡献者（写 author_blocklist）
DELETE /api/v1/studio/contributors/:userId/block    # 解除拉黑（注：系统已迁移至 /studio/*，旧 /dashboard/contributors/* 路径保留兼容但推荐使用新路径）

POST   /api/v1/appeals                  # 用户提交申诉（对被下架内容）
GET    /api/v1/appeals/me               # 我的申诉列表
GET    /api/v1/contents/:id/versions  # 版本历史
POST   /api/v1/contents/oss-token     # 获取 OSS 预签名上传 URL

GET    /api/v1/versions/:id           # 版本详情（含 diff）
POST   /api/v1/pr                     # 提交 PR（修改申请）
GET    /api/v1/pr/:id                 # PR 详情（含 diff 对比）
POST   /api/v1/pr/:id/accept          # 接受 PR
POST   /api/v1/pr/:id/reject          # 拒绝 PR
POST   /api/v1/pr/:id/merge           # 手动合并（含合并结果提交）
GET    /api/v1/contents/:id/prs       # 内容的 PR 列表

GET    /api/v1/social/comments         # 评论列表
POST   /api/v1/social/comments        # 发布评论
PATCH  /api/v1/social/comments/:id    # 编辑评论
DELETE /api/v1/social/comments/:id    # 删除评论
POST   /api/v1/social/comments/:id/report # 举报评论
POST   /api/v1/social/reactions       # 点赞/点踩
GET    /api/v1/social/reactions       # 获取 reactions 列表
POST   /api/v1/favorites              # 收藏内容
DELETE /api/v1/favorites/:contentId   # 取消收藏
GET    /api/v1/users/:id/favorites    # 用户收藏列表（分页）
GET    /api/v1/social/discussions     # 讨论帖列表
POST   /api/v1/social/discussions     # 发帖
GET    /api/v1/social/discussions/:id # 帖子详情
GET    /api/v1/users/:id/discussions  # 用户讨论列表
POST   /api/v1/discussions/:id/comments # 回复讨论帖
PATCH  /api/v1/discussions/:id/pin   # 置顶/取消置顶讨论帖

GET    /api/v1/judge/exam/:category          # 获取考题（按内容类型）
POST   /api/v1/judge/exam/submit             # 提交考题答案
GET    /api/v1/judge/queue                   # 待审内容队列
POST   /api/v1/judge/vote                    # 提交众裁投票
GET    /api/v1/judge/cases/:id/verdict       # 获取判决定案详情（含投票分布）
POST   /api/v1/judge/reasons/:id/vote        # 点赞/点踩判官提交的判定理由

POST   /api/v1/agent/script                  # [未实现] 获取 Agent 执行脚本（Tauri 调用，D-03 后启用）
POST   /api/v1/agent/verify                  # [未实现] 验证脚本签名（Tauri 调用，D-03 后启用）

GET    /api/v1/admin/ips                     # 管理员：IP 审核列表
POST   /api/v1/admin/ips/:id/approve         # 审核通过
POST   /api/v1/admin/ips/:id/reject          # 审核拒绝
GET    /api/v1/admin/contents                # 终审内容列表
POST   /api/v1/admin/contents/:id/ban        # 封禁内容
GET    /api/v1/admin/contents/trash          # 已软删除内容列表
PATCH  /api/v1/admin/contents/:id/restore    # 恢复软删除内容
GET    /api/v1/admin/users                   # 用户列表
POST   /api/v1/admin/users/:id/ban           # 封禁用户
POST   /api/v1/admin/users/:id/unban         # 解封用户
GET    /api/v1/admin/appeals                 # 申诉列表（读取 appeals 表 status=pending）
POST   /api/v1/admin/appeals/:id             # 处理申诉（approved/rejected，approved 则恢复内容）
GET    /api/v1/admin/reports                 # 举报列表
PATCH  /api/v1/admin/reports/:id             # 处理举报
GET    /api/v1/admin/reports/stats           # 举报统计
GET    /api/v1/admin/config                  # 获取系统配置
PATCH  /api/v1/admin/config                  # 更新系统配置
POST   /api/v1/admin/judge/questions         # 创建考题
GET    /api/v1/admin/categories              # 分类列表
POST   /api/v1/admin/categories              # 新增分类
PATCH  /api/v1/admin/categories/:id          # 编辑分类
DELETE /api/v1/admin/categories/:id          # 删除分类
PUT    /api/v1/admin/categories/reorder      # 排序
GET    /api/v1/admin/llm-configs             # LLM 配置列表
POST   /api/v1/admin/llm-configs             # 新增 LLM 配置
PATCH  /api/v1/admin/llm-configs/:id         # 编辑 LLM 配置
DELETE /api/v1/admin/llm-configs/:id         # 删除 LLM 配置
POST   /api/v1/admin/llm-configs/:id/activate # 切换激活配置
POST   /api/v1/admin/llm-configs/:id/test    # 测试连接
GET    /api/v1/admin/feedback                # 反馈工单列表
GET    /api/v1/admin/feedback/:id            # 查看单条反馈
PATCH  /api/v1/admin/feedback/:id            # 更新反馈状态/优先级
POST   /api/v1/admin/feedback/:id/replies    # 管理员回复反馈
GET    /api/v1/admin/audit-logs              # 管理员审计日志
GET    /api/v1/admin/queue/stats             # 异步任务队列状态
GET    /api/v1/admin/queue/dlq               # 死信队列

GET    /api/v1/notifications                 # 通知列表（?channel=reply|like|system）
PATCH  /api/v1/notifications/:id/read        # 标记通知已读
POST   /api/v1/notifications/read-all        # 全部已读（?channel=xxx）
GET    /api/v1/notifications/unread-count    # 各频道未读计数

GET    /api/v1/conversations                 # 我的对话列表
POST   /api/v1/conversations                 # 创建对话（发起私信）
GET    /api/v1/conversations/:id/messages    # 对话消息列表
POST   /api/v1/conversations/:id/messages    # 发送消息
PATCH  /api/v1/conversations/:id/read        # 标记对话已读

GET    /api/v1/rehab/courses                 # 获取可用课程列表（基于用户扣分记录）
GET    /api/v1/rehab/courses/:id             # 课程详情（含 AI 生成教学内容）
POST   /api/v1/rehab/courses/:id/start       # 开始学习（记录 started_at）
POST   /api/v1/rehab/courses/:id/complete    # 完成学习（校验阅读时间 ≥ 180s，加信誉分）
GET    /api/v1/rehab/my-progress             # 我的课程完成进度

GET    /api/v1/categories           # 公开分类列表（?zone=&level=&parent_id=）

GET    /api/v1/reputation-logs/me          # 我的信誉分变动日志

# 内容搜索与下载
GET    /api/v1/contents/search              # 全文搜索（OptionalAuth）
GET    /api/v1/search/suggestions           # 搜索建议（前缀匹配）
GET    /api/v1/search/trending              # 热门搜索词
GET    /api/v1/users/search                 # 用户搜索
GET    /api/v1/contents/:id/download        # 内容下载（返回 OSS 签名 URL）

# 验证码
POST   /api/v1/captcha/verify               # 验证码校验

# 公共配置
GET    /api/v1/config/public                # 公共运行时配置（前端功能开关）

# 用户反馈（用户端）
POST   /api/v1/feedback                     # 提交反馈工单
POST   /api/v1/feedback/attachments/presign # 上传附件预签名 URL
GET    /api/v1/feedback/me                  # 我的反馈列表
GET    /api/v1/feedback/:id                 # 查看反馈工单详情

# 消息管理
DELETE /api/v1/messages/:id                 # 软删除消息
DELETE /api/v1/conversations/:id            # 退出/删除会话

# 平台统计
GET    /api/v1/stats/summary                # 平台统计数据

# Agent 对话持久化
GET    /api/v1/agent/conversations          # 对话列表
GET    /api/v1/agent/conversations/:id      # 对话详情

# 内部回调
POST   /api/v1/internal/ai-callback         # 阿里云内容安全回调

# 桌面端部署
POST   /api/v1/deploy-grants                # 桌面端部署授权
```

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

### 4.1 用户与权限

```sql
-- 用户表
CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    email           VARCHAR(255) UNIQUE NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    username        VARCHAR(64) UNIQUE NOT NULL,
    avatar_url      TEXT,
    bio             TEXT,
    reputation      INT NOT NULL DEFAULT 10,
    preferred_locale VARCHAR(10) NOT NULL DEFAULT 'zh-CN',  -- 'zh-CN' | 'en-US'
    role            VARCHAR(20) NOT NULL DEFAULT 'user',
    -- role: 'user' | 'creator' | 'admin'（判官身份由 judge_qualifications 表管理，不作为 role 值）
    is_banned       BOOLEAN NOT NULL DEFAULT FALSE,
    ban_reason      TEXT,
    support_info    JSONB DEFAULT '{}',    -- 创作者支持信息：{ "donation_image_url": "...", "external_links": [{"title":"...","url":"..."}] }
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 判官资质表（记录各内容类型的判官权限）
CREATE TABLE judge_qualifications (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_type    VARCHAR(50) NOT NULL,
    -- content_type: 'article' | 'image' | 'video' | 'audio' | 'prompt' | 'comment' | 'mod'
    --             | 'sheet_music' | 'template' | 'other'
    -- 与 content_items.content_type 对齐，额外含 'comment' 用于评论众裁
    qualified_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE(user_id, content_type)
);

-- 信誉分变动日志
CREATE TABLE reputation_logs (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    delta           INT NOT NULL,
    reason          VARCHAR(100) NOT NULL,
    -- reason: 'quality_content' | 'pr_merged' | 'quality_comment' | 'tag_recognized'
    --         | 'judge_accuracy' | 'rehab_course' | 'valid_report'
    --         | 'malicious_content' | 'malicious_pr' | 'malicious_comment'
    --         | 'malicious_report' | 'malicious_tag_report' | 'judge_error'
    related_id      BIGINT,   -- 关联内容/评论/PR 的 ID
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- OAuth 预留表（P1 使用）
CREATE TABLE oauth_accounts (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        VARCHAR(20) NOT NULL,  -- 'github' | 'wechat'
    provider_uid    VARCHAR(255) NOT NULL,
    access_token    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_uid)
);
```

### 4.2 IP 库

```sql
-- IP 表
CREATE TABLE ips (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) UNIQUE NOT NULL,  -- URL 友好名称
    description     TEXT,
    cover_url       TEXT,
    category        VARCHAR(50),   -- 'gaming' | 'film_tv' | 'variety' | 'short_drama' | 'animation' | 'comics' | 'novel' | 'celebrity_idol' | 'music' | 'vtuber' | 'other'
    creator_id      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- status: 'pending' | 'approved' | 'rejected' | 'banned'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ips_status ON ips(status);
CREATE INDEX idx_ips_category ON ips(category);
CREATE INDEX idx_ips_name ON ips USING GIN(to_tsvector('simple', name));

-- IP 审核日志
CREATE TABLE ip_review_logs (
    id              BIGSERIAL PRIMARY KEY,
    ip_id           BIGINT NOT NULL REFERENCES ips(id) ON DELETE CASCADE,
    reviewer_id     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action          VARCHAR(20) NOT NULL,  -- 'ai_scan' | 'judge_vote' | 'admin_approve' | 'admin_reject'
    reason          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ip_review_logs_ip ON ip_review_logs(ip_id, created_at DESC);

-- IP 标签
CREATE TABLE ip_tags (
    ip_id           BIGINT NOT NULL REFERENCES ips(id) ON DELETE CASCADE,
    tag             VARCHAR(50) NOT NULL,
    PRIMARY KEY(ip_id, tag)
);
```

### 4.3 内容（统一多类型）

```sql
-- 内容主表（二创区 + 原创区统一）
CREATE TABLE content_items (
    id              BIGSERIAL PRIMARY KEY,
    title           VARCHAR(500) NOT NULL,
    description     TEXT,
    author_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    zone            VARCHAR(10) NOT NULL,   -- 'fanwork' | 'original'
    ip_id           BIGINT REFERENCES ips(id) ON DELETE SET NULL, -- 仅二创区有值
    source_original_id BIGINT REFERENCES content_items(id) ON DELETE SET NULL, -- 二创来源原创，仅 zone='fanwork' 可用
    category        VARCHAR(50),   -- 原创区内容分类（仅 zone='original'）：'film_tv' | 'gaming' | 'literature' | 'pet' | 'food' | 'beauty_fashion' | 'home' | 'tech_digital' | 'travel' | 'sports' | 'productivity'
    -- 应用层约束（GORM Validate Tag）：zone='original' 时 category 必填且须为上述枚举；zone='fanwork' 时 category 须为 NULL。source_original_id 仅允许 fanwork 绑定已发布 original。枚举扩展时在 config/categories 种子数据中补充，不加 DB CHECK 约束（避免迁移负担）
    content_type    VARCHAR(20) NOT NULL,
    -- content_type: 'article' | 'image' | 'video' | 'audio'
    --              | 'mod' | 'prompt' | 'template' | 'sheet_music' | 'other'
    -- 注：表情包/动图归为 'image' 类型，通过标签筛选区分（tag='GIF' / tag='表情包'）
    -- 注：MIDI/MusicXML/MSCZ/MSCX/乐谱 PDF 使用 'sheet_music'；纯音频（mp3/wav/flac）使用 'audio'
    cover_image_url TEXT,                -- 封面图 OSS key（未上传则由前端显示类型默认占位图）
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- status: 'pending' | 'published' | 'hidden' | 'banned' | 'under_review'
    view_count      BIGINT NOT NULL DEFAULT 0,
    like_count      INT NOT NULL DEFAULT 0,
    dislike_count   INT NOT NULL DEFAULT 0,
    -- 权限开关
    is_public       BOOLEAN NOT NULL DEFAULT TRUE,   -- 公开/仅关注者
    allow_copy      BOOLEAN NOT NULL DEFAULT TRUE,   -- 允许复制导出
    agent_enabled   BOOLEAN NOT NULL DEFAULT FALSE,  -- 开启 Agent 部署
    -- 支付预埋（MVP 全隐藏）
    is_paid         BOOLEAN NOT NULL DEFAULT FALSE,
    price           NUMERIC(10,2) DEFAULT 0,
    ban_reason      TEXT,                   -- 封禁原因（migration 040）
    download_count  INTEGER NOT NULL DEFAULT 0,  -- 下载计数（migration 040 新增列；数据库与 Redis ZSET 双写，异步刷入）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ              -- 软删除时间（migration 053）
);

CREATE INDEX idx_content_items_author ON content_items(author_id);
CREATE INDEX idx_content_items_ip ON content_items(ip_id);
CREATE INDEX idx_content_items_zone ON content_items(zone);
CREATE INDEX idx_content_items_type ON content_items(content_type);
CREATE INDEX idx_content_items_category ON content_items(category);
CREATE INDEX idx_content_items_source_original ON content_items(source_original_id, status, created_at DESC) WHERE source_original_id IS NOT NULL;
CREATE INDEX idx_content_items_status ON content_items(status);

-- 注：description 与 source_original_id 由 migration 036 通过 ALTER TABLE 添加。
-- 注：deleted_at 由 migration 053 通过 ALTER TABLE 添加。

-- 内容附件（OSS 存储的文件）
CREATE TABLE content_attachments (
    id              BIGSERIAL PRIMARY KEY,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    file_type       VARCHAR(20) NOT NULL, -- 'markdown' | 'image' | 'video' | 'audio' | 'archive'
                                          -- | 'sheet_music_midi' | 'sheet_music_xml' | 'sheet_music_mscz' | 'sheet_music_mscx' | 'other'
    oss_key         TEXT NOT NULL,        -- OSS 对象 key（相对路径）
    file_size       BIGINT,               -- 字节数
    mime_type       VARCHAR(100),
    duration_sec    INT,                  -- 视频/音频时长（秒）
    width           INT,                  -- 图片/视频宽度
    height          INT,                  -- 图片/视频高度
    is_primary      BOOLEAN DEFAULT TRUE, -- 主文件（Mod 含多个文件时）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 内容标签
CREATE TABLE content_tags (
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    tag             VARCHAR(50) NOT NULL,
    PRIMARY KEY(content_item_id, tag)
);
```

### 4.4 版本管理（协同 PR 引擎）

```sql
-- 内容版本表
CREATE TABLE content_versions (
    id                  BIGSERIAL PRIMARY KEY,
    content_item_id     BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    parent_version_id   BIGINT REFERENCES content_versions(id) ON DELETE SET NULL,
    author_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version_number      INT NOT NULL,        -- 当前版本序号（正式版）
    storage_type        VARCHAR(10) NOT NULL, -- 'diff' | 'full'
    storage_key         TEXT,                -- OSS key（diff patch 文件或全量文件）
    diff_summary        TEXT,                -- 人类可读的修改摘要（自动生成）
    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    -- status: 'active'（正式版）| 'pending'（待审 PR）| 'rejected'（已拒绝）| 'superseded'（被新版本替代）
    is_latest           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(content_item_id, version_number)
);

CREATE INDEX idx_versions_content ON content_versions(content_item_id);
CREATE INDEX idx_versions_status ON content_versions(status);
-- 获取最新版本的部分索引（高频查询）
CREATE INDEX idx_versions_content_latest ON content_versions(content_item_id) WHERE is_latest = TRUE;

-- PR 申请表
CREATE TABLE pull_requests (
    id                  BIGSERIAL PRIMARY KEY,
    content_item_id     BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    submitter_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    base_version_id     BIGINT NOT NULL REFERENCES content_versions(id), -- 基于哪个版本修改
    proposed_version_id BIGINT REFERENCES content_versions(id),          -- 生成的 pending 版本
    status              VARCHAR(20) NOT NULL DEFAULT 'open',
    -- status: 'open' | 'accepted' | 'rejected' | 'merged' | 'conflict'
    message             TEXT,                -- 提交说明
    reject_reason       TEXT,
    resolved_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pr_content ON pull_requests(content_item_id, status);
CREATE INDEX idx_pr_submitter ON pull_requests(submitter_id);

-- 贡献者荣誉名单
CREATE TABLE content_contributors (
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pr_count        INT NOT NULL DEFAULT 1,
    first_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(content_item_id, user_id)
);

-- 作者拉黑的贡献者
CREATE TABLE author_blocklist (
    author_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(author_id, blocked_id)
);
```

### 4.5 社交互动

> 注：DDL 顺序已调整为先创建被引用表（discussions）再创建引用表（comments），确保迁移脚本可顺序执行。

```sql
-- 讨论帖（贴吧式，挂在 IP 或原创区）
CREATE TABLE discussions (
    id              BIGSERIAL PRIMARY KEY,
    ip_id           BIGINT REFERENCES ips(id) ON DELETE CASCADE,
    content_item_id BIGINT REFERENCES content_items(id) ON DELETE CASCADE,
    -- content_item_id 为 P1 预留字段；P0 所有讨论均通过 ip_id 关联，content_item_id 固定为 NULL
    author_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           VARCHAR(500) NOT NULL,
    body            TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'published',
    view_count      BIGINT NOT NULL DEFAULT 0,
    reply_count     INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_discussions_ip ON discussions(ip_id, updated_at DESC);
CREATE INDEX idx_discussions_author ON discussions(author_id);

-- 评论表（支持楼中楼）
CREATE TABLE comments (
    id              BIGSERIAL PRIMARY KEY,
    content_item_id BIGINT REFERENCES content_items(id) ON DELETE CASCADE,
    discussion_id   BIGINT REFERENCES discussions(id) ON DELETE CASCADE,
    parent_id       BIGINT REFERENCES comments(id) ON DELETE CASCADE,
    author_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body            TEXT NOT NULL,   -- 支持简单 Markdown
    status          VARCHAR(20) NOT NULL DEFAULT 'published',
    -- status: 'published' | 'hidden' | 'banned'
    like_count      INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_comments_content ON comments(content_item_id);
CREATE INDEX idx_comments_discussion ON comments(discussion_id);
CREATE INDEX idx_comments_author ON comments(author_id);
CREATE INDEX idx_comments_parent ON comments(parent_id);

-- 点赞/点踩（防重复，支持内容+评论）
CREATE TABLE reactions (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type     VARCHAR(20) NOT NULL,  -- 'content' | 'comment'
    target_id       BIGINT NOT NULL,
    reaction        VARCHAR(10) NOT NULL,  -- 'like' | 'dislike'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, target_type, target_id)
);

-- 举报表
CREATE TABLE reports (
    id              BIGSERIAL PRIMARY KEY,
    reporter_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type     VARCHAR(20) NOT NULL,  -- 'content' | 'comment' | 'ip'
    target_id       BIGINT NOT NULL,
    reason          VARCHAR(100) NOT NULL,
    detail          TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- status: 'pending' | 'valid' | 'invalid'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reports_target ON reports(target_type, target_id);
CREATE INDEX idx_reports_reporter ON reports(reporter_id);
CREATE INDEX idx_reports_status ON reports(status, created_at DESC);

-- 申诉表（用户对被下架内容提起申诉）
CREATE TABLE appeals (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type     VARCHAR(20) NOT NULL,  -- 'content' | 'comment'
    target_id       BIGINT NOT NULL,
    reason          TEXT NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- status: 'pending' | 'approved' | 'rejected'
    admin_response  TEXT,
    resolved_by     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_appeals_user ON appeals(user_id);
CREATE INDEX idx_appeals_status ON appeals(status);

-- 收藏
CREATE TABLE favorites (
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id, content_item_id)
);

-- 浏览历史（仅记录内容浏览；UI 的 /history 页面按日期分组展示）
CREATE TABLE browse_history (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    viewed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, content_item_id)   -- upsert：同一内容重复浏览时更新 viewed_at
);

CREATE INDEX idx_browse_history_user ON browse_history(user_id, viewed_at DESC);

-- 关注关系表（支持关注创作者和关注 IP）
CREATE TABLE follows (
    id              BIGSERIAL PRIMARY KEY,
    follower_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type     VARCHAR(20) NOT NULL,  -- 'user' | 'ip'
    target_id       BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(follower_id, target_type, target_id)
);

CREATE INDEX idx_follows_follower ON follows(follower_id);
CREATE INDEX idx_follows_target ON follows(target_type, target_id);

-- 通知表（回复我的 / 收到的赞 / 系统消息）
CREATE TABLE notifications (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type            VARCHAR(30) NOT NULL,
    -- type: 'comment' | 'like' | 'follow' | 'system' | 'mention' | 'appeal_result' | 'content_status'
    channel         VARCHAR(20) NOT NULL,
    -- channel: 'reply'（回复我的）| 'like'（收到的赞）| 'system'（系统消息）
    title           VARCHAR(500),
    body            TEXT,
    target_type     VARCHAR(20),   -- 'content' | 'comment' | 'pr' | 'user'
    target_id       BIGINT,
    sender_id       BIGINT REFERENCES users(id) ON DELETE SET NULL,
    is_read         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user ON notifications(user_id, is_read, created_at DESC);
CREATE INDEX idx_notifications_channel ON notifications(user_id, channel, created_at DESC);

-- 私信对话表
CREATE TABLE conversations (
    id              BIGSERIAL PRIMARY KEY,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 对话参与者
CREATE TABLE conversation_participants (
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(conversation_id, user_id)
);

CREATE INDEX idx_conv_participants_user ON conversation_participants(user_id);

-- 私信消息表
CREATE TABLE messages (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_conversation ON messages(conversation_id, created_at);

-- 注：以下两条索引由独立迁移 migrations/056_conversation_indexes.sql 创建，
--     不与 messages/conversations 表 DDL（migrations/017_conversations.sql）同时执行。
CREATE INDEX idx_messages_sender ON messages(sender_id, created_at DESC);
CREATE INDEX idx_conversations_updated ON conversations(updated_at DESC);

-- 素质建设课程表（AI 根据系统扣分逻辑生成教学内容，按违规类型分类）
CREATE TABLE rehab_courses (
    id              BIGSERIAL PRIMARY KEY,
    violation_type  VARCHAR(50) NOT NULL UNIQUE,
    -- violation_type: 'malicious_report_tag' | 'malicious_comment' | 'malicious_contribution' | 'malicious_report_comment' | 'judge_error'
    content_i18n    JSONB NOT NULL,   -- { "zh-CN": "<Markdown>", "en-US": "<Markdown>" }（AI 生成，未来可拓展视频教学）
    min_reading_sec INT NOT NULL DEFAULT 180,  -- 最低阅读时间（秒），默认 3 分钟
    reward_points   INT NOT NULL DEFAULT 1,    -- 完成后恢复的信誉分
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 用户课程完成记录（每门课程每人只能完成一次）
CREATE TABLE rehab_completions (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id       BIGINT NOT NULL REFERENCES rehab_courses(id) ON DELETE CASCADE,
    started_at      TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ,
    points_awarded  INT NOT NULL DEFAULT 0,
    UNIQUE(user_id, course_id)
);

CREATE INDEX idx_rehab_completions_user ON rehab_completions(user_id);
```

### 4.6 审核与赛博判官

```sql
-- AI 审核记录
CREATE TABLE ai_review_records (
    id              BIGSERIAL PRIMARY KEY,
    target_type     VARCHAR(20) NOT NULL,  -- 'content' | 'comment' | 'ip'
    target_id       BIGINT NOT NULL,
    provider        VARCHAR(50) NOT NULL DEFAULT 'aliyun',
    result          VARCHAR(20) NOT NULL,  -- 'pass' | 'block' | 'review'
    raw_response    JSONB,  -- 原始响应（provider 自身格式，不做结构约定）
    scanned_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_review_target ON ai_review_records(target_type, target_id, scanned_at DESC);

-- 赛博判官题库
CREATE TABLE judge_questions (
    id              BIGSERIAL PRIMARY KEY,
    content_type    VARCHAR(50) NOT NULL,  -- 对应内容类型
    source_case_id  BIGINT,               -- 来源的真实审核案例 ID（自动更新时用）
    question_data   JSONB NOT NULL,
    -- question_data 结构：{
    --   "stem": "题干（Markdown，可含图片 URL）",
    --   "options": [{"key":"A","text":"选项内容"}, ...],   // 4 个选项
    --   "correct_key": "A",                                  // 正确选项 key
    --   "explanation": "答案解析（Markdown）"
    -- }
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_by      VARCHAR(20) NOT NULL DEFAULT 'admin',  -- 'admin' | 'auto'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_judge_questions_type_active ON judge_questions(content_type, is_active);

-- 众裁案例表
CREATE TABLE judge_cases (
    id              BIGSERIAL PRIMARY KEY,
    target_type     VARCHAR(20) NOT NULL,
    target_id       BIGINT NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'open',
    -- status: 'open' | 'closed_approve' | 'closed_reject'
    vote_approve    INT NOT NULL DEFAULT 0,
    vote_reject     INT NOT NULL DEFAULT 0,
    min_votes       INT NOT NULL DEFAULT 20,  -- 最小投票数，默认从 config.yaml > judge.min_votes 读取，DB 字段作 case 级覆盖
    closed_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_judge_cases_status ON judge_cases(status, created_at DESC);
CREATE INDEX idx_judge_cases_target ON judge_cases(target_type, target_id);

-- 众裁投票记录
CREATE TABLE judge_votes (
    id              BIGSERIAL PRIMARY KEY,
    case_id         BIGINT NOT NULL REFERENCES judge_cases(id) ON DELETE CASCADE,
    judge_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vote            VARCHAR(10) NOT NULL,  -- 'approve' | 'reject'
    reason          TEXT,                  -- 判官可选择性提交判定理由
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(case_id, judge_id)
);

-- 判定理由投票（点赞/点踩其他判官的理由）
CREATE TABLE judge_reason_votes (
    id                      BIGSERIAL PRIMARY KEY,
    reason_owner_vote_id    BIGINT NOT NULL REFERENCES judge_votes(id) ON DELETE CASCADE,
    voter_id                BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vote_type               VARCHAR(10) NOT NULL CHECK (vote_type IN ('up', 'down')),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(reason_owner_vote_id, voter_id)
);

CREATE INDEX idx_judge_reason_votes_owner ON judge_reason_votes(reason_owner_vote_id);

-- 考核记录
CREATE TABLE judge_exam_records (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_type    VARCHAR(50) NOT NULL,
    score           INT NOT NULL,         -- 答对题数
    total           INT NOT NULL,         -- 总题数
    passed          BOOLEAN NOT NULL,
    taken_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_judge_exam_user_type ON judge_exam_records(user_id, content_type, taken_at DESC);
```

### 4.7 分类管理

```sql
-- 动态分类表（管理员可增删改排序）
CREATE TABLE categories (
    id              BIGSERIAL PRIMARY KEY,
    zone            VARCHAR(20) NOT NULL,    -- 'fanwork' | 'original'
    level           VARCHAR(20) NOT NULL,    -- 'ip_category' | 'content_type' | 'primary' | 'secondary'
    parent_id       BIGINT REFERENCES categories(id) ON DELETE SET NULL,
    name_i18n       JSONB NOT NULL,          -- { "zh-CN": "游戏", "en-US": "Gaming" }
    slug            VARCHAR(100) UNIQUE NOT NULL,
    sort_order      INT NOT NULL DEFAULT 0,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 高频查询索引
CREATE INDEX idx_categories_zone_level ON categories(zone, level, sort_order);
CREATE INDEX idx_categories_zone_level_active ON categories(zone, level) WHERE is_active = TRUE;

-- 层级导航索引
CREATE INDEX idx_categories_parent ON categories(parent_id);
CREATE INDEX idx_categories_parent_active ON categories(parent_id, sort_order) WHERE is_active = TRUE;

-- 全文搜索优化（如需搜索分类名称）
-- CREATE INDEX idx_categories_name_gin ON categories USING GIN(name_i18n);
```

### 4.8 Agent 管理

```sql
-- LLM 配置表（管理员可视化管理 LLM 提供商）
CREATE TABLE llm_configs (
    id              BIGSERIAL PRIMARY KEY,
    config_name     VARCHAR(100) NOT NULL UNIQUE,    -- 配置名称（如 "通义千问-生产"、"DeepSeek-测试"）
    provider_type   VARCHAR(50) NOT NULL,            -- 'qwen' | 'openai_compat'
    api_base        VARCHAR(500) NOT NULL,           -- API 地址
    model           VARCHAR(100) NOT NULL,           -- 模型名称
    api_key_enc     TEXT NOT NULL,                   -- 加密存储的 API Key
    is_active       BOOLEAN NOT NULL DEFAULT FALSE,  -- 是否为当前激活配置
    extra_params    JSONB DEFAULT '{}',              -- 预留扩展参数（temperature/max_tokens 等）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 确保全局仅一个激活配置
CREATE UNIQUE INDEX idx_llm_configs_active ON llm_configs(is_active) WHERE is_active = TRUE;
```

---

### 4.9 国际化字段规范（JSONB i18n）

所有系统中的多语言字符串字段遵循统一的 JSONB 结构约定：

```javascript
// 短字符串字段（用于分类、标签、枚举值等）
name_i18n: { "zh-CN": "游戏", "en-US": "Gaming" }

// 长内容字段（用于教学内容、描述等）
content_i18n: { "zh-CN": "<Markdown内容>", "en-US": "<Markdown内容>" }

// 任意扩展参数字段
extra_params: { "temperature": 0.7, "max_tokens": 1000 }
```

应用层在读取时按用户 preferred_locale（默认 'zh-CN'）安全访问，缺失语言版本时 fallback 到 'zh-CN'。

---

### 4.10 IP 被永久封禁时的关联内容处理

当 IP 因黄赌毒或其他黄金规则违规被 AI 审核或管理员标记为 'banned' 时：

- IP 表：`ips.status = 'banned'`
- 关联的二创内容：`content_items.status = 'banned'`（即使内容本身通过了 AI 审核，也一并下架）
- 数据库实现：通过在 `ip_service.BanIP()` 或 AI 回调处理逻辑中执行级联更新，将所有关联内容的 status 设为 'banned'
- 用户感知：这些内容在浏览页面中直接隐藏不显示，内容作者可通过 REST API 查询 status=banned 后发起申诉

---

### 4.11 用户反馈 (Feedback)

```sql
CREATE TABLE feedback_tickets (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT REFERENCES users(id),
    contact_email     VARCHAR(255),
    category          VARCHAR(32) NOT NULL CHECK (category IN (
                        'web_bug', 'desktop_deploy', 'content_or_community',
                        'account_or_security', 'agent_quality', 'feature_request', 'other'
                      )),
    title             VARCHAR(160) NOT NULL,
    description       TEXT NOT NULL,
    diagnostic_summary JSONB NOT NULL DEFAULT '{}',
    status            VARCHAR(24) NOT NULL DEFAULT 'open' CHECK (status IN (
                        'open', 'in_progress', 'resolved', 'closed'
                      )),
    priority          VARCHAR(24) NOT NULL DEFAULT 'normal' CHECK (priority IN (
                        'low', 'normal', 'high', 'urgent'
                      )),
    assignee_admin_id BIGINT REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at       TIMESTAMPTZ
);

CREATE TABLE feedback_replies (
    id                BIGSERIAL PRIMARY KEY,
    ticket_id         BIGINT NOT NULL REFERENCES feedback_tickets(id) ON DELETE CASCADE,
    author_user_id    BIGINT REFERENCES users(id),
    author_admin_id   BIGINT REFERENCES users(id),
    body              TEXT NOT NULL,
    is_internal_note  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (author_user_id IS NOT NULL OR author_admin_id IS NOT NULL),
    CHECK (NOT (author_user_id IS NOT NULL AND author_admin_id IS NOT NULL))
);

CREATE TABLE feedback_attachments (
    id            BIGSERIAL PRIMARY KEY,
    ticket_id     BIGINT NOT NULL REFERENCES feedback_tickets(id) ON DELETE CASCADE,
    oss_key       TEXT NOT NULL,
    file_type     VARCHAR(32) NOT NULL,
    mime_type     VARCHAR(100) NOT NULL,
    size_bytes    BIGINT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_feedback_tickets_user_id ON feedback_tickets(user_id);
CREATE INDEX idx_feedback_tickets_status ON feedback_tickets(status);
CREATE INDEX idx_feedback_replies_ticket_id ON feedback_replies(ticket_id);
CREATE INDEX idx_feedback_attachments_ticket_id ON feedback_attachments(ticket_id);
```

- 用户通过 `/feedback` 页面提交反馈工单（web_bug / feature_request 等类别）。
- 管理员在 `/admin/feedback` 查看、分配、回复工单。
- `feedback_attachments` 存储 OSS 对象键（截图/日志等）。

### 4.12 管理员审计日志 (Admin Audit Logs)

```sql
CREATE TABLE admin_audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    admin_user_id   BIGINT NOT NULL REFERENCES users(id),
    action          VARCHAR(96) NOT NULL,
    target_type     VARCHAR(48) NOT NULL,
    target_id       VARCHAR(96),
    trace_id        VARCHAR(96),
    metadata        JSONB NOT NULL DEFAULT '{}',
    result          VARCHAR(24) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_audit_logs_created_at ON admin_audit_logs(created_at DESC);
CREATE INDEX idx_admin_audit_logs_action ON admin_audit_logs(action);
```

- 所有管理员敏感操作（封禁用户、删除内容、修改配置等）自动记录审计日志。
- `metadata` 为 JSONB 字段存储操作上下文（如修改前后的值）。
- `trace_id` 关联请求日志便于排查。

### 4.13 收藏集 (Collections)

> **状态**: 设计已确认，代码尚未实现。见 DEC-010。

```sql
CREATE TABLE collections (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           VARCHAR(255) NOT NULL,
    description     TEXT,
    is_public       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_collections_user ON collections(user_id);

CREATE TABLE collection_items (
    id              BIGSERIAL PRIMARY KEY,
    collection_id   BIGINT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    content_id      BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    added_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    note            TEXT,
    UNIQUE(collection_id, content_id)
);

CREATE INDEX idx_collection_items_collection ON collection_items(collection_id, added_at DESC);
CREATE INDEX idx_collection_items_content ON collection_items(content_id);
```

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

```yaml
server:
  port: "8080"                        # 监听端口
  mode: "development"                 # development | release
  shutdown_timeout: 15                # 优雅关机超时（秒）

features:
  payment_enabled: false             # 支付模块全局开关（MVP 关闭）
  creator_support_enabled: false     # 创作者支持模块（P1 开启）
  desktop_deploy_enabled: false      # 桌面端一键部署（D-02~D-05 完成后开启）

limits:
  video_max_mb: 300                  # 视频最大文件大小（MB）
  video_max_sec: 180                 # 视频最大时长（秒）
  image_max_mb: 20
  text_max_mb: 10
  mod_max_mb: 500                    # Mod 打包最大大小
  sheet_music_max_mb: 50             # 乐谱文件最大大小

reputation:
  min_score_for_interaction: 3       # 低于此分禁止发布/互动/下载（统一门槛）
  quality_content_threshold: 10      # 优质内容点赞阈值
  quality_comment_threshold: 5       # 优质评论点赞阈值
  repeat_violation_window_days: 7   # 滑动窗口天数
  repeat_violation_threshold: 2      # 窗口内违规计数阈值
  repeat_violation_extra_penalty: -1 # 超阈后额外扣分
  # 以下 score_* 字段控制实际加减分值（0 = 使用 reputation_service.go 内置默认值）
  score_quality_content: 3
  score_pr_merged: 3
  score_quality_comment: 2
  score_tag_recognized: 1
  score_judge_accuracy: 1
  score_rehab_course: 1
  score_valid_report: 1
  score_malicious_content: -3
  score_malicious_pr: -3
  score_malicious_comment: -2
  score_malicious_report: -2
  score_malicious_tag_report: -1
  score_judge_error: -1

judge:
  min_votes_required: 20             # 众裁最小投票数
  pass_threshold: 0.60               # 判决通过线（不违规比例 ≥ 60% 恢复展示）
  exam_pass_rate: 0.80               # 考核通过分率（80%）
  error_rate_revoke: 0.50            # 错误率超过此值撤权
  error_rate_window: 10              # 触发撤权检查的最小判定次数

social:
  report_auto_hide_rate: 0.10       # 内容举报率阈值（自动隐藏 + 触发众裁）
  comment_fold_threshold: 0.30      # 评论区点踩/点赞比阈值（自动折叠）

recommendation:
  enabled: true
  hot_decay_hours: 48
  personalization_weight: 0.6
  min_interaction_for_personalize: 10
  embedding_topk: 200
  trending_window_days: 7
  refresh_interval_h: 2
  rank_interval_min: 10                 # 热门排行更新间隔（分钟）
  embedding_multiplier: 1               # 内容向量化权重乘数

publish:
  type_order_original: ["image", "article", "video", "audio", "template", "sheet_music", "other"]
  type_order_fanwork: ["image", "article", "video", "audio", "mod", "prompt", "sheet_music", "other"]
  require_review: false                  # 发布是否需要审核（P0 直接发布，P1 开启）
  max_daily_posts: 50                    # 每日发布上限
  freeze_on_violation: true              # 违规后是否冻结发布权限

upload:
  sheet_music_extensions: [".mid", ".midi", ".xml", ".mxl", ".mscz", ".mscx", ".pdf"]

feedback:
  upload_grant_ttl_sec: 300              # 反馈附件上传授权有效期（秒）

# --- 以下配置段在 config.yaml 存在 ---

oss:
  # 凭证通过环境变量注入（OSS_ACCESS_KEY_ID / OSS_ACCESS_KEY_SECRET）
  endpoint: "https://oss-cn-hangzhou.aliyuncs.com"
  bucket_name: "omnicraft-prod"
  domain: ""                          # OSS Bucket 域名（或 CDN 加速域名）
  download_url_ttl_sec: 300          # 下载签名 URL 有效期（秒）

green:
  # 凭证通过环境变量 GREEN_ACCESS_KEY_ID / GREEN_ACCESS_KEY_SECRET 注入
  region: "cn-shanghai"
  callback_url: ""                       # 阿里云内容安全异步回调地址
  callback_allowed_ips: []               # 回调来源 IP 白名单

agent:
  web_agent_enabled: false           # 网页端 Agent 总开关（默认关闭）
  llm_provider: "openai_compat"
  llm_model: "deepseek-chat"
  llm_api_base: "https://api.deepseek.com"
  llm_api_key: ""                    # 通过 env AGENT_LLM_API_KEY 注入
  embedding_model: "text-embedding-3-small"
  embedding_dimensions: 1536
  rate_limit_per_day: 50
  upload_assist_max_file_mb: 10
  max_user_message_chars: 4000
  chat_max_context_messages: 10
  hmac_secret: ""                    # Tauri Agent HMAC 签名密钥

captcha:
  provider: "aliyun"
  # 凭证通过环境变量 CAPTCHA_ACCESS_KEY_ID / CAPTCHA_ACCESS_KEY_SECRET 注入
  prefix: ""
  scene_id: ""
  region: "cn-shanghai"
  ticket_ttl_sec: 300

smtp:
  mode: "logger"                     # "live" | "logger"（开发模式用 logger bypass）
  host: ""
  port: 587
  user: ""
  password: ""                       # 通过 env SMTP_PASSWORD 注入
  from_address: ""

verification:
  email_ttl_sec: 3600               # 邮箱验证链接有效期
  reset_ttl_sec: 3600               # 密码重置 token 有效期
  resend_cooldown_sec: 60           # 重发冷却时间
  login_captcha_threshold: 3        # 连续登录失败触发验证码的阈值
  password_min_length: 8
  register_pending_ttl_sec: 86400   # 注册未验证自动清理时间

legal:
  current_terms_version: ""         # 当前用户协议版本号
  current_privacy_version: ""       # 当前隐私政策版本号

cache:
  # Redis 缓存 TTL 配置（CacheConfig）；非"向量嵌入缓存"
  content_list_ttl: 30              # 内容列表缓存（秒）
  content_detail_ttl: 300           # 内容详情缓存（秒）
  ip_list_ttl: 60                   # IP 列表缓存（秒）
  ip_detail_ttl: 600                # IP 详情缓存（秒）
  view_count_flush_interval: 30     # 浏览计数刷入 DB 间隔（秒）
  hot_rank_zset_ttl: 600            # 热门排行 ZSET TTL（秒）
  user_status_ttl: 300              # 用户运行时状态缓存（秒）
  tag_cache_ttl: 600                # 标签缓存（秒）
  email_verify_ttl: 3600            # 邮箱验证链接有效期（秒）
  password_reset_ttl: 3600          # 密码重置 token 有效期（秒）
  publish_freeze_ttl: 604800        # 发布冻结时长（秒，默认 7 天）

queue:
  enabled: false                     # 异步任务队列（MVP 关闭）
  max_attempts: 3                    # 最大重试次数
  retry_backoff_sec: [60, 120, 300] # 重试退避间隔（秒，数组）
  dlq_ttl_hours: 72                  # 死信队列保留时间（小时）
  maxlen: 100000                     # Stream 最大长度
  worker_review: 1                   # 审核 Worker 并发数
  worker_notification: 1             # 通知 Worker 并发数

rate_limit:
  enabled: true
  normal_per_minute: 60             # 普通请求每 IP 每分钟
  normal_window_sec: 60             # 普通请求窗口大小（秒）
  upload_per_hour: 10               # 上传每 IP 每小时
  upload_window_sec: 3600           # 上传窗口大小（秒）
  agent_window_sec: 86400           # Agent 请求窗口大小（秒）
  credential_per_minute: 5          # 认证类请求每账号每分钟
  search_per_minute: 30             # 搜索每 IP 每分钟
  max_json_body_bytes: 1048576      # JSON Body 最大字节数（1MB）
  max_query_chars: 256              # 查询字符串最大字符数
  max_search_limit: 50              # 搜索单页最大条数
  max_search_page: 100              # 搜索最大页数

web:
  public_base_url: "https://app.leeppp.online"

security:
  allowed_origins:                   # CORS 白名单（生产环境必填，禁止 *）
    - "https://app.leeppp.online"
  trusted_proxies:                   # 信任的代理 IP（用于 X-Forwarded-For 解析）
    - "10.0.0.0/8"

client:
  download_enabled: false            # 桌面客户端下载入口
  download_url: ""
  latest_version: ""
```

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
| | 粉丝分析 | 👥 | `/studio/followers` | 粉丝增长趋势、粉丝画像（新增） |
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
