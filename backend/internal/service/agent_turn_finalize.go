package service

// finalizeTurn is the single terminal collection point for every answer-turn
// ending (#661): persistence, the terminal trace run row, the SLA metric and
// the terminal stream event — the quartet — fire here and nowhere else, for
// all three outcomes (success / stream error / chitchat shortcut). The
// pre-#661 shape wrote the quartet by hand on each path and sprinkled four
// partial-persist scatter points; missing any piece was silent drift (the
// same disease family the composition-root closure batch rooted out on the
// wiring side). Adding a new outcome now means adding it to this switch —
// the quartet cannot be half-emmitted.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability/agenttrace"
)

// turnOutcome drives finalizeTurn.
type turnOutcome int

const (
	// outcomeSuccess finalizes a completed answer turn (any answer kind).
	outcomeSuccess turnOutcome = iota
	// outcomeStreamError finalizes a failed turn: provider error, timeout,
	// client cancellation, SSE handler failure or an internal loop abort.
	outcomeStreamError
	// outcomeShortcut finalizes the chitchat rule-layer template turn.
	outcomeShortcut
)

// turnFinalizeInput is the outcome payload. Done assembly (final answer,
// citations, follow-ups, usage, answer kind) stays on the success path —
// only the assembled result travels into the finalizer.
type turnFinalizeInput struct {
	outcome   turnOutcome
	answer    string          // success
	think     string          // success: display-only reasoning phase row
	kind      AgentAnswerKind // success
	citations []AgentCitation // success
	followUps []string        // success
	usage     *AgentUsage     // success
	template  string          // shortcut
}

// finalizeTurn runs the terminal quartet for the given outcome and returns
// the turn's terminal error (nil for success, the stream error otherwise).
func (r *turnRunner) finalizeTurn(ctx context.Context, in turnFinalizeInput) error {
	switch in.outcome {
	case outcomeShortcut:
		return r.finalizeShortcut(ctx, in)
	case outcomeStreamError:
		return r.finalizeStreamError()
	default:
		return r.finalizeSuccess(ctx, in)
	}
}

// finalizeStreamError closes a failed turn: error event → partial persist →
// terminal trace row → SLA metric, then the (possibly joined) stream error.
func (r *turnRunner) finalizeStreamError() error {
	err := r.streamErr
	traceAgentEvent(r.traceID, "chat_failed", "safe_error", safeAgentStreamCode(err))
	var code AgentStreamEventType = AgentErrorCodeProvider
	switch {
	case errors.Is(err, context.Canceled):
		code = AgentErrorCodeCancelled
	case errors.Is(err, context.DeadlineExceeded):
		code = AgentErrorCodeProviderTimeout
	}
	providerFallback := code != AgentErrorCodeCancelled
	degradedReason := ""
	if providerFallback {
		degradedReason = "provider_error"
	}
	if emitErr := r.handler(AgentStreamEvent{
		Type:           AgentEventError,
		Degraded:       providerFallback,
		DegradedReason: degradedReason,
		ErrorCode:      string(code),
		ErrorMessage:   safeAgentStreamMessage(code),
	}); emitErr != nil {
		err = errors.Join(err, emitErr)
	}
	// A-01: stop/failure keeps the conversation and whatever partial content
	// already streamed out; deletion happens only on the user's explicit
	// DELETE. The request context is commonly canceled on client disconnect,
	// so persistence runs on a detached bounded context.
	r.svc.persistPartialTurn(r.conv.ID, r.answerBuf.String(), r.ownImagePrefixes, 0, r.answerLang)
	r.recorder.RecordRunEnd(agenttrace.RunEnd{
		Status:            runTerminalStatus(err),
		ErrorCode:         safeAgentStreamCode(err),
		StartedAt:         r.started,
		FirstDisplayDelta: r.firstDisplayDelta,
		Model:             r.svc.servingModel(r.recorder, r.turn.Model),
	})
	// SP-24 R7: run-level SLA metrics fire for every terminal turn,
	// independent of the recorder's sampling gate.
	recordAgentRunMetrics(runTerminalStatus(err), "", r.firstDisplayDelta, r.started)
	return err
}

