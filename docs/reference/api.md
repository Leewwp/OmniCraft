# API 接口详细说明

> 本文档由 2026-07-23 文档瘦身从 `architecture.md` §5 抽取，章节号保持原编号以便深链兼容。
> 同步快照；运行时真源：backend 路由注册与 handler 实现。

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
  "source_original_id": 123,
  "source_fanwork_id": null,
  "source_original": { "id": 123, "title": "源原创标题", "zone": "original" },
  "source_fanwork": null,
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

二创来源使用三选一模型：`ip_id`、`source_original_id`、`source_fanwork_id` 至少提供一个；`source_original_id` 与 `source_fanwork_id` 互斥。原创内容携带任一来源字段返回 `SOURCE_NOT_ALLOWED_FOR_ORIGINAL`；二创不带任何来源返回 `FANWORK_SOURCE_REQUIRED`；同时携带两个内容来源返回 `MULTIPLE_SOURCE_CONFLICT`；来源原创必须满足 `zone='original' AND status='published'`，来源二创必须满足 `zone='fanwork' AND status='published'`。

```json
// Request
{
  "title": "基于某原创世界观的二创短篇",
  "description": "Markdown 正文或简介",
  "zone": "fanwork",
  "ip_id": 5,
  "source_original_id": 123,
  "source_fanwork_id": null,
  "content_type": "article",
  "tags": ["短篇"]
}
```

#### 相关二创（GET /api/v1/contents/:id/related-fanworks）

查询某个原创内容下已发布的二创，或某个二创内容下已发布的衍生二创。目标内容为原创时查询 `source_original_id = :id`；目标内容为二创时查询 `source_fanwork_id = :id`。支持 `page`、`page_size`、`sort`、`content_type` 筛选；`content_type` 可支持逗号分隔多值，例如 `article,prompt` 或 `audio,sheet_music`。

```json
// Response 200
{
  "source_content_id": 123,
  "source_zone": "original",
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
