package service

// #661 finalizeTurn 单元族：三种 outcome（成功/错误/规则短路）各验终局
// 四件套齐发——落库（sqlite 内存行断言）、trace 终行（假 agenttrace Store
// 捕获 RunEnd）、SLA metrics（与 RecordRunEnd 同址由守门测试钉住）、终局
// 事件（handler 收集断言）。

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"omnicraft/backend/config"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability/agenttrace"
)

// fakeTraceStore captures flushed trace runs for quartet assertions.
type fakeTraceStore struct {
	runs []model.AgentTraceRun
}

func (s *fakeTraceStore) UpsertRuns(_ context.Context, runs []model.AgentTraceRun) error {
	s.runs = append(s.runs, runs...)
	return nil
}
func (s *fakeTraceStore) UpsertNodes(_ context.Context, _ []model.AgentTraceNode) error { return nil }
func (s *fakeTraceStore) PurgeBefore(_ context.Context, _ time.Time) (int64, error)     { return 0, nil }

func newFinalizeTestStack(t *testing.T) (*AgentService, *gorm.DB, *fakeTraceStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}, &model.User{}, &model.ContentItem{}, &model.ContentVersion{}))
	require.NoError(t, db.Create(&model.User{ID: 1, Username: "quartet", Email: "q@example.com"}).Error)

	svc := NewAgentService(nil, nil, nil, nil, db, &config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, MaxToolCallsPerTurn: 8, CitationMaxCount: 5}})
	store := &fakeTraceStore{}
	writer := agenttrace.NewWriter(store, agenttrace.Options{
		Enabled: true, SampleRatio: 1, ChannelSize: 64,
		FlushInterval: time.Millisecond, FlushBatchSize: 16,
	})
	writer.Start(context.Background())
	t.Cleanup(func() { writer.Stop(context.Background()) })
	svc.traceWriter = writer
	return svc, db, store
}

func newQuartetRunner(svc *AgentService, conv *model.AgentConversation, handler func(AgentStreamEvent) error) *turnRunner {
	recorder := agenttrace.NewTurnRecorder(svc.traceWriter, "quartet-trace-1")
	return svc.newTurnRunner(1, ChatTurnInput{Message: "quartet"}, conv, "quartet-trace-1", time.Now(), "global", false, "first message", recorder, handler)
}

func (s *fakeTraceStore) awaitRunEnd(t *testing.T) model.AgentTraceRun {
	t.Helper()
	require.Eventually(t, func() bool { return len(s.runs) > 0 }, 2*time.Second, 5*time.Millisecond)
	return s.runs[len(s.runs)-1]
}

func TestFinalizeSuccessEmitsFullQuartet(t *testing.T) {
	svc, db, store := newFinalizeTestStack(t)
	conv := &model.AgentConversation{ID: 1, UserID: 1, ContextType: "general"}
	require.NoError(t, db.Create(conv).Error)

	var events []AgentStreamEvent
	r := newQuartetRunner(svc, conv, func(ev AgentStreamEvent) error { events = append(events, ev); return nil })
	r.thinkingBuf.WriteString("思考片段")
	r.executedTools = append(r.executedTools, AgentToolExecution{Name: "search_content", Status: AgentToolStatusSuccess})
	r.answerBuf.WriteString("draft")

	err := r.finalizeTurn(context.Background(), turnFinalizeInput{
		outcome:   outcomeSuccess,
		answer:    "终稿",
		think:     r.thinkingBuf.String(),
		kind:      AgentAnswerGroundedContent,
		citations: []AgentCitation{{ContentID: 88, Title: "t", Zone: "fanwork"}},
		usage:     &AgentUsage{PromptTokens: 3, CompletionTokens: 5},
	})
	require.NoError(t, err)

	// ① 落库：think 行 + tools 行 + 答案行（带引用）+ 会话时间戳更新。
	var rows []model.AgentMessage
	require.NoError(t, db.Where("conversation_id = ?", conv.ID).Order("id").Find(&rows).Error)
	require.Len(t, rows, 3)
	require.Equal(t, model.JSONMap{"phase": "think"}, rows[0].ToolCalls)
	require.Equal(t, "tools", rows[1].ToolCalls["phase"])
	require.NotNil(t, rows[2].Content)
	require.Equal(t, "终稿", *rows[2].Content)
	require.Len(t, rows[2].Citations, 1)

	// ② 终局事件：done 携带终稿/引用/kind/usage/工具步骤。
	done := events[len(events)-1]
	require.Equal(t, AgentEventDone, done.Type)
	require.Equal(t, AgentAnswerGroundedContent, done.AnswerKind)
	require.Equal(t, "终稿", done.Answer)
	require.Len(t, done.Citations, 1)
	require.Len(t, done.Tools, 1)
	require.Equal(t, rows[2].ID, done.MessageID)

	// ③ trace 终行：success + kind + message 关联（④metrics 与终行同址，
	// 由守门测试钉住）。
	run := store.awaitRunEnd(t)
	require.Equal(t, model.AgentTraceStatusSuccess, run.Status)
	require.Equal(t, string(AgentAnswerGroundedContent), run.AnswerKind)
	require.NotNil(t, run.MessageID)
	require.Equal(t, rows[2].ID, *run.MessageID)
}

