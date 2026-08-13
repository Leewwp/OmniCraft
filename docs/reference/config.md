# 配置化管理（config.yaml）

> 本文档由 2026-07-23 文档瘦身从 `architecture.md` §7 抽取，章节号保持原编号以便深链兼容。
> 同步快照；运行时真源：backend/config.yaml 与 backend/internal/config。

## 7. 配置化开关与参数（config.yaml）

所有可动态调整的参数集中在 `config.yaml`，通过管理员 API 热更新。
字段名与 `config/config.go` 中结构体的 `mapstructure` tag 一一对应。

<!-- AUTO-GENERATED: §7 配置字段注册表 | source: backend/config/config.go | DO NOT EDIT MANUALLY -->

| 配置路径 | 类型 | 说明 |
|----------|------|------|
| `agent.chat_max_context_messages` | `int` | ChatMaxContextMsgs |
| `agent.citation_max_count` | `int` | CitationMaxCount |
| `agent.conversation_list_limit` | `int` | ConversationListLimit |
| `agent.conversation_page_size` | `int` | ConversationPageSize |
| `agent.embedding_dimensions` | `int` | EmbeddingDimensions |
| `agent.embedding_model` | `string` | EmbeddingModel |
| `agent.hmac_secret` | `string` | HMACSecret |
| `agent.llm_api_base` | `string` | LLMAPIBase |
| `agent.llm_api_key` | `string` | LLMAPIKey |
| `agent.llm_model` | `string` | LLMModel |
| `agent.llm_provider` | `string` | LLMProvider |
| `agent.max_output_tokens` | `int` | MaxOutputTokens |
| `agent.max_tool_calls_per_turn` | `int` | MaxToolCallsPerTurn |
| `agent.max_user_message_chars` | `int` | MaxUserMessageChars |
| `agent.provider_max_retries` | `int` | ProviderMaxRetries |
| `agent.provider_timeout_sec` | `int` | ProviderTimeoutSec |
| `agent.rate_limit_per_day` | `int` | RateLimitPerDay |
| `agent.rate_limit_per_minute` | `int` | RateLimitPerMinute |
| `agent.upload_assist_max_file_mb` | `int` | UploadAssistMaxFileMB |
| `agent.web_agent_enabled` | `bool` | WebAgentEnabled |
| `browse_history.cleanup_time` | `string` | CleanupTime |
| `browse_history.retention_days` | `int` | RetentionDays |
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
| `collaboration.invite_daily_limit` | `int` | InviteDailyLimit |
| `collaboration.invite_expire_days` | `int` | InviteExpireDays |
| `collaboration.max_contributors_per_item` | `int` | MaxContributorsPerItem |
| `collaboration.max_invitees_per_publish` | `int` | MaxInviteesPerPublish |
| `database.dsn` | `string` | DSN |
| `database.read_dsn` | `string` | ReadDSN |
| `features.creator_support_enabled` | `bool` | CreatorSupportEnabled |
| `features.desktop_deploy_enabled` | `bool` | DesktopDeployEnabled |
| `features.payment_enabled` | `bool` | PaymentEnabled |
| `feedback.upload_grant_ttl_sec` | `int` | UploadGrantTTLSec |
| `green.access_key_id` | `string` | AccessKeyID |
| `green.access_key_secret` | `string` | AccessKeySecret |
| `green.callback_url` | `string` | CallbackURL |
| `green.region` | `string` | Region |
| `green.seed` | `string` | Seed is the callback signature seed (green.seed): release-required, [A-Za-z0-9_], max 64 chars. |
| `green.uid` | `string` | UID is the Aliyun main account UID (green.uid): release-required, digits only (console account info, not RAM UID). |
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
| `observability.ip_key_rotation.active_from` | `string` | ActiveFrom |
| `observability.ip_key_rotation.active_until` | `string` | ActiveUntil |
| `observability.ip_key_rotation.previous_key_id` | `string` | PreviousKeyID |
| `observability.ip_key_rotation.previous_secret` | `string` | PreviousSecret |
| `observability.log_ip_hash_secret` | `string` | LogIPHashSecret |
| `observability.log_ip_key_id` | `string` | LogIPKeyID |
| `observability.log_level` | `string` | LogLevel |
| `observability.metrics_port` | `string` | MetricsPort |
| `observability.read_header_timeout_sec` | `int` | ReadHeaderTimeoutSec |
| `observability.readiness.db_timeout_sec` | `int` | DBTimeoutSec |
| `observability.readiness.redis_timeout_sec` | `int` | RedisTimeoutSec |
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
| `rate_limit.agent_minute_window_sec` | `int` | AgentMinuteWindowSec |
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
| `server.idle_timeout` | `int` | IdleTimeout |
| `server.mode` | `string` | Mode |
| `server.port` | `string` | Port |
| `server.read_timeout` | `int` | ReadTimeout |
| `server.shutdown_timeout` | `int` | ShutdownTimeout |
| `server.write_timeout` | `int` | WriteTimeout |
| `smtp.from_address` | `string` | FromAddress |
| `smtp.host` | `string` | Host |
| `smtp.mode` | `string` | Mode |
| `smtp.password` | `string` | Password |
| `smtp.port` | `int` | Port |
| `smtp.user` | `string` | User |
| `social.comment_fold_threshold` | `float64` | CommentFoldThreshold |
| `social.report_auto_hide_rate` | `float64` | ReportAutoHideRate |
| `upload.image_gallery_max_items` | `int` | ImageGalleryMaxItems |
| `upload.image_gallery_min_items` | `int` | Media set (media gallery) size bounds for newly published image/video
content. Zero means "use the specification defa... |
| `upload.sheet_music_extensions` | `[]string` | SheetMusicExtensions |
| `upload.video_gallery_max_items` | `int` | VideoGalleryMaxItems |
| `upload.video_gallery_min_items` | `int` | VideoGalleryMinItems |
| `verification.email_ttl_sec` | `int` | EmailTTLSec |
| `verification.login_captcha_threshold` | `int` | LoginCaptchaThreshold |
| `verification.password_min_length` | `int` | PasswordMinLength |
| `verification.register_pending_ttl_sec` | `int` | RegisterPendingTTLSec |
| `verification.resend_cooldown_sec` | `int` | ResendCooldownSec |
| `verification.reset_ttl_sec` | `int` | ResetTTLSec |
| `web.public_base_url` | `string` | PublicBaseURL |

<!-- END AUTO-GENERATED: §7 -->

---
