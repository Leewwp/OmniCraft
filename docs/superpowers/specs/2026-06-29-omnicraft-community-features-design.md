# OmniCraft 社区功能增强设计规格书

> 日期：2026-06-29
> 状态：设计校准完成，人工决策已确认，可进入实现计划生成
> 来源：/superpowers:brainstorming 会话
> 实施策略：方案 A（串行垂直切片）

## 目录

- [0. 文档概述](#0-文档概述)
  - [0.1 背景](#01-背景)
  - [0.2 涉及子系统](#02-涉及子系统)
  - [0.3 实施策略](#03-实施策略)
  - [0.4 设计原则](#04-设计原则)
  - [0.5 成功指标（KPI）](#05-成功指标kpi)
- [1. 私信 + 通知增强](#1-私信--通知增强)
  - [1.1 背景](#11-背景)
  - [1.2 私信会话冷启动限制](#12-私信会话冷启动限制)
  - [1.3 管理员系统通知广播](#13-管理员系统通知广播)
  - [1.4 前端消息中心重构](#14-前端消息中心重构)
  - [1.5 业务约束](#15-业务约束)
  - [1.6 涉及文件清单](#16-涉及文件清单)
- [2. 浏览足迹增强](#2-浏览足迹增强)
  - [2.1 背景](#21-背景)
  - [2.2 数据保留策略（双重保障）](#22-数据保留策略双重保障)
  - [2.3 API 增强](#23-api-增强)
  - [2.4 前端页面设计](#24-前端页面设计)
  - [2.5 业务约束](#25-业务约束)
  - [2.6 涉及文件清单](#26-涉及文件清单)
- [3. 收藏集文件夹系统](#3-收藏集文件夹系统)
  - [3.1 背景](#31-背景)
  - [3.2 数据模型](#32-数据模型)
  - [3.3 业务约束](#33-业务约束)
  - [3.4 API 设计](#34-api-设计)
  - [3.5 前端设计](#35-前端设计)
  - [3.6 涉及文件清单](#36-涉及文件清单)
- [4. 内容系列 (Content Series) 功能](#4-内容系列-content-series-功能)
  - [4.1 背景](#41-背景)
  - [4.2 数据模型](#42-数据模型)
  - [4.3 业务约束](#43-业务约束)
  - [4.4 API 设计](#44-api-设计)
  - [4.5 前端设计](#45-前端设计)
  - [4.6 涉及文件清单](#46-涉及文件清单)
- [5. 联合创作邀请](#5-联合创作邀请)
  - [5.1 背景](#51-背景)
  - [5.2 数据模型](#52-数据模型)
  - [5.3 发送邀请 — 防骚扰校验链](#53-发送邀请--防骚扰校验链)
  - [5.4 私信邀请卡片 UI](#54-私信邀请卡片-ui)
  - [5.5 用户设置扩展](#55-用户设置扩展)
  - [5.6 涉及文件清单](#56-涉及文件清单)
- [6. 原创/二创区联动增强](#6-原创二创区联动增强)
  - [6.1 背景](#61-背景)
  - [6.2 来源链模型重构](#62-来源链模型重构)
  - [6.3 API 调整](#63-api-调整)
  - [6.4 前端设计（克制原则，不喧宾夺主）](#64-前端设计克制原则不喧宾夺主)
  - [6.5 旧数据迁移](#65-旧数据迁移)
  - [6.6 涉及文件清单](#66-涉及文件清单)
- [附录 A：已确认的设计决策汇总](#附录-a已确认的设计决策汇总)
- [附录 B：config.yaml 新增配置](#附录-bconfigyaml-新增配置)
- [附录 C：新增数据库迁移文件一览](#附录-c新增数据库迁移文件一览)
- [附录 D：实施范围外（明确不做）](#附录-d实施范围外明确不做)
- [附录 E：跨计划共享文件矩阵](#附录-e跨计划共享文件矩阵)
- [附录 F：已确认的人工决策](#附录-f已确认的人工决策)

---

## 0. 文档概述

### 0.1 背景

OmniCraft 已完成 MVP 开发，Web Beta 加固主线已基本完成；桌面端一键部署安全链路（D-02 至 D-05、R-02）仍处于延后/条件发布状态，后续计划不得把桌面部署能力视为已完成。当前系统具备基础的内容发布、浏览、评论、收藏、私信等功能，但在社区互动深度和创作者体验方面仍存在明显短板。本轮设计旨在补齐以下 6 个社区功能模块，提升用户留存和创作生态活力。

### 0.2 涉及子系统

| # | 子系统 | 类型 | 复杂度 | 依赖 |
|---|--------|------|--------|------|
| 1 | 私信 + 通知增强 | 增强现有功能 | 中 | 无 |
| 2 | 浏览足迹增强 | 增强现有功能 | 低 | 无 |
| 3 | 收藏集文件夹系统 | 重构现有功能 | 中 | 无 |
| 4 | 内容系列 (Content Series) 功能 | 全新功能 | 高 | 无 |
| 5 | 联合创作邀请 | 全新功能 | 高 | 私信系统（#1） |
| 6 | 原创/二创区联动增强 | 扩展已有功能 | 中 | 无外部依赖；本子系统内先完成来源链 schema |

### 0.3 实施策略

采用**方案 A：串行垂直切片**。按依赖关系排序，每次完成一个子系统的完整垂直切片（数据层 → 服务层 → 表示层 → 测试），交付一个可用的功能后再进入下一个。

#### 计划生成前约束

- 后续使用 `superpowers:writing-plans` 时，应拆成 6 份实现计划；每份计划只覆盖一个子系统，并显式列出 TDD 步骤、浏览器验证步骤和迁移验证步骤。
- 本文档中的路径采用仓库根目录相对路径。后端文件必须带 `backend/` 前缀，前端文件必须带 `frontend/` 前缀，避免把 Next.js route group 文件创建到错误目录。
- UI 细节以 `design/ui-spec.md` 为视觉权威。若本文新增组件或页面在 `design/ui-spec.md` 中不存在，对应实现计划必须先补 UI spec，再写代码。
- 已知需要在实现前补充/修订的 UI spec 包括：`/admin/notifications`、`/series/[id]`、`/studio/series`、`CollectionPicker`、`SeriesNav`、`CollabUserPicker`、`CollabInviteCard`、`RelatedFanworks`、`SourceAttribution`；`/collections/[id]` 已有规格但需按附录 F 修正公开浏览范围。
- 若实施计划修改 `backend/config/config.go`、`backend/migrations/*.sql` 或当前唯一 route owner（初始为 `backend/internal/handler/routes.go`，hardening Task 3 后为 `backend/internal/router/routes.go`），提交前必须运行 `cd tools/doc-validator && go run . --fix` 并提交自动文档更新。
- API 分页统一优先使用现有项目约定 `page` + `page_size`。为兼容文档早期草案，后端可接受 `limit` 作为 `page_size` 的别名，但响应字段统一返回 `page_size`。

### 0.4 设计原则

- **克制 UI**：新增入口不喧宾夺主，不影响用户正常浏览体验
- **防滥用优先**：涉及用户间互动的功能必须先通过防骚扰链路
- **复用优先**：优先扩展现有表和 API，避免不必要的建表
- **配置驱动**：所有阈值、限制从 config.yaml 读取，不硬编码
- **软删除优先**：删除操作保留数据用于审计

### 0.5 成功指标（KPI）

以下指标用于上线后评估各子系统的采用率和健康度，不作为硬性上线门禁：

| 子系统 | 指标 | 测量方式 |
|--------|------|---------|
| 私信冷启动 | 冷启动拒绝率 < 5%（被 DM_REPLY_REQUIRED 拦截的消息占比） | 后端计数，每周统计 |
| 管理员广播 | 广播覆盖率 ≥ 95%（收到广播的用户 / 当前活跃用户数） | 广播发送时记录 recipient_count 与活跃用户总数对比 |
| 浏览足迹 | 足迹页面周活 ≥ 30% 活跃用户 | 前端埋点 / 路由访问统计 |
| 收藏集 | 收藏集创建率 ≥ 30% 活跃用户至少拥有 2 个以上收藏集 | 数据库查询，每月统计 |
| 内容系列 | 系列采用率 ≥ 15% 活跃创作者至少创建 1 个系列 | 数据库查询，每月统计 |
| 联合创作 | 邀请接受率 ≥ 50%（accepted / (accepted + declined)） | 数据库查询，每月统计 |
| 联动增强 | 来源绑定率：≥ 60% 新二创使用了 source_fanwork_id 或 source_original_id | 数据库查询，每月统计 |

---

## 1. 私信 + 通知增强

### 1.1 背景

当前系统已具备基础的私信（conversations / messages 表）和通知（notifications 表）基础设施。前端消息页面 `/messages` 已有基础布局。本章增强以下三点：

1. 私信冷启动防骚扰限制
2. 管理员系统通知广播
3. 前端消息中心重构为双栏布局

### 1.2 私信会话冷启动限制

#### 规则

会话中对方**从未回复过**时，当前用户只能发一条消息。一旦对方回复（哪怕一次），限制永久解除。

#### 校验逻辑（POST /api/v1/messages 前置校验）

```
1. 查询该会话中 messages WHERE sender_id != 当前用户 的记录数
   （即对方发过的消息数）
2. 若 count > 0 → 对方已回复过 → 放行
3. 若 count == 0 → 冷启动状态：
   a. 查该会话最后一条消息（ORDER BY created_at DESC LIMIT 1）
   b. 若最后一条 sender_id == 当前用户 → 拒绝 403
   c. 否则 → 放行（这是第一条消息）
```

#### 错误响应

```json
{
  "code": "DM_REPLY_REQUIRED",
  "message": "对方尚未回复，请等待回复后再发送新消息"
}
```

#### 场景覆盖

| 场景 | 结果 |
|------|------|
| A 首次给 B 发消息 | 放行 |
| A 再发第二条（B 没回） | 拒绝 DM_REPLY_REQUIRED |
| B 回复了 A | 放行 |
| A 回复 B（对话已建立） | 放行 |
| 双方持续正常聊天 | 无限制 |

#### 实现说明

- 无需改表，纯业务逻辑校验
- 现有后端真实发送接口为 `POST /api/v1/messages`，请求体包含 `recipient_id` 与 `text`，后端内部查找或创建 1:1 conversation 后写入 messages。
- 当前前端 `frontend/components/social/ChatWindow.tsx` 仍调用 `/api/v1/conversations/:id/messages`，`frontend/components/social/ConversationList.tsx` 仍调用 `/api/v1/conversations`；实施 §1 时必须一并修正为现有 `/api/v1/messages` / `/api/v1/messages/:id` 体系，避免在新布局上继续叠加旧 API 错位。
- 修改文件：`backend/internal/handler/message.go`、`backend/internal/repository/message_repo.go`；如抽出 service 层，新增 `backend/internal/service/message_service.go`。

### 1.3 管理员系统通知广播

#### 新增接口

**POST /api/v1/admin/notifications/broadcast**

```json
// Request
{
  "title": "系统维护公告",
  "body": "本站将于 2026-07-01 02:00-04:00 进行系统维护，届时服务暂不可用。",
  "channel": "broadcast"
}

// Response 200
{
  "data": {
    "recipient_count": 1234,
    "broadcast_at": "2026-06-29T15:30:00Z"
  }
}
```

#### 后端逻辑

1. 校验请求者 role == 'admin'
2. 查询所有活跃用户：`SELECT id FROM users WHERE is_banned = FALSE AND deleted_at IS NULL`
3. 批量 INSERT 到 notifications 表：
   - type = 'system'
   - channel = 'broadcast'
   - title / body 取自请求
   - user_id 遍历全部活跃用户
4. 批量操作使用 GORM CreateInBatches，每批 500 条
5. 返回 recipient_count
6. 写入 admin_audit_logs：action='broadcast_notification', target_type='users', result='success'
7. 若请求校验失败或批量发送失败，也写入 admin_audit_logs，result 分别为 `rejected` / `failed`，metadata 记录安全摘要（不得记录完整正文）

#### 输入约束

| 字段 | 约束 |
|------|------|
| `title` | 必填，1-120 字符，去除首尾空白后不能为空 |
| `body` | 必填，1-5000 字符，允许 Markdown 图片和链接；前端预览和通知详情渲染均需走现有 Markdown 安全渲染链路，禁止原始 HTML 绕过消毒 |
| `channel` | 可省略；若传入只能为 `broadcast`，后端不得信任任意 channel |

> **实现前**：需先执行 `057_add_broadcast_channel.sql` 迁移（见附录 C；若实施时编号已被占用则顺延），解除并重建 `notifications_channel_check` 约束，新增 `'broadcast'` 值。迁移必须基于当前最新约束值集合重建，保留既有 `reply` / `like` / `system` / `pr` / `follow`，最终允许集合为 `reply` / `like` / `system` / `pr` / `follow` / `broadcast`。否则步骤 3 的 INSERT 会因违反 CHECK 约束失败，或误删既有通知频道能力。

> **已知限制**：步骤 2（SELECT 活跃用户）与步骤 3（批量 INSERT 通知）之间存在极短的竞态窗口。
> 窗口期内新注册的用户不会收到本次广播。当前 <1000 用户规模下该窗口可忽略。
> 未来扩容时可改用 REPEATABLE READ 事务快照或消息队列模式消除此窗口。

#### 为什么逐用户创建记录而非单条广播标记

- MVP 用户量可控（< 1000），逐条记录成本极低
- 复用现有 `GET /api/v1/notifications` 和 `is_read` 追踪机制
- 后续扩容时可改为「1 条广播 + 独立已读表」架构，不影响前端

#### 前端新增

**管理员广播编辑页** `frontend/app/(protected)/admin/notifications/page.tsx`：
- 标题输入框
- Markdown 正文编辑器（复用 MarkdownEditor 组件）
- 实时预览
- 一键发送按钮 → 确认对话框 → Toast 结果

### 1.4 前端消息中心重构

#### 布局（左侧导航 + 右侧内容双栏结构）

```
+----------------------------------------------------+
|  ← 返回           消息中心                         |
+------------+---------------------------------------+
| [通知]     |                                       |
| [私信]     |        右侧内容区                      |
|            |                                       |
| 通知列表：  |  · 系统广播（蓝色标记）                |
| · 未读高亮  |  · 评论通知                            |
| · 已读置灰  |  · 点赞通知                            |
|            |  · 关注通知                            |
|            |  · 申诉结果通知                         |
| 私信列表：  |                                       |
| · 会话头像  |  或：私信对话窗口                        |
| · 最后消息  |  · 消息气泡                             |
| · 未读红点  |  · 输入框                               |
| · 时间戳    |  · 发送按钮                             |
+------------+---------------------------------------+
```

#### 关键交互

| 交互 | 行为 |
|------|------|
| 通知 Tab | 混合时间线（系统通知 + 个人通知），按时间倒序，未读高亮 |
| 私信 Tab | 左侧会话列表 + 右侧对话窗口，点击会话切换 |
| 未读红点 | 会话列表项红点 + Header 铃铛红点（继续轮询 unread-count） |
| 系统通知 | 蓝色左边框标记，与个人通知视觉区分 |

### 1.5 业务约束

| 约束 | 说明 |
|------|------|
| 广播不可撤回 | 通知一旦逐用户创建即不可撤回；前端需二次确认后才能发送 |
| 广播幂等 | 请求必须携带 `Idempotency-Key`；同一管理员、同一 key、同一规范化正文的重试返回原结果，不得重复创建逐用户通知；同 key 不同正文返回 409。key 哈希、payload 哈希和安全响应摘要持久化到数据库，并与逐用户通知及成功审计在同一事务提交；Redis 不作为幂等事实源。 |
| 冷启动限制 | 详见 §1.2（纯业务逻辑，不改表） |
| 广播权限与批量 | 详见 §1.3（仅 admin、CreateInBatches 每批 500） |
| 系统通知视觉区分 | 详见 §1.4（蓝色左边框） |

### 1.6 涉及文件清单

| 层 | 文件路径 | 改动类型 |
|----|---------|---------|
| 后端 Handler | `backend/internal/handler/message.go` | 增加冷启动校验 |
| 后端 Handler | `backend/internal/handler/notification.go` | 新增 Broadcast 处理器 |
| 后端 Handler | `backend/internal/handler/admin.go` | 注册广播路由 |
| 后端 Router | 当前唯一 route owner | 挂载 `/api/v1/admin/notifications/broadcast`；hardening Task 3 后使用 `internal/router/routes.go` |
| 后端 Repo | `backend/internal/repository/message_repo.go` | 增加对方消息计数查询 |
| 后端 Service | `backend/internal/service/notification_service.go` | 新增 Broadcast 业务方法 |
| 后端 Repo | `backend/internal/repository/notification_repo.go` | 新增批量插入方法 |
| 后端 Migration | `backend/migrations/057_add_broadcast_channel.sql` | 重建 notifications channel CHECK 约束 |
| 前端页面 | `frontend/app/(protected)/messages/page.tsx` | 重构为左侧会话列表 + 右侧对话窗口双栏布局 |
| 前端页面 | `frontend/app/(protected)/admin/notifications/page.tsx` | **新增**广播编辑页 |
| 前端组件 | `frontend/components/social/ChatWindow.tsx` | 修正私信 API 调用并处理 DM_REPLY_REQUIRED |
| 前端组件 | `frontend/components/social/NotificationList.tsx` | 支持系统通知样式区分 |
| 前端组件 | `frontend/components/social/ConversationList.tsx` | 适配新布局 |
| 前端 i18n | `frontend/messages/zh.json` | 新增翻译 key |
| 前端 i18n | `frontend/messages/en.json` | 新增翻译 key |

---

## 2. 浏览足迹增强

### 2.1 背景

当前系统已有 `browse_history` 表和基础 CRUD API（GET / POST / DELETE `/api/v1/users/me/history`），前端已有基础页面 `/history`。本章增强数据保留策略、查询能力和前端交互体验。

### 2.2 数据保留策略（双重保障）

| 层 | 策略 | 实现 |
|----|------|------|
| 查询层 | 所有查询强制 `WHERE viewed_at >= NOW() - INTERVAL '<retention_days> days'`（retention_days 从 config.yaml > browse_history.retention_days 参数化读取，禁止硬编码） | Service 层 WHERE 条件 |
| 存储层 | 定时任务每日凌晨清理 | `DELETE FROM browse_history WHERE viewed_at < NOW() - INTERVAL '<retention_days> days'`（参数化同上） |

定时任务复用现有 scheduler 框架（`backend/internal/pkg/scheduler/`），每天凌晨 3:00 执行。

> **实现注意**：当前 scheduler 框架为 ticker 固定间隔模式，不支持 cron 表达式。实现浏览足迹清理任务时需选择以下方案之一（**推荐方案 B**，实现简单、无框架改动风险；方案 A 为长期方向，后续可统一迁移）：
> - 方案 A（长期方向）：扩展 scheduler 框架增加 CronJob 接口，支持 cron 标签
> - **方案 B（MVP 推荐）**：读取 `config.yaml > browse_history.cleanup_time`（格式 `"HH:MM"`，如 `"03:00"`），使用 time.AfterFunc 计算到次日该时刻的首次延迟执行清理，**在清理函数末尾重新调用 time.AfterFunc 计算下一个 24h 延迟以形成自循环**（time.AfterFunc 为一次性触发，不会自动重复）
> - 方案 C：将清理逻辑作为 24h 间隔 ticker，在每日首次 tick 时检查是否已过 cleanup_time

### 2.3 API 增强

#### GET /api/v1/users/me/history（改造）

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `content_type` | string | 否 | 按内容类型筛选 |
| `start_date` | string | 否 | 筛选起始日期（YYYY-MM-DD） |
| `end_date` | string | 否 | 筛选结束日期（YYYY-MM-DD） |
| `page` | int | 否 | 分页页码，默认 1 |
| `page_size` | int | 否 | 每页条数，默认 20，最大 100 |
| `limit` | int | 否 | `page_size` 的兼容别名；若两者同时存在，以 `page_size` 为准 |

`content_type` 仅接受标准内容类型枚举值（`image` / `article` / `video` / `audio` / `template` / `sheet_music` / `mod` / `prompt` / `other`）。非法值返回 `400 INVALID_CONTENT_TYPE`。

**Response**:
```json
{
  "items": [
    {
      "id": 1234,
      "content": {
        "id": 567,
        "title": "示例内容标题",
        "zone": "original",
        "content_type": "article",
        "cover_image_url": "https://oss.example.com/uploads/...",
        "author": {
          "id": 1,
          "username": "作者名",
          "avatar_url": "..."
        }
      },
      "viewed_at": "2026-06-29T15:30:00Z"
    }
  ],
  "total": 150,
  "page": 1,
  "page_size": 20
}
```

> **兼容说明**：现有前端和后端使用 `history` 字段。实施时后端响应应在一个 Beta 版本窗口内同时返回 `items` 和 `history`，两者为同一 DTO 数组、排序一致、分页元数据一致；新前端使用 `items`，旧前端不立即破坏。版本窗口结束后如需移除 `history`，必须先确认无旧前端调用。

> **已失效内容处理**：满足以下任一条件的内容视为「已失效」：
> - `content_items.status != 'published'`（已知非 published 状态包括 `pending`、`under_review`、`author_deleted`；后者将在 Task 103 完成后启用）
> - `content_items.deleted_at IS NOT NULL`
>
> 对于已失效内容，API 响应中 `items[*].content` 与兼容字段 `history[*].content` 均返回 `null`，`viewed_at` 正常返回。
> 前端渲染为灰色占位卡片，显示「内容已删除或下架」，不可点击跳转。
> 仅影响浏览足迹列表的展示；该足迹记录本身正常保留至过期清理。

#### DELETE /api/v1/users/me/history（改造）

```json
// 删除指定记录
{ "ids": [1234, 1235, 1236] }

// 清空全部（必须显式确认模式）
{ "clear_all": true }
```

缺失 body、空对象或 `{ "ids": [] }` 均返回 `400 CLEAR_CONFIRMATION_REQUIRED`；`clear_all:true` 与非空 `ids` 同时出现返回 `400 DELETE_MODE_CONFLICT`。该规则防止代理/中间件意外丢弃 DELETE body 后清空全部数据。

### 2.4 前端页面设计

#### 状态覆盖

| 状态 | 展示 |
|------|------|
| 加载中 | Skeleton 卡片 x 5（封面灰色块 + 文字线） |
| 空状态 | 插图 +「暂无浏览记录，去发现有趣的内容吧」+「去首页」按钮 |
| 网络错误 | Toast「加载失败，请稍后重试」+ 保留最后成功加载的数据 |
| 全部加载 | 底部文字「仅展示最近 7 天记录」 |

#### 交互

| 交互 | 行为 |
|------|------|
| 类型筛选 | 点击筛选芯片 → 重新请求 API（带 content_type），当前选中高亮 |
| 日期筛选 | 点击日历图标 → 弹出简化日期范围选择器 |
| 批量管理 | 切换开关 → 每个卡片出现 checkbox → 选中后顶部出现「删除选中(N)」按钮 |
| 清空全部 | 点击 → 确认对话框 → 调 DELETE → 刷新为空状态 |
| 点击有效卡片 | 跳转到 content/[contentId] |
| 已失效卡片 | 灰色不可点击 |

### 2.5 业务约束

| 约束 | 说明 |
|------|------|
| 保留天数配置驱动 | 7 天默认值从 config.yaml > browse_history.retention_days 读取，SQL 参数化注入，禁止硬编码 |
| 物理删除 | 浏览历史无审计价值，采用物理删除（DELETE），不软删除；与本文设计原则中的「软删除优先」例外兼容 |
| 已失效内容处理 | status != 'published' 或 deleted_at IS NOT NULL 的内容返回 content: null，前端灰色占位不可点击 |
| 权限隔离 | 仅查询/删除自己 history，禁止跨用户访问 |
| 批量删除上限 | 单次请求 ids 数组上限 100 条，防止误操作 |
| 清空全部语义 | 仅 `{ "clear_all": true }` 清空全部；前端必须 ConfirmModal 二次确认，空/缺失 body 不执行删除 |
| 时间语义 | 保留期按数据库 `viewed_at` 与服务器当前时间计算；清理任务的 `cleanup_time` 固定按 Asia/Shanghai 解释，日志中记录下一次计划执行时间 |
| 多实例清理 | 每次清理在删除事务内使用 `pg_try_advisory_xact_lock` 竞争 leader；未获锁的副本记录 skipped 并正常重排下次执行 |

### 2.6 涉及文件清单

| 层 | 文件路径 | 改动类型 |
|----|---------|---------|
| 后端 Handler | `backend/internal/handler/browse_history.go` | 增加筛选参数、批量删除 |
| 后端 Service | `backend/internal/service/history_service.go` | **可选新增**，若保持薄 handler 则逻辑留在 repo/handler |
| 后端 Repo | `backend/internal/repository/browse_history_repo.go` | 条件查询、批量删除、7天过滤 |
| 后端 Scheduler | `backend/internal/pkg/scheduler/browse_history_cleanup.go` | **新增**每日清理任务 |
| 后端 Config | `backend/config.yaml` | 新增 browse_history 配置段 |
| 后端 Config | `backend/config/config.go` | 新增 BrowseHistoryConfig 结构体及 Config 字段映射；首版仅从 `backend/config.yaml` 加载，不纳入 Admin 配置页或 SaveOverride |
| 前端页面 | `frontend/app/(protected)/history/page.tsx` | 重构为分类筛选标签 + 卡片列表布局 |
| 前端 i18n | `frontend/messages/zh.json` | 新增翻译 key |
| 前端 i18n | `frontend/messages/en.json` | 新增翻译 key |

---

## 3. 收藏集文件夹系统

### 3.1 背景

本功能最初用于替代只能表达二元关系的 `favorites`。截至 2026-07-16，迁移 `058_create_collections.sql`、`collections` / `collection_items` 模型和原计划 Tasks 1–10 已实现并合入 `main`；旧 `favorites` 仍处于兼容窗口，默认集自愈、对账证据和最终 cutover 条件由收藏集计划 Task 11 继续跟踪。

### 3.2 数据模型

#### collections 表（新建）

```sql
CREATE TABLE collections (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    description TEXT DEFAULT '',
    zone VARCHAR(10) NOT NULL CHECK (zone IN ('original', 'fanwork')),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_collections_user_zone ON collections(user_id, zone);
CREATE UNIQUE INDEX idx_collections_one_default_per_zone
    ON collections(user_id, zone)
    WHERE is_default = TRUE;
```

| 字段 | 说明 |
|------|------|
| `user_id` | 收藏集所有者 |
| `zone` | 该收藏集只能收藏此 zone 的内容（original / fanwork） |
| `is_default` | 系统迁移或首次使用时创建的默认收藏集标记；默认收藏集不可删除 |
| `is_public` | 公开收藏集所有人可浏览；私有仅所有者可见 |
| `sort_order` | 用户在收藏集列表中的自定义排序 |

#### collection_items 表（新建）

```sql
CREATE TABLE collection_items (
    id BIGSERIAL PRIMARY KEY,
    collection_id BIGINT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    note TEXT DEFAULT '',
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (collection_id, content_item_id)
);

CREATE INDEX idx_collection_items_collection ON collection_items(collection_id);
```

| 字段 | 说明 |
|------|------|
| `note` | 用户对该收藏条目的备注（可选） |
| UNIQUE 约束 | 同一收藏集内不能重复添加同一内容 |

#### 数据迁移策略

- 迁移 `058_create_collections.sql` 已将旧 `favorites` 数据复制到新模型，并按 `content_items.zone` 归入原创区或二创区默认集。
- 注册和首次使用路径负责幂等创建两个 `is_default=TRUE` 的默认收藏集；注册辅助写失败不回滚已验证账号，缺失默认集由 Task 11 补齐 own-list 自愈和 reconciliation 证据。
- 兼容窗口内旧 favorite mutation 与默认收藏集保持双写，推荐读取 `collection_items` 与历史 `favorites` 的去重并集，避免新收藏行为从画像中丢失。
- 至少保留 `favorites` 30 天作为回滚窗口；删除不能只按日历到期触发，还必须满足：所有受支持客户端不再调用旧 mutation、只读 reconciliation 连续 7 天零漂移、推荐在只读 `collection_items` 模式下通过回归、回滚版本不再依赖旧表。最终删除由独立 forward-only 清理计划执行。

### 3.3 业务约束

| 约束 | 说明 |
|------|------|
| Zone 锁定 | 收藏集 zone 创建时确定，不可更改。添加内容时必须同 zone，跨 zone 返回 400 |
| 去重 | 同一收藏集内同一内容只能出现一次 |
| 默认收藏集保护 | 每个用户每个 zone 至少保留一个默认收藏集，不可删除 |
| 公开/私有 | 公开收藏集任何人可浏览（内容过滤 status=published）；私有收藏集仅所有者可见 |
| 内容可见性 | 收藏集内被下架/删除的内容自动过滤，不在详情页展示 |
| 旧收藏兼容 | 回滚窗口内不得破坏现有 `/api/v1/favorites` 和 `/api/v1/users/:id/favorites` 调用；旧接口应映射到默认收藏集或继续读写兼容表，避免详情页收藏按钮、用户主页和推荐画像失效 |
| 默认集自愈 | 注册后的默认集创建可非致命，但首次 authenticated own-list 和 add-to-default 必须幂等补齐；公开 owner_id 查询不得产生写入 |
| 兼容对账 | 提供默认只读 reconciliation，报告 legacy/default 两侧缺失、逻辑重复和缺少默认集；`--apply` 只能幂等补齐，不删除任一侧 |

### 3.4 API 设计

| 方法 | 路径 | 说明 | 认证 | 权限 |
|------|------|------|------|------|
| `POST` | `/api/v1/collections` | 创建收藏集 | 需要 | 信誉分 >= min_score_for_interaction |
| `GET` | `/api/v1/collections` | 列出我的收藏集 | 需要 | 仅自己的 |
| `GET` | `/api/v1/collections/:id` | 收藏集详情 + 内容列表 | 不强制 | 公开收藏集所有人可看；私有仅所有者 |
| `PUT` | `/api/v1/collections/:id` | 编辑收藏集 | 需要 | 仅所有者；仅允许修改 title/description/is_public/sort_order；修改 zone 返回 400 ZONE_IMMUTABLE |
| `DELETE` | `/api/v1/collections/:id` | 删除收藏集（级联） | 需要 | 仅所有者；默认收藏集不可删 |
| `POST` | `/api/v1/collections/:id/items` | 添加内容 | 需要 | 仅所有者 |
| `DELETE` | `/api/v1/collections/:id/items/:itemId` | 移除内容 | 需要 | 仅所有者 |
| `PUT` | `/api/v1/collections/:id/items/:itemId` | 编辑备注 | 需要 | 仅所有者 |

#### 错误码

| 错误码 | HTTP | 场景 |
|--------|------|------|
| `COLLECTION_NOT_FOUND` | 404 | 收藏集不存在或无权访问（私有收藏集非所有者） |
| `ZONE_MISMATCH` | 400 | 跨 zone 添加内容（如将 original 内容加入 fanwork 收藏集） |
| `DUPLICATE_COLLECTION_ITEM` | 409 | 同一收藏集内重复添加同一内容 |
| `DEFAULT_COLLECTION_PROTECTED` | 400 | 不允许删除默认收藏集 |
| `ZONE_IMMUTABLE` | 400 | 不允许修改已创建收藏集的 zone 字段 |
| `INVALID_CONTENT` | 400 | 内容不存在或 status 非 published |
| `REPUTATION_TOO_LOW` | 403 | 信誉分低于 min_score_for_interaction |

**POST /api/v1/collections 请求体**:
```json
{
  "title": "神作收藏",
  "description": "个人认为的神作",
  "zone": "original",
  "is_public": true
}
```

**POST /api/v1/collections/:id/items 请求体**:
```json
{
  "content_item_id": 567,
  "note": "第三章特别精彩"
}
```

**GET /api/v1/collections/:id 响应**:
```json
{
  "collection": {
    "id": 1,
    "title": "神作收藏",
    "description": "个人认为的神作",
    "zone": "original",
    "is_default": false,
    "is_public": true,
    "owner": { "id": 1, "username": "xxx", "avatar_url": "..." },
    "item_count": 8,
    "created_at": "2026-06-01T00:00:00Z"
  },
  "items": [
    {
      "id": 1,
      "content": { "...": "content DTO" },
      "note": "第三章特别精彩",
      "added_at": "2026-06-15T12:00:00Z"
    }
  ],
  "total": 8,
  "page": 1
}
```

### 3.5 前端设计

#### 收藏流程

点击内容详情页的收藏按钮 → 弹出 CollectionPicker 弹窗：

```
+----------------------------------+
|  选择收藏集         [+ 新建]     |
|  [🔍 搜索...]                    |
|                                  |
|  ● 默认收藏集 (12)              |
|  ○ 神作收藏集 (8)               |
|  ○ 教程收藏集 (3)               |
|                                  |
|  ---- 已添加 神作收藏集 ----    |
+----------------------------------+
```

- 弹窗列出当前 zone 的收藏集
- 点选即保存，无需二次确认
- 「已添加」提示 2 秒后自动消失
- 「+ 新建」直接在弹窗内展开简易表单（标题 + 公开性），创建后自动选中
- 收藏集数量 ≥ 10 个时，列表顶部显示搜索框，支持按标题实时过滤

#### 收藏集管理页 /studio/favorites

按 zone 分区展示收藏集列表，每个收藏集显示名称、内容数量、公开/隐私图标、编辑和删除按钮。

#### 收藏集详情页 /collections/[id]

- 展示收藏集标题、作者信息、内容数量
- 内容以卡片流展示
- 支持分页加载
- 若为所有者 → 卡片右上角显示移除按钮
- 公开收藏集：其他用户可浏览，卡片无操作按钮
- 路由必须放在 `frontend/app/(public)/collections/[id]/page.tsx`，因为公开收藏集允许未登录或非所有者浏览；私有收藏集由后端权限返回 403/404，前端展示 EmptyState。

### 3.6 涉及文件清单

| 层 | 文件路径 | 改动类型 |
|----|---------|---------|
| 后端 Migration | `backend/migrations/058_create_collections.sql` | **新增**建表 + 迁移旧数据 |
| 后端 Model | `backend/internal/model/collection.go` | **新增** |
| 后端 Handler | `backend/internal/handler/collection.go` | **新增** |
| 后端 Service | `backend/internal/service/collection_service.go` | **新增** |
| 后端 Repo | `backend/internal/repository/collection_repo.go` | **新增** |
| 后端 Router | 当前唯一 route owner | 注册新路由；hardening Task 3 后使用 `internal/router/routes.go` |
| 前端页面 | `frontend/app/(protected)/studio/favorites/page.tsx` | 重构为收藏集管理页 |
| 前端页面 | `frontend/app/(public)/collections/[id]/page.tsx` | **新增**收藏集详情页 |
| 前端组件 | `frontend/components/content/CollectionPicker.tsx` | **新增**收藏集选择弹窗 |
| 前端 i18n | `frontend/messages/zh.json` | 新增翻译 key |
| 前端 i18n | `frontend/messages/en.json` | 新增翻译 key |

---

## 4. 内容系列 (Content Series) 功能

### 4.1 背景

当前系统缺少「内容系列」概念。创作者发布系列内容（如连载小说、系列视频）时，用户看完一章后需要回到作者主页手动寻找下一章。内容系列功能允许创作者将自己的内容归入有序系列，用户在内容详情页即可直接导航到上一章/下一章。

### 4.2 数据模型

#### content_series 表（新建）

```sql
CREATE TABLE content_series (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    description TEXT DEFAULT '',
    cover_content_id BIGINT REFERENCES content_items(id) ON DELETE SET NULL,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    zone VARCHAR(10) NOT NULL CHECK (zone IN ('original', 'fanwork')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_content_series_owner ON content_series(owner_id);
```

| 字段 | 说明 |
|------|------|
| `cover_content_id` | 可选 FK，引用 `content_items(id) ON DELETE SET NULL`。后端 GET /series/:id 响应中的 `cover` 字段逻辑：① 若 cover 内容仍在系列且可见 → 返回首图；② 否则返回 `sort_order` 最小的可见内容封面；③ 无可见内容时 `cover: null`。前端直接使用 `cover` 字段。 |
| `zone` | 内容系列只能包含该 zone 的内容 |

#### content_series_items 表（新建）

```sql
CREATE TABLE content_series_items (
    id BIGSERIAL PRIMARY KEY,
    series_id BIGINT NOT NULL REFERENCES content_series(id) ON DELETE CASCADE,
    content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    sort_order INT NOT NULL DEFAULT 0,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (series_id, content_item_id)
);

CREATE INDEX idx_series_items_series ON content_series_items(series_id);
CREATE INDEX idx_series_items_content ON content_series_items(content_item_id);
```

### 4.3 业务约束

| 约束 | 说明 |
|------|------|
| 作者权限 | 只有内容系列 owner 可以管理（添加/移除/排序）。可添加的内容必须满足 `content_items.author_id = owner_id` 或 `content_contributors` 中包含 owner；不能只检查 content_contributors，否则作者自己的历史内容会被误拒 |
| 多对多 | 一个内容可以属于多个内容系列 |
| 手动排序 | `sort_order` 由用户控制，非自动 |
| Zone 一致 | 内容系列 zone 固定，不能混入跨区内容 |
| 公开性 | 首版只做公开系列，所有内容系列对所有用户可见；私有系列延后 |
| 删除级联 | 删除内容系列 → 级联删除其中的条目关系（不删除内容本身） |
| 管理与公开可见性 | 当前状态机没有独立 draft/pending_review；owner 可把自己/已贡献的 `pending`、`published` 内容加入系列预排，`under_review` 不可加入；匿名/非 owner 的系列详情、item_count 和前后导航只返回 published、非删除、非 banned/author_deleted 内容 |

### 4.4 API 设计

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| `POST` | `/api/v1/series` | 创建内容系列 | 需要 |
| `GET` | `/api/v1/series` | 列出我的内容系列 | 需要 |
| `GET` | `/api/v1/series/:id` | 内容系列详情 + 内容列表 | 不强制 |
| `PUT` | `/api/v1/series/:id` | 编辑内容系列信息 | 需要（仅 owner） |
| `DELETE` | `/api/v1/series/:id` | 删除内容系列 | 需要（仅 owner） |
| `POST` | `/api/v1/series/:id/items` | 添加内容 | 需要（仅 owner） |
| `DELETE` | `/api/v1/series/:id/items/:itemId` | 移除内容 | 需要（仅 owner） |
| `PUT` | `/api/v1/series/:id/items/reorder` | 重排序 | 需要（仅 owner） |

#### 错误码

| 错误码 | HTTP | 场景 |
|--------|------|------|
| `SERIES_NOT_FOUND` | 404 | 内容系列不存在 |
| `NOT_SERIES_OWNER` | 403 | 非系列 owner 无权管理（添加/移除/排序/编辑） |
| `CONTENT_NOT_OWNED_OR_CONTRIBUTED` | 400 | 添加的内容既不是 owner 发布，也不在 content_contributors 中包含 owner |
| `ZONE_MISMATCH` | 400 | 跨 zone 添加内容到系列 |
| `DUPLICATE_SERIES_ITEM` | 409 | 同一系列内重复添加同一内容 |
| `COVER_NOT_IN_SERIES` | 400 | cover_content_id 指向的内容不在该系列中 |

**PUT .../reorder 请求体**:
```json
{
  "item_ids": [102, 98, 105, 99]
}
```
按数组顺序更新 sort_order（index 0 → sort_order 0, index 1 → sort_order 1...）

**事务实现说明**：重排序必须在单个数据库事务内完成，使用 `BEGIN ... COMMIT`（或 GORM `Transaction` 回调）包裹全部 UPDATE 语句；并发 reorder 请求应通过 `SELECT ... FOR UPDATE` 对目标 series_id 行加锁，避免两个标签页同时拖拽导致 sort_order 错乱。事务失败时全部回滚，不产生部分更新。

### 4.5 前端设计

#### 内容详情页 — 内容系列导航栏

位于正文下方、评论区上方：

```
+-------------------------------------------------+
|  所属内容系列：「某某小说」  第 3 章 / 共 12 章      |
|                                                  |
|  ← 上一章：第二章 · 启程                          |
|  → 下一章：第四章 · 转机                          |
|                                                  |
|  [查看完整内容系列目录]                                |
+-------------------------------------------------+
```

- 内容属于多个内容系列时，前三个以 Tab 切换；其余通过可键盘访问的 `更多(N)` 菜单逐项链接到对应系列。后端返回全部紧凑 membership，不得在 normalizer 中先截断；前端以 `memberships.length` 作为唯一总数，不新增冗余 total 字段。
- 处于内容系列第一项时「上一章」灰显 + 「已是第一章」
- 处于内容系列最后一项时「下一章」灰显 + 「已是最后一章」
- 点击「查看完整内容系列目录」→ 跳转到内容系列详情页
- 整体视觉克制，低饱和度色彩，不抢占阅读注意力

#### 内容系列详情页 /series/[id]

展示内容系列封面、标题、作者信息、内容数量，内容以列表形式展示（按 sort_order），支持点击跳转到对应内容详情页。

路由放在 `frontend/app/(public)/series/[id]/page.tsx`。首版只做公开系列，允许未登录浏览；不存在的系列返回 404，前端展示 EmptyState。

#### Studio 内容系列管理页 /studio/series

- 侧边栏新增入口「内容系列管理」
- 内容区：我的内容系列列表 +「+ 新建内容系列」
- 点击内容系列进入编辑模式 → 可添加/移除内容 + 拖拽排序

#### 排序交互

- 拖拽手柄上下拖动条目
- 或使用上/下箭头按钮逐级移动
- 每次调整立即调 PUT reorder 接口保存

### 4.6 涉及文件清单

| 层 | 文件路径 | 改动类型 |
|----|---------|---------|
| 后端 Migration | `backend/migrations/059_create_content_series.sql` | **新增** |
| 后端 Model | `backend/internal/model/series.go` | **新增** |
| 后端 Handler | `backend/internal/handler/series.go` | **新增** |
| 后端 Service | `backend/internal/service/series_service.go` | **新增** |
| 后端 Repo | `backend/internal/repository/series_repo.go` | **新增** |
| 后端 Router | 当前唯一 route owner | 注册新路由；hardening Task 3 后使用 `internal/router/routes.go` |
| 前端页面 | `frontend/app/(public)/series/[id]/page.tsx` | **新增**内容系列详情页 |
| 前端页面 | `frontend/app/(protected)/studio/series/page.tsx` | **新增**内容系列管理页 |
| 前端组件 | `frontend/components/content/SeriesNav.tsx` | **新增**内容系列导航栏 |
| 前端组件 | `frontend/components/studio/StudioSidebar.tsx` | 新增内容系列管理入口 |
| 前端 i18n | `frontend/messages/zh.json` | 新增翻译 key |
| 前端 i18n | `frontend/messages/en.json` | 新增翻译 key |

---

## 5. 联合创作邀请

### 5.1 背景

当前系统支持通过 PR（Pull Request）进行内容协作，但缺少在发布时直接邀请联合创作者的能力。用户希望发布内容时能勾选联合创作者，被邀请者通过私信确认后方可列入创作者名单。

> **前置依赖**：本子系统依赖 §1 的私信基础设施（conversations / conversation_participants / messages 表）。
> 实现前需熟悉 §1.2 冷启动限制规则（对方未回复时限发 1 条）和 §1.4 私信对话窗口设计。
> 联合创作邀请卡片通过 messages 表的 `msg_type='collab_invite'` + `metadata` 字段承载，metadata 至少包含 `invite_id`、`content_id`、`content_title`、`inviter_id`、`inviter_username`，前端据此渲染卡片并调用 accept/decline API。

### 5.2 数据模型

#### collaboration_invites 表（新建）

```sql
CREATE TABLE collaboration_invites (
    id BIGSERIAL PRIMARY KEY,
    content_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'declined', 'expired')),
    message_id BIGINT REFERENCES messages(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    responded_at TIMESTAMPTZ
);

CREATE INDEX idx_collab_invites_invitee ON collaboration_invites(invitee_id, status);
CREATE INDEX idx_collab_invites_inviter ON collaboration_invites(inviter_id);
-- 部分唯一索引：仅对 pending/accepted 状态强制 (content_id, invitee_id) 唯一，允许 expired 后重新邀请
CREATE UNIQUE INDEX idx_collab_invites_active ON collaboration_invites (content_id, invitee_id)
    WHERE status IN ('pending', 'accepted');
```

#### 邀请过期规则

| 项目 | 规则 |
|------|------|
| 过期时长 | config.yaml > collaboration.invite_expire_days 天（默认 7 天） |
| 触发方式 | scheduler 定时任务每日扫描 status='pending' 且 created_at 早于 invite_expire_days 天前的记录；更新事务先竞争 PostgreSQL advisory leader lock，避免多副本重复执行 |
| 过期后前端 | 邀请卡片灰色显示「已过期」，无操作按钮 |
| 过期提醒 | 不发送即将过期提醒；仅在邀请卡片中展示过期状态 |
| 过期后重新邀请 | 允许邀请者重新发送（走完整 8 阶段校验链，视为新邀请）。**注意**：需要将 UNIQUE 约束改为 PostgreSQL 部分唯一索引 `CREATE UNIQUE INDEX idx_collab_invites_active ON collaboration_invites (content_id, invitee_id) WHERE status IN ('pending', 'accepted')`，否则旧 expired 记录仍会阻止新邀请插入 |
| 迁移文件 | `061_collaboration_invites.sql` 的 CREATE TABLE 语句中**不使用**行内 `UNIQUE (content_id, invitee_id)` 约束；表创建后用 `CREATE UNIQUE INDEX idx_collab_invites_active ON collaboration_invites (content_id, invitee_id) WHERE status IN ('pending', 'accepted')` 建立部分唯一索引 |

#### users 表新增字段

```sql
ALTER TABLE users ADD COLUMN accept_collab_invites BOOLEAN NOT NULL DEFAULT TRUE;
```

#### messages 表扩展

```sql
ALTER TABLE messages ADD COLUMN msg_type VARCHAR(20) NOT NULL DEFAULT 'text'
    CHECK (msg_type IN ('text', 'collab_invite'));

ALTER TABLE messages ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}';
```

| 字段 | 说明 |
|------|------|
| `msg_type` | `text` 为普通私信，`collab_invite` 为联合创作邀请卡片 |
| `metadata` | 邀请卡片渲染所需摘要数据；不得包含敏感信息，状态最终以 collaboration_invites 表为准 |

### 5.3 发送邀请 — 防骚扰校验链

**POST /api/v1/contents/:id/collab-invites**

请求体：
```json
{ "invitee_id": 42 }
```

校验链（按顺序）：

1. 内容存在、未软删除、状态为 `pending` 或 `published`，且当前用户是 author 或已确认 contributor；`under_review`/`banned`/`author_deleted` 均不可邀请 → 否则 403/404
2. invitee 是未封禁、未删除的普通用户，且不是 inviter、内容 author 或现有 contributor；先做容量预检，最终容量在创建事务中锁定内容行后复核
3. 当前用户信誉分 >= config.yaml reputation.min_score_for_interaction → 否则 403
4. 今天已发送邀请数 < `config.yaml > collaboration.invite_daily_limit`（默认 20）→ 否则 429「今日邀请次数已达上限」
5. 今天未对该 invitee 发过邀请 → 否则 409「今日已向该用户发送过邀请」
6. inviter 与 invitee 之间不存在任意方向的拉黑关系（author_blocklist 双向检查）
   - 检查 (author_id=inviter, blocked_id=invitee) 或 (author_id=invitee, blocked_id=inviter)
   - 任一方向命中即拒绝 403 INVITE_BLOCKED
7. invitee.accept_collab_invites == TRUE → 否则 403「该用户已关闭联合创作邀请接收」
8. 该内容尚未向该 invitee 发过 active 邀请（部分唯一索引）→ 否则 409

全部通过 → 在事务中锁定内容行，重新校验内容/邀请对象，并计算“已确认 contributor + 非 contributor 的 active pending invite”；加入本次邀请后不得超过 `max_contributors_per_item` → 创建 collaboration_invites 记录 → 查找或创建 invitee_inviter 私信会话 → 创建消息 `msg_type='collab_invite'`、`body='联合创作邀请'`、`metadata` 摘要 → 回填 collaboration_invites.message_id。内容行锁保证同一内容的并发邀请不能超额预占可接受名额。

#### 响应邀请接口

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| `POST` | `/api/v1/collab-invites/:id/accept` | 接受邀请，写入 `content_contributors`，更新 status/responded_at | 需要，仅 invitee |
| `POST` | `/api/v1/collab-invites/:id/decline` | 拒绝邀请，更新 status/responded_at | 需要，仅 invitee |

接受邀请必须在数据库事务内完成：
1. `SELECT ... FOR UPDATE` 锁定 collaboration_invites 行，并锁定其 content_items 父行
2. 校验 status='pending' 且未过期
3. invitee 尚非 contributor 时，在内容锁下复核 contributor 数；达到 `max_contributors_per_item` 则返回 `CONTRIBUTOR_LIMIT_REACHED` 并保持 invite pending
4. 插入 `content_contributors(content_item_id, user_id)`，使用 upsert 保证幂等
5. 更新 invite status='accepted' 与 responded_at
6. 返回最新 invite DTO；前端据此将卡片变为「已接受」

#### 错误码

| 错误码 | HTTP | 场景 |
|--------|------|------|
| `NOT_CONTENT_OWNER` | 403 | 当前用户非内容作者或已确认 contributor |
| `REPUTATION_TOO_LOW` | 403 | 信誉分低于 min_score_for_interaction |
| `INVITE_DAILY_LIMIT` | 429 | 今日邀请次数已达 collaboration.invite_daily_limit 上限 |
| `INVITE_DUPLICATE_USER` | 409 | 今日已向该 invitee 发送过邀请 |
| `INVITE_BLOCKED` | 403 | inviter 与 invitee 存在任意方向的拉黑关系（author_blocklist 双向检查） |
| `INVITE_NOT_ACCEPTING` | 403 | invitee.accept_collab_invites 为 FALSE |
| `INVITE_ALREADY_EXISTS` | 409 | 该内容已向该 invitee 发送过邀请（UNIQUE 约束冲突） |
| `INVITE_SELF_NOT_ALLOWED` | 400 | inviter 与 invitee 相同 |
| `INVITE_AUTHOR_NOT_ALLOWED` | 400 | invitee 是内容作者 |
| `INVITE_ALREADY_CONTRIBUTOR` | 409 | invitee 已是内容 contributor |
| `INVITEE_UNAVAILABLE` | 404 | invitee 被封禁、删除或不可用 |
| `CONTENT_UNAVAILABLE` | 404 | 内容被删除、banned 或 author_deleted |
| `CONTRIBUTOR_LIMIT_REACHED` | 409 | 已达到 `max_contributors_per_item` |
| `INVITE_EXPIRED` | 400 | 邀请已过期，无法执行 accept/decline |
| `INVITE_NOT_FOUND` | 404 | 邀请记录不存在 |

#### 发送频率限制实现

使用 Redis 原子 reservation：
- Key: `collab_invite_count:{inviter_id}:{YYYY-MM-DD}`，每次发送 +1，EXPIRE 86400 秒
- 每日上限 20（从 `config.yaml > collaboration.invite_daily_limit` 读取）
- 同用户每日限邀：`collab_invite_user:{inviter_id}:{invitee_id}:{YYYY-MM-DD}`，SET NX EX 86400
- 日计数和同用户 key 必须由一个 Lua 脚本原子检查/预留；DB 失败后的补偿同样使用带 reservation token 的 Lua，不能由分离的 GET/INCR/SET/DECR 组成

#### 1:1 会话查找实现说明

查找或创建 invitee_inviter 1:1 私信会话时（不变更现有 conversations / conversation_participants 数据模型）：

- 查询同时包含 inviter_id 和 invitee_id 两个 participant 的 conversation_id（JOIN conversation_participants 自连接，按 conversation_id GROUP BY HAVING COUNT(DISTINCT user_id) = 2）
- 若已存在 → 复用该会话；若不存在 → 新建 conversation + 插入两条 conversation_participants 记录
- 现有表无 (user_a, user_b) 复合查找索引；建议在 `061_collaboration_invites.sql` 迁移中补充 `idx_conv_participants_user_pair` 加速查找（非强制，可按用户量评估）

### 5.4 私信邀请卡片 UI

```
+--------------------------------------+
|  联合创作邀请                         |
|                                      |
|  @创作者A 邀请你参与联合创作         |
|  内容：「某某小说」                   |
|                                      |
|  [接受]  [拒绝]                      |
+--------------------------------------+
```

- 接受 → `POST /api/v1/collab-invites/:id/accept` → content_contributors 插入记录 → 卡片变为「已接受」
- 拒绝 → `POST /api/v1/collab-invites/:id/decline` → 卡片变为「已拒绝」
- 只在 status='pending' 时展示按钮
- 内容发布后邀请仍可响应（不阻断）
- `status='expired'` → 卡片灰色，显示「已过期」，无操作按钮

### 5.5 用户设置扩展

前端设置页 `/settings` 新增开关：「接收联合创作邀请」，默认开启，对应 `users.accept_collab_invites` 字段。

该字段必须贯穿用户数据链路：
- `backend/internal/model/user.go` 的 `User` 增加 `AcceptCollabInvites bool`
- `backend/internal/handler/user.go` 的 `PATCH /api/v1/users/:id` 允许当前用户更新该字段
- `GET /api/v1/auth/me` 与 `sanitizeUser`/用户 DTO 返回该字段，前端 `AuthContext.User` 类型同步增加
- 前端设置页保存后调用 `refreshUser()`，保证开关状态立即反映在当前会话

### 5.6 涉及文件清单

| 层 | 文件路径 | 改动类型 |
|----|---------|---------|
| 后端 Migration | `backend/migrations/061_collaboration_invites.sql` | **新增**建表 + users.accept_collab_invites + messages.msg_type/metadata |
| 后端 Model | `backend/internal/model/collab_invite.go` | **新增** |
| 后端 Handler | `backend/internal/handler/collab_invite.go` | **新增** |
| 后端 Service | `backend/internal/service/collab_invite_service.go` | **新增**（含防骚扰校验链） |
| 后端 Repo | `backend/internal/repository/collab_invite_repo.go` | **新增** |
| 后端 Scheduler | `backend/internal/pkg/scheduler/collab_invite_expiry.go` | **新增**邀请过期清理任务（time.AfterFunc 自循环，详见 §2.2 方案 B） |
| 后端 Handler | `backend/internal/handler/message.go` | 消息创建支持 msg_type/metadata |
| 后端 Model | `backend/internal/model/user.go` | 增加 accept_collab_invites 字段映射 |
| 后端 Handler | `backend/internal/handler/user.go` | 允许用户更新 accept_collab_invites，并在用户 DTO 中返回 |
| 后端 Router | 当前唯一 route owner | 注册新路由；hardening Task 3 后使用 `internal/router/routes.go` |
| 后端 Config | `backend/config.yaml` | 新增 collaboration 配置段 |
| 后端 Config | `backend/config/config.go` | 新增 CollaborationConfig 结构体及 Config 字段映射；不纳入 Admin 配置页或 SaveOverride；PublicConfig 仅暴露安全的 max_invitees_per_publish |
| 后端 Public Config | `backend/internal/handler/public_config.go` | 新增 `collaboration.max_invitees_per_publish`，不暴露日限、过期或 contributor cap |
| 前端组件 | `frontend/components/content/CollabUserPicker.tsx` | **新增**联合创作者搜索选择 |
| 前端组件 | `frontend/components/social/CollabInviteCard.tsx` | **新增**私信邀请卡片 |
| 前端页面 | `frontend/app/(protected)/settings/page.tsx` | 新增邀请接收开关 |
| 前端 i18n | `frontend/messages/zh.json` | 新增翻译 key |
| 前端 i18n | `frontend/messages/en.json` | 新增翻译 key |

---

## 6. 原创/二创区联动增强

### 6.1 背景

当前系统已实现基于 `source_original_id` 的二创→原创单向绑定，前端原创详情页已有「相关二创」入口。但体验仍较基础，且缺少二创基于二创（衍生创作）的来源追溯能力。

### 6.2 来源链模型重构

#### 新增字段

```sql
ALTER TABLE content_items ADD COLUMN source_fanwork_id BIGINT
    REFERENCES content_items(id) ON DELETE SET NULL;

-- 参照 migration 036 的 idx_content_items_source_original 格式
CREATE INDEX IF NOT EXISTS idx_content_items_source_fanwork
    ON content_items (source_fanwork_id, status, created_at DESC)
    WHERE source_fanwork_id IS NOT NULL;
```

#### 发布时校验规则

```
zone = 'original':
  source_original_id IS NOT NULL → 400 "原创内容不能绑定来源原创"
  source_fanwork_id IS NOT NULL   → 400 "原创内容不能绑定来源二创"

zone = 'fanwork':
  若 ip_id IS NULL
     AND source_original_id IS NULL
     AND source_fanwork_id IS NULL
       → 400 "二创内容必须指定 IP 或灵感来源"

  若 source_original_id IS NOT NULL AND source_fanwork_id IS NOT NULL
       → 400 "只能指定一个灵感来源（原创或二创，不可同时指定）"

  若 source_original_id IS NOT NULL:
     → 目标 content 必须 zone='original' AND status='published'
     → 否则 400 "来源原创内容不存在或不可用"

  若 source_fanwork_id IS NOT NULL:
     → 目标 content 必须 zone='fanwork' AND status='published'
     → 否则 400 "来源二创内容不存在或不可用"

  更新已有内容的 ip_id/source_original_id/source_fanwork_id 时:
     → 首版来源归因创建后不可变，返回 400 SOURCE_IMMUTABLE
```

#### 来源链示意图

```
原创内容                  二创(基于原创)            衍生创作(基于二创)
zone=original             zone=fanwork              zone=fanwork
ip_id: set                source_original_id: set    source_fanwork_id: set
                          ip_id: optional            ip_id: optional
```

### 6.3 API 调整

#### GET /api/v1/contents/:id/related-fanworks（增强）

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `content_type` | string | 否 | 按内容类型筛选；允许逗号分隔 allowlisted 多值，如 `article,prompt` |
| `sort` | string | 否 | `hot`（热度默认）/ `new`（最新） |
| `page` | int | 否 | 分页页码 |
| `page_size` | int | 否 | 每页条数，默认 24，最大 100 |
| `limit` | int | 否 | `page_size` 的兼容别名；若两者同时存在，以 `page_size` 为准 |

**查询逻辑增强**：
- 当内容 zone = 'original' → 查所有 `source_original_id = :id` 的二创
- 当内容 zone = 'fanwork' → 额外查 `source_fanwork_id = :id` 的衍生作品

#### POST /api/v1/contents（改造）

- 增加 `source_fanwork_id` 可选字段
- 执行上述校验规则
- GET content 响应 DTO 增加 `source_fanwork_id` 字段；若来源可见，响应顶层额外返回 `source_fanwork: { id, title, zone }`，并将现有 `source_original` 摘要统一为 `{ id, title, zone }`
- 前端 `frontend/lib/content.ts` 的 normalize 需兼容 snake_case 与 PascalCase，新增 `source_fanwork_id`，缺少有效 id/title/zone 的内容不得渲染为可点击卡片

#### 错误码

| 错误码 | HTTP | 场景 |
|--------|------|------|
| `SOURCE_NOT_ALLOWED_FOR_ORIGINAL` | 400 | zone='original' 时携带 source_original_id 或 source_fanwork_id |
| `FANWORK_SOURCE_REQUIRED` | 400 | zone='fanwork' 且 ip_id / source_original_id / source_fanwork_id 三者均为空 |
| `MULTIPLE_SOURCE_CONFLICT` | 400 | 同时指定 source_original_id 与 source_fanwork_id |
| `SOURCE_ORIGINAL_UNAVAILABLE` | 400 | source_original_id 指向的内容非 zone='original' AND status='published' |
| `SOURCE_FANWORK_UNAVAILABLE` | 400 | source_fanwork_id 指向的内容非 zone='fanwork' AND status='published' |
| `SOURCE_IMMUTABLE` | 400 | 更新接口尝试修改 ip_id/source_original_id/source_fanwork_id；首版来源归因创建后不可变 |

### 6.4 前端设计（克制原则，不喧宾夺主）

#### 1. 原创详情页 — 相关二创作品集

位置：正文下方，评论区上方。仅在 `related-fanworks.total > 0` 时展示。

- 横向滚动卡片行，每张卡片为小号 ContentCard（封面 + 标题 + @作者 + 点赞数）
- 最多 8 张，超出 →「查看全部」
- 底部「开始创作」入口：`/studio/publish/fanwork?source_original_id=<id>`

#### 2. 二创详情页 — 灵感来源

位置：内容标题区下方，正文上方。小字，轻量。

- source_original_id → 链接到原创详情页
- source_fanwork_id → 链接到来源二创详情页
- 仅 ip_id（无内容级来源）→ 不显示此行
- 来源内容已删除/下架 → 灰色文字「灵感来源：内容已下架」，不可点击

#### 3. 二创详情页 — 衍生创作

位置：正文下方、评论区上方。仅在该二创有三创时展示。

- 横向滚动卡片行，与相关二创作品集组件一致
- 对外统一称为「衍生作品」而非「三创」，链路仅展示一层

### 6.5 旧数据迁移

- 现有二创的 `source_fanwork_id` 全部设为 NULL（不回填历史数据）
- 现有 `source_original_id` 数据保持不变
- `ip_id` 为 NULL 且无来源绑定的历史二创：不强制修复（不阻断展示），仅新发布时执行三选一校验

### 6.6 涉及文件清单

| 层 | 文件路径 | 改动类型 |
|----|---------|---------|
| 后端 Migration | `backend/migrations/060_add_source_fanwork_id.sql` | **新增** |
| 后端 Model | `backend/internal/model/content.go` | 新增 SourceFanworkID / SourceFanwork 字段 |
| 后端 Handler | `backend/internal/handler/content.go` | 更新发布/查询校验，返回 source_fanwork 摘要 |
| 后端 Service | `backend/internal/service/content_service.go` | 更新校验逻辑 + 链式查询 |
| 后端 Repo | `backend/internal/repository/content_repo.go` | related-fanworks 增强查询，ListContentsFilter 增加 SourceFanworkID |
| 前端 Lib | `frontend/lib/content.ts` | normalize source_fanwork_id |
| 前端页面 | `frontend/app/(public)/content/[contentId]/page.tsx` | 灵感来源 + 衍生作品区块 |
| 前端页面 | `frontend/app/(public)/original/[contentId]/page.tsx` | 相关二创作品集 + 创作引导 |
| 前端组件 | `frontend/components/content/RelatedFanworks.tsx` | **新增**横向滚动卡片行 |
| 前端组件 | `frontend/components/content/SourceAttribution.tsx` | **新增**灵感来源小字 |
| 前端 i18n | `frontend/messages/zh.json` | 新增翻译 key |
| 前端 i18n | `frontend/messages/en.json` | 新增翻译 key |

---

## 附录 A：已确认的设计决策汇总

| 决策 ID | 内容 |
|----------|------|
| DEC-001 | 实施策略：方案 A 串行垂直切片，按依赖顺序逐一交付 |
| DEC-002 | 管理员通知推送为「系统通知频道」形式，非全员私信 |
| DEC-003 | 私信限制为「会话冷启动」模式（对方回复后永久解锁） |
| DEC-004 | 收藏集按 zone 分区（原创/二创），创建时锁定不可更改 |
| DEC-005 | 浏览足迹保留 7 天，双保障（查询过滤 + 定时清理），带分类筛选标签的独立页面 |
| DEC-006 | 内容系列仅作者管理，可手动排序，一个内容可属于多个内容系列 |
| DEC-007 | 联合创作需对方确认，全套防骚扰措施启用 |
| DEC-008 | 来源链：fanwork zone 下，ip_id / source_original_id / source_fanwork_id 三者至少填一个 |
| DEC-009 | UI 克制原则：联动引导不喧宾夺主，灵感来源展示为小字轻量链接 |
| DEC-010 | 收藏集文件夹数据模型（collections + collection_items） |
| DEC-011 | 联合创作频率限制 20 条/天，信誉分门槛复用 min_score_for_interaction，配置驱动 |
| DEC-012 | 浏览足迹已失效内容展示灰色占位卡片，不可点击 |
| DEC-013 | 内容系列导航在内容详情页正文下方、评论区上方，视觉克制 |
| DEC-014 | 对外统一称「衍生作品」而非「三创」，链路仅展示一层 |
| DEC-015 | 旧 favorites 数据迁移后保留 30 天回滚窗口 |
| DEC-016 | collections 模型扩展了 AGENTS.md Task 122-123 的旧计划：新增 zone/sort_order/is_default 字段，collection_items.content_id 更名为 content_item_id |
| DEC-017 | 内容系列添加内容时允许 owner 自己发布的内容或 owner 已确认贡献的内容，不只依赖 content_contributors |
| DEC-018 | 联合创作邀请消息使用 msg_type + metadata 承载卡片摘要，最终状态以 collaboration_invites 表为准 |
| DEC-019 | 公开可浏览页面（收藏集详情、系列详情）放在 `(public)` route group，私有访问控制由后端兜底 |
| DEC-020 | 内容系列首版只做公开系列，私有系列延后，降低权限和 UI 分支复杂度 |

## 附录 B：config.yaml 新增配置

```yaml
# 联合创作
collaboration:
  invite_daily_limit: 20           # 每人每天最多发送邀请数
  invite_expire_days: 7            # 邀请未响应过期天数
  max_invitees_per_publish: 5      # 单次发布 UI 最多选择邀请人数
  max_contributors_per_item: 10    # 单内容 contributor 上限

# 浏览历史
browse_history:
  retention_days: 7                # 保留天数
  cleanup_time: "03:00"            # 清理定时任务执行时刻（每天凌晨 3 点，HH:MM 格式）
```

> **config.go 结构体改动说明**：以上新增配置需在 `backend/config/config.go` 中映射为对应结构体与字段：
> - `CollaborationConfig` 结构体：字段 `InviteDailyLimit int`、`InviteExpireDays int`、`MaxInviteesPerPublish int`、`MaxContributorsPerItem int`，挂载到 `Config.Collaboration`
> - `BrowseHistoryConfig` 结构体：字段 `RetentionDays int`、`CleanupTime string`，挂载到 `Config.BrowseHistory`
> - 字段名遵循 Go 命名规范（驼峰），YAML tag 使用下划线小写；yaml.Unmarshal 自动完成映射
> - 首版不将 `collaboration` 与 `browse_history` 暴露到 Admin 配置页，也不纳入 `SaveOverride` 写出的配置覆盖文件。面向普通前端的 `GET /api/v1/config/public` 只新增 `collaboration.max_invitees_per_publish`；不得暴露日限、过期时长或 contributor cap。后续若确有运营调整需求，再单独设计 Admin 配置暴露范围与审计规则。

## 附录 C：新增数据库迁移文件一览

> **编号说明**：当前 `backend/migrations/` 最大编号为 `056_conversation_indexes.sql`，本设计建议从 `057_` 起连续编号。若实施前已有新的 migration 合入，按当时目录最大编号继续递增；每个迁移文件需在事务内完成 schema 变更并幂等可重放。
> **回滚说明**：每个新增迁移文件都必须包含 `-- ROLLBACK:` 注释块，说明本地测试环境如何撤销该迁移。已进入共享环境或生产环境的迁移不得自动执行破坏性回滚；需要优先评估向前修复或数据保留方案。

| 子系统 | Migration | 说明 |
|--------|-----------|------|
| 私信+通知 | `057_add_broadcast_channel.sql` | 解除并重建 notifications.channel_check 约束，新增 'broadcast' 值 |
| 收藏集 | `058_create_collections.sql` | 创建 collections + collection_items 表 + 旧 favorites 数据迁移 + is_default 默认集约束 |
| 广播幂等跟进 | `062_notification_broadcast_idempotency.sql` | 创建 notification_broadcast_requests；唯一约束 `(actor_id, key_hash)`，只保存 payload 哈希与安全响应摘要，不保存广播正文。若实施时 062 已被占用则顺延并同步引用 |
| 内容系列 | `059_create_content_series.sql` | 创建 content_series + content_series_items 表 |
| 联动增强 | `060_add_source_fanwork_id.sql` | content_items 新增 source_fanwork_id 列 |
| 联合创作 | `061_collaboration_invites.sql` | 创建 collaboration_invites 表 + users.accept_collab_invites + messages.msg_type/metadata + 部分唯一索引（WHERE status IN ('pending','accepted')） |

## 附录 C.1：首版数据量和性能预期

这些数字用于指导首版索引、分页、批处理和浏览器验证规模，不是产品硬上限。若实际运营数据超过预期，应新增性能优化任务而不是在当前计划中临时扩大范围。

| 场景 | 首版预期 | 实现含义 |
|------|----------|----------|
| 单次管理员广播收件人 | ≤ 10,000 活跃用户 | 后端批量写入，批大小 500；管理员页面只展示摘要，不预览收件人列表 |
| 单用户浏览历史保留窗口 | 7 天内 ≤ 2,000 条 | 必须分页；清理任务按配置时间每日运行 |
| 单用户收藏集数量 | ≤ 100 个 | 收藏集列表分页；默认 original/fanwork 收藏集不计入删除范围 |
| 单收藏集条目数量 | ≤ 1,000 条 | 详情页分页；推荐画像读取去重并集时避免全量加载到前端 |
| 单内容系列条目数量 | ≤ 500 条 | 详情页和 Studio 管理页分页或虚拟化可后续优化；首版 reorder API 仍提交完整 `item_ids` |
| 单内容相关/衍生作品展示 | inline 最多 8 张卡片 | 详情页避免深度递归；完整列表页按计划裁剪规则延后 |
| 单用户每日协作邀请 | 默认 20 次 | Redis 日限必须 fail-closed；前端发布后邀请失败不回滚已发布内容 |

## 附录 D：实施范围外（明确不做）

- 方案 B（并行独立流）和方案 C（混合波浪式）
- 管理员以私信形式推送通知（采用系统通知频道）
- 收藏集不分 zone（已确认分区，创建时锁定）
- 内容系列允许非 owner 管理（仅系列 owner 管理）
- UI 中「醒目」的双向链接展示（克制设计原则）
- 来源链无限递归追溯（仅展示一层）

## 附录 E：跨计划共享文件矩阵

| 文件 | 涉及子系统 | 冲突风险 | 处理规则 |
|------|------------|----------|----------|
| 当前唯一 route owner：`backend/internal/handler/routes.go`，hardening Task 3 后为 `backend/internal/router/routes.go` | 1、3、4、5、6 | 多计划注册路由 | 串行编辑；实现前探测当前 owner，不得重建已删除文件；提交前跑路由/handler 测试 |
| `backend/config/config.go` / `backend/config.yaml` | 2、5 | 新增配置段和结构体映射 | 串行编辑；更新附录 B；只允许 collaboration publish-selection cap 进入 PublicConfig，不改 PatchConfig/SaveOverride；提交前跑 config/public-config 测试和 doc-validator |
| `backend/migrations/` | 1、3、4、5、6 | migration 编号冲突 | 每个计划开始时重新查看最大编号；不得复用已存在编号 |
| `frontend/messages/zh.json` / `frontend/messages/en.json` | 1-6 | i18n key 合并冲突 | 按页面/组件命名空间追加 key；禁止硬编码新增 UI 文案 |
| `frontend/components/content/ContentDetail.tsx` | 3、4、6 | 收藏入口、系列导航、来源展示插入点冲突 | 按顺序集成；每次改动后浏览器验证内容详情页 |
| `frontend/components/studio/PublishForm.tsx` | 6、5 | 来源字段和联合创作者选择同时改发布表单 | 严格按 source-linkage -> collaboration-invites 集成；先实现 §6 来源字段，再实现 §5 联合创作者选择，两个计划不得同时编辑该文件 |
| `frontend/app/(protected)/messages/page.tsx` / `frontend/components/social/ChatWindow.tsx` / `frontend/components/social/ConversationList.tsx` | 1、5 | 私信布局、旧 API 纠偏和邀请卡片共用消息模型 | §1 先修 API 和布局，统一改用 `/api/v1/messages` 体系；§5 再扩展 msg_type/metadata 渲染 |
| `design/ui-spec.md` | 1、2、3、4、5、6 | 新页面/组件视觉规范缺失 | 对应实现计划先补相关 `## Page:` / `## Component:` 段落，再编码 |

## 附录 F：已确认的人工决策

以下决策已由用户确认，后续 implementation plan 应直接采用，不再重复询问：

| 决策 | 影响子系统 | 确认结果 |
|------|------------|----------|
| 管理员广播 Markdown 能力 | §1 | 允许 Markdown 图片和链接；渲染必须走安全 Markdown 链路，不允许原始 HTML 绕过消毒 |
| 浏览足迹清理时区 | §2 | 生产服务器时区固定为 Asia/Shanghai；`cleanup_time` 按 Asia/Shanghai 解释 |
| 收藏集公开浏览范围 | §3 | 公开收藏集允许未登录用户浏览 published 内容，详情页继续放在 `(public)` route group |
| 收藏集回滚窗口 | §3 | 旧 `favorites` 表保留 30 天足够；30 天后再由独立清理任务/人工迁移删除 |
| 联合创作邀请过期提醒 | §5 | 不需要即将过期提醒；仅在邀请卡片显示过期状态 |
| 联动来源必填范围 | §6 | 新 fanwork 必须绑定 IP、原创内容或二创内容之一；无来源内容不属于「二创」范畴 |
| 内容系列公开性 | §4 | 首版只做公开系列，私有系列延后，避免额外权限和 UI 分支 |
| 新增配置管理范围 | §2、§5 | `browse_history` 与 `collaboration` 首版仅通过 `backend/config.yaml` 管理，不进入 Admin 配置页、不纳入 SaveOverride |

---

> **下一步**：用户审阅本文档 → 确认无误后 → 转入 writing-plans 生成各子系统的实现计划，按顺序逐一执行。
