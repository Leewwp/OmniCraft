# 推荐页面（原创区推荐页）

> 本文档由 2026-07-23 文档瘦身从 `architecture.md` §12 抽取，章节号保持原编号以便深链兼容。

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
