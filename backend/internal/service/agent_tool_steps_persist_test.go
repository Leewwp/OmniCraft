package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// #538: a grounded turn persists its tool-step summary as a phase="tools" row
// between the think row and the answer row, replaying the same steps the live
// stream emitted. A turn without tools persists no tools row.
func TestAgentStreamPersistsToolStepsRow(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{
			{Thinking: "先想一下"},
			toolCallDelta("get_content_detail", `{"content_id":88}`),
		},
		{
			{Content: "Grounded answer about Published Test Content"},
			{Done: true},
		},
	}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "看看 88 号内容"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		return nil
	})
	require.NoError(t, err)

	var rows []model.AgentMessage
	require.NoError(t, db.Where("conversation_id > 0").Order("id ASC").Find(&rows).Error)

	phases := make([]string, 0, 4)
	for _, row := range rows {
		phase := ""
		if row.ToolCalls != nil {
			phase, _ = row.ToolCalls["phase"].(string)
		}
		phases = append(phases, row.Role+":"+phase)
	}
	// Row order mirrors the streamed order: user, think, tools, answer.
	require.Equal(t, []string{"user:", "assistant:think", "assistant:tools", "assistant:"}, phases)

	var toolsRow *model.AgentMessage
	for i := range rows {
		if row := rows[i]; row.ToolCalls != nil {
			if phase, _ := row.ToolCalls["phase"].(string); phase == "tools" {
				toolsRow = &rows[i]
			}
		}
	}
	require.NotNil(t, toolsRow, "turn with executed tools must persist a tools row")
	require.Nil(t, toolsRow.Content, "tools row is summary-only; it carries no content")

	// JSONMap reads the JSONB back as []any of maps — decode exactly like the
	// history projection does, asserting the persisted wire shape.
	b, err := json.Marshal(toolsRow.ToolCalls["steps"])
	require.NoError(t, err)
	var steps []AgentToolExecution
	require.NoError(t, json.Unmarshal(b, &steps))
	require.Len(t, steps, 1)
	require.Equal(t, "get_content_detail", steps[0].Name)
	require.Equal(t, AgentToolStatusSuccess, steps[0].Status)
	require.Equal(t, "content_id=88", steps[0].ArgsSummary)
}

func TestAgentStreamNoToolsRowWithoutToolCalls(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{{Content: "直接回答"}, {Done: true}},
	}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "hi"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		return nil
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&model.AgentMessage{}).Where("tool_calls ->> 'phase' = 'tools'").Count(&count).Error)
	require.Zero(t, count, "a turn without tool calls must not persist a tools row")
}

// #538: the persisted tools row is replay-only. A follow-up turn's provider
// request must not contain the empty assistant message of the tools row.
func TestAgentStreamToolStepsRowExcludedFromFollowUpContext(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{
			toolCallDelta("get_content_detail", `{"content_id":88}`),
		},
		{
			{Content: "Grounded answer about Published Test Content"},
			{Done: true},
		},
	}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))

	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "看看 88"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		return nil
	})
	require.NoError(t, err)

	var conv model.AgentConversation
	require.NoError(t, db.First(&conv).Error)

	provider.calls = 0
	provider.rounds = [][]llm.ChatDelta{{{Content: "第二段回答"}, {Done: true}}}
	err = svc.ChatStream(context.Background(), 7, ChatTurnInput{ConversationID: conv.ID, Message: "还有吗"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		return nil
	})
	require.NoError(t, err)

	require.Equal(t, 1, provider.calls, "follow-up turn runs one provider round")
	for _, msg := range provider.lastReq.Messages {
		if msg.Role != "assistant" {
			continue
		}
		require.NotEmpty(t, msg.Content, "no empty assistant message (tools row) may reach the provider context")
	}
}

// #539: the deep-think toggle maps onto the provider request — default off
// pins thinking disabled, on leaves the provider default (no field).
func TestAgentStreamDeepThinkToggleControlsProviderThinking(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{{{Content: "ok"}, {Done: true}}}}
	svc, _ := newStreamTestService(t, provider, continuationTestConfig(100000))

	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "hi"}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, llm.ThinkingDisabled, provider.lastReq.Thinking, "default (toggle off) must pin thinking disabled")

	provider.calls = 0
	err = svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "hi", DeepThink: true}, resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error {
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, provider.lastReq.Thinking, "toggle on keeps the provider default (adaptive)")
}

// assembleChatContext skips phase="tools" replay rows but keeps every other
// row (think rows included, matching existing behavior).
func TestAssembleChatContextSkipsToolsPhaseRows(t *testing.T) {
	think := "思考内容"
	user := "问题"
	answer := "回答"
	history := []model.AgentMessage{
		{Role: "user", Content: &user},
		{Role: "assistant", Content: &think, ToolCalls: model.JSONMap{"phase": "think"}},
		{Role: "assistant", ToolCalls: model.JSONMap{"phase": "tools", "steps": []any{map[string]any{"name": "search_content"}}}},
		{Role: "assistant", Content: &answer},
	}
	out := assembleChatContext(llm.ChatMessage{Role: "system", Content: "s"}, history, 100000, 10)

	require.Len(t, out, 4, "system + user + think + answer; the tools row is skipped")
	for _, msg := range out {
		if msg.Role == "assistant" {
			require.NotEmpty(t, msg.Content)
		}
	}
}
