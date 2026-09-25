package config

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Ticket #671 census (docs/working/2026-09-25-671-config-census.md): every
// reachable leaf field of Config carries an explicit classification. The
// reflection walk below recomputes the reachable leaf set from the struct
// definition; any leaf missing from the map (a newly added config field that
// was never classified) or any stale map entry (a removed field) fails here,
// forcing the new field through the census/classification workflow.

const (
	classRequired     = "required"      // all-mode non-credential structural requirement
	classOptionalZero = "optional-zero" // zero value legal; consumer default or absence tolerated
	classConditional  = "conditional"   // gated by a feature/enabled switch; zero legal while off
	classRegistry     = "registry"      // dynamic registry element (agent.models entries)
	classDead         = "dead"          // unset in YAML and no consumer anywhere (kept documented)
)

var leafClassification = map[string]string{
	"agent.chat_context_token_budget":                "conditional",
	"agent.chat_max_context_messages":                "conditional",
	"agent.chitchat_patterns":                        "conditional",
	"agent.chitchat_shortcut_enabled":                "conditional",
	"agent.citation_max_count":                       "conditional",
	"agent.conversation_list_limit":                  "conditional",
	"agent.conversation_page_size":                   "conditional",
	"agent.conversational_max_runes":                 "conditional",
	"agent.embedding_api_base":                       "conditional",
	"agent.embedding_api_key":                        "conditional",
	"agent.embedding_dimensions":                     "conditional",
	"agent.embedding_group_id":                       "conditional",
	"agent.embedding_model":                          "conditional",
	"agent.embedding_provider":                       "conditional",
	"agent.guardrails.fence_external_tool_results":   "conditional",
	"agent.guardrails.image_url_allow_hosts":         "conditional",
	"agent.guardrails.session_tool_call_limit":       "conditional",
	"agent.guardrails.session_tool_turn_limit":       "conditional",
	"agent.hmac_secret":                              "conditional",
	"agent.image.api_base":                           "conditional",
	"agent.image.api_key":                            "conditional",
	"agent.image.enabled":                            "conditional",
	"agent.image.max_image_bytes":                    "conditional",
	"agent.image.model":                              "conditional",
	"agent.image.price_per_image_cny":                "conditional",
	"agent.image.provider":                           "conditional",
	"agent.image.session_image_limit":                "conditional",
	"agent.image.size_default":                       "conditional",
	"agent.image.size_options":                       "conditional",
	"agent.image.timeout_sec":                        "conditional",
	"agent.llm_api_base":                             "conditional",
	"agent.llm_api_key":                              "conditional",
	"agent.llm_model":                                "conditional",
	"agent.llm_provider":                             "conditional",
	"agent.max_output_tokens":                        "conditional",
	"agent.max_tool_calls_per_turn":                  "conditional",
	"agent.max_user_message_chars":                   "conditional",
	"agent.mcp.call_timeout_sec":                     "conditional",
	"agent.mcp.enabled":                              "conditional",
	"agent.mcp.external_answer_max_runes":            "conditional",
	"agent.mcp.result_max_bytes":                     "conditional",
	"agent.mcp.servers.[].args":                      "conditional",
	"agent.mcp.servers.[].command":                   "conditional",
	"agent.mcp.servers.[].id":                        "conditional",
	"agent.mcp.servers.[].tools":                     "conditional",
	"agent.models.[].api_base":                       "registry",
	"agent.models.[].api_key":                        "registry",
	"agent.models.[].cost_in_per_m_tokens":           "registry",
	"agent.models.[].cost_out_per_m_tokens":          "registry",
	"agent.models.[].display_name":                   "registry",
	"agent.models.[].id":                             "registry",
	"agent.models.[].model":                          "registry",
	"agent.models.[].provider":                       "registry",
	"agent.provider_max_retries":                     "conditional",
	"agent.provider_timeout_sec":                     "conditional",
	"agent.rate_limit_per_day":                       "conditional",
	"agent.rate_limit_per_minute":                    "conditional",
	"agent.routing.fallbacks":                        "conditional",
	"agent.routing.primary":                          "conditional",
	"agent.routing.retry_on":                         "conditional",
	"agent.upload_assist_max_file_mb":                "conditional",
	"agent.web_agent_enabled":                        "conditional",
	"agent_access.max_tokens_per_user":               "optional-zero",
	"archive_scan.clamd_address":                     "conditional",
	"archive_scan.max_entry_uncompressed_mb":         "conditional",
	"archive_scan.max_recursion_depth":               "conditional",
	"archive_scan.max_total_uncompressed_mb":         "conditional",
	"archive_scan.max_upload_size_mb":                "conditional",
	"archive_scan.max_zip_entries":                   "conditional",
	"archive_scan.retry_backoff_sec":                 "conditional",
	"archive_scan.scan_timeout_sec":                  "conditional",
	"archive_scan.url_ttl_sec":                       "conditional",
	"browse_history.cleanup_time":                    "optional-zero",
	"browse_history.retention_days":                  "optional-zero",
	"cache.content_detail_ttl":                       "optional-zero",
	"cache.content_list_ttl":                         "optional-zero",
	"cache.email_verify_ttl":                         "dead",
	"cache.hot_rank_zset_ttl":                        "dead",
	"cache.ip_detail_ttl":                            "optional-zero",
	"cache.ip_list_ttl":                              "optional-zero",
	"cache.password_reset_ttl":                       "dead",
	"cache.publish_freeze_ttl":                       "optional-zero",
	"cache.tag_cache_ttl":                            "optional-zero",
	"cache.user_status_ttl":                          "optional-zero",
	"cache.view_count_flush_interval":                "optional-zero",
	"captcha.access_key_id":                          "conditional",
	"captcha.access_key_secret":                      "conditional",
	"captcha.prefix":                                 "conditional",
	"captcha.provider":                               "conditional",
	"captcha.region":                                 "conditional",
	"captcha.scene_id":                               "conditional",
	"captcha.ticket_ttl_sec":                         "conditional",
	"client.download_enabled":                        "optional-zero",
	"client.download_url":                            "optional-zero",
	"client.latest_version":                          "optional-zero",
	"collaboration.invite_daily_limit":               "optional-zero",
	"collaboration.invite_expire_days":               "optional-zero",
	"collaboration.max_contributors_per_item":        "optional-zero",
	"collaboration.max_invitees_per_publish":         "optional-zero",
	"database.dsn":                                   "required",
	"database.read_dsn":                              "optional-zero",
	"discussion.hot_decay_hours":                     "optional-zero",
	"features.archive_malware_scan_enabled":          "optional-zero",
	"features.creator_support_enabled":               "optional-zero",
	"features.desktop_deploy_enabled":                "optional-zero",
	"features.payment_enabled":                       "optional-zero",
	"features.rag_hybrid_enabled":                    "optional-zero",
	"features.rag_query_expansion_enabled":           "optional-zero",
	"features.rag_rerank_enabled":                    "optional-zero",
	"feedback.upload_grant_ttl_sec":                  "optional-zero",
	"green.access_key_id":                            "conditional",
	"green.access_key_secret":                        "conditional",
	"green.callback_url":                             "conditional",
	"green.region":                                   "conditional",
	"green.seed":                                     "conditional",
	"green.uid":                                      "conditional",
	"ip_categories":                                  "optional-zero",
	"ip_proposal.deadline_days":                      "optional-zero",
	"ip_proposal.min_votes":                          "optional-zero",
	"ip_proposal.pass_threshold":                     "optional-zero",
	"judge.error_rate_revoke":                        "optional-zero",
	"judge.error_rate_window":                        "optional-zero",
	"judge.exam_pass_rate":                           "optional-zero",
	"judge.min_votes_required":                       "optional-zero",
	"judge.pass_threshold":                           "optional-zero",
	"jwt.access_token_ttl":                           "optional-zero",
	"jwt.refresh_token_ttl":                          "optional-zero",
	"jwt.secret":                                     "required",
	"legal.current_privacy_version":                  "optional-zero",
	"legal.current_terms_version":                    "optional-zero",
	"limits.dm_max_length":                           "optional-zero",
	"limits.image_max_mb":                            "optional-zero",
	"limits.mod_max_mb":                              "optional-zero",
	"limits.sheet_music_max_mb":                      "optional-zero",
	"limits.text_max_mb":                             "optional-zero",
	"limits.video_max_mb":                            "optional-zero",
	"limits.video_max_sec":                           "optional-zero",
	"observability.agent_trace.channel_size":         "conditional",
	"observability.agent_trace.digest_max_runes":     "conditional",
	"observability.agent_trace.enabled":              "conditional",
	"observability.agent_trace.flush_batch_size":     "conditional",
	"observability.agent_trace.flush_interval_ms":    "conditional",
	"observability.agent_trace.keep_full_prompt":     "conditional",
	"observability.agent_trace.retention_days":       "conditional",
	"observability.agent_trace.sample_ratio":         "conditional",
	"observability.ip_key_rotation.active_from":      "conditional",
	"observability.ip_key_rotation.active_until":     "conditional",
	"observability.ip_key_rotation.previous_key_id":  "conditional",
	"observability.ip_key_rotation.previous_secret":  "conditional",
	"observability.log_ip_hash_secret":               "optional-zero",
	"observability.log_ip_key_id":                    "optional-zero",
	"observability.log_level":                        "optional-zero",
	"observability.metrics_port":                     "required",
	"observability.read_header_timeout_sec":          "optional-zero",
	"observability.readiness.db_timeout_sec":         "optional-zero",
	"observability.readiness.redis_timeout_sec":      "optional-zero",
	"observability.tracing.backend":                  "conditional",
	"observability.tracing.enabled":                  "conditional",
	"observability.tracing.endpoint":                 "conditional",
	"observability.tracing.sample_ratio":             "conditional",
	"observability.tracing.service_name":             "conditional",
	"oss.access_key_id":                              "conditional",
	"oss.access_key_secret":                          "conditional",
	"oss.bucket_name":                                "conditional",
	"oss.display_url_ttl_sec":                        "conditional",
	"oss.domain":                                     "conditional",
	"oss.download_url_ttl_sec":                       "conditional",
	"oss.endpoint":                                   "conditional",
	"publish.freeze_on_violation":                    "dead",
	"publish.max_daily_posts":                        "dead",
	"publish.require_review":                         "dead",
	"publish.type_order_fanwork":                     "optional-zero",
	"publish.type_order_original":                    "optional-zero",
	"queue.dlq_ttl_hours":                            "conditional",
	"queue.enabled":                                  "conditional",
	"queue.max_attempts":                             "conditional",
	"queue.maxlen":                                   "conditional",
	"queue.retry_backoff_sec":                        "conditional",
	"queue.worker_count":                             "conditional",
	"queue.worker_embedding":                         "conditional",
	"queue.worker_notification":                      "conditional",
	"queue.worker_review":                            "conditional",
	"rag.chunking.max_tokens":                        "conditional",
	"rag.chunking.overlap_tokens":                    "conditional",
	"rag.chunking.tokenizer_encoding":                "conditional",
	"rag.chunking.version":                           "conditional",
	"rag.contextual.api_base":                        "conditional",
	"rag.contextual.api_key":                         "conditional",
	"rag.contextual.concurrency":                     "conditional",
	"rag.contextual.doc_context_chars":               "conditional",
	"rag.contextual.enabled":                         "conditional",
	"rag.contextual.max_prefix_tokens":               "conditional",
	"rag.contextual.max_retries":                     "conditional",
	"rag.contextual.model":                           "conditional",
	"rag.contextual.provider":                        "conditional",
	"rag.contextual.request_interval_ms":             "conditional",
	"rag.contextual.timeout_sec":                     "conditional",
	"rag.hybrid.bm25_topk":                           "conditional",
	"rag.hybrid.final_topk":                          "conditional",
	"rag.hybrid.keyword_source":                      "conditional",
	"rag.hybrid.rrf_k":                               "conditional",
	"rag.hybrid.vector_topk":                         "conditional",
	"rag.index.audit_timeout_sec":                    "conditional",
	"rag.index.embedding_model":                      "conditional",
	"rag.index.error_body_max_bytes":                 "conditional",
	"rag.index.generation_start":                     "conditional",
	"rag.index.health_poll_interval_sec":             "conditional",
	"rag.index.lock_cleanup_timeout_sec":             "conditional",
	"rag.index.response_body_max_bytes":              "conditional",
	"rag.index.timeout_sec":                          "conditional",
	"rag.index.url":                                  "conditional",
	"rag.refusal.min_surviving_citations":            "conditional",
	"rag.refusal.min_top_relevance_score":            "conditional",
	"rag.rerank.api_base":                            "conditional",
	"rag.rerank.api_key":                             "conditional",
	"rag.rerank.fallback_api_base":                   "conditional",
	"rag.rerank.fallback_api_key":                    "conditional",
	"rag.rerank.fallback_model":                      "conditional",
	"rag.rerank.fallback_provider":                   "conditional",
	"rag.rerank.input_topk":                          "conditional",
	"rag.rerank.model":                               "conditional",
	"rag.rerank.provider":                            "conditional",
	"rag.rerank.timeout_sec":                         "conditional",
	"rate_limit.agent_minute_window_sec":             "optional-zero",
	"rate_limit.agent_window_sec":                    "optional-zero",
	"rate_limit.ai_callback_per_minute":              "optional-zero",
	"rate_limit.credential_per_minute":               "optional-zero",
	"rate_limit.enabled":                             "optional-zero",
	"rate_limit.max_json_body_bytes":                 "optional-zero",
	"rate_limit.max_query_chars":                     "optional-zero",
	"rate_limit.max_search_limit":                    "optional-zero",
	"rate_limit.max_search_page":                     "optional-zero",
	"rate_limit.mcp_per_minute":                      "optional-zero",
	"rate_limit.normal_per_minute":                   "optional-zero",
	"rate_limit.normal_window_sec":                   "optional-zero",
	"rate_limit.pat_per_minute":                      "optional-zero",
	"rate_limit.pat_window_sec":                      "optional-zero",
	"rate_limit.search_per_minute":                   "optional-zero",
	"rate_limit.upload_per_hour":                     "optional-zero",
	"rate_limit.upload_window_sec":                   "optional-zero",
	"recommendation.embedding_multiplier":            "optional-zero",
	"recommendation.embedding_topk":                  "optional-zero",
	"recommendation.enabled":                         "optional-zero",
	"recommendation.hot_decay_hours":                 "optional-zero",
	"recommendation.min_interaction_for_personalize": "optional-zero",
	"recommendation.personalization_weight":          "optional-zero",
	"recommendation.rank_interval_min":               "optional-zero",
	"recommendation.refresh_interval_h":              "optional-zero",
	"recommendation.trending_window_days":            "optional-zero",
	"redis.addr":                                     "required",
	"redis.db":                                       "optional-zero",
	"redis.password":                                 "optional-zero",
	"relay.batch_size":                               "required",
	"relay.poll_interval_sec":                        "required",
	"reputation.min_score_for_interaction":           "optional-zero",
	"reputation.quality_comment_threshold":           "optional-zero",
	"reputation.quality_content_threshold":           "optional-zero",
	"reputation.repeat_violation_extra_penalty":      "optional-zero",
	"reputation.repeat_violation_threshold":          "optional-zero",
	"reputation.repeat_violation_window_days":        "optional-zero",
	"reputation.score_judge_accuracy":                "optional-zero",
	"reputation.score_judge_error":                   "optional-zero",
	"reputation.score_malicious_comment":             "optional-zero",
	"reputation.score_malicious_content":             "optional-zero",
	"reputation.score_malicious_pr":                  "optional-zero",
	"reputation.score_malicious_report":              "optional-zero",
	"reputation.score_malicious_tag_report":          "optional-zero",
	"reputation.score_pr_merged":                     "optional-zero",
	"reputation.score_quality_comment":               "optional-zero",
	"reputation.score_quality_content":               "optional-zero",
	"reputation.score_rehab_course":                  "optional-zero",
	"reputation.score_valid_report":                  "optional-zero",
	"resilience.aux_cache.expander.enabled":          "conditional",
	"resilience.aux_cache.expander.ttl_sec":          "conditional",
	"resilience.aux_cache.title.enabled":             "conditional",
	"resilience.aux_cache.title.ttl_sec":             "conditional",
	"resilience.breaker.failure_threshold":           "optional-zero",
	"resilience.breaker.open_timeout_sec":            "optional-zero",
	"resilience.llm_concurrency.enabled":             "conditional",
	"resilience.llm_concurrency.max_per_provider":    "conditional",
	"security.allowed_origins":                       "required",
	"security.trusted_proxies":                       "optional-zero",
	"server.idle_timeout":                            "required",
	"server.mode":                                    "required",
	"server.port":                                    "required",
	"server.read_timeout":                            "required",
	"server.shutdown_timeout":                        "optional-zero",
	"server.write_timeout":                           "required",
	"smtp.from_address":                              "conditional",
	"smtp.host":                                      "conditional",
	"smtp.mode":                                      "conditional",
	"smtp.password":                                  "conditional",
	"smtp.port":                                      "conditional",
	"smtp.user":                                      "conditional",
	"social.comment_fold_threshold":                  "optional-zero",
	"social.report_auto_hide_rate":                   "optional-zero",
	"upload.image_gallery_max_items":                 "optional-zero",
	"upload.image_gallery_min_items":                 "optional-zero",
	"upload.sheet_music_extensions":                  "optional-zero",
	"upload.video_gallery_max_items":                 "optional-zero",
	"upload.video_gallery_min_items":                 "optional-zero",
	"verification.email_ttl_sec":                     "optional-zero",
	"verification.login_captcha_threshold":           "optional-zero",
	"verification.password_min_length":               "optional-zero",
	"verification.register_pending_ttl_sec":          "optional-zero",
	"verification.resend_cooldown_sec":               "optional-zero",
	"verification.reset_ttl_sec":                     "optional-zero",
	"web.public_base_url":                            "required",
	"worker.concurrency":                             "optional-zero",
	"worker.enabled":                                 "optional-zero",
	"worker.service_name":                            "optional-zero",
}

