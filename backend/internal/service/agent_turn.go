package service

// turnRunner orchestrates the answer turn's tool loop (#661): it owns the
// per-turn streaming state (answer/thinking buffers, executed tool steps,
// citation candidate pool, retrieval sources, signed-image prefixes,
// speculative follow-up) and drives the LLM round loop through the
// ToolRuntime seam. ChatStream shrinks to the composition shell — context
// resolution → chitchat shortcut → runner.run → finalizeTurn — and every
// terminal quartet lands in agent_turn_finalize.go. Deletion test for the
// split: fold this file back and the complexity returns to one function;
// the split is real decomposition, not relocation.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"omnicraft/backend/internal/agentmcp"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability/agenttrace"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/recovery"
)

// turnRunner is per-turn state; one instance per ChatStream call.
type turnRunner struct {
	svc     *AgentService
	userID  int64
	turn    ChatTurnInput
	conv    *model.AgentConversation
	traceID string
	policy  AgentToolPolicy
	// answerLang picks the image-removal placeholder language (chitchat
	// containsCJK convention, 低-1).
	answerLang string
	// handler is the SSE event callback.
	handler func(AgentStreamEvent) error

	answerBuf     strings.Builder
	thinkingBuf   strings.Builder
	executedTools []AgentToolExecution
	// SP-23 M4: conversation-level budget baseline (persisted #538 rows).
	persistedToolCalls int
	persistedToolTurns int
	// ownImagePrefixes: this conversation's own signed-image URL prefixes
	// (piece-2 allowlist is dynamic: only hosts we ourselves issued survive).
	ownImagePrefixes   []string
	citationCandidates []AgentCitation
	seenCitationKeys   map[string]bool
	retrievalSources   map[string]string
	degraded           bool
	streamErr          error
	// skipFinalize marks exits that pre-#661 returned without the terminal
	// quartet (SSE handler failure = client disconnect; internal marshal
	// abort): the flag lets the caller reproduce that exit exactly; the
	// finalizeTurn convergence takes over from commit ④.
	skipFinalize bool
	// SP-21 T2: user-perceived TTFT — first forwarded display delta.
	firstDisplayDelta time.Time
	lastUsage         *llm.TokenUsage
	// SP-15 B speculative follow-up (#435): started at the first answer
	// delta of a tool-executing turn, joined before done assembly.
	followUpCh        chan []string
	followUpStarted   bool
	followUpStartedAt time.Time
	roundIndex        int
	recorder          *agenttrace.TurnRecorder
}

// newTurnRunner assembles the per-turn orchestrator. No IO happens here; the
// budget baseline loads at the head of run.
func (s *AgentService) newTurnRunner(userID int64, turn ChatTurnInput, conv *model.AgentConversation, traceID string, recorder *agenttrace.TurnRecorder, handler func(AgentStreamEvent) error) *turnRunner {
	answerLang := "en"
	if containsCJK(turn.Message) {
		answerLang = "zh"
	}
	return &turnRunner{
		svc:                s,
		userID:             userID,
		turn:               turn,
		conv:               conv,
		traceID:            traceID,
		policy:             s.ToolPolicy(),
		answerLang:         answerLang,
		handler:            handler,
		ownImagePrefixes:   []string{},
		citationCandidates: make([]AgentCitation, 0, s.ToolPolicy().CitationMaxCount),
		seenCitationKeys:   make(map[string]bool, s.ToolPolicy().CitationMaxCount),
		retrievalSources:   make(map[string]string),
		followUpCh:         make(chan []string, 1),
		recorder:           recorder,
	}
}