// finalizeSuccess closes a completed turn: think/tools/answer rows + citation
// payload + conversation timestamp (+ auto title, output moderation), then
// the terminal trace row, SLA metric and the done event. Storage failures
// degrade to the storage error event exactly as pre-#661.
func (r *turnRunner) finalizeSuccess(ctx context.Context, in turnFinalizeInput) error {
	answerMessageID := int64(0)
	if r.svc.db != nil {
		// The provider already finished; the client may disconnect at any
		// moment, so the final answer persists on a detached bounded context.
		storeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if think := in.think; think != "" {
			// A-02: the reasoning block persists as its own phase-marked row
			// (tool_calls = {"phase":"think"}) ahead of the answer row, for
			// history replay and audit; readers treat it as display-only.
			if err := r.svc.db.WithContext(storeCtx).Create(&model.AgentMessage{
				ConversationID: r.conv.ID,
				Role:           "assistant",
				Content:        &think,
				ToolCalls:      model.JSONMap{"phase": "think"},
				CreatedAt:      time.Now(),
			}).Error; err != nil {
				cancel()
				slog.Error("failed to persist agent thinking message", "error", err)
				return emitAgentStreamError(r.handler, AgentErrorCodeStorage, err)
			}
		}
		if len(r.executedTools) > 0 {
			// #538: the turn's tool-step summary persists as its own phase row
			// (tool_calls = {"phase":"tools","steps":[...]}) between the think
			// row and the answer row, so history replay shows the same tool
			// steps the live stream emitted. Steps carry only the
			// server-derived summary shape — raw tool arguments never reach
			// storage.
			if err := r.svc.db.WithContext(storeCtx).Create(&model.AgentMessage{
				ConversationID: r.conv.ID,
				Role:           "assistant",
				ToolCalls:      model.JSONMap{"phase": "tools", "steps": r.executedTools},
				CreatedAt:      time.Now(),
			}).Error; err != nil {
				cancel()
				slog.Error("failed to persist agent tool steps message", "error", err)
				return emitAgentStreamError(r.handler, AgentErrorCodeStorage, err)
			}
		}
		answerRow := model.AgentMessage{
			ConversationID: r.conv.ID,
			Role:           "assistant",
			Content:        &in.answer,
			// N4：引用随答案行落库（完整 9 字段形态，含 RAG 溯源），历史端点
			// 直接回放跳转入口；think 行不落引用。
			Citations: citationsToModel(in.citations),
			CreatedAt: time.Now(),
		}
		if err := r.svc.db.WithContext(storeCtx).Create(&answerRow).Error; err != nil {
			cancel()
			slog.Error("failed to persist agent assistant message", "error", err)
			return emitAgentStreamError(r.handler, AgentErrorCodeStorage, err)
		}
		answerMessageID = answerRow.ID
		if err := r.svc.db.WithContext(storeCtx).Model(r.conv).Update("updated_at", time.Now()).Error; err != nil {
			cancel()
			slog.Error("failed to update agent conversation timestamp", "error", err)
			return emitAgentStreamError(r.handler, AgentErrorCodeStorage, err)
		}
		cancel()
		if !r.hadAssistantBefore {
			r.svc.scheduleAutoTitle(r.traceID, r.recorder, r.conv.ID, r.firstUserMsg)
		}
		// A-05: the persisted answer is audited asynchronously after the
		// turn; a flagged row is redacted by the history endpoint.
		r.svc.scheduleOutputModeration(r.traceID, answerMessageID, in.answer)
	}

	// SP-21 T2: terminal run row — TTFT measured to the first forwarded
	// display delta, model attribution honors routing events, and the
	// message id links the trace back into the conversation history.
	msgID := answerMessageID
	r.recorder.RecordRunEnd(agenttrace.RunEnd{
		Status:            model.AgentTraceStatusSuccess,
		StartedAt:         r.started,
		FirstDisplayDelta: r.firstDisplayDelta,
		AnswerKind:        string(in.kind),
		Model:             r.svc.servingModel(r.recorder, r.turn.Model),
		MessageID:         &msgID,
	})
	recordAgentRunMetrics(model.AgentTraceStatusSuccess, string(in.kind), r.firstDisplayDelta, r.started)

	if err := r.handler(AgentStreamEvent{
		Type:           AgentEventDone,
		TraceID:        r.traceID,
		ConversationID: r.conv.ID,
		MessageID:      answerMessageID,
		AnswerKind:     in.kind,
		Answer:         in.answer,
		Citations:      in.citations,
		Tools:          r.executedTools,
		Usage:          in.usage,
		Degraded:       r.degraded,
		FollowUps:      in.followUps,
	}); err != nil {
		return err
	}

	traceAgentEvent(r.traceID, "chat_done", "conversation_id", r.conv.ID, "surface", r.surface, "answer_kind", in.kind, "tools", len(r.executedTools))
	return nil
}

// finalizeShortcut closes the chitchat rule-layer turn: template row +
// conversation timestamp, terminal trace row (conversational), SLA metric
// and the done event.
func (r *turnRunner) finalizeShortcut(ctx context.Context, in turnFinalizeInput) error {
	messageID := int64(0)
	if r.svc.db != nil {
		storeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		answerRow := model.AgentMessage{
			ConversationID: r.conv.ID,
			Role:           "assistant",
			Content:        &in.template,
			CreatedAt:      time.Now(),
		}
		if err := r.svc.db.WithContext(storeCtx).Create(&answerRow).Error; err != nil {
			cancel()
			slog.Error("failed to persist agent chitchat template message", "error", err)
			return emitAgentStreamError(r.handler, AgentErrorCodeStorage, err)
		}
		messageID = answerRow.ID
		if err := r.svc.db.WithContext(storeCtx).Model(r.conv).Update("updated_at", time.Now()).Error; err != nil {
			cancel()
			slog.Error("failed to update agent conversation timestamp", "error", err)
			return emitAgentStreamError(r.handler, AgentErrorCodeStorage, err)
		}
		cancel()
	}
	r.recorder.RecordRunEnd(agenttrace.RunEnd{
		Status:            model.AgentTraceStatusSuccess,
		StartedAt:         r.started,
		FirstDisplayDelta: r.firstDisplayDelta,
		AnswerKind:        string(AgentAnswerConversational),
		Model:             r.svc.servingModel(r.recorder, r.turn.Model),
		MessageID:         &messageID,
	})
	recordAgentRunMetrics(model.AgentTraceStatusSuccess, string(AgentAnswerConversational), r.firstDisplayDelta, r.started)
	if err := r.handler(AgentStreamEvent{
		Type:           AgentEventDone,
		TraceID:        r.traceID,
		ConversationID: r.conv.ID,
		MessageID:      messageID,
		AnswerKind:     AgentAnswerConversational,
		Answer:         in.template,
		Usage:          &AgentUsage{},
	}); err != nil {
		return err
	}
	traceAgentEvent(r.traceID, "chat_done", "conversation_id", r.conv.ID, "answer_kind", AgentAnswerConversational, "shortcut", "chitchat")
	return nil
}

var _ = fmt.Sprintf // placeholder to keep fmt if future edits need it; remove when unused