// walkLeafPaths mirrors the census walk: struct fields descend, slice-of-struct
// and map-of-struct elements are expanded with []/{} markers, scalar slices
// and scalars are leaves.
func walkLeafPaths(t reflect.Type, path string, out map[string]bool) {
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		tag := sf.Tag.Get("mapstructure")
		if tag == "-" {
			continue
		}
		name := tag
		if name == "" {
			name = strings.ToLower(sf.Name)
		}
		child := name
		if path != "" {
			child = path + "." + name
		}
		ft := sf.Type
		switch ft.Kind() {
		case reflect.Struct:
			if ft == reflect.TypeOf(time.Duration(0)) {
				out[child] = true
				continue
			}
			walkLeafPaths(ft, child, out)
		case reflect.Slice:
			if ft.Elem().Kind() == reflect.Struct {
				walkLeafPaths(ft.Elem(), child+".[]", out)
			} else {
				out[child] = true
			}
		case reflect.Map:
			// Map containers are dynamic collections (census rule): struct
			// elements expand with the {} marker; a map-of-scalars stays an
			// exempt dynamic container (its keys are operator-chosen, not
			// part of the Config contract).
			if ft.Elem().Kind() == reflect.Struct {
				walkLeafPaths(ft.Elem(), child+".{}", out)
			}
		default:
			out[child] = true
		}
	}
}