// run drives the LLM/tool rounds until the model stops calling tools, a
// policy limit trips, or the stream fails. Terminal assembly is NOT done
// here — the caller reads the runner state and finalizes.
func (r *turnRunner) run(ctx context.Context, req *llm.ChatRequest) {
	// SP-25 中-2：预算基线加载失败时保守拦截（视为已达预算）+ Warn——
	// 护栏在 DB 故障窗口静默放行等于成本护栏失效。
	persistedToolCalls, persistedToolTurns, budgetLoadErr := r.svc.conversationBudgetBaseline(ctx, convIDForTools(r.conv))
	if budgetLoadErr != nil {
		slog.Warn("session budget baseline load failed; failing closed for this turn", "conversation_id", convIDForTools(r.conv), "error", budgetLoadErr)
		r.persistedToolCalls, r.persistedToolTurns = budgetExhaustedSentinel, budgetExhaustedSentinel
	} else {
		r.persistedToolCalls, r.persistedToolTurns = persistedToolCalls, persistedToolTurns
	}
	for {
		acc := newStreamedToolCallAccumulator()
		if r.svc.chatStreamer == nil {
			r.streamErr = errors.New("agent streaming provider unavailable")
			break
		}
		r.roundIndex++
		roundSpan := r.recorder.StartNode(agenttrace.NodeTypeLLMRound, fmt.Sprintf("llm_round_%d", r.roundIndex), nil, r.turn.Model)
		roundUsage := (*llm.TokenUsage)(nil)
		roundAnswer := strings.Builder{}
		err := r.svc.chatStreamer.ChatStream(ctx, *req, func(delta llm.ChatDelta) error {
			if len(delta.ToolCalls) > 0 {
				acc.add(delta.ToolCalls)
			}
			if delta.Usage != nil {
				r.lastUsage = delta.Usage
				roundUsage = delta.Usage
			}
			// A-02 real streaming: reasoning and body increments are
			// forwarded as they arrive. Thinking is display-only; it never
			// enters the answer buffer, tool results or citation revalidation.
			if delta.Thinking != "" {
				r.thinkingBuf.WriteString(delta.Thinking)
				if r.firstDisplayDelta.IsZero() {
					r.firstDisplayDelta = time.Now()
				}
				if err := r.handler(AgentStreamEvent{Type: AgentEventThinkDelta, Delta: delta.Thinking}); err != nil {
					return err
				}
			}
			if delta.Content != "" {
				r.answerBuf.WriteString(delta.Content)
				roundAnswer.WriteString(delta.Content)
				if r.firstDisplayDelta.IsZero() {
					r.firstDisplayDelta = time.Now()
				}
				if !r.followUpStarted && len(r.executedTools) > 0 && r.svc.llmProvider != nil {
					r.followUpStarted = true
					r.followUpStartedAt = time.Now()
					question := r.turn.Message
					titles := followUpTitles(r.citationCandidates)
					prefix := delta.Content
					provider := r.svc.llmProvider
					resolver := r.svc.prompts
					sid := r.traceID
					followUpRec := r.recorder
					recovery.GoSafe(func() {
						ctx, cancel := context.WithTimeout(context.Background(), followUpBudget)
						defer cancel()
						r.followUpCh <- generateFollowUps(ctx, followUpRec, resolver, provider, sid, question, titles, prefix)
					})
				}
				if err := r.handler(AgentStreamEvent{Type: AgentEventDelta, Delta: delta.Content}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			r.streamErr = err
			roundSpan.EndWithError(safeAgentStreamCode(err), "")
			break
		}
		roundSpan.End(agenttrace.NodeEndOptions{
			TokensIn:  usagePtr(roundUsage, func(u *llm.TokenUsage) int64 { return int64(u.PromptTokens) }),
			TokensOut: usagePtr(roundUsage, func(u *llm.TokenUsage) int64 { return int64(u.CompletionTokens) }),
			// SP-21 T7: attribute the round to the model that actually served
			// it — the failover target once a routing event fired, else the
			// pinned preference, else the configured primary. Known only now,
			// after the (possibly retried) stream call returned.
			Model: r.svc.servingModel(r.recorder, r.turn.Model),
		})
		roundCalls := acc.calls()
		if len(roundCalls) == 0 {
			break
		}
		if !r.policy.AllowToolCall(len(r.executedTools)) {
			traceAgentEvent(r.traceID, "tool_limit_reached", "executed", len(r.executedTools))
			break
		}

		// OpenAI-compatible protocol order: the assistant message carrying the
		// tool_calls must precede the tool result messages, and every tool
		// message references its call by tool_call_id. Strict providers reject
		// tool messages that appear without the assistant tool_calls message.
		assistantMsg := llm.ChatMessage{Role: "assistant", ToolCalls: roundCalls}
		req.Messages = append(req.Messages, assistantMsg)

		toolMessages := make([]llm.ChatMessage, 0, len(roundCalls))
		for _, tc := range roundCalls {
			toolStartedAt := time.Now()
			// SP-21 T2: one node per tool execution (#538 step shape:
			// server-derived summary only, raw arguments never recorded).
			toolSpan := r.recorder.StartNode(agenttrace.NodeTypeTool,
				fmt.Sprintf("tool_%s_%d", agentToolNodeKey(tc.Function.Name), len(r.executedTools)+1), roundSpan, "")
			turnImages := 0
			for _, et := range r.executedTools {
				if et.Name == ToolGenerateImage && et.Status == AgentToolStatusSuccess {
					turnImages++
				}
			}
			// SP-23 M4 pieces 3/4: conversation-level budgets gate every
			// tool call; the degradation result stops the loop from
			// burning more rounds against a capped session.
			if exceeded := r.svc.SessionBudgetExceeded(r.persistedToolCalls, r.persistedToolTurns, len(r.executedTools), 1); exceeded != "" {
				execution := AgentToolExecution{Name: tc.Function.Name, Status: AgentToolStatusError, ArgsSummary: "[session-budget]"}
				r.executedTools = append(r.executedTools, execution)
				_ = r.handler(AgentStreamEvent{Type: AgentEventToolStatus, Tool: &execution})
				toolMessages = append(toolMessages, llm.ChatMessage{Role: "tool", ToolCallID: tc.ID, Content: `{"ok":false,"error":"session_budget_exceeded","detail":` + strconv.Quote(exceeded) + `}`})
				continue
			}
			outcome, toolErr := r.svc.toolRuntimeOrFallback().ExecuteTool(ctx, tc.Function.Name, json.RawMessage(tc.Function.Arguments), ToolScope{
				ViewerID:       r.userID,
				ConversationID: convIDForTools(r.conv),
				TurnImages:     turnImages,
			})
			execution := AgentToolExecution{
				Name:        tc.Function.Name,
				ArgsSummary: agentToolArgsSummary(tc.Function.Name, json.RawMessage(tc.Function.Arguments)),
				Status:      AgentToolStatusSuccess,
				DurationMs:  time.Since(toolStartedAt).Milliseconds(),
			}
			// #610 部署冒烟热修（2026-09-20 实机）：external 归属跟工具名而非
			// outcome——generate_image 首调失败（invalid args 等，outcome=nil →
			// External=false）会毒化 allToolsExternal，重试成功的生图轮转述被
			// 引用门清空（用户已付费的图不可见）。外部工具的失败步同样计入
			// 外部车道。
			execution.External = tc.Function.Name == ToolGenerateImage ||
				strings.HasPrefix(tc.Function.Name, agentmcp.ToolNamePrefix)
			result := agentToolResult{OK: true}
			if toolErr != nil {
				execution.Status = AgentToolStatusError
				if errors.Is(toolErr, ErrAgentToolUnknown) {
					execution.Name = "(unknown)"
					result = agentToolResult{OK: false, Error: "tool_error"}
				} else if errors.Is(toolErr, ErrAgentToolInvalidArgs) {
					result = agentToolResult{OK: false, Error: "invalid_args"}
				} else if errors.Is(toolErr, ErrContentNotFound) {
					result = agentToolResult{OK: false, Error: "content_not_found"}
				} else if errors.Is(toolErr, ErrAgentImageQuotaExceeded) {
					// M1 quota contract (live regression 2026-09-18: with the
					// generic tool_error the model retried 5x and the strict
					// citation gate then cleared its relay to an empty bubble):
					// relay a stable code + hint so the model tells the user the
					// cap is reached instead of retrying (external 归属已由上方
					// 工具名判定统一覆盖).
					result = agentToolResult{OK: false, Error: "image_quota_exceeded",
						Message: "本会话配图额度已用完。请直接告知用户额度已满、勿再调用 generate_image。"}
				} else {
					result = agentToolResult{OK: false, Error: "tool_error"}
				}
				traceAgentEvent(r.traceID, "tool_error", "tool", tc.Function.Name, "safe_error", result.Error)
			} else if outcome != nil {
				execution.Hits = agentToolHitCount(outcome)
				// SP-23 M5: external tools badge + their specifics land in
				// the trace node extra (cost for images, server/tool and
				// truncation for MCP). External 归属已在上方按工具名统一判定。
				result.Detail = outcome.Detail
				result.Guide = outcome.Guide
				result.Search = outcome.Search
				result.IPs = outcome.IPs
				result.Suggest = outcome.Suggest
				result.Image = outcome.Image
				result.MCP = outcome.MCP
				// SP-23 M4 piece 1: external payloads carry the
				// not-instructions fence; piece 2 prereq: remember this
				// conversation's own signed URL prefixes.
				if outcome.MCP != nil {
					result.MCP.Result = FenceExternalResult(outcome.MCP.Result, r.svc.cfg.Agent.Guardrails.FenceExternalToolResults)
				}
				if outcome.Image != nil && outcome.Image.URL != "" {
					r.ownImagePrefixes = append(r.ownImagePrefixes, urlPrefixOf(outcome.Image.URL))
				}
				for chunkKey, source := range outcome.RetrievalSources {
					r.retrievalSources[chunkKey] = source
				}
				if outcome.Degraded {
					r.degraded = true
					traceAgentEvent(r.traceID, "retrieval_r.degraded", "tool", tc.Function.Name)
				}
				for _, summary := range outcome.Search {
					citation, ok := citationFromSearchSummary(summary)
					if !ok || r.seenCitationKeys[citation.ChunkKey] {
						continue
					}
					r.seenCitationKeys[citation.ChunkKey] = true
					r.citationCandidates = append(r.citationCandidates, citation)
				}
				// SP-19 G2-1: search_ips results join the same citation
				// candidate pool; the "ip:{id}" dedupe key cannot collide
				// with 64-hex chunk keys.
				for _, ipSummary := range outcome.IPs {
					citation, ok := citationFromIPSummary(ipSummary)
					if !ok {
						continue
					}
					key := fmt.Sprintf("ip:%d", citation.ContentID)
					if r.seenCitationKeys[key] {
						continue
					}
					r.seenCitationKeys[key] = true
					r.citationCandidates = append(r.citationCandidates, citation)
				}
				if outcome.Detail != nil {
					if !r.svc.ragHybridEnabled() {
						legacyKey := fmt.Sprintf("content:%d", outcome.Detail.ID)
						if !r.seenCitationKeys[legacyKey] {
							r.seenCitationKeys[legacyKey] = true
							r.citationCandidates = append(r.citationCandidates, AgentCitation{
								ContentID: outcome.Detail.ID,
								Title:     outcome.Detail.Title,
								Zone:      outcome.Detail.Zone,
								Excerpt:   outcome.Detail.Excerpt,
							})
						}
					} else {
						citation, err := r.svc.citationForContent(ctx, r.userID, outcome.Detail.ID)
						if err != nil {
							traceAgentEvent(r.traceID, "citation_revalidation", "accepted", false, "reason", "citation_truth_unavailable")
						} else if source, ok := r.retrievalSources[citation.ChunkKey]; !ok {
							traceAgentEvent(r.traceID, "citation_revalidation", "accepted", false, "reason", "citation_source_unavailable")
						} else if !r.seenCitationKeys[citation.ChunkKey] {
							citation.Source = source
							r.seenCitationKeys[citation.ChunkKey] = true
							r.citationCandidates = append(r.citationCandidates, citation)
						}
					}
				}
			}
			// A-03: expansion terms surface in the tool step summary so the
			// process panel can show what the retrieval fanned out to. The
			// outcome is nil when the tool itself failed. #619: a truncated
			// over-length query is marked so "why did it find nothing"
			// investigations can see the cut.
			if outcome != nil && outcome.QueryTruncated {
				execution.ArgsSummary += " [query-truncated]"
			}
			if outcome != nil && len(outcome.ExpandedQueries) > 0 {
				execution.ArgsSummary += " +expanded: " + strings.Join(outcome.ExpandedQueries, " / ")
			}
			toolSpan.End(agenttrace.NodeEndOptions{
				NodeName:     tc.Function.Name,
				Status:       toolNodeStatus(toolErr),
				ErrorCode:    result.Error,
				PromptDigest: execution.ArgsSummary,
				Extra:        model.JSONB(agentToolExtraJSON(execution, outcome)),
			})
			r.executedTools = append(r.executedTools, execution)
			if err := r.handler(AgentStreamEvent{Type: AgentEventToolStatus, Tool: &execution}); err != nil {
				// Handler（SSE 写端）失败 = 连接已断：按 #661 前语义保留部分
				// 落库并直接终止（终局四件套由 finalizeTurn 的 handlerAbort
				// outcome 收口）。
				r.svc.persistPartialTurn(r.conv.ID, r.answerBuf.String(), r.ownImagePrefixes, 0, r.answerLang)
				r.streamErr = err
				r.skipFinalize = true
				return
			}
			resultJSON, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				// 服务端自建结构 marshal 失败属内部中止：按 #661 前语义直接
				// 终止（无终局四件套），finalizeTurn 的 skipFinalize 出口收口。
				r.streamErr = fmt.Errorf("marshal agent tool result: %w", marshalErr)
				r.skipFinalize = true
				return
			}
			toolMessages = append(toolMessages, llm.ChatMessage{Role: "tool", ToolCallID: tc.ID, Content: string(resultJSON)})
		}
		req.Messages = append(req.Messages, toolMessages...)
	}
}
