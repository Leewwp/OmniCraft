# 数据库 Schema（PostgreSQL DDL）

> 本文档由 2026-07-23 文档瘦身从 `architecture.md` §4 抽取，章节号保持原编号以便深链兼容。
> 同步快照；运行时真源：backend/migrations/*.sql。

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
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `viewed_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | viewed_at |
| — | — | UNIQUE (`user_id`, `content_item_id`) | table constraint |

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

### collection_items

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `collection_id` | `BIGINT` | NOT NULL -> collections.id | collection_id |
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `note` | `TEXT` | NOT NULL DEFAULT '' | note |
| `added_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | added_at |
| — | — | UNIQUE (`collection_id`, `content_item_id`) | table constraint |

### collections

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `title` | `VARCHAR(200)` | NOT NULL | title |
| `description` | `TEXT` | NOT NULL DEFAULT '' | description |
| `zone` | `VARCHAR(10)` | NOT NULL | zone |
| `is_default` | `BOOLEAN` | NOT NULL DEFAULT FALSE | is_default |
| `is_public` | `BOOLEAN` | NOT NULL DEFAULT FALSE | is_public |
| `sort_order` | `INT` | NOT NULL DEFAULT 0 | sort_order |
| `deleted_at` | `TIMESTAMPTZ` | - | deleted_at |
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
| `target_type` | `VARCHAR(20)` | - | target_type |
| `target_id` | `BIGINT` | - | target_id |
| `content` | `TEXT` | - | content |
| `updated_at` | `TIMESTAMPTZ` | DEFAULT NOW() | updated_at |

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
| `sort_order` | `INT` | - | sort_order |

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
| `description` | `TEXT` | - | description |
| `source_original_id` | `BIGINT` | -> content_items.id | source_original_id |
| `ban_reason` | `TEXT` | - | ban_reason |
| `download_count` | `INTEGER` | NOT NULL DEFAULT 0 | download_count |
| `search_vector` | `TSVECTOR` | - | search_vector |
| `hot_score` | `DOUBLE PRECISION` | DEFAULT 0 | hot_score |
| `rating_score` | `DOUBLE PRECISION` | DEFAULT 0 | rating_score |
| `deleted_at` | `TIMESTAMPTZ` | - | deleted_at |
| `cover_width` | `INT` | - | cover_width |
| `cover_height` | `INT` | - | cover_height |

### content_series

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `title` | `VARCHAR(200)` | NOT NULL | title |
| `description` | `TEXT` | NOT NULL DEFAULT '' | description |
| `cover_content_id` | `BIGINT` | -> content_items.id | cover_content_id |
| `owner_id` | `BIGINT` | NOT NULL -> users.id | owner_id |
| `zone` | `VARCHAR(10)` | NOT NULL | zone |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

### content_series_items

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `series_id` | `BIGINT` | NOT NULL -> content_series.id | series_id |
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `sort_order` | `INT` | NOT NULL DEFAULT 0 | sort_order |
| `added_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | added_at |
| — | — | UNIQUE (`series_id`, `content_item_id`) | table constraint |

### content_tags

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `tag` | `VARCHAR(50)` | NOT NULL | tag |

### content_versions

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `parent_version_id` | `BIGINT` | -> content_versions.id | parent_version_id |
| `author_id` | `BIGINT` | NOT NULL -> users.id | author_id |
| `version_number` | `INT` | NOT NULL | version_number |
| `storage_type` | `VARCHAR(10)` | NOT NULL | storage_type |
| `storage_key` | `TEXT` | - | storage_key |
| `diff_summary` | `TEXT` | - | diff_summary |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'active' | status |
| `is_latest` | `BOOLEAN` | NOT NULL DEFAULT FALSE | is_latest |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| — | — | UNIQUE (`content_item_id`, `version_number`) | table constraint |

### conversation_participants

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `conversation_id` | `BIGINT` | NOT NULL -> conversations.id | conversation_id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `last_read_at` | `TIMESTAMPTZ` | - | last_read_at |
| `unread_count` | `INTEGER` | NOT NULL DEFAULT 0 | unread_count |
| `left_at` | `TIMESTAMPTZ` | - | left_at |

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
| `is_pinned` | `BOOLEAN` | NOT NULL DEFAULT FALSE | is_pinned |
| `last_active_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | last_active_at |

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
| `follower_id` | `BIGINT` | NOT NULL -> users.id | follower_id |
| `target_type` | `VARCHAR(20)` | NOT NULL | target_type |
| `target_id` | `BIGINT` | NOT NULL | target_id |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| — | — | UNIQUE (`follower_id`, `target_type`, `target_id`) | table constraint |

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
| `search_vector` | `TSVECTOR` | - | search_vector |
| `popularity_score` | `DOUBLE PRECISION` | DEFAULT 0 | popularity_score |

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
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `content_type` | `VARCHAR(50)` | NOT NULL | content_type |
| `qualified_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | qualified_at |
| `revoked_at` | `TIMESTAMPTZ` | - | revoked_at |
| `is_active` | `BOOLEAN` | NOT NULL DEFAULT TRUE | is_active |
| — | — | UNIQUE (`user_id`, `content_type`) | table constraint |

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
| `reason_owner_vote_id` | `BIGINT` | NOT NULL -> judge_votes.id | reason_owner_vote_id |
| `voter_id` | `BIGINT` | NOT NULL -> users.id | voter_id |
| `vote_type` | `VARCHAR(10)` | NOT NULL | vote_type |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| — | — | UNIQUE (`reason_owner_vote_id`, `voter_id`) | table constraint |

### judge_votes

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `case_id` | `BIGINT` | NOT NULL -> judge_cases.id | case_id |
| `judge_id` | `BIGINT` | NOT NULL -> users.id | judge_id |
| `vote` | `VARCHAR(10)` | NOT NULL | vote |
| `reason` | `TEXT` | - | reason |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| — | — | UNIQUE (`case_id`, `judge_id`) | table constraint |

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

### notification_broadcast_requests

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `actor_id` | `BIGINT` | NOT NULL -> users.id | actor_id |
| `key_hash` | `VARCHAR(64)` | NOT NULL | key_hash |
| `payload_hash` | `VARCHAR(64)` | NOT NULL | payload_hash |
| `recipient_count` | `INT` | NOT NULL | recipient_count |
| `broadcast_at` | `TIMESTAMPTZ` | NOT NULL | broadcast_at |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | updated_at |

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
| `provider` | `VARCHAR(20)` | NOT NULL | provider |
| `provider_uid` | `VARCHAR(255)` | NOT NULL | provider_uid |
| `access_token` | `TEXT` | - | access_token |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| — | — | UNIQUE (`provider`, `provider_uid`) | table constraint |

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
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `target_type` | `VARCHAR(20)` | NOT NULL | target_type |
| `target_id` | `BIGINT` | NOT NULL | target_id |
| `reaction` | `VARCHAR(10)` | NOT NULL | reaction |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| — | — | UNIQUE (`user_id`, `target_type`, `target_id`) | table constraint |

### rehab_completions

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `id` | `BIGSERIAL` | PK | id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `course_id` | `BIGINT` | NOT NULL -> rehab_courses.id | course_id |
| `completed_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | completed_at |
| `started_at` | `TIMESTAMPTZ` | - | started_at |
| — | — | UNIQUE (`user_id`, `course_id`) | table constraint |

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
| `action_taken` | `TEXT` | - | action_taken |

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
| `content_item_id` | `BIGINT` | NOT NULL -> content_items.id | content_item_id |
| `user_id` | `BIGINT` | NOT NULL -> users.id | user_id |
| `tag` | `VARCHAR(100)` | NOT NULL | tag |
| `action` | `VARCHAR(10)` | NOT NULL | action |
| `status` | `VARCHAR(20)` | NOT NULL DEFAULT 'pending' | status |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT NOW() | created_at |
| — | — | UNIQUE (`content_item_id`, `user_id`, `tag`, `action`) | table constraint |

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
| `email_verified_at` | `TIMESTAMPTZ` | - | email_verified_at |
| `accepted_terms_version` | `VARCHAR(32)` | - | accepted_terms_version |
| `accepted_terms_at` | `TIMESTAMPTZ` | - | accepted_terms_at |
| `accepted_privacy_version` | `VARCHAR(32)` | - | accepted_privacy_version |
| `accepted_privacy_at` | `TIMESTAMPTZ` | - | accepted_privacy_at |
| `deleted_at` | `TIMESTAMPTZ` | - | deleted_at |


<!-- END AUTO-GENERATED: §4 -->

---
