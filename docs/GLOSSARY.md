# OmniCraft 项目术语表

> **权威等级**: authoritative（术语定义的唯一来源）
> **最后更新**: 2026-06-29

## 核心概念

| 术语 | 英文 | 定义 | 使用场景 |
|------|------|------|---------|
| 判官 | Judge | 参与众裁投票的用户 | 禁止使用"审核员"/"评审员" |
| 众裁 | Crowd Judgment | 用户集体投票判定内容是否违规的机制 | 赛博判官功能模块 |
| 信誉分 | Reputation Score | 用户行为信用评分，初始 10 分 | 权限控制、内容发布门槛 |
| 创作者工作室 | Creator Studio | 统一的内容创作和管理工作空间（`/studio`） | 替代旧 `/publish` 和 `/dashboard` 路由 |
| 二创 | Fanwork / Derivative Work | 基于原创内容的二次创作 | 二创发布、来源绑定 |
| 原创区 | Original Zone | 发布原创内容的区域 | `content_items.zone = "original"` |
| 素质建设 | Rehabilitation | 信誉分恢复课程系统 | `/rehab` 路由 |
| PR | Pull Request | 协同修改请求（原创区内容协作） | 协作管理模块 |

## 功能模块

| 术语 | 英文 | 定义 | 使用场景 |
|------|------|------|---------|
| 收藏集 | Collection | 用户创建的内容收藏合集 | `collections` 表和 `collection_items` 关联表 |
| 通知 | Notification | 系统自动创建的事件提醒 | 评论、点赞、关注等事件触发的通知记录 |
| 私信 | Direct Message | 用户间的私人消息 | 消息中心私信对话列表 |
| 赛博判官 | Cyber Judge | 社区内容众裁系统的统称 | 包含众裁、判官考核、判决等机制 |
| 全文搜索 | Full-text Search | 基于 PostgreSQL tsvector 的中文全文搜索 | `content_search_idx` GIN 索引 |
| 热搜 | Trending Search | 热门搜索词建议 | `search_suggestions` 表 Top 10 |
| 密码重置 | Password Reset | 通过邮箱 Token 重置密码的流程 | `/forgot-password` 和 `/reset-password` 页面 |
| 举报 | Report | 用户上报违规内容的机制 | 自动风控触发条件 |
| 申诉 | Appeal | 用户对被下架内容提起恢复请求 | 管理员终审 |
| 判决 | Verdict | 众裁投票的最终结果 | 投票结束后判定内容是否恢复展示 |
| 下架 | Takedown | 违规内容被隐藏或标记为不可见 | 内容 `status` 变更 |
| 风控 | Risk Control | 自动内容审核和风险检测系统 | 举报率触发自动隐藏、评论区折叠 |

## 技术概念

| 术语 | 英文 | 定义 | 使用场景 |
|------|------|------|---------|
| 软删除 | Soft Delete | 设置 `deleted_at` 而非物理删除 | 用户账号、内容数据保留用于审计 |
| 优雅关机 | Graceful Shutdown | 关机时等待现有请求完成再退出 | `http.Server.Shutdown(ctx)` |
| 结构化日志 | Structured Logging | 使用 `slog` 标准库的 JSON 格式日志 | 统一日志格式，禁用 `log.Printf` |
| 国际化 | Internationalization (i18n) | 使用 `next-intl` 的多语言支持 | 翻译文件 `frontend/messages/` |
| 特性开关 | Feature Flag | 通过 `config.yaml > features.*` 控制功能启用 | 支付模块、桌面部署等功能开关 |
| 一键部署 | One-click Deploy | Tauri 桌面端打包部署流程 | Beta D-02 至 D-05 任务 |
| 推荐算法 | Recommendation Algorithm | 个性化内容推荐引擎 | 推荐 Tab 的向量相似度 + 热门度混合排序 |
| 路由迁移 | Route Migration | 旧 `/publish` 和 `/dashboard` 迁移至 `/studio` | 301 重定向规则 |
| 内容类型 | Content Type | 内容格式分类（图文、视频、音频等） | 渲染策略选择（非筛选维度） |
| 内容分类 | Category | 内容所属领域分类（影视、游戏、美食等） | 一级分类体系，发布时必填 |
| EmptyState | EmptyState | 空状态展示组件 | 无数据、功能未开放、封禁用户提示等场景 |
| Toast | Toast | 轻量级操作反馈组件 | 表单提交成功/失败提示 |
| Sidecar | Sidecar | Tauri 管理的 Go 后端子进程 | 桌面端 Go 服务 |