func TestFinalizeStreamErrorEmitsQuartetWithPartialPersist(t *testing.T) {
	svc, db, store := newFinalizeTestStack(t)
	conv := &model.AgentConversation{ID: 1, UserID: 1, ContextType: "general"}
	require.NoError(t, db.Create(conv).Error)

	var events []AgentStreamEvent
	r := newQuartetRunner(svc, conv, func(ev AgentStreamEvent) error { events = append(events, ev); return nil })
	r.answerBuf.WriteString("半截回答")
	r.streamErr = context.Canceled

	err := r.finalizeTurn(context.Background(), turnFinalizeInput{outcome: outcomeStreamError})
	require.ErrorIs(t, err, context.Canceled)

	// ① 落库：部分回答行保留（A-01 语义）。
	var rows []model.AgentMessage
	require.NoError(t, db.Where("conversation_id = ?", conv.ID).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Contains(t, *rows[0].Content, "半截回答")

	// ② 终局事件：error 携带取消码（无降级标记）。
	terminal := events[len(events)-1]
	require.Equal(t, AgentEventError, terminal.Type)
	require.Equal(t, AgentErrorCodeCancelled, terminal.ErrorCode)
	require.False(t, terminal.Degraded)

	// ③ trace 终行：失败状态 + 取消错误码。
	run := store.awaitRunEnd(t)
	require.Equal(t, model.AgentTraceStatusCanceled, run.Status)
	require.Equal(t, safeAgentStreamCode(context.Canceled), run.ErrorCode)
}

func TestFinalizeShortcutEmitsQuartet(t *testing.T) {
	svc, db, store := newFinalizeTestStack(t)
	conv := &model.AgentConversation{ID: 1, UserID: 1, ContextType: "general"}
	require.NoError(t, db.Create(conv).Error)

	var events []AgentStreamEvent
	r := newQuartetRunner(svc, conv, func(ev AgentStreamEvent) error { events = append(events, ev); return nil })
	recordShortcutRun(r.recorder, r.started, conv)

	require.NoError(t, r.finalizeTurn(context.Background(), turnFinalizeInput{outcome: outcomeShortcut, template: "模板回复"}))

	// ① 落库：模板行。
	var rows []model.AgentMessage
	require.NoError(t, db.Where("conversation_id = ?", conv.ID).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "模板回复", *rows[0].Content)

	// ② 终局事件：done = conversational 模板。
	done := events[len(events)-1]
	require.Equal(t, AgentEventDone, done.Type)
	require.Equal(t, AgentAnswerConversational, done.AnswerKind)
	require.Equal(t, "模板回复", done.Answer)
	require.Equal(t, rows[0].ID, done.MessageID)

	// ③ trace 终行：success + conversational。
	run := store.awaitRunEnd(t)
	require.Equal(t, model.AgentTraceStatusSuccess, run.Status)
	require.Equal(t, string(AgentAnswerConversational), run.AnswerKind)
}