func TestEveryReachableLeafIsClassified(t *testing.T) {
	actual := map[string]bool{}
	walkLeafPaths(reflect.TypeOf(Config{}), "", actual)
	require.NotEmpty(t, actual)

	var unclassified []string
	for p := range actual {
		if _, ok := leafClassification[p]; !ok {
			unclassified = append(unclassified, p)
		}
	}
	sort.Strings(unclassified)
	require.Empty(t, unclassified,
		"new config leaf(s) without census classification; classify them in the census doc + this table (ticket #671 workflow)")

	var stale []string
	for p := range leafClassification {
		if !actual[p] {
			stale = append(stale, p)
		}
	}
	sort.Strings(stale)
	require.Empty(t, stale, "classification table references removed field(s)")
}

func TestLeafClassificationOnlyUsesKnownClasses(t *testing.T) {
	allowed := map[string]bool{classRequired: true, classOptionalZero: true, classConditional: true, classRegistry: true, classDead: true}
	for path, class := range leafClassification {
		require.True(t, allowed[class], "path %s has unknown class %q", path, class)
	}
}

// The census ledger (required 13 / conditional 162 / optional-zero 122 /
// registry 8 / dead 6 = 311 leaves) is asserted so the doc and the table
// cannot drift apart silently.
func TestLeafClassificationMatchesCensusLedger(t *testing.T) {
	counts := map[string]int{}
	for _, class := range leafClassification {
		counts[class]++
	}
	require.Equal(t, 13, counts[classRequired])
	require.Equal(t, 162, counts[classConditional])
	require.Equal(t, 122, counts[classOptionalZero])
	require.Equal(t, 8, counts[classRegistry])
	require.Equal(t, 6, counts[classDead])
}