## 基础架构

| 术语 | 英文 | 定义 | 使用场景 |
|------|------|------|---------|
| OSS | Object Storage Service | 阿里云对象存储 | 内容文件存储和 CDN 分发 |
| pgvector | pgvector | PostgreSQL 向量检索扩展 | 推荐算法向量相似度检索 |
| GORM | GORM | Go 语言的 ORM 库 | 数据库操作 |
| Gin | Gin | Go 语言的 HTTP 框架 | API 路由和中间件 |
| Next.js | Next.js | React 全栈框架 | 前端 SSR |
| Tauri | Tauri | Rust 桌面端框架 | PC 客户端 |
| PostgreSQL | PostgreSQL | 关系型数据库 | 主数据库 |
| Redis | Redis | 内存缓存数据库 | 会话、Token、缓存 |

## 安全相关

| 术语 | 英文 | 定义 | 使用场景 |
|------|------|------|---------|
| 封禁 | Ban | 用户或 IP 被禁用 | `users.is_banned = TRUE`，所有功能禁用 |
| 永封 | Permanent Ban | 因黄赌毒内容永久封禁 | 不走信誉分机制 |
| HMAC-SHA256 | HMAC-SHA256 | 动作脚本签名算法（过渡期） | 桌面端文件操作鉴权，D-03 后替换为 Ed25519 |
| Ed25519 | Ed25519 | 动作脚本签名算法（目标） | 桌面端文件操作鉴权，客户端仅持公钥 |
| JWT | JSON Web Token | 用户身份认证 Token | API 鉴权 |
| CORS | Cross-Origin Resource Sharing | 跨域资源共享策略 | 生产环境从 `config.yaml` 读取 AllowedOrigins |
| Goroutine | Goroutine | Go 轻量级线程 | 异步任务，panic 需 recover |

## 文档与阶段

| 术语 | 英文 | 定义 | 使用场景 |
|------|------|------|---------|
| Beta | Beta | 当前公开 Beta 加固阶段 | 替代"MVP"，所有文档统一使用 Beta |
| MVP | Minimum Viable Product | 历史版本阶段 | 已结束，仅用于指代 `task.json` 历史任务 |
| PRD | Product Requirements Document | 产品需求文档 | 需求分析参考 |
| architecture.md | Architecture Document | 技术架构设计文档 | 模块设计参考 |

## 禁止混用的术语对

| 正确 | 错误 | 说明 |
|------|------|------|
| 判官 | 审核员、评审员 | 统一使用"判官" |
| 众裁 | 审核、评审 | 众裁是社区投票，审核是管理员行为 |
| 二创 | 二次创作、衍生作品 | 统一使用"二创" |
| 信誉分 | 信用分、积分 | 统一使用"信誉分" |
| 创作者工作室 | 发布页、仪表盘 | `/studio` 统一名称 |
| Beta | MVP | 当前阶段统一使用 Beta |
| 收藏集 | 收藏夹、收藏合集 | 统一使用"收藏集" |
| 软删除 | 逻辑删除、假删除 | 统一使用"软删除" |
| 私信 | 站内信、消息 | 统一使用"私信" |
| 素质建设 | 康复课程、教育课程 | 统一使用"素质建设" |
| 原创区 | 原创区/原创内容区 | 统一使用"原创区"作为 zone 名称 |
| 举报 | 投诉、反馈 | 统一使用"举报" |
| 申诉 | 复议、复审请求 | 统一使用"申诉" |
| 特性开关 | 功能开关、特性标记 | 统一使用"特性开关" |
| 一键部署 | 一键发布、自动部署 | 统一使用"一键部署" |
