package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// A-02: the provider's content deltas are forwarded as they arrive — one
// delta SSE event per provider chunk, in order — instead of one bulk
// emission after revalidation. The done event still carries the final
// revalidated verdict.
func TestAgentStreamV2ForwardsContentDeltasLive(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{{
		{Content: "第一段"},
		{Content: "第二段"},
		{Content: "第三段"},
		{Done: true},
	}}}
	svc, _ := newStreamTestService(t, provider, continuationTestConfig(100000))

	var deltas []string
	var events []AgentStreamEvent
	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "hi"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		events = append(events, ev)
		if ev.Type == AgentEventDelta {
			deltas = append(deltas, ev.Delta)
		}
		return nil
	})
	require.NoError(t, err)

	require.Equal(t, []string{"第一段", "第二段", "第三段"}, deltas, "each provider chunk must become its own delta event in order")
	var done *AgentStreamEvent
	for i := range events {
		if events[i].Type == AgentEventDone {
			done = &events[i]
		}
	}
	require.NotNil(t, done, "stream must end with done")
}

// A-02: provider reasoning (think) deltas stream as think_delta events and
// stay display-only — they never enter the answer, the persisted answer row,
// or the citation pipeline.
func TestAgentStreamV2ForwardsThinkDeltasSeparately(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id":88}`)},
		{
			{Thinking: "让我想想"},
			{Thinking: "如何回答"},
			{Content: "Grounded answer about Published Test Content"},
			{Done: true},
		},
	}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	var thinkText, answerText string
	var events []AgentStreamEvent
	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "what is this"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		events = append(events, ev)
		switch ev.Type {
		case AgentEventThinkDelta:
			thinkText += ev.Delta
		case AgentEventDelta:
			answerText += ev.Delta
		}
		return nil
	})
	require.NoError(t, err)

	require.Equal(t, "让我想想如何回答", thinkText, "think deltas must stream as think_delta events")
	require.Equal(t, "Grounded answer about Published Test Content", answerText)
	require.NotContains(t, answerText, "让我", "think text must never leak into the answer channel")

	// Persisted rows: think phase row + answer row, think never merged in.
	var rows []model.AgentMessage
	require.NoError(t, db.Where("role = ?", "assistant").Order("id ASC").Find(&rows).Error)
	require.GreaterOrEqual(t, len(rows), 1, "the answer must persist")
	var thinkRows, answerRows []model.AgentMessage
	for _, row := range rows {
		if row.ToolCalls != nil && row.ToolCalls["phase"] == "think" {
			thinkRows = append(thinkRows, row)
		} else {
			answerRows = append(answerRows, row)
		}
	}
	require.Len(t, thinkRows, 1, "non-empty think must persist as its own phase-marked row")
	require.NotNil(t, thinkRows[0].Content)
	require.Contains(t, *thinkRows[0].Content, "让我想想")
	require.Len(t, answerRows, 1)
	require.Equal(t, "Grounded answer about Published Test Content", *answerRows[0].Content)

	// The done verdict keeps think out of the final answer contract.
	var done *AgentStreamEvent
	for i := range events {
		if events[i].Type == AgentEventDone {
			done = &events[i]
		}
	}
	require.NotNil(t, done)
	require.NotContains(t, done.Answer, "让我")
}

// A-02: tool step events carry the upgraded process fields — argument
// summary, hit count and duration — for the workspace process panel.
func TestAgentStreamV2ToolStepCarriesSummaryHitsDuration(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id":88}`)},
		{{Content: "final"}, {Done: true}},
	}}
	svc, _ := newStreamTestService(t, provider, continuationTestConfig(100000))

	var tool *AgentToolExecution
	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "find"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		if ev.Type == AgentEventToolStatus && ev.Tool != nil {
			tool = ev.Tool
		}
		return nil
	})
	require.NoError(t, err)

	require.NotNil(t, tool, "tool_status event missing")
	require.Equal(t, AgentToolStatusSuccess, tool.Status)
	require.NotEmpty(t, tool.ArgsSummary, "tool step must carry an argument summary")
	require.Contains(t, tool.ArgsSummary, "88", "summary should surface the requested id")
	require.Equal(t, 1, tool.Hits, "detail hit count = 1 for the found content")
	require.GreaterOrEqual(t, tool.DurationMs, int64(0), "tool step must carry a duration")
}

// A-02: done carries message_id of the persisted assistant answer.
func TestAgentStreamV2DoneCarriesMessageID(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{{
		{Content: "direct answer"},
		{Done: true},
	}}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	var done *AgentStreamEvent
	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "hi"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		if ev.Type == AgentEventDone {
			done = &ev
		}
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, done)
	require.NotZero(t, done.MessageID, "done must carry the persisted assistant message id")

	var answerRow model.AgentMessage
	require.NoError(t, db.First(&answerRow, done.MessageID).Error)
	require.Equal(t, "assistant", answerRow.Role)
	require.NotEqual(t, "think", answerRow.ToolCalls["phase"], "the answer row must not carry the think phase marker")
}

// A-02 semantic change (truly streaming replaces pseudo-streaming): on a
// no-evidence turn the model text may already have streamed live; the done
// event remains the final verdict (answer_kind=no_evidence, empty final
// answer, empty persisted answer) so the client replaces the streamed text.
func TestAgentStreamV2NoEvidenceStreamsButDoneStaysEmpty(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id": 101}`)},
		{{Content: "hallucinated answer"}, {Done: true}},
	}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	var done *AgentStreamEvent
	streamedDeltas := 0
	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "find it"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		switch ev.Type {
		case AgentEventDelta:
			streamedDeltas++
		case AgentEventDone:
			done = &ev
		}
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, done)
	require.Equal(t, AgentAnswerNoEvidence, done.AnswerKind)
	require.Empty(t, done.Answer, "final verdict answer stays empty on no_evidence")
	require.GreaterOrEqual(t, streamedDeltas, 1, "live deltas are allowed to stream before the verdict")

	var persisted model.AgentMessage
	require.NoError(t, db.First(&persisted, done.MessageID).Error)
	require.Empty(t, *persisted.Content, "persisted answer stays empty on no_evidence")
}
